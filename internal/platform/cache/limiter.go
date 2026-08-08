package cache

import (
	"context"
	"errors"
	"fmt"
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

// Allow checks if an action for key is within quota limit over window.
func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (LimitResult, error) {
	if l.client == nil {
		// Fail open or closed depending on policy; default fail open with log
		return LimitResult{Allowed: true, Remaining: limit, ResetIn: 0}, nil
	}

	windowSeconds := int(window.Seconds())
	if windowSeconds <= 0 {
		windowSeconds = 1
	}

	res, err := rateLimitScript.Run(ctx, l.client, []string{key}, limit, windowSeconds).Result()
	if err != nil {
		return LimitResult{}, fmt.Errorf("rate limit script execution: %w", err)
	}

	slice, ok := res.([]any)
	if !ok || len(slice) < 3 {
		return LimitResult{}, errors.New("rate limit script returned unexpected shape")
	}

	allowedInt, _ := slice[0].(int64)
	remainingInt, _ := slice[1].(int64)
	ttlInt, _ := slice[2].(int64)

	return LimitResult{
		Allowed:   allowedInt == 1,
		Remaining: int(remainingInt),
		ResetIn:   time.Duration(ttlInt) * time.Second,
	}, nil
}
