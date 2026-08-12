package httpx_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// countingLimiter is an in-memory fixed window. Every guard in it mirrors what
// cache.RedisLimiter does with its Lua script, including the one that matters
// most: an unreachable store allows the request and says it was not evaluated.
type countingLimiter struct {
	mu     sync.Mutex
	counts map[string]int
	keys   []string

	// err makes the store unreachable, which is the condition the whole
	// fail-open decision turns on.
	err error
}

func newCountingLimiter() *countingLimiter {
	return &countingLimiter{counts: map[string]int{}}
}

func (l *countingLimiter) Allow(
	_ context.Context, key string, limit int, window time.Duration,
) (httpx.LimitResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.err != nil {
		// What RedisLimiter does: allow, and mark the answer as not actually
		// evaluated so the caller does not advertise a budget it never checked.
		return httpx.LimitResult{Allowed: true, Degraded: true}, l.err
	}

	l.keys = append(l.keys, key)
	l.counts[key]++
	current := l.counts[key]
	if current > limit {
		return httpx.LimitResult{Allowed: false, Remaining: 0, ResetIn: window}, nil
	}
	return httpx.LimitResult{Allowed: true, Remaining: limit - current, ResetIn: window}, nil
}

func (l *countingLimiter) keysSeen() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.keys...)
}

func (l *countingLimiter) fail(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.err = err
}

// okHandler is what the middleware wraps. It records whether it ran, because
// "the request was refused" and "the request was served and then labelled 429"
// are different things and only one of them is a rate limit.
type okHandler struct {
	mu     sync.Mutex
	served int
}

func (h *okHandler) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	h.mu.Lock()
	h.served++
	h.mu.Unlock()
	writer.WriteHeader(http.StatusOK)
}

func (h *okHandler) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.served
}

// send drives one request through the middleware, with the client address the
// resolver would have put in the context.
func send(
	t *testing.T, handler http.Handler, method, target, remoteAddr string, actor *httpx.Actor,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, target, nil)
	request.RemoteAddr = remoteAddr
	if actor != nil {
		request = request.WithContext(httpx.WithActor(request.Context(), *actor))
	}

	recorder := httptest.NewRecorder()
	// The resolver is what puts the address in the context; the limiter reads
	// it from there rather than from RemoteAddr, so the two have to be chained
	// exactly as the router chains them.
	resolver, err := httpx.NewClientIPResolver(nil)
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}
	resolver.Middleware(handler).ServeHTTP(recorder, request)
	return recorder
}

// TestRateLimit_AnUnreachableStoreAllowsTheRequest is the acceptance criterion
// that matters most, and it is the one a careless implementation inverts.
//
// A limiter that denies when its backing store is down converts a Redis outage
// into a total outage of the whole API — every caller, every endpoint, refused.
// The rate limit protects far less than availability costs, so the failure
// direction is to allow and warn (API_GUIDELINE.md §11). The headers are
// deliberately absent in that case: a `RateLimit-Remaining: 59` derived from a
// budget nobody checked is a lie a client would then pace itself against.
func TestRateLimit_AnUnreachableStoreAllowsTheRequest(t *testing.T) {
	limiter := newCountingLimiter()
	limiter.fail(errors.New("dial tcp: connection refused"))
	handler := &okHandler{}

	guarded := httpx.RateLimit(httpx.RateLimitConfig{Limiter: limiter}).Middleware(handler)

	// Far more requests than any class allows. Every one is served.
	const attempts = 200
	for i := range attempts {
		recorder := send(t, guarded, http.MethodGet, "/api/v1/me", "203.0.113.7:5000", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d was refused with %d while the limiter was unreachable", i, recorder.Code)
		}
		if recorder.Header().Get("RateLimit-Limit") != "" {
			t.Fatalf("request %d advertised a budget that was never evaluated", i)
		}
	}
	if handler.count() != attempts {
		t.Errorf("%d of %d requests reached the handler", handler.count(), attempts)
	}
}

