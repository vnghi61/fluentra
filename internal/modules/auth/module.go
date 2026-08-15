package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/auth/contract"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/repository"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/modules/auth/service/oauth/google"
	authhttp "github.com/fluentra/fluentra/internal/modules/auth/transport/http"
	rbaccontract "github.com/fluentra/fluentra/internal/modules/rbac/contract"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/mailer"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/id"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// Advisory lock IDs for scheduled work. They are module-local constants in a
// global namespace, spaced far from existing IDs — two jobs sharing an id
// silently never run together, and the symptom is a job that "sometimes fails
// to happen". Audit occupies 1_700_000_030..031; the next block starts at 040.
const (
	purgeUnverifiedLockID  int64 = 1_700_000_040
	sweepOAuthStatesLockID int64 = 1_700_000_041
)

// mailCategoryTransactional is what every message this module sends is: a
// direct consequence of something the recipient just did, never marketing.
const mailCategoryTransactional = "transactional"

// How often the sweep runs. Once per day is enough: unverified accounts age
// for seven days before they are eligible, so sub-day precision adds nothing.
const purgeUnverifiedInterval = 24 * time.Hour

// Deps are what the composition root supplies to this module.
//
// OTPHMACKey keys the HMAC that stores challenge codes and subject hashes.
// It is required: NewKeyring will return an error (and New will panic) if it is
// shorter than 32 bytes, because a silently weak key protects nothing.
//
// Mailer uses the platform interface so that tests and future provider swaps do
// not reach into the SMTP implementation — the same reason every other module
// declares consumer-side interfaces.
type Deps struct {
	Pool       *pgxpool.Pool
	Clock      clock.Clock
	OTPHMACKey []byte
	Mailer     mailer.Sender
	Registrar  usercontract.Registrar
	Limiter    cache.Limiter

	// Tokens is the JWT signing material. SigningKey is required; PreviousKey
	// is set only during a rotation and accepted-but-never-used-to-sign, which
	// is what stops a rotation signing everybody out.
	Tokens service.TokenConfig

	// Roles resolves the role a new token carries. `auth` asks `rbac` rather
	// than deciding: the arrow in MODULE_INDEX.md §3 points this way, and a
	// role the token layer invented would be a second source of truth for
	// authorisation.
	Roles rbaccontract.RoleReader

	// Denylist is the cache the logout revocation list lives in. Nil disables
	// revocation entirely — every token then works until it expires — so it is
	// nil only in tests that do not exercise logout.
	Denylist cache.Cache[bool]

	// Env namespaces the denylist keys, so a staging deploy pointed at a shared
	// Redis cannot sign somebody out of production, or fail to.
	//
	// It also decides whether the refresh cookie carries `Secure`. Only "local"
	// drops it, and only because the SPA is served over plain HTTP there and a
	// Secure cookie would be discarded rather than merely unprotected. Every
	// other value — including an empty one — gets the flag, so a deployment that
	// forgot to set APP_ENV fails safe.
	Env string

	// RefreshTTL is the idle window a refresh token gets (REFRESH_TOKEN_TTL).
	// Zero means service.DefaultRefreshTTL.
	RefreshTTL time.Duration

	// PasswordResetTTL is how long a reset code lives (PASSWORD_RESET_TTL).
	// Zero leaves the purpose on the shared OTP window.
	PasswordResetTTL time.Duration

	// Windows are the session lifetimes, from the SESSION_* keys. A zero field
	// falls back to the documented default rather than to immediate expiry.
	Windows domain.WindowConfig

	// Google is the provider configuration, from the OAUTH_GOOGLE_* keys. A zero
	// value builds a provider that cannot complete an exchange — every attempt
	// is refused by Google — which is the right failure for a deployment that
	// has not been given credentials, and is what tests that do not exercise
	// Google sign-in leave it as.
	Google google.Options

	// OAuthStateTTL is how long an authorization request stays completable
	// (OAUTH_STATE_TTL). Zero means service.DefaultOAuthStateTTL.
	OAuthStateTTL time.Duration

	// SessionCache remembers which account owns a session, for the five minutes
	// AGENT.md §12 documents. Nil disables the cache and every ownership check
	// reads Postgres, which is correct but chattier — it is nil in tests that do
	// not exercise it.
	SessionCache cache.Cache[uuid.UUID]
}

