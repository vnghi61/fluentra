package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/fluentra/fluentra/internal/modules/admin"
	"github.com/fluentra/fluentra/internal/modules/audit"
	"github.com/fluentra/fluentra/internal/modules/auth"
	authdomain "github.com/fluentra/fluentra/internal/modules/auth/domain"
	authservice "github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/modules/auth/service/oauth/google"
	"github.com/fluentra/fluentra/internal/modules/content"
	"github.com/fluentra/fluentra/internal/modules/learning"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	learningdomain "github.com/fluentra/fluentra/internal/modules/learning/domain"
	learningservice "github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/modules/lesson"
	lessonservice "github.com/fluentra/fluentra/internal/modules/lesson/service"
	"github.com/fluentra/fluentra/internal/modules/rbac"
	rbaccontract "github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/srs"
	srsservice "github.com/fluentra/fluentra/internal/modules/srs/service"
	"github.com/fluentra/fluentra/internal/modules/user"
	"github.com/fluentra/fluentra/internal/modules/vocabulary"
	vocabularycontract "github.com/fluentra/fluentra/internal/modules/vocabulary/contract"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/mailer"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/platform/telemetry"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// identity is WP1+WP2+WP4 assembled: the modules that know who a caller is and what
// they may do, and the record of what they did.
type identity struct {
	audit      *audit.Module
	rbac       *rbac.Module
	user       *user.Module
	auth       *auth.Module
	admin      *admin.Module
	content    *content.Module
	lesson     *lesson.Module
	learning   *learning.Module
	srs        *srs.Module
	vocabulary *vocabulary.Module

	rateLimit *httpx.RateLimiter
}

// identityDeps are the infrastructure the modules are built over.
type identityDeps struct {
	Pool       *pgxpool.Pool
	Redis      redis.Cmdable
	Cache      rbac.PermissionCache
	Limiter    cache.Limiter
	Storage    storage.Store
	Env        string
	OTPHMACKey []byte
	Mailer     mailer.Sender

	// Tokens is the JWT signing material from the JWT_* configuration keys.
	Tokens authservice.TokenConfig

	// RefreshTTL is REFRESH_TOKEN_TTL: the idle window a refresh token gets.
	RefreshTTL time.Duration

	// PasswordResetTTL is PASSWORD_RESET_TTL: how long a reset code lives.
	PasswordResetTTL time.Duration

	// IssuesPerIPPerHour and IssuesPerSubjectPerHour are OTP_ISSUE_PER_IP_PER_HOUR
	// and OTP_ISSUE_PER_SUBJECT_PER_HOUR. They are forwarded to the auth module
	// because the challenge service is what enforces them; the HTTP boundary
	// limiter gets the per-IP one separately and the two are not the same gate.
	IssuesPerIPPerHour      int
	IssuesPerSubjectPerHour int

	// Windows are the session lifetimes, from the SESSION_* keys.
	Windows authdomain.WindowConfig

	// RateLimit applies the classes in API_GUIDELINE.md §11. Nil disables it,
	// which is the same direction the middleware takes when its store is
	// unreachable: a missing limiter must not refuse everything.
	RateLimit *httpx.RateLimiter

	// Denylist backs the logout revocation list. It is a cache rather than a
	// table because ADR-0007 rejected server-side sessions specifically to keep
	// a datastore read off the authentication path — putting the denylist in
	// Postgres would put that read straight back.
	Denylist cache.Cache[bool]

	// Google is the OAuth provider configuration, from the OAUTH_GOOGLE_* keys.
	// A zero value is a provider with no credentials, which refuses every
	// exchange — the correct behaviour for a deployment that has not configured
	// Google sign-in.
	Google google.Options

	// OAuthStateTTL is OAUTH_STATE_TTL: how long an authorization request stays
	// completable.
	OAuthStateTTL time.Duration

	// Enqueuer schedules background River jobs within database transactions.
	Enqueuer job.Enqueuer

	// Instruments are the shared metric instruments, wired into the modules that
	// record to them (auth lockouts and refresh reuse).
	Instruments telemetry.Instruments
}

