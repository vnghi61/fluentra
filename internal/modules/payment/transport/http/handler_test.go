package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/payment/contract"
	"github.com/fluentra/fluentra/internal/modules/payment/domain"
	paymenthttp "github.com/fluentra/fluentra/internal/modules/payment/transport/http"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type mockService struct {
	orders       map[uuid.UUID]*domain.Order
	unmatched    []domain.UnmatchedTransaction
	webhookError error
}

func newMockService() *mockService {
	return &mockService{
		orders: make(map[uuid.UUID]*domain.Order),
	}
}

func (m *mockService) CreateOrder(ctx context.Context, in contract.CreateOrderInput) (*domain.Order, error) {
	o := &domain.Order{
		ID:          uuid.New(),
		UserID:      in.UserID,
		Reference:   "FLU12345ABCDE",
		AmountVND:   in.AmountVND,
		Status:      domain.OrderStatusPending,
		SubjectKind: in.SubjectKind,
		SubjectID:   in.SubjectID,
		QRURL:       "https://vietqr.app/img?acc=1017588888&bank=VCB&amount=490000&des=FLU12345ABCDE&template=compact",
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	m.orders[o.ID] = o
	return o, nil
}

func (m *mockService) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	o, ok := m.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return o, nil
}

func (m *mockService) HandleSepayWebhook(ctx context.Context, authHeader, clientIP string, rawBody []byte, payload *domain.SepayWebhookPayload) error {
	return m.webhookError
}

func (m *mockService) MatchTransaction(ctx context.Context, tx *domain.SepayTransaction) error {
	return nil
}

func (m *mockService) SweepExpiredOrders(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockService) Reconcile(ctx context.Context) error {
	return nil
}

func (m *mockService) ListUnmatchedTransactions(ctx context.Context, limit, offset int32) (*domain.UnmatchedTransactionsList, error) {
	return &domain.UnmatchedTransactionsList{
		Items: m.unmatched,
		Total: int64(len(m.unmatched)),
	}, nil
}

func (m *mockService) RecordRefund(
	_ context.Context, orderID uuid.UUID, amountVND int64, reason string, _ uuid.UUID,
) (*domain.Refund, error) {
	return &domain.Refund{
		ID: uuid.New(), OrderID: orderID, AmountVND: amountVND,
		Reason: reason, Status: domain.RefundStatusRequested,
	}, nil
}

func (m *mockService) ListRefunds(
	_ context.Context, _ *string, _, _ int,
) ([]domain.Refund, int64, error) {
	return nil, 0, nil
}

func (m *mockService) MarkRefundSent(_ context.Context, id uuid.UUID) (*domain.Refund, error) {
	return &domain.Refund{ID: id, Status: domain.RefundStatusSent}, nil
}

func (m *mockService) CreatePayout(_ context.Context, in contract.CreatePayoutInput) (*domain.Payout, error) {
	return &domain.Payout{
		ID:        uuid.New(),
		CreatorID: in.CreatorID,
		AmountVND: in.AmountVND,
		Status:    domain.PayoutStatusPending,
	}, nil
}

func (m *mockService) GetPayout(_ context.Context, id uuid.UUID) (*domain.Payout, error) {
	return &domain.Payout{ID: id, CreatorID: uuid.New(), AmountVND: 500000, Status: domain.PayoutStatusPending}, nil
}

func (m *mockService) ListPayouts(_ context.Context, _ *string, _, _ int) ([]domain.Payout, int64, error) {
	return nil, 0, nil
}

func (m *mockService) ListCreatorPayouts(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.Payout, error) {
	return nil, nil
}

func (m *mockService) GetPendingPayoutTotal(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

func (m *mockService) FulfillPayout(
	_ context.Context, id uuid.UUID, bankReference string, _ uuid.UUID,
) (*domain.Payout, error) {
	return &domain.Payout{
		ID:            id,
		CreatorID:     uuid.New(),
		AmountVND:     500000,
		Status:        domain.PayoutStatusSent,
		BankReference: &bankReference,
	}, nil
}

type mockGuard struct{}

func (g *mockGuard) Require(_ context.Context, _ string) error {
	return nil
}

func TestHandleSepayWebhook(t *testing.T) {
	svc := newMockService()
	h := paymenthttp.NewHandler(svc, &mockGuard{})

	r := chi.NewRouter()
	h.PublicRoutes(r)

	payload := domain.SepayWebhookPayload{
		ID:              92704,
		Gateway:         "Vietcombank",
		TransactionDate: "2024-07-02 11:08:33",
		AccountNumber:   "1017588888",
		TransferType:    "in",
		TransferAmount:  490000,
		Content:         "FLU12345ABCDE",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/payment/sepay", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Apikey test-key")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp paymenthttp.SepayWebhookResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success: true, got %v", resp.Success)
	}
}

func TestGetOrder(t *testing.T) {
	svc := newMockService()
	h := paymenthttp.NewHandler(svc, &mockGuard{})

	r := chi.NewRouter()
	h.AuthenticatedRoutes(r)

	callerID := uuid.New()
	order, _ := svc.CreateOrder(context.Background(), contract.CreateOrderInput{
		UserID:      callerID,
		SubjectKind: "course",
		SubjectID:   uuid.New(),
		AmountVND:   490000,
	})

	// 1. Success when caller matches order owner
	req := httptest.NewRequest(http.MethodGet, "/me/orders/"+order.ID.String(), nil)
	ctx := httpx.WithActor(req.Context(), httpx.Actor{
		UserID: callerID,
		Role:   "user",
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req.WithContext(ctx))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp paymenthttp.BillingOrderResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order response: %v", err)
	}
	if resp.ID != order.ID {
		t.Errorf("expected order id %s, got %s", order.ID, resp.ID)
	}

	// 2. 404 when caller is different user
	otherUserID := uuid.New()
	req2 := httptest.NewRequest(http.MethodGet, "/me/orders/"+order.ID.String(), nil)
	ctx2 := httpx.WithActor(req2.Context(), httpx.Actor{
		UserID: otherUserID,
		Role:   "user",
	})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2.WithContext(ctx2))

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected 404 for other user, got %d", w2.Code)
	}
}

func TestListUnmatchedTransactions(t *testing.T) {
	svc := newMockService()
	svc.unmatched = []domain.UnmatchedTransaction{
		{
			ID:             uuid.New(),
			SepayID:        1234,
			Gateway:        "VCB",
			Content:        "unknown transfer",
			TransferType:   "in",
			TransferAmount: 100000,
		},
	}
	h := paymenthttp.NewHandler(svc, &mockGuard{})

	r := chi.NewRouter()
	h.AdminRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/admin/payments/unmatched?limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var list domain.UnmatchedTransactionsList
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if list.Total != 1 {
		t.Errorf("expected total 1, got %d", list.Total)
	}
}
