package ai_test

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/platform/ai"
)

const (
	resilienceFieldKind      = "Kind"
	resilienceFieldCEFRLevel = "CEFRLevel"
	resilienceFallbackText   = "resilience-fallback-success"
)

// Reuse the existing provider fixture, adding race-safe counts and a blocking
// callback for the timeout/cancellation cases.
type resilienceProvider struct {
	namedProvider
	calls    atomic.Int64
	complete func(context.Context, ai.Request) (ai.Response, error)
}

func (p *resilienceProvider) Complete(ctx context.Context, req ai.Request) (ai.Response, error) {
	p.calls.Add(1)
	if p.complete != nil {
		return p.complete(ctx, req)
	}
	return p.namedProvider.Complete(ctx, req)
}

type resilienceBudgetFunc func(context.Context, string, ai.Task) (bool, error)

func (f resilienceBudgetFunc) CheckQuota(ctx context.Context, provider string, task ai.Task) (bool, error) {
	return f(ctx, provider, task)
}

type resilienceResult struct {
	response ai.Response
	err      error
}

func awaitResilienceResult(ctx context.Context, t *testing.T, results <-chan resilienceResult) resilienceResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-ctx.Done():
		t.Fatalf("router did not finish: %v", ctx.Err())
		return resilienceResult{}
	}
}

func resilienceRequest(t *testing.T, prompts *ai.Registry) ai.Request {
	t.Helper()
	tmpl, err := prompts.Get(ai.TaskPracticeGenerate)
	require.NoError(t, err)
	require.False(t, tmpl.Cache, "identical concurrent requests must not bypass provider rotation via cache")
	return ai.Request{
		Task: ai.TaskPracticeGenerate,
		Vars: map[string]any{resilienceFieldKind: "reading_comprehension", resilienceFieldCEFRLevel: "B1"},
	}
}

// HP-01: Each uncached request rotates its primary, but every attempted provider
// must still pass its own budget check, including a rotated fallback.
func TestRouter_ConcurrentUncachedRequestsBalanceProvidersAndRespectBudgets(t *testing.T) {
	const requests = 20
	for _, tc := range []struct {
		name              string
		primaryAllowed    bool
		wantPrimaryCalls  int64
		wantFallbackCalls int64
	}{
		{"both providers available", true, requests / 2, requests / 2},
		{"configured primary exhausted", false, 0, requests},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prompts, err := ai.NewRegistry()
			require.NoError(t, err)
			req := resilienceRequest(t, prompts)
			primary := &resilienceProvider{namedProvider: namedProvider{
				name: testPrimaryLLM, res: ai.Response{Text: testFromPrimary},
			}}
			fallback := &resilienceProvider{namedProvider: namedProvider{
				name: testFallbackLLM, res: ai.Response{Text: resilienceFallbackText},
			}}
			type quotaCheck struct {
				provider string
				task     ai.Task
			}
			checks := make(chan quotaCheck, 2*requests)
			budget := &mockBudgetChecker{allowed: map[string]bool{
				testPrimaryLLM: tc.primaryAllowed, testFallbackLLM: true,
			}}
			router := ai.NewRouter(ai.RouterOptions{
				Prompts: prompts, Providers: ai.NewProviderRegistry(primary, fallback),
				BalanceProviders: true,
				Budget: resilienceBudgetFunc(func(ctx context.Context, provider string, task ai.Task) (bool, error) {
					checks <- quotaCheck{provider: provider, task: task}
					return budget.CheckQuota(ctx, provider, task)
				}),
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			start := make(chan struct{})
			results := make(chan resilienceResult, requests)
			for range requests {
				go func() {
					<-start
					res, err := router.Complete(ctx, req)
					results <- resilienceResult{response: res, err: err}
				}()
			}
			close(start)

			responses := make(map[string]int64)
			for range requests {
				result := awaitResilienceResult(ctx, t, results)
				require.NoError(t, result.err)
				responses[result.response.Provider]++
				if result.response.Provider == testPrimaryLLM {
					assert.Equal(t, primary.res.Text, result.response.Text)
				} else {
					assert.Equal(t, testFallbackLLM, result.response.Provider)
					assert.Equal(t, fallback.res.Text, result.response.Text)
				}
			}
			close(checks)
			checked := make(map[string]int64)
			for check := range checks {
				assert.Equal(t, req.Task, check.task)
				checked[check.provider]++
			}
			assert.Equal(t, tc.wantPrimaryCalls, primary.calls.Load())
			assert.Equal(t, tc.wantFallbackCalls, fallback.calls.Load())
			assert.Equal(t, tc.wantPrimaryCalls, responses[testPrimaryLLM])
			assert.Equal(t, tc.wantFallbackCalls, responses[testFallbackLLM])
			assert.EqualValues(t, requests/2, checked[testPrimaryLLM])
			assert.Equal(t, tc.wantFallbackCalls, checked[testFallbackLLM])
		})
	}
}