// Module is the auth module, assembled.
type Module struct {
	register *service.RegisterService
	login    *service.LoginService
	tokens   *service.TokenService
	sessions *service.RefreshService
	revoker  *service.SessionService
	oauth    *service.OAuthService
	mailer   mailer.Sender
	handler  *authhttp.Handler
}

// New wires the module. Reading down this function is the fastest way to see
// what the module is made of and what it depends on.
//
// It panics if OTPHMACKey is too short, because a process running with an
// invalid key is worse than one that refuses to start: it would issue challenges
// whose codes are trivially guessable.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	keys, err := domain.NewKeyring(deps.OTPHMACKey)
	if err != nil {
		panic("auth.New: " + err.Error())
	}

	// The repository implements both service.Repository (challenge operations)
	// and service.Credentials (password operations) — two surfaces of the same
	// concrete type. The adapters below narrow each to only the methods the
	// respective service interface declares, so neither service can accidentally
	// reach the other's data through a type assertion.
	repo := repository.New(deps.Pool)
	challengeRepo := repositoryAdapter{Repository: repo}
	credentialRepo := credentialAdapter{Repository: repo}

	// A nil limiter becomes one over a nil client, which allows every request
	// and warns — the same direction cache.RedisLimiter takes when its store is
	// unreachable. A composition root with no Redis serves traffic rather than
	// refusing all of it.
	limiter := deps.Limiter
	if limiter == nil {
		limiter = cache.NewRedisLimiter(nil)
	}

	challenges := service.NewChallengeService(service.ChallengeDeps{
		Repo: challengeRepo,

		// The per-subject issuance caps (BR-AUTH-13) have been coded against
		// this since P2.1b and doing nothing, because it was nil. It is the
		// same limiter the login lockout uses; sharing it is right, because
		// the two count different keys and neither should have its own
		// connection to the same Redis.
		Limiter: limiter,
		Keys:    keys,
		Clock:   timekeeper,
		NewID:   id.NewUUIDv7,
		Config: service.Config{
			// Only the purposes that differ from the ten-minute default are
			// named. A reset email can sit unread through a meeting in a way a
			// signup code typed on the next screen cannot (BR-AUTH-10).
			TTLByPurpose: map[service.Purpose]time.Duration{
				domain.PurposePasswordReset: deps.PasswordResetTTL,
			},
		},
		// Namespaced, so a staging deploy pointed at a shared Redis cannot
		// spend a production learner's issuance budget — or, worse, fail to.
		Env: deps.Env,
	})

	events := outboxWriter{Writer: outbox.NewWriter()}

	// BreachChecker is a separate type because it speaks net/http directly — not
	// via a platform facade — and co-locating it with the repository keeps the
	// HTTP client ownership obvious. A nil checker in domain.Policy means the
	// breach check is skipped (fail-open), which is correct for tests. In
	// production the checker is always constructed; there is no config flag to
	// disable it silently.
	breachChecker := repository.NewBreachChecker(repository.BreachCheckerOptions{})

	// The token service is built before the two that need it, and its failure
	// is fatal. A process that cannot sign tokens can register learners it can
	// never sign in, which is worse than not starting.
	tokens, err := service.NewTokenService(service.TokenDeps{
		Config:   deps.Tokens,
		Roles:    rolesAdapter{Reader: deps.Roles},
		Denylist: repository.NewTokenDenylist(deps.Denylist, deps.Env),
		Clock:    timekeeper,
		NewID:    id.NewUUIDv7,
	})
	if err != nil {
		panic("auth.New: " + err.Error())
	}

	// Sessions are opened here and nowhere else. Login and verification both
	// route through it so there is one place a refresh family is rooted, which
	// is what makes "revoke the family" a complete statement.
	sessions := service.NewRefreshService(service.RefreshDeps{
		Pool:    deps.Pool,
		Repo:    refreshAdapter{Repository: repo},
		Tokens:  tokens,
		Events:  events,
		Keys:    keys,
		Clock:   timekeeper,
		NewID:   id.NewUUIDv7,
		Roles:   rolesAdapter{Reader: deps.Roles},
		Windows: deps.Windows,
		TTL:     deps.RefreshTTL,
	})

	reg := service.NewRegisterService(service.RegisterDeps{
		Pool:        deps.Pool,
		Accounts:    accountsAdapter{Registrar: deps.Registrar},
		Credentials: credentialRepo,
		Challenges:  challenges,
		Hasher:      domain.NewHasher(domain.DefaultHashParams()),
		Policy:      domain.Policy{Breaches: breachChecker},
		Events:      events,
		Clock:       timekeeper,
		NewID:       id.NewUUIDv7,
		Sessions:    sessions,
	})

	loginSvc := service.NewLoginService(service.LoginDeps{
		Accounts:    accountsAdapter{Registrar: deps.Registrar},
		Credentials: credentialRepo,
		Repo:        repo,
		Limiter:     limiter,
		Keys:        keys,
		Hasher:      domain.NewHasher(domain.DefaultHashParams()),
		Clock:       timekeeper,
		NewID:       id.NewUUIDv7,
		Sessions:    sessions,
	})

	// The session service shares the repository and the token service with the
	// two above it: revoking a session has to reach the same refresh rows the
	// refresh service writes, and denylisting an access token has to reach the
	// same token service that issued it.
	sessionSvc := service.NewSessionService(service.SessionDeps{
		Pool:   deps.Pool,
		Repo:   sessionAdapter{Repository: repo},
		Tokens: tokens,
		Cache:  repository.NewSessionOwnerCache(deps.SessionCache, deps.Env),
		Clock:  timekeeper,
		Env:    deps.Env,
	})

	passwordSvc := service.NewPasswordService(service.PasswordDeps{
		Pool:        deps.Pool,
		Accounts:    accountsAdapter{Registrar: deps.Registrar},
		Credentials: credentialRepo,
		Challenges:  challenges,
		Sessions:    sessionSvc,
		Hasher:      domain.NewHasher(domain.DefaultHashParams()),
		Policy:      domain.Policy{Breaches: breachChecker},
		Events:      events,
		Clock:       timekeeper,
		NewID:       id.NewUUIDv7,
	})

	deviceSvc := service.NewDeviceService(service.DeviceDeps{
		Pool:  deps.Pool,
		Repo:  deviceAdapter{Repository: repo},
		Clock: timekeeper,
	})

	// The OAuth service shares `sessions` with login and verification, so a
	// Google sign-in roots a refresh family exactly as the other two do — which
	// is what makes "revoke every session" a complete statement regardless of
	// how the learner got in.
	//
	// It shares the keyring too, and that is load-bearing rather than
	// incidental: the same key derives the PKCE verifier from the state at both
	// ends of the flow, so a deployment running two different OTP_HMAC_KEYs
	// behind one load balancer would fail every Google sign-in that changed
	// instance mid-flow. The key is already required to be identical across
	// instances for the OTP challenges to work at all.
	oauthSvc := service.NewOAuthService(service.OAuthDeps{
		Pool:     deps.Pool,
		Repo:     oauthAdapter{Repository: repo},
		Provider: google.New(deps.Google),
		Accounts: accountsAdapter{Registrar: deps.Registrar},
		Sessions: sessions,
		Events:   events,
		Keys:     keys,
		Clock:    timekeeper,
		NewID:    id.NewUUIDv7,
		StateTTL: deps.OAuthStateTTL,
	})

	return &Module{
		register: reg,
		login:    loginSvc,
		tokens:   tokens,
		sessions: sessions,
		revoker:  sessionSvc,
		mailer:   deps.Mailer,
		oauth:    oauthSvc,
		handler: authhttp.NewHandler(reg, loginSvc, sessions, sessionSvc, passwordSvc, deviceSvc, oauthSvc,
			authhttp.CookieOptions{Secure: deps.Env != "local"}),
	}
}

