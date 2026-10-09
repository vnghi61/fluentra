package admin

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	admincontract "github.com/fluentra/fluentra/internal/modules/admin/contract"
	adminrepo "github.com/fluentra/fluentra/internal/modules/admin/repository"
	adminsvc "github.com/fluentra/fluentra/internal/modules/admin/service"
	adminhttp "github.com/fluentra/fluentra/internal/modules/admin/transport/http"
	auditcontract "github.com/fluentra/fluentra/internal/modules/audit/contract"
	authcontract "github.com/fluentra/fluentra/internal/modules/auth/contract"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const reportExpiringFlagsLockID int64 = 1_700_000_060

// Guard is the authorization surface the HTTP handler needs.
type Guard = adminhttp.Guard

// Deps are what the composition root supplies to the admin module.
type Deps struct {
	Pool           *pgxpool.Pool
	Clock          clock.Clock
	UserReader     usercontract.AdminReader
	UserManager    usercontract.AdminManager
	SessionRevoker authcontract.SessionRevoker
	Audit          auditcontract.Recorder
	Guard          Guard
	AIUsage        adminsvc.AIUsageReporter
	// Storage holds banner images. Optional: without it banners are text-only.
	Storage storage.Store
}

// Module is the admin module.
type Module struct {
	service *adminsvc.Service
	handler *adminhttp.Handler
}

// New wires the admin module.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	repo := adminrepo.New(deps.Pool)
	serviceDeps := adminsvc.Deps{
		Pool:           deps.Pool,
		Repo:           repo,
		UserReader:     deps.UserReader,
		UserManager:    deps.UserManager,
		SessionRevoker: deps.SessionRevoker,
		Audit:          deps.Audit,
		Clock:          timekeeper,
		AIUsage:        deps.AIUsage,
		Banners:        repo,
	}
	if deps.Storage != nil {
		serviceDeps.Images = deps.Storage
	}
	service := adminsvc.New(serviceDeps)

	handler := adminhttp.NewHandler(service, deps.Guard)

	return &Module{
		service: service,
		handler: handler,
	}
}

// Routes mounts the admin operations on chi.Router.
func (m *Module) Routes(router chi.Router) {
	m.handler.Routes(router)
}

// LearnerRoutes mounts the operations any signed-in account may call (the
// dashboard's active banners). cmd/api mounts them outside the admin-only group.
func (m *Module) LearnerRoutes(router chi.Router) {
	m.handler.LearnerRoutes(router)
}

// Service returns the underlying service instance.
func (m *Module) Service() *adminsvc.Service {
	return m.service
}

// FlagReader exposes the feature flag evaluation contract.
func (m *Module) FlagReader() admincontract.FlagReader {
	return m.service
}

// CronJobs returns the scheduled maintenance jobs for the admin module.
func (m *Module) CronJobs() []job.CronJob {
	return []job.CronJob{
		{
			Name:     "admin.report_expiring_flags",
			LockID:   reportExpiringFlagsLockID,
			Interval: 7 * 24 * time.Hour,
			Task:     m.service.ReportExpiringFlags,
		},
	}
}