// newIdentity constructs the modules in dependency order — audit, then rbac,
// then user, then auth. That is the order MODULE_INDEX.md §3 draws the arrows
// in: `rbac` and `user` record into `audit`, and `auth` depends on `user`.
//
// There is one circle, and it is worth naming. `audit`'s own admin operations
// are guarded by `rbac`, while `rbac` records into `audit`. Today the second
// half is not a constructor argument — `rbac` writes outbox events in the same
// transaction as its business writes (rule L4) and the worker's consumer turns
// those into audit rows — so building rbac first would compile. It would also
// quietly stop compiling the day `rbac` takes a Recorder. The guard below
// resolves its authorizer at call time instead, which breaks the circle where
// it actually is rather than where it currently is not.
func newIdentity(deps identityDeps) *identity {
	assembled := &identity{rateLimit: deps.RateLimit}

	assembled.audit = audit.New(audit.Deps{
		Pool: deps.Pool,

		// Resolved on the first request, by which point rbac is built. It
		// cannot be called earlier: nothing serves until this function returns.
		Guard: lazyGuard{of: assembled},

		// IPHashKey is deliberately unset. The module stores a client address
		// only as a keyed HMAC, and stores nothing at all without a key — which
		// is the right state while there is no key to give it, because an
		// unkeyed digest of an IPv4 address is reversible by anyone willing to
		// hash four billion values. There is nothing to hash yet either: no
		// request carries an authenticated actor until P2.4. Tracked in
		// internal/modules/audit/TODO.md.
		IPHashKey: nil,
	})

	assembled.rbac = rbac.New(rbac.Deps{
		Pool:  deps.Pool,
		Cache: deps.Cache,
		Env:   deps.Env,
	})

	// `user` comes after `rbac` because every account it creates is granted the
	// baseline role in the same call. Without it, registration produced accounts
	// the access token called `user` and core.user_roles knew nothing about, and
	// the guard reads core.user_roles.
	assembled.user = user.New(user.Deps{
		Pool:     deps.Pool,
		Storage:  deps.Storage,
		Enqueuer: deps.Enqueuer,
		Roles:    assembled.rbac,
	})

	// `auth` comes after `user` because it holds a reference to user.Registrar.
	assembled.auth = auth.New(auth.Deps{
		Pool:                    deps.Pool,
		OTPHMACKey:              deps.OTPHMACKey,
		Mailer:                  deps.Mailer,
		Registrar:               assembled.user.Registrar(),
		Limiter:                 deps.Limiter,
		Tokens:                  deps.Tokens,
		Roles:                   assembled.rbac.RoleReader(),
		Denylist:                deps.Denylist,
		Env:                     deps.Env,
		RefreshTTL:              deps.RefreshTTL,
		PasswordResetTTL:        deps.PasswordResetTTL,
		IssuesPerIPPerHour:      deps.IssuesPerIPPerHour,
		IssuesPerSubjectPerHour: deps.IssuesPerSubjectPerHour,
		Windows:                 deps.Windows,
		Google:                  deps.Google,
		OAuthStateTTL:           deps.OAuthStateTTL,
		Telemetry:               deps.Instruments,
	})

	assembled.admin = admin.New(admin.Deps{
		Pool:           deps.Pool,
		UserReader:     assembled.user.AdminReader(),
		UserManager:    assembled.user.AdminManager(),
		SessionRevoker: assembled.auth.SessionRevoker(),
		Audit:          assembled.audit.Recorder(),
		Guard:          lazyGuard{of: assembled},
	})

	assembled.content = content.New(content.Deps{
		Pool:  deps.Pool,
		Guard: lazyGuard{of: assembled},
	})

	assembled.lesson = lesson.New(lesson.Deps{
		Pool:      deps.Pool,
		Caches:    newLessonCaches(deps.Redis),
		Guard:     lazyGuard{of: assembled},
		Content:   assembled.content.Reader(),
		Unlocker:  lazyUnlocker{of: assembled},
		Completed: lazyLessonProgress{of: assembled},
		Env:       deps.Env,
	})

	assembled.srs = srs.New(srs.Deps{
		Pool:    deps.Pool,
		Caches:  newSRSCaches(deps.Redis),
		Guard:   lazyGuard{of: assembled},
		Users:   assembled.user.Reader(),
		Content: assembled.content.Reader(),
		Env:     deps.Env,
	})

	assembled.vocabulary = vocabulary.New(vocabulary.Deps{
		Pool:    deps.Pool,
		Guard:   lazyGuard{of: assembled},
		Content: assembled.content.Reader(),
		Reviews: assembled.srs.CardWriter(),
	})

	assembled.learning = learning.New(learning.Deps{
		Pool:          deps.Pool,
		Caches:        newLearningCaches(deps.Redis),
		Guard:         lazyGuard{of: assembled},
		Lesson:        assembled.lesson.Reader(),
		SRSDue:        assembled.srs.QueueReader(),
		SRSCards:      assembled.srs.CardWriter(),
		Graders:       vocabularyGraders(assembled.vocabulary.Grader()),
		Metrics:       deps.Instruments,
		DeclaredKinds: vocabularycontract.GradedKinds(),
		Env:           deps.Env,
	})

	return assembled
}

