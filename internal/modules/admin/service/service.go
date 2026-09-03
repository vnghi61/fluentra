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

// AIUsageStatus is one provider-and-task row of today's AI consumption.
//
// Declared here rather than reused from platform/ai, and the arch lint is what
// asked for it: m_admin_service and m_admin_http are allowed contracts and their
// own layers, and no platform package at all. That is deliberate across the
// whole file -- admin reads what other components publish and calls no
// capability directly -- so importing platform/ai to borrow a struct would have
// widened a boundary for the sake of six fields.
//
// The composition root converts. That is the same shape as every other seam in
// cmd/api.
type AIUsageStatus struct {
	Provider          string
	Task              string
	RequestsToday     int64
	TokensToday       int64
	DailyRequestLimit *int
	DailyTokenLimit   *int64
	IsExhausted       bool
}

// AIUsageReporter queries AI consumption and quota status across providers.
type AIUsageReporter interface {
	GetUsageOverview(ctx context.Context) ([]AIUsageStatus, error)
}

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
	aiUsage        AIUsageReporter
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
	AIUsage        AIUsageReporter
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
		aiUsage:        deps.AIUsage,
	}
}

// GetAIUsage returns current daily usage and quota status for all configured AI providers and tasks.
func (s *Service) GetAIUsage(ctx context.Context) ([]AIUsageStatus, error) {
	if s.aiUsage == nil {
		return []AIUsageStatus{}, nil
	}
	return s.aiUsage.GetUsageOverview(ctx)
}