// Routes mounts this module's HTTP operations on router, relative to /api/v1.
func (m *Module) Routes(router chi.Router) { m.handler.Routes(router) }

// TokenVerifier is this module's contract for validating an access token.
func (m *Module) TokenVerifier() contract.TokenVerifier { return m.tokens }

// SessionRevoker is this module's contract for ending every session an account
// has. `user` calls it on account deletion and `admin` on suspension; neither
// module exists yet, so today it is published and unconsumed.
func (m *Module) SessionRevoker() contract.SessionRevoker { return m.revoker }

// Authenticate is the middleware that turns a bearer token into an actor in the
// request context.
//
// It is safe to mount over the whole API: a request with no Authorization
// header passes through unauthenticated, so `/auth/register` and `/auth/login`
// still work, and handlers that need a caller already refuse without one.
func (m *Module) Authenticate() func(http.Handler) http.Handler {
	return authhttp.Authenticate(m.tokens)
}

// rolesAdapter narrows rbac's RoleReader to the single question the token
// service asks. Doing the "which of these is highest" reduction here rather
// than in the service keeps `auth/service` free of rbac's types, and keeps the
// answer in the module that knows there are exactly two roles.
type rolesAdapter struct {
	Reader rbaccontract.RoleReader
}

func (a rolesAdapter) RoleOf(ctx context.Context, userID uuid.UUID) (string, error) {
	if a.Reader == nil {
		return domain.RoleUser, nil
	}
	roles, err := a.Reader.RolesOf(ctx, userID)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, string(role))
	}
	return domain.HighestRole(names), nil
}

