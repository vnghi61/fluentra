package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	authhttp "github.com/fluentra/fluentra/internal/modules/auth/transport/http"
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
	purgeUnverifiedLockID int64 = 1_700_000_040
)

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
}

// Module is the auth module, assembled.
type Module struct {
	register *service.RegisterService
	login    *service.LoginService
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

	challenges := service.NewChallengeService(service.ChallengeDeps{
		Repo:    challengeRepo,
		Limiter: nil, // rate limiter arrives in P2.8; nil degrades to allow-all
		Keys:    keys,
		Clock:   timekeeper,
		NewID:   id.NewUUIDv7,
		Env:     "", // namespacing arrives with config in P2.8
	})

	events := outboxWriter{Writer: outbox.NewWriter()}

	// BreachChecker is a separate type because it speaks net/http directly — not
	// via a platform facade — and co-locating it with the repository keeps the
	// HTTP client ownership obvious. A nil checker in domain.Policy means the
	// breach check is skipped (fail-open), which is correct for tests. In
	// production the checker is always constructed; there is no config flag to
	// disable it silently.
	breachChecker := repository.NewBreachChecker(repository.BreachCheckerOptions{})

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
	})

	limiter := deps.Limiter
	if limiter == nil {
		limiter = cache.NewRedisLimiter(nil)
	}

	loginSvc := service.NewLoginService(service.LoginDeps{
		Accounts:    accountsAdapter{Registrar: deps.Registrar},
		Credentials: credentialRepo,
		Repo:        repo,
		Limiter:     limiter,
		Keys:        keys,
		Hasher:      domain.NewHasher(domain.DefaultHashParams()),
		Clock:       timekeeper,
		NewID:       id.NewUUIDv7,
	})

	return &Module{
		register: reg,
		login:    loginSvc,
		mailer:   deps.Mailer,
		handler:  authhttp.NewHandler(reg, loginSvc),
	}
}

// Routes mounts this module's HTTP operations on router, relative to /api/v1.
func (m *Module) Routes(router chi.Router) { m.handler.Routes(router) }

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
		Category: "transactional",
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
		Category: "transactional",
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
	}
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
