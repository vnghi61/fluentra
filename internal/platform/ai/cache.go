package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ResponseCache interface for exact-hash response caching.
type ResponseCache interface {
	Get(ctx context.Context, key string) (Response, bool)
	Set(ctx context.Context, key string, task Task, res Response, ttl time.Duration)
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
func (c *MemoryCache) Set(_ context.Context, key string, _ Task, res Response, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheItem{
		res:       res,
		expiresAt: time.Now().Add(ttl),
	}
}

// DBCache is a PostgreSQL-backed implementation of ResponseCache.
type DBCache struct {
	pool *pgxpool.Pool
}

// NewDBCache creates a new database-backed response cache.
func NewDBCache(pool *pgxpool.Pool) *DBCache {
	return &DBCache{pool: pool}
}

// Get retrieves a response from ai.ai_cache_entries if present and not expired.
func (c *DBCache) Get(ctx context.Context, key string) (Response, bool) {
	if c.pool == nil {
		return Response{}, false
	}

	const query = `
		SELECT response_text, model, provider
		FROM ai.ai_cache_entries
		WHERE cache_key = $1 AND expires_at > now()`

	var text, model, provider string
	err := c.pool.QueryRow(ctx, query, key).Scan(&text, &model, &provider)
	if err != nil {
		// A miss and a broken cache look identical to the caller, and they
		// should: either way the provider answers and the learner is served.
		// They must not look identical in the log. A cache that errors on every
		// read spends real money on every request and reports nothing, which is
		// the failure this whole table exists to prevent.
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.WarnContext(ctx, "ai: response cache read failed; treating as a miss",
				"error", err, "cache_key", key)
		}
		return Response{}, false
	}
	return Response{
		Text:     text,
		Model:    model,
		Provider: provider,
	}, true
}

// Set stores a response in ai.ai_cache_entries with the given TTL.
func (c *DBCache) Set(ctx context.Context, key string, task Task, res Response, ttl time.Duration) {
	if c.pool == nil {
		return
	}

	const upsert = `
		INSERT INTO ai.ai_cache_entries (
			cache_key, task, model, provider, response_text, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (cache_key) DO UPDATE
		SET task = EXCLUDED.task,
		    model = EXCLUDED.model,
		    provider = EXCLUDED.provider,
		    response_text = EXCLUDED.response_text,
		    expires_at = EXCLUDED.expires_at,
		    created_at = now()`

	expiresAt := time.Now().Add(ttl)
	// Deliberately not returned: the answer is already in the caller's hands and
	// refusing it because the cache could not be written would trade a saved
	// call for a lost one. Logged, though -- a write that always fails is a
	// cache that never fills, and the only symptom is the bill.
	if _, err := c.pool.Exec(ctx, upsert, key, string(task), res.Model, res.Provider, res.Text, expiresAt); err != nil {
		slog.WarnContext(ctx, "ai: response cache write failed; the next identical request will pay again",
			"error", err, "task", string(task))
	}
}

// PruneExpired deletes cache rows that have outlived their TTL.
func (c *DBCache) PruneExpired(ctx context.Context) (int64, error) {
	if c.pool == nil {
		return 0, nil
	}
	res, err := c.pool.Exec(ctx, `DELETE FROM ai.ai_cache_entries WHERE expires_at <= now()`)
	if err != nil {
		return 0, fmt.Errorf("ai: prune expired cache entries: %w", err)
	}
	return res.RowsAffected(), nil
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