// Subscribe registers the event consumers this module runs in the worker.
//
// Two topics arrive here:
//   - auth.verification_requested  — deliver the OTP code to the registrant.
//   - auth.registration_attempted  — deliver the warning to the existing owner.
//
// Both are fire-and-forget in the direction that matters: the outbox row was
// committed in the same transaction as the business write, so the 201 has
// already been returned by the time this consumer is called. A mailer outage
// does not fail registration; it delays delivery until the outbox retries.
func (m *Module) Subscribe(bus eventbus.EventBus) error {
	for _, topic := range []string{
		contract.EventVerificationRequested,
		contract.EventRegistrationAttempted,
		contract.EventPasswordResetRequested,
	} {
		if err := bus.Subscribe(topic, m.handleMailEvent); err != nil {
			return fmt.Errorf("subscribe auth consumer to %s: %w", topic, err)
		}
	}
	return nil
}

// handleMailEvent is the single dispatcher for both mail topics. Keeping both
// paths here rather than in separate handlers avoids duplicating the JSON
// decode and error-log wiring.
func (m *Module) handleMailEvent(ctx context.Context, msg eventbus.Message) error {
	switch msg.Topic {
	case contract.EventVerificationRequested:
		return m.sendVerificationEmail(ctx, msg.Payload)
	case contract.EventRegistrationAttempted:
		return m.sendRegistrationAttemptEmail(ctx, msg.Payload)
	case contract.EventPasswordResetRequested:
		return m.sendPasswordResetEmail(ctx, msg.Payload)
	default:
		// A topic we were subscribed to but do not recognise. Log and return nil
		// rather than blocking redelivery — an unknown topic today is most likely
		// an event written by a newer binary, not a condition worth crashing on.
		slog.WarnContext(ctx, "auth mailer consumer received unknown topic",
			"module", "auth", "topic", msg.Topic)
		return nil
	}
}

