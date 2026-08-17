package admin

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	admincontract "github.com/fluentra/fluentra/internal/modules/admin/contract"
	adminrepo "github.com/fluentra/fluentra/internal/modules/admin/repository"
	adminsvc "github.com/fluentra/fluentra/internal/modules/admin/service"
	adminhttp "github.com/fluentra/fluentra/internal/modules/admin/transport/http"
	auditcontract "github.com/fluentra/fluentra/internal/modules/audit/contract"
	authcontract "github.com/fluentra/fluentra/internal/modules/auth/contract"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

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
	service := adminsvc.New(adminsvc.Deps{
		Pool:           deps.Pool,
		Repo:           repo,
		UserReader:     deps.UserReader,
		UserManager:    deps.UserManager,
		SessionRevoker: deps.SessionRevoker,
		Audit:          deps.Audit,
		Clock:          timekeeper,
	})

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

// Service returns the underlying service instance.
func (m *Module) Service() *adminsvc.Service {
	return m.service
}

// FlagReader exposes the feature flag evaluation contract.
func (m *Module) FlagReader() admincontract.FlagReader {
	return m.service
}