// TestRateLimit_ThePerIPChallengeCapBlocksASpreadAcrossManyAddresses is the
// second trap, and it is the one a per-subject limiter waves straight through.
//
// Three challenges per address per hour stops somebody hammering one inbox. It
// does nothing at all about a script that asks for one challenge each against
// ten thousand different addresses — which is the attack that actually matters,
// because it is how you enumerate accounts and how you make Fluentra send spam.
// The per-IP cap is what catches it, and the two are counted independently.
func TestRateLimit_ThePerIPChallengeCapBlocksASpreadAcrossManyAddresses(t *testing.T) {
	limiter := newCountingLimiter()
	handler := &okHandler{}

	const perIPHourly = 20
	guarded := httpx.RateLimit(httpx.RateLimitConfig{
		Limiter:             limiter,
		ChallengeIPPerHour:  perIPHourly,
		AnonymousPerMinute:  10_000, // out of the way; this test is about the challenge cap
		CredentialPerMinute: 10_000,
	}).Middleware(handler)

	// Every request names a different address, so no per-subject counter is
	// ever charged twice. Only the per-IP cap can stop this.
	refusedAt := 0
	for i := 1; i <= perIPHourly+5; i++ {
		// Each iteration would name a different address in its body. The path
		// and the source IP are all the middleware sees, and the IP is the one
		// thing the attacker cannot vary for free.
		recorder := send(t, guarded, http.MethodPost, "/api/v1/auth/register", "198.51.100.4:5000", nil)
		if recorder.Code == http.StatusTooManyRequests && refusedAt == 0 {
			refusedAt = i
		}
	}

	if refusedAt == 0 {
		t.Fatal("a script spreading challenges across many addresses was never refused")
	}
	if refusedAt != perIPHourly+1 {
		t.Errorf("refused at request %d, want the first one over the cap (%d)", refusedAt, perIPHourly+1)
	}
}

// TestRateLimit_ClassesAreCountedIndependently pins that a caller exhausting one
// budget does not spend another's. The credential class is the tight one, and if
// it shared a counter with the anonymous class a learner mistyping a password
// five times would also lose their ability to read anything.
func TestRateLimit_ClassesAreCountedIndependently(t *testing.T) {
	limiter := newCountingLimiter()
	handler := &okHandler{}

	guarded := httpx.RateLimit(httpx.RateLimitConfig{
		Limiter:             limiter,
		AnonymousPerMinute:  60,
		CredentialPerMinute: 2,
	}).Middleware(handler)

	const address = "203.0.113.9:5000"

	// Spend the credential budget.
	for range 2 {
		recorder := send(t, guarded, http.MethodPost, "/api/v1/auth/login", address, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("a request inside the credential budget was refused with %d", recorder.Code)
		}
	}
	over := send(t, guarded, http.MethodPost, "/api/v1/auth/login", address, nil)
	if over.Code != http.StatusTooManyRequests {
		t.Fatalf("the request over the credential budget returned %d, want 429", over.Code)
	}

	// The same address on an ordinary endpoint is untouched.
	ordinary := send(t, guarded, http.MethodGet, "/api/v1/me", address, nil)
	if ordinary.Code != http.StatusOK {
		t.Errorf("exhausting the credential budget also refused an ordinary request (%d)", ordinary.Code)
	}
}

// TestRateLimit_TheBoundaryIsAtTheLimitAndNotBeforeIt catches an off-by-one in
// either direction: refusing the request that is exactly at the limit, or
// allowing the first one past it.
func TestRateLimit_TheBoundaryIsAtTheLimitAndNotBeforeIt(t *testing.T) {
	const limit = 5

	limiter := newCountingLimiter()
	handler := &okHandler{}
	guarded := httpx.RateLimit(httpx.RateLimitConfig{
		Limiter:            limiter,
		AnonymousPerMinute: limit,
	}).Middleware(handler)

	for i := 1; i <= limit; i++ {
		recorder := send(t, guarded, http.MethodGet, "/api/v1/ping", "192.0.2.10:5000", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d of %d was refused; the limit is inclusive", i, limit)
		}
		if remaining := recorder.Header().Get("RateLimit-Remaining"); remaining != strconv.Itoa(limit-i) {
			t.Errorf("request %d: RateLimit-Remaining = %q, want %d", i, remaining, limit-i)
		}
	}

	recorder := send(t, guarded, http.MethodGet, "/api/v1/ping", "192.0.2.10:5000", nil)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("request %d returned %d, want 429", limit+1, recorder.Code)
	}
	if handler.count() != limit {
		t.Errorf("%d requests reached the handler, want %d", handler.count(), limit)
	}

	// The refusal has to tell the client how long to wait, and against which
	// budget. Retry-After alone leaves it guessing which limit it hit.
	if retry := recorder.Header().Get("Retry-After"); retry == "" || retry == "0" {
		t.Errorf("Retry-After = %q, want a positive number of seconds", retry)
	}
	if recorder.Header().Get("RateLimit-Limit") != strconv.Itoa(limit) {
		t.Errorf("RateLimit-Limit = %q, want %d", recorder.Header().Get("RateLimit-Limit"), limit)
	}
	if recorder.Header().Get("RateLimit-Remaining") != "0" {
		t.Errorf("RateLimit-Remaining = %q, want 0", recorder.Header().Get("RateLimit-Remaining"))
	}
	// The media type, ignoring the charset parameter WriteProblem appends.
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/problem+json") {
		t.Errorf("Content-Type = %q, want the RFC 9457 media type", contentType)
	}
}

