package ai

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Router handles task routing, caching, resilience, and fallback chains.
type Router struct {
	prompts   *Registry
	providers *ProviderRegistry
	cache     ResponseCache
	cacheTTL  time.Duration
	usage     UsageRecorder
	budget    BudgetChecker
}

// RouterOptions configures router behavior.
type RouterOptions struct {
	Prompts   *Registry
	Providers *ProviderRegistry
	Cache     ResponseCache
	CacheTTL  time.Duration
	Usage     UsageRecorder
	Budget    BudgetChecker
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
	usage := opts.Usage
	if usage == nil {
		usage = NoopUsageRecorder{}
	}
	budget := opts.Budget
	if budget == nil {
		budget = NoopBudgetChecker{}
	}
	return &Router{
		prompts:   opts.Prompts,
		providers: opts.Providers,
		cache:     cache,
		cacheTTL:  cacheTTL,
		usage:     usage,
		budget:    budget,
	}
}

// Complete implements Client interface.
func (r *Router) Complete(ctx context.Context, req Request) (Response, error) {
	start := time.Now()
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
		r.record(ctx, RequestLog{
			Task:      req.Task,
			Provider:  cached.Provider,
			Model:     cached.Model,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Status:    StatusCached,
			CreatedAt: time.Now(),
		})
		return cached, nil
	}

	if r.providers == nil {
		return Response{}, ErrDisabled
	}

	primary, err := r.providers.Primary()
	if err != nil {
		return Response{}, err
	}

	// 2. Try primary provider if budget allows
	primaryAllowed, primaryBudgetErr := r.budget.CheckQuota(ctx, primary.Name(), req.Task)
	var primaryExecErr error
	if primaryAllowed && primaryBudgetErr == nil {
		res, err := r.executeWithRetry(ctx, primary, req)
		if err == nil {
			res.Provider = primary.Name()
			r.cache.Set(ctx, cacheKey, req.Task, res, r.cacheTTL)
			r.record(ctx, RequestLog{
				Task:             req.Task,
				Provider:         primary.Name(),
				Model:            res.Model,
				PromptTokens:     res.PromptTokens,
				CompletionTokens: res.CompletionTokens,
				LatencyMs:        int(time.Since(start).Milliseconds()),
				Status:           StatusSuccess,
				CreatedAt:        time.Now(),
			})
			return res, nil
		}
		primaryExecErr = err
	} else if primaryBudgetErr != nil {
		primaryExecErr = primaryBudgetErr
	} else {
		primaryExecErr = fmt.Errorf("provider %s quota exhausted", primary.Name())
	}

	return r.executeFallback(ctx, req, cacheKey, primary, primaryExecErr, primaryAllowed, start)
}

