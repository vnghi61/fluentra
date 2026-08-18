// Package service orchestrates the admin module's use cases: user management
// and feature-flag administration, both over contracts so no write here spans
// another module's transaction.
package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	sqlcadmin "github.com/fluentra/fluentra/internal/generated/admin/sqlc"
	auditcontract "github.com/fluentra/fluentra/internal/modules/audit/contract"
	authcontract "github.com/fluentra/fluentra/internal/modules/auth/contract"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository is the persistence layer needed by admin service.
type Repository interface {
	LogAdminAction(
		ctx context.Context, actorID, targetID uuid.UUID, action, reason string,
	) (sqlcadmin.CoreAdminAction, error)
	ListFeatureFlags(ctx context.Context) ([]sqlcadmin.CoreFeatureFlag, error)
	GetFeatureFlagByKey(ctx context.Context, key string) (sqlcadmin.CoreFeatureFlag, error)
	CreateFeatureFlag(
		ctx context.Context, key string, enabled bool, rolloutPercent int32, owner string,
		expiresOn time.Time, description string,
	) (sqlcadmin.CoreFeatureFlag, error)
	UpdateFeatureFlag(
		ctx context.Context, key string, enabled bool, rolloutPercent int32,
		expiresOn time.Time, description string,
	) (sqlcadmin.CoreFeatureFlag, error)
	DeleteFeatureFlag(ctx context.Context, key string) error
	GetFlagsExpiringWithin(ctx context.Context, cutoff time.Time) ([]sqlcadmin.CoreFeatureFlag, error)
}

// Service orchestrates back-office administration use cases.
type Service struct {
	pool           dbx.Beginner
	repo           Repository
	userReader     usercontract.AdminReader
	userManager    usercontract.AdminManager
	sessionRevoker authcontract.SessionRevoker
	audit          auditcontract.Recorder
	clock          clock.Clock
}

// Deps are the service collaborators.
type Deps struct {
	Pool           dbx.Beginner
	Repo           Repository
	UserReader     usercontract.AdminReader
	UserManager    usercontract.AdminManager
	SessionRevoker authcontract.SessionRevoker
	Audit          auditcontract.Recorder
	Clock          clock.Clock
}

// New creates a new admin service.
func New(deps Deps) *Service {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}
	return &Service{
		pool:           deps.Pool,
		repo:           deps.Repo,
		userReader:     deps.UserReader,
		userManager:    deps.UserManager,
		sessionRevoker: deps.SessionRevoker,
		audit:          deps.Audit,
		clock:          timekeeper,
	}
}