// vocabularyGraders registers one grader under every kind it claims.
//
// The map and DeclaredKinds are built from the same list on purpose: a kind
// declared with no grader behind it fails the process at boot, and building the
// two from one source is what makes that impossible to do by accident.
func vocabularyGraders(grader learningcontract.ExerciseGrader) map[string]learningcontract.ExerciseGrader {
	graders := make(map[string]learningcontract.ExerciseGrader, len(vocabularycontract.GradedKinds()))
	for _, kind := range vocabularycontract.GradedKinds() {
		graders[kind] = grader
	}
	return graders
}

func newSRSCaches(client redis.Cmdable) srsservice.SRSCaches {
	if client == nil {
		return srsservice.SRSCaches{}
	}
	return srsservice.SRSCaches{
		DueCount: cache.NewRedisCache[int](client),
	}
}

func newLearningCaches(client redis.Cmdable) learningservice.LearningCaches {
	if client == nil {
		return learningservice.LearningCaches{}
	}
	return learningservice.LearningCaches{
		Dashboard: cache.NewRedisCache[*learningdomain.DashboardData](client),
		Progress:  cache.NewRedisCache[*learningdomain.ProgressData](client),
	}
}

func newLessonCaches(client redis.Cmdable) lessonservice.LessonCaches {
	if client == nil {
		return lessonservice.LessonCaches{}
	}
	return lessonservice.LessonCaches{
		Detail:    cache.NewRedisCache[*lessonservice.LessonDetailDTO](client),
		Tree:      cache.NewRedisCache[*lessonservice.CourseTreeData](client),
		Catalogue: cache.NewRedisCache[*lessonservice.CatalogueData](client),
		Gen:       cache.NewRedisCache[int64](client),
	}
}

// Routes mounts every module's operations under the caller's router.
//
// `rbac` mounts its own `/admin` group with the role guard inside it, and chi
// allows one handler per mount point — so `audit` registers its `/admin` paths
// on this router and they are wrapped here in the same guard. Both layers stay
// in force: the route group says "you are staff", and each handler still calls
// Require with the permission its own operation needs.
//
// The auth middleware wraps everything, including `/auth/*`. It has to: a
// request with no Authorization header passes through unauthenticated, so the
// public operations still work, while a request that presents a **bad** token
// anywhere is rejected with the reason rather than silently downgraded to
// anonymous. Mounting it selectively would mean every future route has to
// remember to opt in, and the one that forgets is the one that leaks.
// It is a Group rather than api.Use, and chi enforces the difference: a mux
// refuses middleware once it has routes, and this router already carries
// /ping, /health and /ready by the time it gets here. Those are the
// infrastructure endpoints, which must answer whether or not a caller has a
// token, so leaving them outside is correct as well as necessary.
func (i *identity) Routes(api chi.Router) {
	api.Group(func(authenticated chi.Router) {
		authenticated.Use(i.auth.Authenticate())

		// After Authenticate and not before it. The class a request belongs to
		// depends on whether there is an actor in the context: a signed-in
		// caller charged against the 60/min anonymous budget would be cut off
		// ten times sooner than API_GUIDELINE.md §11 promises, and — worse —
		// every learner behind one office NAT would share it.
		//
		// Inside this Group for the reason the Group exists at all: `/health`,
		// `/ready` and `/ping` are registered outside it. Those are what the
		// orchestrator polls, and an instance that answers 429 to a liveness
		// probe is an instance that gets killed for being busy.
		if i.rateLimit != nil {
			authenticated.Use(i.rateLimit.Middleware)
		}

		i.user.Routes(authenticated)
		i.rbac.Routes(authenticated)
		i.auth.Routes(authenticated)
		i.content.Routes(authenticated)
		i.lesson.Routes(authenticated)
		i.learning.Routes(authenticated)
		i.srs.Routes(authenticated)
		i.vocabulary.Routes(authenticated)

		authenticated.Group(func(admin chi.Router) {
			admin.Use(i.rbac.AdminOnly())
			i.audit.Routes(admin)
			i.admin.Routes(admin)
			i.content.AdminRoutes(admin)
			i.lesson.AdminRoutes(admin)
			i.vocabulary.AdminRoutes(admin)
		})
	})
}

