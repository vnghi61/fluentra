package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// The default budgets, mirroring the RATE_LIMIT_* keys in `.env.example` and
// the table in API_GUIDELINE.md §11.
const (
	DefaultAnonymousPerMinute  = 60
	DefaultUserPerMinute       = 600
	DefaultCredentialPerMinute = 5
	DefaultUploadPerHour       = 30
	DefaultChallengeIPPerHour  = 20
)

// rateLimitVersion is the `v1` in the key. Bump it if the counting changes, so
// a deploy cannot read an old window as a new one.
const rateLimitVersion = "v1"

// LimitResult is one budget's answer.
//
// It mirrors `platform/cache.LimitResult` field for field and is declared here
// rather than imported, because the shared kernel sits below the platform
// packages and go-arch-lint enforces it — which it did, on the first run of this
// card. The composition root adapts one to the other in four lines; the
// alternative is an edge in `.go-arch-lint.yml` that inverts the layering for
// the convenience of one struct.
type LimitResult struct {
	Allowed   bool
	Remaining int
	ResetIn   time.Duration
	// Degraded reports that the limit was not actually evaluated because the
	// backing store was unreachable, so nothing derived from it should be
	// advertised to a client.
	Degraded bool
}

// Limiter is the counting surface this middleware needs, declared by the
// consumer for the reason above.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (LimitResult, error)
}

// RateLimitConfig is the budget for each class.
//
// Every field is optional and falls back to the documented default, so a
// partially-populated config cannot silently produce a limit of zero — which
// would refuse everything, and is the one value that must never be reachable by
// forgetting to set something.
type RateLimitConfig struct {
	Limiter Limiter

	AnonymousPerMinute  int
	UserPerMinute       int
	CredentialPerMinute int
	UploadPerHour       int

	// ChallengeIPPerHour caps how many challenges one address can cause,
	// across every subject it names. The per-subject cap lives in the auth
	// module, next to the code that knows what a subject is; this is the half
	// that has to live at the boundary, because only here is the address known
	// before any handler has run.
	ChallengeIPPerHour int

	// Env namespaces the keys, so a staging deploy pointed at a shared Redis
	// cannot spend a production caller's budget.
	Env string
}

func (c RateLimitConfig) withDefaults() RateLimitConfig {
	if c.AnonymousPerMinute <= 0 {
		c.AnonymousPerMinute = DefaultAnonymousPerMinute
	}
	if c.UserPerMinute <= 0 {
		c.UserPerMinute = DefaultUserPerMinute
	}
	if c.CredentialPerMinute <= 0 {
		c.CredentialPerMinute = DefaultCredentialPerMinute
	}
	if c.UploadPerHour <= 0 {
		c.UploadPerHour = DefaultUploadPerHour
	}
	if c.ChallengeIPPerHour <= 0 {
		c.ChallengeIPPerHour = DefaultChallengeIPPerHour
	}
	return c
}

// RateLimiter applies the classes in API_GUIDELINE.md §11.
type RateLimiter struct {
	config RateLimitConfig
}

// RateLimit builds the middleware.
func RateLimit(config RateLimitConfig) *RateLimiter {
	return &RateLimiter{config: config.withDefaults()}
}

// bucket is one budget a request is charged against. A request can be charged
// against more than one — the credential class counts the address and the
// account separately, because either alone leaves a hole.
type bucket struct {
	key    string
	limit  int
	window time.Duration
}

// Middleware counts the request and refuses it when any of its budgets is spent.
//
// A nil limiter disables the whole thing rather than refusing everything, which
// is the same direction as the outage case below and is what lets a test or a
// composition root that has no Redis still serve requests.
func (r *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if r.config.Limiter == nil {
			next.ServeHTTP(writer, request)
			return
		}

		ctx := request.Context()
		buckets := r.bucketsFor(request)

		// The tightest answer wins, and every bucket is charged even when an
		// earlier one already refused. Stopping at the first refusal would let
		// a caller who has exhausted the cheap per-IP budget stop accruing
		// against the expensive per-account one, and then come back from a new
		// address with a full allowance.
		var (
			refused       *LimitResult
			tightest      *LimitResult
			tightestLimit int
		)
		for _, charged := range buckets {
			result, err := r.config.Limiter.Allow(ctx, charged.key, charged.limit, charged.window)
			if err != nil || result.Degraded {
				// Allow. A limiter that denies when its backing store is down
				// turns a cache outage into a total outage of every endpoint
				// for every caller, and the rate limit protects far less than
				// availability costs (API_GUIDELINE.md §11).
				//
				// Nothing is advertised either: a RateLimit-Remaining derived
				// from a budget that was never evaluated is a number a client
				// would pace itself against, and it would be fiction.
				//
				// Nothing is logged here either. `cache.RedisLimiter` already
				// warns and increments `cache_unavailable_total` on the way
				// out, and a second line per request would double the noise of
				// an outage in the logs that are being read to diagnose it.
				// A malformed reply is a different matter — that is a bug in
				// this stack rather than an outage, and it arrives as a real
				// error with Degraded unset, so it is worth one line.
				if err != nil && !result.Degraded {
					slog.WarnContext(ctx, "rate limiter returned an error, allowing the request",
						"module", "httpx", "op", "RateLimit", "error", err)
				}
				next.ServeHTTP(writer, request)
				return
			}

			outcome := result
			if !outcome.Allowed && refused == nil {
				refused = &outcome
				tightestLimit = charged.limit
			}
			if tightest == nil || outcome.Remaining < tightest.Remaining {
				tightest = &outcome
				if refused == nil {
					tightestLimit = charged.limit
				}
			}
		}

		if refused != nil {
			r.writeHeaders(writer, tightestLimit, *refused)
			WriteProblem(writer, request, apperr.New(
				apperr.RateLimited, "RATE_LIMITED", "Too many requests. Retry after the indicated interval."))
			return
		}
		if tightest != nil {
			r.writeHeaders(writer, tightestLimit, *tightest)
		}
		next.ServeHTTP(writer, request)
	})
}

