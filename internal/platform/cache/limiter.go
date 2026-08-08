package cache

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var rateLimitScript = redis.NewScript(`
	local key = KEYS[1]
	local limit = tonumber(ARGV[1])
	local window = tonumber(ARGV[2])

	local current = redis.call("INCR", key)
	if current == 1 then
		redis.call("EXPIRE", key, window)
	end

	local ttl = redis.call("TTL", key)
	if current > limit then
		return {0, 0, ttl}
	else
		return {1, limit - current, ttl}
	end
`)

// LimitResult contains rate limiting evaluation outputs.
type LimitResult struct {
	Allowed   bool
	Remaining int
	ResetIn   time.Duration
	// Degraded reports that the limit was not actually evaluated because the
	// backing store was unreachable, so callers should not advertise
	// RateLimit-* headers from this result.
	Degraded bool
}

// Limiter interface evaluates rate limit quotas.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (LimitResult, error)
}

// RedisLimiter implements fixed/sliding window rate limiting over Redis.
type RedisLimiter struct {
	client redis.Cmdable
}

// NewRedisLimiter creates a new rate limiter facade.
func NewRedisLimiter(client redis.Cmdable) *RedisLimiter {
	return &RedisLimiter{client: client}
}

// Allow checks whether an action for key is within its quota over window.
//
// Degradation is the point: when Redis is unreachable the limiter allows the
// request and warns. A limiter that denies when its backing store is down
// turns a cache outage into a total outage, and the rate limit protects far
// less than availability costs (API_GUIDELINE.md §11).
func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (LimitResult, error) {
	if l.client == nil {
		return l.degrade(ctx, limit, errors.New("redis client not configured")), nil
	}

	windowSeconds := int(window.Seconds())
	if windowSeconds <= 0 {
		windowSeconds = 1
	}

	res, err := rateLimitScript.Run(ctx, l.client, []string{key}, limit, windowSeconds).Result()
	if err != nil {
		return l.degrade(ctx, limit, err), nil
	}

	slice, ok := res.([]any)
	if !ok || len(slice) < 3 {
		// A malformed reply is a bug in this package, not an outage. Surface it
		// rather than silently allowing everything.
		return LimitResult{}, errors.New("rate limit script returned unexpected shape")
	}

	allowedInt, _ := slice[0].(int64)
	remainingInt, _ := slice[1].(int64)
	ttlInt, _ := slice[2].(int64)

	resetIn := time.Duration(ttlInt) * time.Second
	if ttlInt < 0 {
		// TTL -1 means the key has no expiry, -2 that it vanished between the
		// INCR and the TTL. Neither is a negative reset time.
		resetIn = window
	}

	return LimitResult{
		Allowed:   allowedInt == 1,
		Remaining: max(int(remainingInt), 0),
		ResetIn:   resetIn,
	}, nil
}

// degrade records the outage and lets the caller through.
func (l *RedisLimiter) degrade(ctx context.Context, limit int, cause error) LimitResult {
	slog.WarnContext(ctx, "rate limiter unavailable, allowing request", "module", "cache", "op", "limiter", "error", cause)
	if cacheUnavailableCounter != nil {
		cacheUnavailableCounter.Add(ctx, 1)
	}
	return LimitResult{Allowed: true, Remaining: limit, ResetIn: 0, Degraded: true}
}
