package cache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/redis/go-redis/v9"
)

type mockRedisClient struct {
	redis.Cmdable
	data map[string][]byte
	mu   sync.Mutex
	down bool
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{data: make(map[string][]byte)}
}

func (m *mockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx, "get", key)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.down {
		cmd.SetErr(errors.New("redis down"))
		return cmd
	}
	val, ok := m.data[key]
	if !ok {
		cmd.SetErr(redis.Nil)
		return cmd
	}
	cmd.SetVal(string(val))
	return cmd
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value any, _ time.Duration) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx, "set", key, value)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.down {
		cmd.SetErr(errors.New("redis down"))
		return cmd
	}
	switch v := value.(type) {
	case string:
		m.data[key] = []byte(v)
	case []byte:
		m.data[key] = v
	}
	cmd.SetVal("OK")
	return cmd
}

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx, "del")
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.down {
		cmd.SetErr(errors.New("redis down"))
		return cmd
	}
	var count int64
	for _, k := range keys {
		if _, ok := m.data[k]; ok {
			delete(m.data, k)
			count++
		}
	}
	cmd.SetVal(count)
	return cmd
}

func TestKeyBuilder(t *testing.T) {
	key := cache.Key("dev", "auth", "user", "12345", 1)
	expected := "fluentra:dev:auth:user:12345:v1"
	if key != expected {
		t.Errorf("got %q, want %q", key, expected)
	}
}

func TestCache_GetOrLoad_Singleflight(t *testing.T) {
	client := newMockRedisClient()
	c := cache.NewRedisCache[string](client)
	ctx := context.Background()

	var loaderCalls int32
	loader := func(context.Context) (string, error) {
		atomic.AddInt32(&loaderCalls, 1)
		time.Sleep(50 * time.Millisecond)
		return "loaded_value", nil
	}

	const goroutines = 50
	var wg sync.WaitGroup
	results := make([]string, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			val, err := c.GetOrLoad(ctx, "test_key", time.Minute, loader)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			results[idx] = val
		}(i)
	}

	wg.Wait()

	if loaderCalls != 1 {
		t.Errorf("expected 1 loader call due to singleflight, got %d", loaderCalls)
	}
	for i, res := range results {
		if res != "loaded_value" {
			t.Errorf("goroutine %d got %q, want %q", i, res, "loaded_value")
		}
	}
}

func TestCache_RedisDown_Degradation(t *testing.T) {
	client := newMockRedisClient()
	client.down = true // Simulate Redis failure
	c := cache.NewRedisCache[string](client)
	ctx := context.Background()

	loaderCalled := false
	loader := func(context.Context) (string, error) {
		loaderCalled = true
		return "fallback_value", nil
	}

	val, err := c.GetOrLoad(ctx, "degraded_key", time.Minute, loader)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got err: %v", err)
	}

	if !loaderCalled {
		t.Error("expected loader to be called when Redis is down")
	}

	if val != "fallback_value" {
		t.Errorf("got %q, want %q", val, "fallback_value")
	}
}

// TestLimiter_AllowsWhenRedisIsUnreachable is the P2.8 requirement: a Redis
// outage must degrade to allow-with-warn, not deny-all. A limiter that denies
// when its store is down converts a cache outage into a full outage.
func TestLimiter_AllowsWhenRedisIsUnreachable(t *testing.T) {
	t.Parallel()
	// Port 1 is reserved and never listening, so every command fails fast.
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 50 * time.Millisecond})
	t.Cleanup(func() { _ = client.Close() })

	result, err := cache.NewRedisLimiter(client).Allow(context.Background(), "auth:ip:203.0.113.1", 5, time.Minute)
	if err != nil {
		t.Fatalf("a Redis outage must not surface as an error: %v", err)
	}
	if !result.Allowed {
		t.Fatal("limiter denied the request while Redis was unreachable")
	}
	if !result.Degraded {
		t.Error("result should be marked degraded so callers omit RateLimit-* headers")
	}
}

func TestLimiter_AllowsWhenClientIsNotConfigured(t *testing.T) {
	t.Parallel()
	result, err := cache.NewRedisLimiter(nil).Allow(context.Background(), "k", 10, time.Minute)
	if err != nil || !result.Allowed || !result.Degraded {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

// TestLocker_DoesNotFailOpen guards the asymmetry: a lock that is handed out
// when Redis is down is not a lock.
func TestLocker_DoesNotFailOpen(t *testing.T) {
	t.Parallel()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 50 * time.Millisecond})
	t.Cleanup(func() { _ = client.Close() })

	if _, err := cache.NewRedisLocker(client).Acquire(context.Background(), "lock:k", time.Minute); err == nil {
		t.Fatal("Acquire returned a lock while Redis was unreachable")
	}
}