func (r *Router) executeFallback(
	ctx context.Context,
	req Request,
	cacheKey string,
	primary Provider,
	primaryErr error,
	primaryAllowed bool,
	start time.Time,
) (Response, error) {
	fallbacks := r.providers.Fallbacks()
	if len(fallbacks) == 0 {
		if !primaryAllowed {
			r.record(ctx, RequestLog{
				Task:         req.Task,
				Provider:     primary.Name(),
				LatencyMs:    int(time.Since(start).Milliseconds()),
				Status:       StatusRateLimited,
				ErrorMessage: ErrQuotaExhausted.Error(),
				CreatedAt:    time.Now(),
			})
			return Response{}, ErrQuotaExhausted
		}
		r.record(ctx, RequestLog{
			Task:         req.Task,
			Provider:     primary.Name(),
			Model:        "",
			LatencyMs:    int(time.Since(start).Milliseconds()),
			Status:       StatusFailed,
			ErrorMessage: primaryErr.Error(),
			CreatedAt:    time.Now(),
		})
		return Response{}, fmt.Errorf("ai: provider %s failed for task %s: %w", primary.Name(), req.Task, primaryErr)
	}

	allExhausted := !primaryAllowed
	var errMsgs []string
	if primaryErr != nil {
		errMsgs = append(errMsgs, fmt.Sprintf("primary (%s): %v", primary.Name(), primaryErr))
	}

	for _, fallback := range fallbacks {
		slog.WarnContext(ctx, "ai: attempting fallback provider",
			"task", req.Task,
			"primary", primary.Name(),
			"fallback", fallback.Name(),
			"previous_error", primaryErr)

		fallbackAllowed, fallbackBudgetErr := r.budget.CheckQuota(ctx, fallback.Name(), req.Task)
		if !fallbackAllowed {
			errMsgs = append(errMsgs, fmt.Sprintf("fallback (%s): quota exhausted", fallback.Name()))
			continue
		}
		allExhausted = false

		if fallbackBudgetErr != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("fallback (%s) budget check failed: %v", fallback.Name(), fallbackBudgetErr))
			continue
		}

		res, err := r.executeWithRetry(ctx, fallback, req)
		if err == nil {
			res.Provider = fallback.Name()
			r.cache.Set(ctx, cacheKey, req.Task, res, r.cacheTTL)
			r.record(ctx, RequestLog{
				Task:             req.Task,
				Provider:         fallback.Name(),
				Model:            res.Model,
				PromptTokens:     res.PromptTokens,
				CompletionTokens: res.CompletionTokens,
				LatencyMs:        int(time.Since(start).Milliseconds()),
				Status:           StatusSuccess,
				CreatedAt:        time.Now(),
			})
			return res, nil
		}

		errMsgs = append(errMsgs, fmt.Sprintf("fallback (%s): %v", fallback.Name(), err))
	}

	if allExhausted {
		r.record(ctx, RequestLog{
			Task:         req.Task,
			Provider:     primary.Name(),
			LatencyMs:    int(time.Since(start).Milliseconds()),
			Status:       StatusRateLimited,
			ErrorMessage: ErrQuotaExhausted.Error(),
			CreatedAt:    time.Now(),
		})
		return Response{}, ErrQuotaExhausted
	}

	combinedErr := strings.Join(errMsgs, ", ")
	r.record(ctx, RequestLog{
		Task:         req.Task,
		Provider:     primary.Name(),
		Model:        "",
		LatencyMs:    int(time.Since(start).Milliseconds()),
		Status:       StatusFailed,
		ErrorMessage: fmt.Sprintf("all providers failed (%s)", combinedErr),
		CreatedAt:    time.Now(),
	})
	return Response{}, fmt.Errorf("ai: all providers failed (%s)", combinedErr)
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

// record writes one usage entry, and never fails the request that produced it.
//
// The answer is already in the caller's hands by the time this runs; refusing it
// because the bookkeeping failed would trade a served learner for a tidy table.
// But it is logged rather than discarded: a recorder that errors on every call
// leaves ai_requests empty, and an empty usage table reads exactly like an
// unused feature. That is the reading that matters, because WP18's budgets are
// enforced against these rows -- a quota with nothing recording spend is a quota
// that never triggers.
//
// The nil check NewRouter already makes unnecessary is not repeated here.
func (r *Router) record(ctx context.Context, entry RequestLog) {
	if err := r.usage.Record(ctx, entry); err != nil {
		slog.WarnContext(ctx, "ai: usage recording failed",
			"error", err, "task", string(entry.Task), "status", string(entry.Status))
	}
}

// HasQuota reports whether any configured provider has available quota for task.
func (r *Router) HasQuota(ctx context.Context, task Task) (bool, error) {
	if r.budget == nil || r.providers == nil {
		return true, nil
	}
	primary, err := r.providers.Primary()
	if err == nil {
		allowed, checkErr := r.budget.CheckQuota(ctx, primary.Name(), task)
		if checkErr == nil && allowed {
			return true, nil
		}
	}
	for _, fallback := range r.providers.Fallbacks() {
		allowed, checkErr := r.budget.CheckQuota(ctx, fallback.Name(), task)
		if checkErr == nil && allowed {
			return true, nil
		}
	}
	return false, nil
}
