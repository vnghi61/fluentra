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
	_ contract.OrderCreator  = (*Module)(nil)
	_ contract.OrderReader   = (*Module)(nil)
	_ contract.PayoutManager = (*Module)(nil)
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

// SetPayoutAccountReader injects the payout account reader for single payout admin inspection.
func (m *Module) SetPayoutAccountReader(reader paymenthttp.PayoutAccountReader) {
	m.handler.SetPayoutAccountReader(reader)
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

// RefundRecorder returns the RefundRecorder contract interface.
func (m *Module) RefundRecorder() contract.RefundRecorder {
	return m
}

// RecordRefund records that money is owed back on a paid order and moves the
// order to refunded. It does not move money: SePay only receives, so the
// transfer is made by an admin against this record.
func (m *Module) RecordRefund(
	ctx context.Context, orderID uuid.UUID, amountVND int64, reason string, actorID uuid.UUID,
) (*contract.Refund, error) {
	refund, err := m.service.RecordRefund(ctx, orderID, amountVND, reason, actorID)
	if err != nil {
		return nil, err
	}
	return &contract.Refund{
		ID:        refund.ID,
		OrderID:   refund.OrderID,
		AmountVND: refund.AmountVND,
		Reason:    refund.Reason,
		Status:    refund.Status,
		SentAt:    refund.SentAt,
		CreatedAt: refund.CreatedAt,
	}, nil
}

// PayoutManager returns the PayoutManager contract interface.
func (m *Module) PayoutManager() contract.PayoutManager {
	return m
}

// CreatePayout creates a new pending payout request.
func (m *Module) CreatePayout(ctx context.Context, in contract.CreatePayoutInput) (*contract.Payout, error) {
	p, err := m.service.CreatePayout(ctx, in)
	if err != nil {
		return nil, err
	}
	return toContractPayout(p), nil
}

// GetPayout retrieves a payout by its ID.
func (m *Module) GetPayout(ctx context.Context, id uuid.UUID) (*contract.Payout, error) {
	p, err := m.service.GetPayout(ctx, id)
	if err != nil {
		return nil, err
	}
	return toContractPayout(p), nil
}

// ListPayouts returns paginated payouts filtered optionally by status.
func (m *Module) ListPayouts(ctx context.Context, status *string, limit, offset int) ([]contract.Payout, int64, error) {
	items, total, err := m.service.ListPayouts(ctx, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	res := make([]contract.Payout, len(items))
	for i, p := range items {
		res[i] = *toContractPayout(&p)
	}
	return res, total, nil
}

// ListCreatorPayouts returns all payouts for a specific creator.
func (m *Module) ListCreatorPayouts(
	ctx context.Context, creatorID uuid.UUID, limit, offset int,
) ([]contract.Payout, error) {
	items, err := m.service.ListCreatorPayouts(ctx, creatorID, limit, offset)
	if err != nil {
		return nil, err
	}
	res := make([]contract.Payout, len(items))
	for i, p := range items {
		res[i] = *toContractPayout(&p)
	}
	return res, nil
}

// GetPendingPayoutTotal returns the sum of pending payout requests for a creator.
func (m *Module) GetPendingPayoutTotal(ctx context.Context, creatorID uuid.UUID) (int64, error) {
	return m.service.GetPendingPayoutTotal(ctx, creatorID)
}

// FulfillPayout marks a pending payout as sent with its bank transfer reference.
func (m *Module) FulfillPayout(
	ctx context.Context, id uuid.UUID, bankReference string, actorID uuid.UUID,
) (*contract.Payout, error) {
	p, err := m.service.FulfillPayout(ctx, id, bankReference, actorID)
	if err != nil {
		return nil, err
	}
	return toContractPayout(p), nil
}

func toContractPayout(p *domain.Payout) *contract.Payout {
	return &contract.Payout{
		ID:            p.ID,
		CreatorID:     p.CreatorID,
		AmountVND:     p.AmountVND,
		Status:        string(p.Status),
		BankReference: p.BankReference,
		ActorID:       p.ActorID,
		SentAt:        p.SentAt,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
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