// sendVerificationEmail delivers the OTP code to the address the account
// registered with. The code is in the outbox payload because the challenge row
// stores only an HMAC and there is nowhere else to recover it from after the
// fact. See the comment on contract.VerificationRequested for the security
// trade-off and where the prune fix lives.
func (m *Module) sendVerificationEmail(ctx context.Context, payload []byte) error {
	var event contract.VerificationRequested
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("decode auth.verification_requested payload: %w", err)
	}

	return m.mailer.Send(ctx, mailer.Message{
		To:       event.Email,
		Template: "verify_email",
		Locale:   event.Locale,
		Data: map[string]any{
			"DisplayName": event.DisplayName,
			"Code":        event.Code,
		},
		Category: mailCategoryTransactional,
	})
}

// sendRegistrationAttemptEmail warns the address owner that somebody tried to
// register with their already-verified address (BR-AUTH-14).
//
// It carries no code and no challenge id: the message exists to inform, not to
// ask the recipient to do anything. The right response is "someone else wanted
// this address — keep my account; ignore this."
func (m *Module) sendRegistrationAttemptEmail(ctx context.Context, payload []byte) error {
	var event contract.RegistrationAttempted
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("decode auth.registration_attempted payload: %w", err)
	}

	return m.mailer.Send(ctx, mailer.Message{
		To:       event.Email,
		Template: "registration_attempt",
		Locale:   event.Locale,
		Data: map[string]any{
			"Email": event.Email,
		},
		Category: mailCategoryTransactional,
	})
}

// sendPasswordResetEmail delivers the reset code. The code is in the payload
// for the reason the verification one is: the challenge row stores only an HMAC,
// so by the time this consumer runs it cannot be recovered from anywhere else.
func (m *Module) sendPasswordResetEmail(ctx context.Context, payload []byte) error {
	var event contract.PasswordResetRequested
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("decode auth.password_reset_requested payload: %w", err)
	}

	return m.mailer.Send(ctx, mailer.Message{
		To:       event.Email,
		Template: "reset",
		Locale:   event.Locale,
		Data: map[string]any{
			"DisplayName": event.DisplayName,
			"Code":        event.Code,
		},
		Category: mailCategoryTransactional,
	})
}

// CronJobs returns the scheduled work this module owns, for the worker to
// register. The sweep is the enforcement half of BR-AUTH-14's promise that a
// claimed address is not permanently blocked — after seven days without
// verification the claim lapses and the address is free again.
func (m *Module) CronJobs() []job.CronJob {
	return []job.CronJob{
		{
			Name:     "auth.purge_unverified",
			LockID:   purgeUnverifiedLockID,
			Interval: purgeUnverifiedInterval,
			Task:     m.register.PurgeUnverified,
		},
		{
			// Abandoned authorization requests. An expired state is already
			// refused, so this reclaims space rather than enforcing anything —
			// but most rows in that table are consent screens nobody finished,
			// and without a sweep it only ever grows.
			Name:     "auth.sweep_oauth_states",
			LockID:   sweepOAuthStatesLockID,
			Interval: sweepOAuthStatesInterval,
			Task:     m.oauth.SweepOAuthStates,
		},
	}
}

// sweepOAuthStatesInterval is hourly, which is often enough that the table
// tracks the last hour of abandoned flows rather than the last day.
const sweepOAuthStatesInterval = time.Hour

// refreshAdapter narrows *repository.Repository to service.RefreshRepo, for the
// same covariance reason as the two adapters below it.
type refreshAdapter struct {
	*repository.Repository
}

func (a refreshAdapter) WithTx(tx pgx.Tx) service.RefreshRepo {
	return refreshAdapter{Repository: a.Repository.WithTx(tx)}
}

// deviceAdapter narrows *repository.Repository to service.DeviceRepo.
type deviceAdapter struct {
	*repository.Repository
}

func (a deviceAdapter) WithTx(tx pgx.Tx) service.DeviceRepo {
	return deviceAdapter{Repository: a.Repository.WithTx(tx)}
}

