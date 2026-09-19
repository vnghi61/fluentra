package service_test

import (
	"context"
	"fmt"
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
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		orders:       make(map[uuid.UUID]*domain.Order),
		transactions: make(map[int64]*domain.SepayTransaction),
	}
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
