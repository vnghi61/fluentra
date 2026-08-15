package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/fluentra/fluentra/internal/shared/id"
)

var (
	// ErrLockNotAcquired reports that another holder owns the lock. Unlike a cache
	// read, this is never degraded to success: fail-open here would hand the same
	// lock to two callers and destroy the mutual exclusion the lock exists for.
	ErrLockNotAcquired = errors.New("lock: failed to acquire lock")

	releaseScript = redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)
)

// Locker defines distributed lock capabilities.
type Locker interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (func(context.Context) error, error)
}

// RedisLocker implements distributed lock using SET NX PX with token validation.
type RedisLocker struct {
	client redis.Cmdable
}

// NewRedisLocker creates a distributed locker facade.
func NewRedisLocker(client redis.Cmdable) *RedisLocker {
	return &RedisLocker{client: client}
}

// Acquire attempts to acquire a lock on key for ttl. Returns a safe release function.
func (l *RedisLocker) Acquire(ctx context.Context, key string, ttl time.Duration) (func(context.Context) error, error) {
	if l.client == nil {
		return nil, errors.New("redis client not configured")
	}

	tokenID, err := id.NewULID(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate lock token: %w", err)
	}
	token := tokenID.String()

	ok, err := l.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	if !ok {
		return nil, ErrLockNotAcquired
	}

	release := func(relCtx context.Context) error {
		res, err := releaseScript.Run(relCtx, l.client, []string{key}, token).Result()
		if err != nil {
			return fmt.Errorf("release lock script: %w", err)
		}
		if resInt, ok := res.(int64); ok && resInt == 0 {
			// Lock was either expired or released by another holder
			return errors.New("lock: token mismatch or lock expired before release")
		}
		return nil
	}

	return release, nil
}