// oauthAdapter narrows *repository.Repository to service.OAuthRepo.
type oauthAdapter struct {
	*repository.Repository
}

func (a oauthAdapter) WithTx(tx pgx.Tx) service.OAuthRepo {
	return oauthAdapter{Repository: a.Repository.WithTx(tx)}
}

// sessionAdapter narrows *repository.Repository to service.SessionRepo.
type sessionAdapter struct {
	*repository.Repository
}

func (a sessionAdapter) WithTx(tx pgx.Tx) service.SessionRepo {
	return sessionAdapter{Repository: a.Repository.WithTx(tx)}
}

// repositoryAdapter narrows *repository.Repository to service.Repository (the
// challenge interface). Go has no covariant return types, so WithTx returning
// *Repository cannot satisfy a method returning service.Repository — these four
// lines bridge that.
type repositoryAdapter struct {
	*repository.Repository
}

func (a repositoryAdapter) WithTx(tx pgx.Tx) service.Repository {
	return repositoryAdapter{Repository: a.Repository.WithTx(tx)}
}

// credentialAdapter narrows *repository.Repository to service.Credentials. A
// separate adapter keeps the two surfaces independent: the challenge service
// cannot reach credential methods through its Repository interface, and the
// register service cannot reach challenge methods through its Credentials one.
type credentialAdapter struct {
	*repository.Repository
}

func (a credentialAdapter) WithTx(tx pgx.Tx) service.Credentials {
	return credentialAdapter{Repository: a.Repository.WithTx(tx)}
}

// outboxWriter adapts shared/outbox to service.EventWriter. The method
// signatures already match; the adapter exists because the service declares its
// own OutboxTx interface rather than importing the outbox package's — which is
// what lets the service be tested without a real transaction.
type outboxWriter struct {
	*outbox.Writer
}

func (w outboxWriter) Write(
	ctx context.Context, tx service.OutboxTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	return w.Writer.Write(ctx, outboxTx{tx}, aggregate, event, payload)
}

type outboxTx struct{ inner service.OutboxTx }

func (t outboxTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return t.inner.Exec(ctx, sql, arguments...)
}

// accountsAdapter adapts user/contract.Registrar to service.Accounts.
//
// The service declares its own Account and Contact types (not the contract
// ones) so it can be tested without the user module. Both sets of types carry
// the same fields, and this adapter is where the two spellings meet.
type accountsAdapter struct {
	Registrar usercontract.Registrar
}

func (a accountsAdapter) CreateAccount(ctx context.Context, input service.NewAccount) (uuid.UUID, error) {
	return a.Registrar.CreateUser(ctx, usercontract.NewUser{
		Email:       input.Email,
		DisplayName: input.DisplayName,
		Locale:      input.Locale,
		Timezone:    input.Timezone,
	})
}

func (a accountsAdapter) FindByEmail(ctx context.Context, email string) (service.Account, bool, error) {
	account, found, err := a.Registrar.FindByEmail(ctx, email)
	if err != nil || !found {
		return service.Account{}, found, err
	}
	return service.Account{
		ID:       account.ID,
		Verified: account.Verified,
		Status:   account.Status,
	}, true, nil
}

func (a accountsAdapter) Recipient(ctx context.Context, userID uuid.UUID) (service.Contact, error) {
	contact, err := a.Registrar.Recipient(ctx, userID)
	if err != nil {
		return service.Contact{}, err
	}
	return service.Contact{
		Email:       contact.Email,
		DisplayName: contact.DisplayName,
		Locale:      contact.Locale,
	}, nil
}

func (a accountsAdapter) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	return a.Registrar.MarkEmailVerified(ctx, userID)
}

func (a accountsAdapter) PurgeUnverifiedBefore(ctx context.Context, cutoff time.Time) (int, error) {
	return a.Registrar.PurgeUnverifiedBefore(ctx, cutoff)
}
