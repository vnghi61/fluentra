// Package service holds the user module's use cases: the orchestration,
// transactions and event publishing that sit between the HTTP layer and the
// database. It knows nothing about HTTP types and writes no SQL.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository is the persistence surface this service needs.
//
// It is declared here, by the consumer, rather than imported from the
// repository package. That is what lets the service be tested against a fake
// with no database, and it keeps the dependency pointing inward: the
// repository satisfies the service's interface, not the other way round.
type Repository interface {
	GetUser(ctx context.Context, id uuid.UUID) (domain.User, error)
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	CreateUser(ctx context.Context, id uuid.UUID, email string, status domain.Status) (domain.User, error)

	GetProfile(ctx context.Context, userID uuid.UUID) (domain.Profile, error)
	CreateProfile(ctx context.Context, id, userID uuid.UUID, profile domain.Profile) (domain.Profile, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, change domain.ProfileChange) (domain.Profile, error)

	GetPreferences(ctx context.Context, userID uuid.UUID) (domain.Preferences, error)
	CreatePreferences(ctx context.Context, id, userID uuid.UUID) (domain.Preferences, error)
	ReplacePreferences(ctx context.Context, preferences domain.Preferences) (domain.Preferences, error)

	GetSummary(ctx context.Context, id uuid.UUID) (domain.Summary, error)
	ListSummaries(ctx context.Context, ids []uuid.UUID) ([]domain.Summary, error)

	// The registration lifecycle, for contract.Registrar. `auth` owns no user
	// table and rule L2 forbids it reading one, so each of these exists to
	// replace a cross-schema join it would otherwise have to write.
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	MarkEmailVerified(ctx context.Context, userID uuid.UUID) (domain.User, error)
	PurgeUnverifiedBefore(ctx context.Context, cutoff time.Time) (int, error)

	WithTx(tx pgx.Tx) Repository
}

// OutboxTx is the transaction surface the outbox writer needs. Its shape
// matches shared/outbox.DBTx, so the real writer satisfies EventWriter without
// an adapter.
type OutboxTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// EventWriter records a domain event in the same transaction as the write that
// caused it. Rule L4: no transaction spans two modules, so `audit` learns what
// happened by consuming the outbox rather than by being called from here.
type EventWriter interface {
	Write(ctx context.Context, tx OutboxTx, aggregate, event string, payload any) (uuid.UUID, error)
}

// IDGenerator produces the identifiers new rows are written with. It is a
// dependency rather than a direct call into shared/id so a test can make ids
// deterministic without also having to control the clock.
type IDGenerator func(ctx context.Context) (uuid.UUID, error)

// Service implements the user module's use cases.
type Service struct {
	pool   dbx.Beginner
	repo   Repository
	events EventWriter
	clock  clock.Clock
	ids    IDGenerator
}

// Deps are the service's collaborators.
type Deps struct {
	Pool   dbx.Beginner
	Repo   Repository
	Events EventWriter
	Clock  clock.Clock
	NewID  IDGenerator
}

// New creates the user service.
func New(deps Deps) *Service {
	return &Service{pool: deps.Pool, repo: deps.Repo, events: deps.Events, clock: deps.Clock, ids: deps.NewID}
}

// Account is everything `GET /me` renders: the identity record and the
// profile, read together.
type Account struct {
	User    domain.User
	Profile domain.Profile
}

// GetAccount reads the caller's own account.
//
// It takes the actor's id and nothing else. There is no variant that takes a
// separate target id, which is what makes reading somebody else's account
// impossible by construction rather than by a permission check somebody has to
// remember to write.
func (s *Service) GetAccount(ctx context.Context, actorID uuid.UUID) (Account, error) {
	user, err := s.repo.GetUser(ctx, actorID)
	if err != nil {
		return Account{}, err
	}
	profile, err := s.repo.GetProfile(ctx, actorID)
	if err != nil {
		return Account{}, err
	}
	return Account{User: user, Profile: profile}, nil
}

// UpdateProfile applies a partial change to the caller's own profile and
// records it for the audit trail.
func (s *Service) UpdateProfile(
	ctx context.Context, actorID uuid.UUID, change domain.ProfileChange,
) (Account, error) {
	normalised := change.Normalised()
	if err := normalised.Validate(s.clock.Now()); err != nil {
		return Account{}, err
	}

	user, err := s.requireUsableAccount(ctx, actorID)
	if err != nil {
		return Account{}, err
	}

	var profile domain.Profile
	err = dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		updated, updateErr := repo.UpdateProfile(ctx, actorID, normalised)
		if updateErr != nil {
			return updateErr
		}
		profile = updated

		// The event is written in the same transaction as the update. If the
		// commit fails there is no event; if it succeeds the event cannot be
		// lost. That is the only arrangement in which "every write appears in
		// audit_logs" is a guarantee rather than a hope.
		_, eventErr := s.events.Write(ctx, tx, contract.Aggregate, contract.EventProfileUpdated,
			contract.ProfileUpdated{
				UserID:        actorID,
				ChangedFields: normalised.ChangedFields(),
				ActorID:       actorID,
				OccurredAt:    s.clock.Now(),
			})
		return eventErr
	})
	if err != nil {
		return Account{}, err
	}

	return Account{User: user, Profile: profile}, nil
}

// GetPreferences reads the caller's own preferences.
func (s *Service) GetPreferences(ctx context.Context, actorID uuid.UUID) (domain.Preferences, error) {
	return s.repo.GetPreferences(ctx, actorID)
}

// ReplacePreferences writes the whole preference set for the caller.
func (s *Service) ReplacePreferences(
	ctx context.Context, actorID uuid.UUID, wanted domain.Preferences,
) (domain.Preferences, error) {
	wanted.UserID = actorID
	if err := wanted.Validate(); err != nil {
		return domain.Preferences{}, err
	}
	// Stored in the declared order, so what comes back does not depend on the
	// order the client happened to send.
	wanted.NotificationChannels = wanted.CanonicalChannels()

	if _, err := s.requireUsableAccount(ctx, actorID); err != nil {
		return domain.Preferences{}, err
	}

	var stored domain.Preferences
	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		replaced, replaceErr := repo.ReplacePreferences(ctx, wanted)
		if replaceErr != nil {
			return replaceErr
		}
		stored = replaced

		_, eventErr := s.events.Write(ctx, tx, contract.Aggregate, contract.EventPreferencesUpdated,
			contract.PreferencesUpdated{UserID: actorID, ActorID: actorID, OccurredAt: s.clock.Now()})
		return eventErr
	})
	if err != nil {
		return domain.Preferences{}, err
	}
	return stored, nil
}

// requireUsableAccount loads the account and refuses the write unless it is
// active. A suspended learner keeps read access to their own data — they need
// to see why they are locked out — but changing it is not allowed (BR-USER-08).
func (s *Service) requireUsableAccount(ctx context.Context, actorID uuid.UUID) (domain.User, error) {
	user, err := s.repo.GetUser(ctx, actorID)
	if err != nil {
		return domain.User{}, err
	}
	if !user.Status.Usable() {
		return domain.User{}, domain.ErrAccountNotUsable
	}
	return user, nil
}

// newID generates an identifier, saying what it was for when it fails.
func (s *Service) newID(ctx context.Context) (uuid.UUID, error) {
	generated, err := s.ids(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("generate id: %w", err)
	}
	return generated, nil
}
