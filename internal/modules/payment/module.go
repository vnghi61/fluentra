package payment

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/payment/contract"
	"github.com/fluentra/fluentra/internal/modules/payment/domain"
	"github.com/fluentra/fluentra/internal/modules/payment/job"
	"github.com/fluentra/fluentra/internal/modules/payment/repository"
	"github.com/fluentra/fluentra/internal/modules/payment/service"
	paymenthttp "github.com/fluentra/fluentra/internal/modules/payment/transport/http"
	platformjob "github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
)

var (
	_ contract.OrderCreator = (*Module)(nil)
	_ contract.OrderReader  = (*Module)(nil)
)

// Dependencies declares external inputs to the payment module.
type Dependencies struct {
	Pool  *pgxpool.Pool
	Guard paymenthttp.Guard
	Cfg   service.Config
	Bus   eventbus.EventBus
}

// Module encapsulates the payment and SePay processing domain.
type Module struct {
	repo            repository.Repository
	service         service.Service
	handler         *paymenthttp.Handler
	sweepWorker     *job.ExpirySweepWorker
	reconcileWorker *job.ReconciliationWorker
}

// NewModule constructs the payment module.
func NewModule(deps Dependencies) (*Module, error) {
	repo := repository.NewRepository(deps.Pool)
	svc := service.NewService(repo, deps.Cfg, deps.Bus)
	handler := paymenthttp.NewHandler(svc, deps.Guard)
	sweepWorker := job.NewExpirySweepWorker(svc)
	reconcileWorker := job.NewReconciliationWorker(svc)

	return &Module{
		repo:            repo,
		service:         svc,
		handler:         handler,
		sweepWorker:     sweepWorker,
		reconcileWorker: reconcileWorker,
	}, nil
}

// PublicRoutes mounts unauthenticated routes (e.g. SePay webhook).
func (m *Module) PublicRoutes(r chi.Router) {
	m.handler.PublicRoutes(r)
}

// AuthenticatedRoutes mounts routes requiring user session (e.g. /me/orders/{id}).
func (m *Module) AuthenticatedRoutes(r chi.Router) {
	m.handler.AuthenticatedRoutes(r)
}

// AdminRoutes mounts administrative routes (e.g. /admin/payments/unmatched).
func (m *Module) AdminRoutes(r chi.Router) {
	m.handler.AdminRoutes(r)
}

// CronJobs returns background workers for expiry sweep and daily reconciliation.
func (m *Module) CronJobs() []platformjob.CronJob {
	return []platformjob.CronJob{
		m.sweepWorker.CronJob(),
		m.reconcileWorker.CronJob(),
	}
}

// Service returns the underlying payment service.
func (m *Module) Service() service.Service {
	return m.service
}

// Repository returns the underlying payment repository.
func (m *Module) Repository() repository.Repository {
	return m.repo
}

// CreateOrder satisfies contract.OrderCreator.
func (m *Module) CreateOrder(ctx context.Context, in contract.CreateOrderInput) (*contract.Order, error) {
	o, err := m.service.CreateOrder(ctx, in)
	if err != nil {
		return nil, err
	}
	return toContractOrder(o), nil
}

// OrderCreator returns the OrderCreator contract interface.
func (m *Module) OrderCreator() contract.OrderCreator {
	return m
}

// OrderReader returns the OrderReader contract interface.
func (m *Module) OrderReader() contract.OrderReader {
	return m
}

// GetOrder satisfies contract.OrderReader.
func (m *Module) GetOrder(ctx context.Context, id uuid.UUID) (*contract.Order, error) {
	o, err := m.service.GetOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	return toContractOrder(o), nil
}


func toContractOrder(o *domain.Order) *contract.Order {
	return &contract.Order{
		ID:                o.ID,
		UserID:            o.UserID,
		Reference:         o.Reference,
		AmountVND:         o.AmountVND,
		Status:            string(o.Status),
		SubjectKind:       o.SubjectKind,
		SubjectID:         o.SubjectID,
		QRURL:             o.QRURL,
		BankCode:          o.BankCode,
		AccountNumber:     o.AccountNumber,
		AccountHolderName: o.AccountHolderName,
		ExpiresAt:         o.ExpiresAt,
		PaidAt:            o.PaidAt,
		CreatedAt:         o.CreatedAt,
		UpdatedAt:         o.UpdatedAt,
	}
}