// ERR-01: A provider that honors its attempt deadline gets one call, not three;
// the fallback receives a fresh context while the parent remains live.
func TestRouter_AttemptDeadlineFallsBackWithoutRetryingStalledProvider(t *testing.T) {
	prompts, err := ai.NewRegistry()
	require.NoError(t, err)
	req := resilienceRequest(t, prompts)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var attemptCtx context.Context
	primary := &resilienceProvider{namedProvider: namedProvider{name: testPrimaryLLM}}
	primary.complete = func(ctx context.Context, _ ai.Request) (ai.Response, error) {
		attemptCtx = ctx
		<-ctx.Done()
		return ai.Response{}, fmt.Errorf("provider stopped: %w", ctx.Err())
	}
	fallback := &resilienceProvider{namedProvider: namedProvider{
		name: testFallbackLLM, res: ai.Response{Text: resilienceFallbackText},
	}}
	fallback.complete = func(ctx context.Context, _ ai.Request) (ai.Response, error) {
		return fallback.res, ctx.Err()
	}
	router := ai.NewRouter(ai.RouterOptions{
		Prompts: prompts, Providers: ai.NewProviderRegistry(primary, fallback),
		AttemptTimeout: 50 * time.Millisecond,
	})

	res, err := router.Complete(ctx, req)

	require.NoError(t, err)
	assert.NoError(t, ctx.Err(), "only the attempt, not the parent, should expire")
	require.NotNil(t, attemptCtx)
	assert.ErrorIs(t, attemptCtx.Err(), context.DeadlineExceeded)
	assert.Equal(t, int64(1), primary.calls.Load())
	assert.Equal(t, int64(1), fallback.calls.Load())
	assert.Equal(t, testFallbackLLM, res.Provider)
	assert.Equal(t, fallback.res.Text, res.Text)
}

// ERR-02: Cancel only after the provider has entered, and release it only after
// cancellation, so scheduling and the 50ms attempt deadline cannot race the test.
func TestRouter_ParentCancellationNeverCallsFallback(t *testing.T) {
	prompts, err := ai.NewRegistry()
	require.NoError(t, err)
	req := resilienceRequest(t, prompts)
	waitCtx, stopWaiting := context.WithTimeout(t.Context(), 5*time.Second)
	defer stopWaiting()
	entered := make(chan struct{}, 1)
	resume := make(chan struct{}, 1)
	defer close(resume)
	ctx, cancel := context.WithCancel(waitCtx)
	defer cancel()
	primary := &resilienceProvider{namedProvider: namedProvider{name: testPrimaryLLM}}
	primary.complete = func(ctx context.Context, _ ai.Request) (ai.Response, error) {
		entered <- struct{}{}
		<-resume
		<-ctx.Done()
		return ai.Response{}, ctx.Err()
	}
	fallback := &resilienceProvider{namedProvider: namedProvider{
		name: testFallbackLLM, res: ai.Response{Text: resilienceFallbackText},
	}}
	var fallbackChecks atomic.Int64
	router := ai.NewRouter(ai.RouterOptions{
		Prompts: prompts, Providers: ai.NewProviderRegistry(primary, fallback),
		AttemptTimeout: 50 * time.Millisecond,
		Budget: resilienceBudgetFunc(func(_ context.Context, provider string, _ ai.Task) (bool, error) {
			if provider == testFallbackLLM {
				fallbackChecks.Add(1)
			}
			return true, nil
		}),
	})
	results := make(chan resilienceResult, 1)
	go func() {
		res, err := router.Complete(ctx, req)
		results <- resilienceResult{response: res, err: err}
	}()
	select {
	case <-entered:
	case <-waitCtx.Done():
		t.Fatal("primary provider did not start")
	}
	cancel()
	resume <- struct{}{}
	result := awaitResilienceResult(waitCtx, t, results)

	assert.ErrorIs(t, result.err, context.Canceled)
	assert.Empty(t, result.response)
	assert.Equal(t, int64(1), primary.calls.Load())
	assert.Zero(t, fallback.calls.Load())
	assert.Zero(t, fallbackChecks.Load(), "cancellation must stop before even checking fallback quota")
}

// ERR-03: Permanent client errors, including wrapped status errors, go straight
// to the next provider instead of spending three attempts on the same refusal.
func TestRouter_PermanentClientErrorFallsBackAfterOneCall(t *testing.T) {
	for _, status := range []int{
		http.StatusBadRequest, http.StatusUnauthorized, http.StatusPaymentRequired,
		http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			prompts, err := ai.NewRegistry()
			require.NoError(t, err)
			req := resilienceRequest(t, prompts)
			primary := &resilienceProvider{namedProvider: namedProvider{
				name: testPrimaryLLM,
				err:  fmt.Errorf("provider refused: %w", &ai.ProviderStatusError{Status: status}),
			}}
			fallback := &resilienceProvider{namedProvider: namedProvider{
				name: testFallbackLLM, res: ai.Response{Text: resilienceFallbackText},
			}}
			router := ai.NewRouter(ai.RouterOptions{
				Prompts: prompts, Providers: ai.NewProviderRegistry(primary, fallback),
				AttemptTimeout: 50 * time.Millisecond,
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			res, err := router.Complete(ctx, req)

			require.NoError(t, err)
			assert.Equal(t, int64(1), primary.calls.Load())
			assert.Equal(t, int64(1), fallback.calls.Load())
			assert.Equal(t, testFallbackLLM, res.Provider)
			assert.Equal(t, fallback.res.Text, res.Text)
		})
	}
}