// TestRateLimit_ASignedInCallerIsCountedPerAccountNotPerAddress is what stops a
// university or an office sharing one budget between everybody behind the NAT.
func TestRateLimit_ASignedInCallerIsCountedPerAccountNotPerAddress(t *testing.T) {
	limiter := newCountingLimiter()
	handler := &okHandler{}
	guarded := httpx.RateLimit(httpx.RateLimitConfig{
		Limiter:            limiter,
		AnonymousPerMinute: 1,
		UserPerMinute:      4,
	}).Middleware(handler)

	first := httpx.Actor{UserID: uuid.New(), SessionID: uuid.New()}
	second := httpx.Actor{UserID: uuid.New(), SessionID: uuid.New()}

	// Two accounts, one address. Each gets its own budget, and neither is
	// charged to the one-per-minute anonymous class.
	for i := range 4 {
		for _, actor := range []httpx.Actor{first, second} {
			recorder := send(t, guarded, http.MethodGet, "/api/v1/me", "203.0.113.50:5000", &actor)
			if recorder.Code != http.StatusOK {
				t.Fatalf("request %d for %s was refused with %d", i, actor.UserID, recorder.Code)
			}
		}
	}

	// And the fifth exhausts one account without touching the other.
	exhausted := send(t, guarded, http.MethodGet, "/api/v1/me", "203.0.113.50:5000", &first)
	if exhausted.Code != http.StatusTooManyRequests {
		t.Fatalf("the request over the per-user budget returned %d, want 429", exhausted.Code)
	}
}

// TestRateLimit_TheCredentialClassCountsTheAddressAndTheAccountSeparately is
// API_GUIDELINE §11's "5/min per IP **and** per account". Either one alone
// leaves a hole: per-IP only lets a botnet spread a credential-stuffing run
// across addresses, and per-account only lets one address work through a list of
// accounts.
func TestRateLimit_TheCredentialClassCountsTheAddressAndTheAccountSeparately(t *testing.T) {
	limiter := newCountingLimiter()
	guarded := httpx.RateLimit(httpx.RateLimitConfig{
		Limiter:             limiter,
		CredentialPerMinute: 5,
	}).Middleware(&okHandler{})

	actor := httpx.Actor{UserID: uuid.New(), SessionID: uuid.New()}
	send(t, guarded, http.MethodPost, "/api/v1/auth/change-password", "203.0.113.77:5000", &actor)

	var sawIP, sawAccount bool
	for _, key := range limiter.keysSeen() {
		if containsAll(key, "credential", "ip") {
			sawIP = true
		}
		if containsAll(key, "credential", actor.UserID.String()) {
			sawAccount = true
		}
	}
	if !sawIP {
		t.Error("the credential class charged no per-address counter")
	}
	if !sawAccount {
		t.Error("the credential class charged no per-account counter")
	}
}

func containsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}

// TestRouter_TheProbesAreNotRateLimited is why the middleware is mounted inside
// the group the modules register into rather than on the root router.
//
// `/health` and `/ready` are what the orchestrator polls, from one address, as
// often as it likes. An instance that answers 429 to a liveness probe is an
// instance that gets killed for being busy — and it gets killed during exactly
// the traffic spike that made it busy, which is when it is least replaceable.
func TestRouter_TheProbesAreNotRateLimited(t *testing.T) {
	limiter := newCountingLimiter()
	ok := func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusOK) }

	guard := httpx.RateLimit(httpx.RateLimitConfig{Limiter: limiter, AnonymousPerMinute: 2})
	router := httpx.NewRouter(httpx.RouterDependencies{
		Health:  ok,
		Ready:   ok,
		Version: ok,
		// Mounted the way cmd/api mounts it: inside the group the modules
		// register into, which is also where the auth middleware runs. The
		// infrastructure endpoints are registered outside it and stay outside.
		Modules: func(api chi.Router) {
			api.Group(func(limited chi.Router) {
				limited.Use(guard.Middleware)
				limited.Get("/limited", ok)
			})
		},
	})

	// Well past the two-per-minute budget, from one address.
	for _, path := range []string{"/health", "/ready", "/version"} {
		for i := range 5 {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.RemoteAddr = "203.0.113.200:5000"
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("%s probe %d returned %d; probes must never be limited", path, i, recorder.Code)
			}
		}
	}

	// A real endpoint from the same address still is.
	var lastCode int
	for range 5 {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/limited", nil)
		request.RemoteAddr = "203.0.113.200:5000"
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		lastCode = recorder.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Errorf("a versioned endpoint returned %d after exceeding the budget, want 429", lastCode)
	}
}