// writeHeaders advertises the budget the request was charged against.
//
// They are set before the handler runs, because a handler that has already
// written its status cannot have headers added afterwards — and on the refusal
// path they must reach the client alongside the 429 rather than instead of it.
func (r *RateLimiter) writeHeaders(writer http.ResponseWriter, limit int, result LimitResult) {
	header := writer.Header()
	header.Set("RateLimit-Limit", strconv.Itoa(limit))
	header.Set("RateLimit-Remaining", strconv.Itoa(max(result.Remaining, 0)))

	seconds := int(result.ResetIn.Seconds())
	if seconds < 1 {
		// Never zero. A client reading "reset in 0" retries immediately and
		// spends the next window's budget on the same burst.
		seconds = 1
	}
	header.Set("RateLimit-Reset", strconv.Itoa(seconds))
	if !result.Allowed {
		header.Set("Retry-After", strconv.Itoa(seconds))
	}
}

// bucketsFor decides which budgets this request is charged against.
//
// The classes are mutually exclusive in the sense that a request belongs to one
// of them, but a class may charge more than one counter — see the credential
// class, which counts the address and the account separately.
func (r *RateLimiter) bucketsFor(request *http.Request) []bucket {
	ctx := request.Context()
	address := clientAddress(ctx)
	actor, signedIn := ActorFrom(ctx)

	path := request.URL.Path
	switch {
	case isChallengeIssuing(path):
		// Charged against the address alone. The per-subject cap is the auth
		// module's, and the two are independent on purpose: three per address
		// stops somebody hammering one inbox, and this stops a script asking
		// for one each against ten thousand different ones.
		return []bucket{{
			key:    r.key("challenge", "ip", address),
			limit:  r.config.ChallengeIPPerHour,
			window: time.Hour,
		}}

	case isCredentialPath(path):
		// "5/min per IP **and** per account" (API_GUIDELINE.md §11). Per-IP
		// alone lets a botnet spread a credential-stuffing run across
		// addresses; per-account alone lets one address work through a list of
		// accounts. Both, or neither is worth much.
		charged := []bucket{{
			key:    r.key("credential", "ip", address),
			limit:  r.config.CredentialPerMinute,
			window: time.Minute,
		}}
		if signedIn {
			charged = append(charged, bucket{
				key:    r.key("credential", "user", actor.UserID.String()),
				limit:  r.config.CredentialPerMinute,
				window: time.Minute,
			})
		}
		return charged

	case isUploadPath(path):
		return []bucket{{
			key:    r.key("upload", "user", subjectOf(actor, signedIn, address)),
			limit:  r.config.UploadPerHour,
			window: time.Hour,
		}}

	case signedIn:
		// Per account and not per address, so an office or a university behind
		// one NAT does not share a single budget between everybody in it.
		return []bucket{{
			key:    r.key("user", "user", actor.UserID.String()),
			limit:  r.config.UserPerMinute,
			window: time.Minute,
		}}

	default:
		return []bucket{{
			key:    r.key("anon", "ip", address),
			limit:  r.config.AnonymousPerMinute,
			window: time.Minute,
		}}
	}
}

func (r *RateLimiter) key(class, scope, identifier string) string {
	return strings.Join([]string{"fluentra", r.config.Env, "ratelimit", class, scope, identifier, rateLimitVersion}, ":")
}

// clientAddress reads the resolved address, falling back to a shared bucket.
//
// The fallback is deliberate and it is the strict direction: a request whose
// address could not be resolved is counted against one shared counter, so a
// caller cannot escape the limit by arranging to be unidentifiable.
func clientAddress(ctx context.Context) string {
	if address := ClientIP(ctx); address.IsValid() {
		return address.String()
	}
	return "unknown"
}

func subjectOf(actor Actor, signedIn bool, address string) string {
	if signedIn {
		return actor.UserID.String()
	}
	return address
}

// The credential operations: everything that hands out, rotates or replaces a
// way into an account. Matched by suffix on the routed path rather than by
// listing full paths, so a version prefix change does not silently unprotect
// them.
var credentialSuffixes = []string{
	"/auth/login",
	"/auth/register",
	"/auth/forgot-password",
	"/auth/reset-password",
	"/auth/change-password",
	"/auth/refresh",
}

func isCredentialPath(path string) bool {
	for _, suffix := range credentialSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

// isChallengeIssuing matches the operations that cause an email to be sent to an
// address the caller chose. Those are the ones a script uses to make Fluentra
// spam a list, and they are capped per address rather than per credential.
func isChallengeIssuing(path string) bool {
	return strings.HasSuffix(path, "/auth/register") ||
		strings.HasSuffix(path, "/auth/forgot-password") ||
		strings.HasSuffix(path, "/resend")
}

func isUploadPath(path string) bool {
	return strings.Contains(path, "/uploads") || strings.HasSuffix(path, "/avatar")
}
