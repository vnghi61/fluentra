package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/platform/cache"
)

// SessionOwnerCache remembers which account owns a session.
//
// It caches ownership and not liveness, and that is what makes it safe: the
// owner of a session row never changes, so a cached answer cannot go stale into
// something dangerous. Whether the session is still live is asked of Postgres
// every time, by an UPDATE guarded on `revoked_at IS NULL` — a cache that
// answered that question could resurrect a revoked session for up to its TTL.
type SessionOwnerCache struct {
	cache cache.Cache[uuid.UUID]
	env   string
}

// NewSessionOwnerCache builds the cache. A nil store returns nil, so the service
// sees "no cache" rather than a wrapper that silently does nothing.
func NewSessionOwnerCache(store cache.Cache[uuid.UUID], env string) *SessionOwnerCache {
	if store == nil {
		return nil
	}
	return &SessionOwnerCache{cache: store, env: env}
}

// Get returns the owning account, or an error when the answer is not cached.
func (c *SessionOwnerCache) Get(ctx context.Context, key string) (uuid.UUID, error) {
	if c == nil || c.cache == nil {
		return uuid.Nil, cache.ErrMiss
	}
	owner, err := c.cache.Get(ctx, key)
	if err != nil {
		return uuid.Nil, fmt.Errorf("read cached session owner: %w", err)
	}
	return owner, nil
}

// Set records the owning account for ttl.
func (c *SessionOwnerCache) Set(ctx context.Context, key string, value uuid.UUID, ttl time.Duration) error {
	if c == nil || c.cache == nil {
		return nil
	}
	return c.cache.Set(ctx, key, value, ttl)
}

// Delete drops entries for sessions that are now dead.
func (c *SessionOwnerCache) Delete(ctx context.Context, keys ...string) error {
	if c == nil || c.cache == nil {
		return nil
	}
	return c.cache.Delete(ctx, keys...)
}
