package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ResponseCache interface for exact-hash response caching.
type ResponseCache interface {
	Get(ctx context.Context, key string) (Response, bool)
	Set(ctx context.Context, key string, res Response, ttl time.Duration)
}

// MemoryCache is a thread-safe in-memory implementation of ResponseCache.
type MemoryCache struct {
	mu      sync.RWMutex
	entries map[string]cacheItem
}

type cacheItem struct {
	res       Response
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory response cache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		entries: make(map[string]cacheItem),
	}
}

// Get retrieves a response from cache if present and not expired.
func (c *MemoryCache) Get(_ context.Context, key string) (Response, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.entries[key]
	if !exists {
		return Response{}, false
	}
	if time.Now().After(item.expiresAt) {
		return Response{}, false
	}
	return item.res, true
}

// Set stores a response in cache with the given TTL.
func (c *MemoryCache) Set(_ context.Context, key string, res Response, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheItem{
		res:       res,
		expiresAt: time.Now().Add(ttl),
	}
}

// ComputeCacheKey calculates a deterministic SHA-256 hash for task, version and inputs.
func ComputeCacheKey(task Task, version int, vars map[string]any) string {
	// Canonicalize map keys
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ordered := make([]string, 0, len(keys))
	for _, k := range keys {
		valBytes, _ := json.Marshal(vars[k])
		ordered = append(ordered, fmt.Sprintf("%s=%s", k, string(valBytes)))
	}

	h := sha256.New()
	_, _ = fmt.Fprintf(h, "task:%s:v:%d:%s", task, version, stringsJoin(ordered, ";"))
	return hex.EncodeToString(h.Sum(nil))
}

func stringsJoin(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	res := items[0]
	for i := 1; i < len(items); i++ {
		res += sep + items[i]
	}
	return res
}
