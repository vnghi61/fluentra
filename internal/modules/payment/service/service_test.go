package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/payment/contract"
	"github.com/fluentra/fluentra/internal/modules/payment/domain"
	"github.com/fluentra/fluentra/internal/modules/payment/service"
)

type mockRepository struct {
	orders       map[uuid.UUID]*domain.Order
	transactions map[int64]*domain.SepayTransaction
	refunds      []*domain.Refund
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		orders:       make(map[uuid.UUID]*domain.Order),
		transactions: make(map[int64]*domain.SepayTransaction),
	}
}

// MarkOrderPaid mirrors the query's WHERE clause: only an order that is
// pending or expired becomes paid.
func (m *mockRepository) MarkOrderPaid(
	_ context.Context, id uuid.UUID, paidAt time.Time,
) (*domain.Order, error) {
	o, ok := m.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	if o.Status != domain.OrderStatusPending && o.Status != domain.OrderStatusExpired {
		return nil, domain.ErrOrderNotPayable
	}
	o.Status = domain.OrderStatusPaid
	o.PaidAt = &paidAt
	return o, nil
}

func (m *mockRepository) MarkOrderRefunded(_ context.Context, id uuid.UUID) (*domain.Order, error) {
	o, ok := m.orders[id]
	if !ok || o.Status != domain.OrderStatusPaid {
		return nil, domain.ErrOrderNotPayable
	}
	o.Status = domain.OrderStatusRefunded
	return o, nil
}

func (m *mockRepository) CreateRefund(
	_ context.Context, orderID uuid.UUID, amountVND int64, reason string, actorID uuid.UUID,
) (*domain.Refund, error) {
	r := &domain.Refund{
		ID: uuid.New(), OrderID: orderID, AmountVND: amountVND,
		Reason: reason, ActorID: actorID, Status: domain.RefundStatusRequested,
	}
	m.refunds = append(m.refunds, r)
	return r, nil
}

func (m *mockRepository) ListRefunds(
	_ context.Context, _ *string, _, _ int32,
) ([]domain.Refund, int64, error) {
	out := make([]domain.Refund, 0, len(m.refunds))
	for _, r := range m.refunds {
		out = append(out, *r)
	}
	return out, int64(len(out)), nil
}

func (m *mockRepository) MarkRefundSent(_ context.Context, id uuid.UUID) (*domain.Refund, error) {
	for _, r := range m.refunds {
		if r.ID == id {
			r.Status = domain.RefundStatusSent
			return r, nil
		}
	}
	return nil, domain.ErrRefundNotFound
}

func (m *mockRepository) CreateOrder(_ context.Context, order *domain.Order) (*domain.Order, error) {
	m.orders[order.ID] = order
	return order, nil
}

func (m *mockRepository) GetOrderByID(_ context.Context, id uuid.UUID) (*domain.Order, error) {
	o, ok := m.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return o, nil
}

func (m *mockRepository) GetOrderByReference(_ context.Context, ref string) (*domain.Order, error) {
	for _, o := range m.orders {
		if o.Reference == ref {
			return o, nil
		}
	}
	return nil, domain.ErrOrderNotFound
}

func (m *mockRepository) UpdateOrderStatus(
	_ context.Context,
	id uuid.UUID,
	status domain.OrderStatus,
	paidAt *time.Time,
) (*domain.Order, error) {
	o, ok := m.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	o.Status = status
	o.PaidAt = paidAt
	o.UpdatedAt = time.Now().UTC()
	return o, nil
}

func (m *mockRepository) ListExpiredPendingOrders(_ context.Context, _ int32) ([]*domain.Order, error) {
	var list []*domain.Order
	now := time.Now().UTC()
	for _, o := range m.orders {
		if o.Status == domain.OrderStatusPending && o.ExpiresAt.Before(now) {
			list = append(list, o)
		}
	}
	return list, nil
}

func (m *mockRepository) InsertPaymentWebhook(_ context.Context, _, _ string, _ []byte, _ bool, _ *string) error {
	return nil
}

func (m *mockRepository) InsertSepayTransaction(
	_ context.Context, tx *domain.SepayTransaction,
) (*domain.SepayTransaction, error) {
	if tx.ID == uuid.Nil {
		tx.ID = uuid.New()
	}
	m.transactions[tx.SepayID] = tx
	return tx, nil
}

