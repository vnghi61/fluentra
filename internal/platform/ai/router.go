package ai

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Router handles task routing, caching, resilience, and fallback chains.
type Router struct {
	prompts   *Registry
	providers *ProviderRegistry
	cache     ResponseCache
	cacheTTL  time.Duration
}

// RouterOptions configures router behavior.
type RouterOptions struct {
	Prompts   *Registry
	Providers *ProviderRegistry
	Cache     ResponseCache
	CacheTTL  time.Duration
}

// NewRouter creates a new AI task router.
func NewRouter(opts RouterOptions) *Router {
	cache := opts.Cache
	if cache == nil {
		cache = NewMemoryCache()
	}
	cacheTTL := opts.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 24 * time.Hour
	}
	return &Router{
		prompts:   opts.Prompts,
		providers: opts.Providers,
		cache:     cache,
		cacheTTL:  cacheTTL,
	}
}

// Complete implements Client interface.
func (r *Router) Complete(ctx context.Context, req Request) (Response, error) {
	if r.prompts == nil {
		return Response{}, fmt.Errorf("ai: prompt registry not initialized")
	}

	tmpl, err := r.prompts.Get(req.Task)
	if err != nil {
		return Response{}, err
	}

	// 1. Consult exact-hash response cache
	cacheKey := ComputeCacheKey(req.Task, tmpl.Version, req.Vars)
	if cached, found := r.cache.Get(ctx, cacheKey); found {
		return cached, nil
	}

	if r.providers == nil {
		return Response{}, ErrDisabled
	}

	// 2. Try primary provider with retry
	primary, err := r.providers.Primary()
	if err != nil {
		return Response{}, err
	}

	res, err := r.executeWithRetry(ctx, primary, req)
	if err == nil {
		r.cache.Set(ctx, cacheKey, res, r.cacheTTL)
		return res, nil
	}

	// 3. Try fallback provider if primary failed
	if fallback, ok := r.providers.Fallback(); ok {
		slog.WarnContext(ctx, "ai: primary provider failed, attempting fallback",
			"task", req.Task,
			"primary", primary.Name(),
			"fallback", fallback.Name(),
			"error", err)

		res, fbErr := r.executeWithRetry(ctx, fallback, req)
		if fbErr == nil {
			r.cache.Set(ctx, cacheKey, res, r.cacheTTL)
			return res, nil
		}
		return Response{}, fmt.Errorf("ai: all providers failed (primary: %v, fallback: %w)", err, fbErr)
	}

	return Response{}, fmt.Errorf("ai: provider %s failed for task %s: %w", primary.Name(), req.Task, err)
}

func (r *Router) executeWithRetry(
	ctx context.Context,
	p Provider,
	req Request,
) (Response, error) {
	var lastErr error
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Response{}, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		res, err := p.Complete(ctx, req)
		if err == nil {
			return res, nil
		}
		lastErr = err

		// Do not retry context cancellation
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
	}
	return Response{}, lastErr
}