// lazyGuard adapts rbac's Authorizer to the string-keyed guard `audit`
// declares, resolving it when the check runs rather than when the module is
// built.
//
// The string conversion is safe in the direction that matters: a name with no
// row in the permission catalogue is denied by the same code path that denies
// any other permission the actor lacks, so a typo fails closed.
type lazyGuard struct{ of *identity }

var _ audit.Guard = lazyGuard{}
var _ admin.Guard = lazyGuard{}
var _ content.Guard = lazyGuard{}
var _ lesson.Guard = lazyGuard{}
var _ learning.Guard = lazyGuard{}
var _ srs.Guard = lazyGuard{}
var _ vocabulary.Guard = lazyGuard{}

func (g lazyGuard) Require(ctx context.Context, permission string) error {
	return g.authorizer().Require(ctx, rbaccontract.Permission(permission))
}

func (g lazyGuard) authorizer() rbaccontract.Authorizer { return g.of.rbac.Authorizer() }

// lazyUnlocker adapts learning's batched UnlockChecker to lesson's consumer interface,
// resolving it when unlock checks run rather than on construction (P8.4 Trap 1).
type lazyUnlocker struct{ of *identity }

var _ lessonservice.UnlockChecker = lazyUnlocker{}

func (u lazyUnlocker) IsUnlocked(
	ctx context.Context, userID uuid.UUID, lessonIDs []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	if u.of.learning == nil {
		// Not a silent pass: a nil map here would read as "every lesson locked"
		// in GetCourseDetail, and a nil error would hide the wiring fault that
		// caused it. The composition root always builds learning, so this is a
		// programming error, and it says so.
		return nil, fmt.Errorf("learning module is not assembled")
	}
	return u.of.learning.UnlockChecker().IsUnlocked(ctx, userID, lessonIDs)
}

// lazyLessonProgress adapts learning's ProgressReader to the set of finished
// lessons the catalogue needs, resolved at call time for the same reason
// lazyUnlocker is.
//
// The fold lives here rather than in lesson because the scope enum and the
// Progress row belong to learning, and lesson is not permitted to know them.
// What crosses the seam is a set of ids, which is all the catalogue renders.
type lazyLessonProgress struct{ of *identity }

var _ lessonservice.CompletedLessons = lazyLessonProgress{}

func (p lazyLessonProgress) CompletedLessonIDs(
	ctx context.Context, userID uuid.UUID,
) (map[uuid.UUID]bool, error) {
	if p.of.learning == nil {
		return nil, fmt.Errorf("learning module is not assembled")
	}
	rows, err := p.of.learning.ProgressReader().ProgressOf(ctx, userID, learningcontract.ScopeLesson)
	if err != nil {
		return nil, err
	}
	completed := make(map[uuid.UUID]bool, len(rows))
	for _, row := range rows {
		if row.Status == learningStatusCompleted {
			completed[row.ScopeID] = true
		}
	}
	return completed, nil
}

// learningStatusCompleted is the one progress status the catalogue cares about.
// learning stores it as a string; comparing to a literal in three places is how
// a typo becomes a lesson that never shows a tick.
const learningStatusCompleted = "completed"

// rateLimiterAdapter bridges platform/cache's limiter to the one httpx declares.
//
// The two structs are identical field for field, and they are two structs
// because the shared kernel may not import a platform package — go-arch-lint
// enforces the direction, and caught the import when this card first tried it.
// The composition root is exactly where the seam belongs: it is the only place
// that is allowed to know about both.
type rateLimiterAdapter struct {
	inner cache.Limiter
}

func (a rateLimiterAdapter) Allow(
	ctx context.Context, key string, limit int, window time.Duration,
) (httpx.LimitResult, error) {
	result, err := a.inner.Allow(ctx, key, limit, window)
	return httpx.LimitResult{
		Allowed:   result.Allowed,
		Remaining: result.Remaining,
		ResetIn:   result.ResetIn,
		Degraded:  result.Degraded,
	}, err
}