func (m *mockRepository) GetSepayTransactionBySepayID(
	_ context.Context, sepayID int64,
) (*domain.SepayTransaction, error) {
	return m.transactions[sepayID], nil
}

func (m *mockRepository) UpdateSepayTransactionMatch(
	_ context.Context,
	id uuid.UUID,
	orderID *uuid.UUID,
	matchedAt *time.Time,
	unmatchedReason *string,
) (*domain.SepayTransaction, error) {
	for _, tx := range m.transactions {
		if tx.ID == id {
			tx.OrderID = orderID
			tx.MatchedAt = matchedAt
			tx.UnmatchedReason = unmatchedReason
			return tx, nil
		}
	}
	return nil, fmt.Errorf("transaction not found")
}

func (m *mockRepository) ListUnmatchedTransactions(
	_ context.Context, _, _ int32,
) ([]domain.UnmatchedTransaction, int64, error) {
	var items []domain.UnmatchedTransaction
	for _, tx := range m.transactions {
		if tx.MatchedAt == nil {
			items = append(items, domain.UnmatchedTransaction{
				ID:              tx.ID,
				SepayID:         tx.SepayID,
				Gateway:         tx.Gateway,
				TransactionDate: tx.TransactionDate,
				AccountNumber:   tx.AccountNumber,
				Content:         tx.Content,
				TransferType:    tx.TransferType,
				TransferAmount:  tx.TransferAmount,
				UnmatchedReason: tx.UnmatchedReason,
				CreatedAt:       tx.CreatedAt,
			})
		}
	}
	return items, int64(len(items)), nil
}

func (m *mockRepository) GetLastSeenSepayID(_ context.Context) (int64, error) {
	var maxID int64
	for id := range m.transactions {
		if id > maxID {
			maxID = id
		}
	}
	return maxID, nil
}

func (m *mockRepository) CreatePayout(_ context.Context, p *domain.Payout) (*domain.Payout, error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	p.Status = domain.PayoutStatusPending
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	return p, nil
}

func (m *mockRepository) GetPayoutByID(_ context.Context, id uuid.UUID) (*domain.Payout, error) {
	return &domain.Payout{ID: id, CreatorID: uuid.New(), AmountVND: 500000, Status: domain.PayoutStatusPending}, nil
}

func (m *mockRepository) UpdatePayoutStatus(
	_ context.Context,
	id uuid.UUID,
	status domain.PayoutStatus,
	bankRef *string,
	actorID uuid.UUID,
	sentAt *time.Time,
) (*domain.Payout, error) {
	return &domain.Payout{
		ID:            id,
		CreatorID:     uuid.New(),
		AmountVND:     500000,
		Status:        status,
		BankReference: bankRef,
		ActorID:       actorID,
		SentAt:        sentAt,
	}, nil
}

func (m *mockRepository) ListPayouts(_ context.Context, _ *string, _, _ int32) ([]domain.Payout, int64, error) {
	return nil, 0, nil
}

func (m *mockRepository) ListPayoutsByCreatorID(_ context.Context, _ uuid.UUID, _, _ int32) ([]domain.Payout, error) {
	return nil, nil
}

