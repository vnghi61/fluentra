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
	UpdateProfileAvatar(ctx context.Context, userID uuid.UUID, avatarAssetID *uuid.UUID) (domain.Profile, error)

	InsertAvatarAsset(ctx context.Context, asset domain.AvatarAsset) error
	GetAvatarAsset(
		ctx context.Context, assetID uuid.UUID, variant domain.AvatarVariant,
	) (domain.AvatarAsset, error)
	DeleteAvatarAssetsByAssetID(ctx context.Context, assetID uuid.UUID) error

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

	// The export lifecycle (GDPR).
	CreateExportRequest(ctx context.Context, id, userID uuid.UUID) (domain.ExportRequest, error)
	GetPendingExportForUser(ctx context.Context, userID uuid.UUID) (domain.ExportRequest, bool, error)
	GetExportByID(ctx context.Context, id uuid.UUID) (domain.ExportRequest, error)
	UpdateExportStatus(
		ctx context.Context,
		id uuid.UUID,
		status domain.ExportStatus,
		startedAt, completedAt, expiresAt *time.Time,
		objectKey, errorMessage *string,
	) error
	GetExpiredExports(ctx context.Context, limit int32) ([]domain.ExportRequest, error)
	DeleteExport(ctx context.Context, id uuid.UUID) error

	// The deletion lifecycle (GDPR).
	CreateDeletionRequest(ctx context.Context, id, userID uuid.UUID, executeAt time.Time) (domain.DeletionRequest, error)
	GetPendingDeletionForUser(ctx context.Context, userID uuid.UUID) (domain.DeletionRequest, bool, error)
	GetDeletionByID(ctx context.Context, id uuid.UUID) (domain.DeletionRequest, error)
	CancelDeletion(ctx context.Context, id uuid.UUID, cancelledAt time.Time) error
	UpdateDeletionStatus(
		ctx context.Context,
		id uuid.UUID,
		status domain.DeletionStatus,
		startedAt, completedAt *time.Time,
		errorMessage *string,
	) error
	GetDueDeletions(ctx context.Context, cutoff time.Time, limit int32) ([]domain.DeletionRequest, error)
	AnonymiseUser(ctx context.Context, userID uuid.UUID, anonymisedEmail string) error
	AnonymiseProfile(ctx context.Context, userID uuid.UUID) error
	DeletePreferences(ctx context.Context, userID uuid.UUID) error
	DeleteLearningProfile(ctx context.Context, userID uuid.UUID) error
	UpdateUserStatus(ctx context.Context, userID uuid.UUID, status domain.Status) error
	SearchUsersAdmin(
		ctx context.Context,
		filter contract.UserFilter,
		cursorID *uuid.UUID,
		cursorTime *time.Time,
		limit int,
	) ([]SearchUserRow, error)

	WithTx(tx pgx.Tx) Repository
}

// SearchUserRow is the search result row for admin user search.
type SearchUserRow struct {
	ID          uuid.UUID
	Email       string
	Status      domain.Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DisplayName string
	Locale      string
	Timezone    string
}

// JobEnqueuer is what the service uses to schedule background work inside a transaction.
type JobEnqueuer interface {
	EnqueueExportTx(ctx context.Context, tx pgx.Tx, exportID, userID uuid.UUID) error
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

// BaselineRoles grants the role every account holds.
//
// Declared here rather than imported from rbac/contract so this module keeps
// depending on nothing but its own domain: the interface is one method wide and
// `rbac` satisfies it without knowing that `user` exists. The composition root
// is what puts the two together.
type BaselineRoles interface {
	GrantBaselineRole(ctx context.Context, userID uuid.UUID) error
}

// Service implements the user module's use cases.
type Service struct {
	pool     dbx.Beginner
	repo     Repository
	events   EventWriter
	clock    clock.Clock
	ids      IDGenerator
	storage  StorageStore
	enqueuer JobEnqueuer
	roles    BaselineRoles
}

// Deps are the service's collaborators.
type Deps struct {
	Pool     dbx.Beginner
	Repo     Repository
	Events   EventWriter
	Clock    clock.Clock
	NewID    IDGenerator
	Storage  StorageStore
	Enqueuer JobEnqueuer
	// Roles is optional: a build with no rbac wired creates accounts that hold
	// no role, which is the behaviour this field exists to end. cmd/ supplies it.
	Roles BaselineRoles
}

// New creates the user service.
func New(deps Deps) *Service {
	return &Service{
		pool:     deps.Pool,
		repo:     deps.Repo,
		events:   deps.Events,
		clock:    deps.Clock,
		ids:      deps.NewID,
		storage:  deps.Storage,
		enqueuer: deps.Enqueuer,
		roles:    deps.Roles,
	}
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
