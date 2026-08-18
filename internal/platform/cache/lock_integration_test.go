//go:build integration

package cache_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/fluentra/fluentra/internal/platform/cache"
)

// The unit suite covers this behaviour against a fake whose Eval reimplements
// the compare-and-delete in Go. That fake pins the caller's contract, but it
// cannot fail the way the real thing can: the release path is a Lua script, and
// a wrong KEYS/ARGV index or a `del` where `get` was meant is invisible to a
// fake that never evaluates it. P0.8's acceptance — "a lock cannot be released
// by a different holder" — is about the script, so it is proved here, against a
// Redis that actually runs it.

func newLockTestClient(t *testing.T) *redis.Client {
	t.Helper()

	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR is not set")
	}

	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("reach redis at %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestLocker_ReleaseIsTokenCheckedAgainstRealRedis(t *testing.T) {
	client := newLockTestClient(t)
	ctx := context.Background()
	locker := cache.NewRedisLocker(client)

	// A key per run: the suite runs with -race against a shared container, and
	// a fixed key would make two packages' tests each other's flakes.
	key := "fluentra:test:lock:" + t.Name() + ":" + time.Now().Format("150405.000000000")
	t.Cleanup(func() { _ = client.Del(ctx, key).Err() })

	release, err := locker.Acquire(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// A second holder cannot take a lock that is held.
	if _, err := locker.Acquire(ctx, key, time.Minute); err == nil {
		t.Fatal("a second Acquire succeeded while the lock was held")
	}

	// The holder releases its own lock, and the key goes with it.
	if err := release(ctx); err != nil {
		t.Fatalf("release by the holder: %v", err)
	}
	if exists, err := client.Exists(ctx, key).Result(); err != nil || exists != 0 {
		t.Fatalf("key still present after its holder released it (exists=%d, err=%v)", exists, err)
	}
}

func TestLocker_ReleaseDoesNotStealAnotherHoldersLock(t *testing.T) {
	client := newLockTestClient(t)
	ctx := context.Background()
	locker := cache.NewRedisLocker(client)

	key := "fluentra:test:lock:" + t.Name() + ":" + time.Now().Format("150405.000000000")
	t.Cleanup(func() { _ = client.Del(ctx, key).Err() })

	staleRelease, err := locker.Acquire(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// Stand in for the sequence this guard exists for: the first holder's lock
	// lapsed, a second holder took the key, and the first holder's deferred
	// release then runs. Overwriting the value is the same state Redis would be
	// in, without waiting out a TTL.
	const secondHolderToken = "second-holder-token"
	if err := client.Set(ctx, key, secondHolderToken, time.Minute).Err(); err != nil {
		t.Fatalf("simulate the second holder: %v", err)
	}

	if err := staleRelease(ctx); err == nil {
		t.Fatal("a stale release reported success against another holder's lock")
	}

	// The point of the whole exercise: the second holder still has its lock.
	value, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("read the key back: %v", err)
	}
	if value != secondHolderToken {
		t.Fatalf("the second holder's lock was destroyed: key = %q, want %q", value, secondHolderToken)
	}

	// And it stays destroyed-proof on a repeat, because release is not
	// idempotent-by-deleting — it is idempotent by refusing.
	if err := staleRelease(ctx); err == nil {
		t.Fatal("a repeated stale release reported success")
	}
}