func (m *mockRepository) GetPendingPayoutTotalByCreatorID(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

func setupTestService() (service.Service, *mockRepository) {
	repo := newMockRepository()
	cfg := service.Config{
		WebhookAPIKey: "test-sepay-key-12345",
		AccountNumber: "1017588888",
		BankCode:      "VCB",
		AccountHolder: "FLUENTRA",
		OrderTTL:      24 * time.Hour,
	}
	svc := service.NewService(repo, cfg)
	return svc, repo
}

func TestCreateOrder(t *testing.T) {
	svc, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()

	// 1. Invalid amount
	_, err := svc.CreateOrder(ctx, contract.CreateOrderInput{
		UserID:      userID,
		SubjectKind: "course",
		SubjectID:   courseID,
		AmountVND:   0,
	})
	if err == nil {
		t.Fatal("expected error for zero amount, got nil")
	}

	// 2. Valid order
	order, err := svc.CreateOrder(ctx, contract.CreateOrderInput{
		UserID:      userID,
		SubjectKind: "course",
		SubjectID:   courseID,
		AmountVND:   490000,
	})
	if err != nil {
		t.Fatalf("unexpected error creating order: %v", err)
	}

	if order.AmountVND != 490000 {
		t.Errorf("expected amount 490000, got %d", order.AmountVND)
	}
	if order.Status != domain.OrderStatusPending {
		t.Errorf("expected status pending, got %s", order.Status)
	}
	if order.QRURL == "" {
		t.Error("expected non-empty QRURL")
	}
}

func TestHandleSepayWebhook_InvalidAPIKey(t *testing.T) {
	svc, _ := setupTestService()
	ctx := context.Background()

	payload := &domain.SepayWebhookPayload{
		ID:             1001,
		TransferAmount: 490000,
		TransferType:   "in",
	}

	err := svc.HandleSepayWebhook(ctx, "Apikey wrong-key", "127.0.0.1", []byte("{}"), payload)
	if err == nil {
		t.Fatal("expected unauthorized error for wrong api key, got nil")
	}
}

func TestHandleSepayWebhook_TransferTypeOut(t *testing.T) {
	svc, repo := setupTestService()
	ctx := context.Background()

	payload := &domain.SepayWebhookPayload{
		ID:              1002,
		TransferAmount:  490000,
		TransferType:    "out",
		Content:         "FLU1234567890 refund",
		TransactionDate: "2026-09-20 12:00:00",
	}

	err := svc.HandleSepayWebhook(ctx, "Apikey test-sepay-key-12345", "127.0.0.1", []byte("{}"), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tx := repo.transactions[1002]
	if tx == nil {
		t.Fatal("expected transaction to be stored")
	}
	if tx.UnmatchedReason == nil || *tx.UnmatchedReason != "transfer_type_out" {
		t.Errorf("expected unmatched reason transfer_type_out, got %v", tx.UnmatchedReason)
	}
}

func TestHandleSepayWebhook_OrderNotFound(t *testing.T) {
	svc, repo := setupTestService()
	ctx := context.Background()

	payload := &domain.SepayWebhookPayload{
		ID:              1003,
		TransferAmount:  490000,
		TransferType:    "in",
		Content:         "Chuyen tien khong co reference dung",
		TransactionDate: "2026-09-20 12:00:00",
	}

	err := svc.HandleSepayWebhook(ctx, "Apikey test-sepay-key-12345", "127.0.0.1", []byte("{}"), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tx := repo.transactions[1003]
	if tx == nil || tx.UnmatchedReason == nil || *tx.UnmatchedReason != "order_not_found" {
		t.Errorf("expected unmatched reason order_not_found, got %v", tx.UnmatchedReason)
	}
}

func TestHandleSepayWebhook_AmountMismatch(t *testing.T) {
	svc, repo := setupTestService()
	ctx := context.Background()

	order, err := svc.CreateOrder(ctx, contract.CreateOrderInput{
		UserID:      uuid.New(),
		SubjectKind: "course",
		SubjectID:   uuid.New(),
		AmountVND:   490000,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	// Paid 49,000 instead of 490,000 (BR-PAYMENT-12: never partially credit)
	payload := &domain.SepayWebhookPayload{
		ID:              1004,
		TransferAmount:  49000,
		TransferType:    "in",
		Content:         fmt.Sprintf("Chuyen tien %s them chu", order.Reference),
		TransactionDate: "2026-09-20 12:00:00",
	}

	err = svc.HandleSepayWebhook(ctx, "Apikey test-sepay-key-12345", "127.0.0.1", []byte("{}"), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tx := repo.transactions[1004]
	if tx == nil || tx.UnmatchedReason == nil {
		t.Fatal("expected transaction with unmatched reason")
	}
	if *tx.UnmatchedReason != "amount_mismatch: got 49000 want 490000" {
		t.Errorf("unexpected unmatched reason: %s", *tx.UnmatchedReason)
	}

	// Order must still be pending
	if repo.orders[order.ID].Status != domain.OrderStatusPending {
		t.Errorf("expected order to remain pending, got %s", repo.orders[order.ID].Status)
	}
}

func TestHandleSepayWebhook_Success(t *testing.T) {
	svc, repo := setupTestService()
	ctx := context.Background()

	order, err := svc.CreateOrder(ctx, contract.CreateOrderInput{
		UserID:      uuid.New(),
		SubjectKind: "course",
		SubjectID:   uuid.New(),
		AmountVND:   490000,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	payload := &domain.SepayWebhookPayload{
		ID:              1005,
		TransferAmount:  490000,
		TransferType:    "in",
		Content:         fmt.Sprintf("NGUYEN VAN A chuyen tien %s cam on", order.Reference),
		TransactionDate: "2026-09-20 12:00:00",
	}

	err = svc.HandleSepayWebhook(ctx, "Apikey test-sepay-key-12345", "127.0.0.1", []byte("{}"), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tx := repo.transactions[1005]
	if tx == nil || tx.MatchedAt == nil || tx.OrderID == nil || *tx.OrderID != order.ID {
		t.Fatalf("expected transaction matched to order %s", order.ID)
	}

	if repo.orders[order.ID].Status != domain.OrderStatusPaid {
		t.Errorf("expected order status paid, got %s", repo.orders[order.ID].Status)
	}
}

func TestHandleSepayWebhook_ExpiredOrderMatches(t *testing.T) {
	svc, repo := setupTestService()
	ctx := context.Background()

	order, err := svc.CreateOrder(ctx, contract.CreateOrderInput{
		UserID:      uuid.New(),
		SubjectKind: "course",
		SubjectID:   uuid.New(),
		AmountVND:   490000,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	// Mark order expired
	order.Status = domain.OrderStatusExpired
	repo.orders[order.ID] = order

	// Payment arrives late for expired order (Work order §10.5)
	payload := &domain.SepayWebhookPayload{
		ID:              1006,
		TransferAmount:  490000,
		TransferType:    "in",
		Content:         fmt.Sprintf("THANH TOAN %s", order.Reference),
		TransactionDate: "2026-09-20 12:00:00",
	}

	err = svc.HandleSepayWebhook(ctx, "Apikey test-sepay-key-12345", "127.0.0.1", []byte("{}"), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.orders[order.ID].Status != domain.OrderStatusPaid {
		t.Errorf("expected expired order to be reopened and marked paid, got %s", repo.orders[order.ID].Status)
	}
}

func TestSweepExpiredOrders(t *testing.T) {
	svc, repo := setupTestService()
	ctx := context.Background()

	// 1. Order that is expired
	order1 := &domain.Order{
		ID:        uuid.New(),
		Status:    domain.OrderStatusPending,
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour),
	}
	repo.orders[order1.ID] = order1

	// 2. Order that is still valid
	order2 := &domain.Order{
		ID:        uuid.New(),
		Status:    domain.OrderStatusPending,
		ExpiresAt: time.Now().UTC().Add(10 * time.Hour),
	}
	repo.orders[order2.ID] = order2

	count, err := svc.SweepExpiredOrders(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 order swept, got %d", count)
	}

	if repo.orders[order1.ID].Status != domain.OrderStatusExpired {
		t.Errorf("expected order1 to be expired, got %s", repo.orders[order1.ID].Status)
	}
	if repo.orders[order2.ID].Status != domain.OrderStatusPending {
		t.Errorf("expected order2 to remain pending, got %s", repo.orders[order2.ID].Status)
	}
}

// TestMatchTransaction_SecondPaymentIsNotSwallowed is the regression for a
// learner who transfers twice because they thought the first attempt failed.
//
// Matching had no order-status guard: the second transaction re-marked a paid
// order, republished payment.succeeded, and was filed as matched with no
// reason — so their second payment left no trace anybody would ever look at.
// It belongs in the unmatched queue, where a human can see it and refund it.
func TestMatchTransaction_SecondPaymentIsNotSwallowed(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	svc := service.NewService(repo, service.Config{
		WebhookAPIKey: "k", BankCode: "VCB", AccountNumber: "1", OrderTTL: time.Hour,
	})

	order, err := svc.CreateOrder(ctx, contract.CreateOrderInput{
		UserID: uuid.New(), SubjectKind: "course", SubjectID: uuid.New(), AmountVND: 200000,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	pay := func(sepayID int64) *domain.SepayTransaction {
		tx := &domain.SepayTransaction{
			ID: uuid.New(), SepayID: sepayID, TransferType: "in",
			TransferAmount: 200000, Content: "CT " + order.Reference + " chuyen tien",
		}
		repo.transactions[sepayID] = tx
		if err := svc.MatchTransaction(ctx, tx); err != nil {
			t.Fatalf("match %d: %v", sepayID, err)
		}
		return tx
	}

	first := pay(1)
	if first.OrderID == nil || first.MatchedAt == nil {
		t.Fatalf("the first payment should have matched the order")
	}

	second := pay(2)
	if second.OrderID != nil {
		t.Errorf("the second payment was matched to the order again")
	}
	if second.UnmatchedReason == nil {
		t.Fatalf("the second payment was filed with no reason, so nobody will see it")
	}
	if !strings.Contains(*second.UnmatchedReason, "duplicate_payment") {
		t.Errorf("unmatched reason %q does not say this was a duplicate payment", *second.UnmatchedReason)
	}
}

// TestMatchTransaction_ExpiredOrderStillHonoured. BR-PAYMENT-13: the money is
// real, so an order that timed out before the transfer landed is still paid.
func TestMatchTransaction_ExpiredOrderStillHonoured(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	svc := service.NewService(repo, service.Config{
		WebhookAPIKey: "k", BankCode: "VCB", AccountNumber: "1", OrderTTL: time.Hour,
	})

	order, err := svc.CreateOrder(ctx, contract.CreateOrderInput{
		UserID: uuid.New(), SubjectKind: "course", SubjectID: uuid.New(), AmountVND: 50000,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	repo.orders[order.ID].Status = domain.OrderStatusExpired

	tx := &domain.SepayTransaction{
		ID: uuid.New(), SepayID: 9, TransferType: "in",
		TransferAmount: 50000, Content: order.Reference,
	}
	repo.transactions[tx.SepayID] = tx
	if err := svc.MatchTransaction(ctx, tx); err != nil {
		t.Fatalf("match: %v", err)
	}
	if repo.orders[order.ID].Status != domain.OrderStatusPaid {
		t.Errorf("an expired order that was actually paid should be honoured, got %s",
			repo.orders[order.ID].Status)
	}
}

// TestSePayListResponseDecodes is the regression for reconciliation, which
// could not process a single transaction.
//
// SePay's userapi sends every number as a JSON string — `"id": "49682"`,
// `"amount_in": "18067000.00"` — unlike its webhook, which sends numbers. The
// struct declared int64 and float64, so decoding failed on the first field and
// Reconcile returned an error on every run. Nothing about it was visible
// except a line in a cron log, and it is the only thing that catches a webhook
// that never arrived.
func TestSePayListResponseDecodes(t *testing.T) {
	// The shape docs.sepay.vn documents, verbatim.
	const body = `{"status":200,"error":null,"messages":{"success":true},"transactions":[` +
		`{"id":"49682","bank_brand_name":"Vietcombank","account_number":"0071000888888",` +
		`"transaction_date":"2023-05-05 19:59:48","amount_out":"0.00","amount_in":"18067000.00",` +
		`"accumulated":"1200541768.00","transaction_content":"DUONG THUY ANH chuyen tien",` +
		`"reference_number":"677760.050523.080001","code":null,"sub_account":"VCB0011ABC004",` +
		`"bank_account_id":"19"}]}`

	var resp service.SePayAPIResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("a documented SePay list response must decode: %v", err)
	}
	if len(resp.Transactions) != 1 {
		t.Fatalf("decoded %d transactions, want 1", len(resp.Transactions))
	}
	got := resp.Transactions[0]
	if got.ID != "49682" {
		t.Errorf("id = %q, want 49682", got.ID)
	}
	if got.AmountIn != "18067000.00" {
		t.Errorf("amount_in = %q", got.AmountIn)
	}
}

// TestRecordRefund_WritesTheObligation. A refund records that money is owed
// and moves the order; it does not move money, because SePay only receives.
func TestRecordRefund_WritesTheObligation(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	svc := service.NewService(repo, service.Config{OrderTTL: time.Hour})

	order, err := svc.CreateOrder(ctx, contract.CreateOrderInput{
		UserID: uuid.New(), SubjectKind: "course", SubjectID: uuid.New(), AmountVND: 120000,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := repo.MarkOrderPaid(ctx, order.ID, time.Now()); err != nil {
		t.Fatalf("mark paid: %v", err)
	}

	refund, err := svc.RecordRefund(ctx, order.ID, 120000, "learner_refund", uuid.New())
	if err != nil {
		t.Fatalf("record refund: %v", err)
	}
	if refund.Status != domain.RefundStatusRequested {
		t.Errorf("refund status = %q, want requested", refund.Status)
	}
	if len(repo.refunds) != 1 {
		t.Fatalf("expected the obligation to be written, got %d rows", len(repo.refunds))
	}
	if repo.orders[order.ID].Status != domain.OrderStatusRefunded {
		t.Errorf("order status = %q, want refunded", repo.orders[order.ID].Status)
	}
}
