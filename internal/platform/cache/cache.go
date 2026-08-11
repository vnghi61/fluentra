package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/sync/singleflight"
)

var (
	tracer                  = otel.Tracer("fluentra.platform.cache")
	meter                   = otel.Meter("fluentra.platform.cache")
	cacheUnavailableCounter metric.Int64Counter
)

func init() {
	var err error
	cacheUnavailableCounter, err = meter.Int64Counter(
		"cache_unavailable_total",
		metric.WithDescription("Total count of times Redis was unavailable and request degraded to loader"),
	)
	if err != nil {
		slog.Error("failed to create cache_unavailable_total metric", "error", err)
	}
}

// ErrMiss reports that a key is absent. It is distinct from a failure to reach
// the store, and callers need the difference: a miss is the ordinary case, a
// failure is an outage worth logging.
//
// It exists because this package is a facade. Returning the driver's own
// `redis.Nil` forced every caller to import `github.com/redis/go-redis` to
// recognise the commonest outcome — which business modules may not do
// (`depOnAnyVendor: false`), so they collapsed miss and outage into "treat any
// error as absent" and lost the distinction. A facade whose sentinel is the
// vendor's is not a facade.
var ErrMiss = errors.New("cache: key not found")

// Cache defines typed operations over Redis with singleflight stampede prevention and fallback.
type Cache[T any] interface {
	Get(ctx context.Context, key string) (T, error)
	Set(ctx context.Context, key string, value T, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	GetOrLoad(ctx context.Context, key string, ttl time.Duration, loader func(ctx context.Context) (T, error)) (T, error)
}

// RedisCache is the Redis-backed implementation of Cache[T].
type RedisCache[T any] struct {
	client redis.Cmdable
	sf     singleflight.Group
}

// NewRedisCache creates a new typed Redis cache instance.
func NewRedisCache[T any](client redis.Cmdable) *RedisCache[T] {
	return &RedisCache[T]{client: client}
}

// Get fetches a value from Redis and unmarshals it. Returns redis.Nil error if missing.
func (c *RedisCache[T]) Get(ctx context.Context, key string) (T, error) {
	var zero T
	if c.client == nil {
		return zero, redis.Nil
	}
	ctx, span := tracer.Start(ctx, "cache.Get")
	defer span.End()

	valBytes, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Wrapped, not replaced: a caller already matching on redis.Nil
			// keeps working while new callers match on ErrMiss.
			return zero, fmt.Errorf("%w: %w", ErrMiss, err)
		}
		c.recordUnavailable(ctx, "get", err)
		return zero, err
	}

	var target T
	if err := json.Unmarshal(valBytes, &target); err != nil {
		return zero, err
	}
	return target, nil
}

// Set serializes a value and stores it in Redis with jittered TTL.
func (c *RedisCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}
	ctx, span := tracer.Start(ctx, "cache.Set")
	defer span.End()

	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	jitteredTTL := addJitter(ttl, 0.10)
	if err := c.client.Set(ctx, key, bytes, jitteredTTL).Err(); err != nil {
		c.recordUnavailable(ctx, "set", err)
		return err
	}
	return nil
}

// Delete removes one or more keys from Redis.
func (c *RedisCache[T]) Delete(ctx context.Context, keys ...string) error {
	if c.client == nil || len(keys) == 0 {
		return nil
	}
	ctx, span := tracer.Start(ctx, "cache.Delete")
	defer span.End()

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		c.recordUnavailable(ctx, "delete", err)
		return err
	}
	return nil
}

// GetOrLoad attempts to read key from cache. On miss or Redis error, uses singleflight to call loader once.
func (c *RedisCache[T]) GetOrLoad(
	ctx context.Context, key string, ttl time.Duration, loader func(ctx context.Context) (T, error),
) (T, error) {
	var zero T

	// 1. Try cache hit
	if val, err := c.Get(ctx, key); err == nil {
		return val, nil
	}

	// 2. Use singleflight to prevent thundering herd / stampede
	val, err, _ := c.sf.Do(key, func() (any, error) {
		// Re-check cache inside singleflight callback in case another goroutine loaded it
		if val, err := c.Get(ctx, key); err == nil {
			return val, nil
		}

		// Fallback to loader
		loadedVal, err := loader(ctx)
		if err != nil {
			return zero, err
		}

		// Asynchronously save to cache; ignore errors so loader result is still returned
		storeCtx := context.WithoutCancel(ctx)
		go func() {
			setCtx, cancel := context.WithTimeout(storeCtx, 5*time.Second)
			defer cancel()
			_ = c.Set(setCtx, key, loadedVal, ttl)
		}()

		return loadedVal, nil
	})

	if err != nil {
		return zero, err
	}

	typedVal, ok := val.(T)
	if !ok {
		return zero, errors.New("singleflight value type assertion failed")
	}
	return typedVal, nil
}

// recordUnavailable reports the degradation without the cache key: a key
// embeds the entity id it caches, and LOGGING_GUIDELINE.md §4 asks for
// `module` and `op` here, not the identifier.
func (c *RedisCache[T]) recordUnavailable(ctx context.Context, op string, err error) {
	slog.WarnContext(ctx, "redis unavailable, degrading operation",
		"module", "cache",
		"op", op,
		"error", err,
	)
	if cacheUnavailableCounter != nil {
		cacheUnavailableCounter.Add(ctx, 1)
	}
}

// addJitter modifies duration by +/- pct (e.g. 0.10 for +/- 10%).
func addJitter(d time.Duration, pct float64) time.Duration {
	if d <= 0 || pct <= 0 {
		return d
	}
	delta := float64(d) * pct
	// TTL jitter spreads expiries; it is not a security decision, so the
	// cheap PRNG is the right choice here.
	jitter := (rand.Float64()*2 - 1) * delta //nolint:gosec // [-delta, +delta]
	return time.Duration(float64(d) + jitter)
}
