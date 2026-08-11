package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fluentra/fluentra/internal/platform/cache"
)

// denylistVersion is the `v1` in the cache key. Bump it if the stored shape
// changes, so a deploy cannot read an old value as a new one.
const denylistVersion = 1

// TokenDenylist records access tokens that must stop working before they
// expire.
//
// Redis rather than Postgres, and that is the whole reason the denylist is
// affordable. ADR-0007 rejected server-side sessions because they put a
// database read on every request; a denylist stored in Postgres would put the
// same read back. A Redis GET on a key that is almost always absent is the cost
// this design was chosen to accept.
//
// Entries expire on their own. There is no sweep and nothing to clean up: a
// denied token is only interesting until it would have expired anyway, so the
// TTL the caller passes is the retention policy.
type TokenDenylist struct {
	cache cache.Cache[bool]
	env   string
}

// NewTokenDenylist builds a denylist over a typed cache.
//
// env namespaces the keys so a staging deploy pointed at a shared Redis cannot
// sign somebody out of production — or, worse, fail to.
func NewTokenDenylist(store cache.Cache[bool], env string) *TokenDenylist {
	return &TokenDenylist{cache: store, env: env}
}

// Deny records a token id for ttl.
func (d *TokenDenylist) Deny(ctx context.Context, tokenID string, ttl time.Duration) error {
	if d.cache == nil || ttl <= 0 {
		return nil
	}
	if err := d.cache.Set(ctx, d.key(tokenID), true, ttl); err != nil {
		// Returned rather than swallowed. This is the write half, and a logout
		// that silently failed to revoke anything is a logout that lied to the
		// learner — the read half is where unavailability is tolerated.
		return fmt.Errorf("denylist token: %w", err)
	}
	return nil
}

// IsDenied reports whether the token id has been denied.
//
// A cache miss is the overwhelmingly common case and is not an error: almost no
// token is ever denylisted. Only a genuine failure to reach the store returns
// one, and TokenService.Verify is where the decision about what to do with that
// lives.
func (d *TokenDenylist) IsDenied(ctx context.Context, tokenID string) (bool, error) {
	if d.cache == nil {
		return false, nil
	}
	denied, err := d.cache.Get(ctx, d.key(tokenID))
	if err != nil {
		if errors.Is(err, cache.ErrMiss) {
			return false, nil
		}
		return false, fmt.Errorf("read token denylist: %w", err)
	}
	return denied, nil
}

func (d *TokenDenylist) key(tokenID string) string {
	return cache.Key(d.env, "auth", "denylist", tokenID, denylistVersion)
}
