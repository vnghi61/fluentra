package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/generated/payment/sqlc"
	"github.com/fluentra/fluentra/internal/modules/payment/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository defines data access operations for the payment module.
type Repository interface {
	CreateOrder(ctx context.Context, order *domain.Order) (*domain.Order, error)
	GetOrderByID(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	GetOrderByReference(ctx context.Context, ref string) (*domain.Order, error)
	UpdateOrderStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus, paidAt *time.Time) (*domain.Order, error)
	ListExpiredPendingOrders(ctx context.Context, limit int32) ([]*domain.Order, error)

	InsertPaymentWebhook(ctx context.Context, provider, providerEventID string, payload []byte, valid bool, errStr *string) error
	InsertSepayTransaction(ctx context.Context, tx *domain.SepayTransaction) (*domain.SepayTransaction, error)
	GetSepayTransactionBySepayID(ctx context.Context, sepayID int64) (*domain.SepayTransaction, error)
	UpdateSepayTransactionMatch(ctx context.Context, id uuid.UUID, orderID *uuid.UUID, matchedAt *time.Time, unmatchedReason *string) (*domain.SepayTransaction, error)
	ListUnmatchedTransactions(ctx context.Context, limit, offset int32) ([]domain.UnmatchedTransaction, int64, error)
	GetLastSeenSepayID(ctx context.Context) (int64, error)
}

type pgRepository struct {
	q *sqlc.Queries
}

// NewRepository creates a new payment repository.
func NewRepository(db dbx.Querier) Repository {
	var q *sqlc.Queries
	if db != nil {
		q = sqlc.New(db)
	}
	return &pgRepository{q: q}
}

func (r *pgRepository) CreateOrder(ctx context.Context, order *domain.Order) (*domain.Order, error) {
	row, err := r.q.CreateOrder(ctx, sqlc.CreateOrderParams{
		UserID:      order.UserID,
		Reference:   order.Reference,
		AmountVnd:   order.AmountVND,
		SubjectKind: order.SubjectKind,
		SubjectID:   order.SubjectID,
		ExpiresAt:   order.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}
	return mapOrderRow(&row, order.QRURL, order.BankCode, order.AccountNumber, order.AccountHolderName), nil
}

func (r *pgRepository) GetOrderByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	row, err := r.q.GetOrderByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}
	return mapOrderRow(&row, "", "", "", ""), nil
}

func (r *pgRepository) GetOrderByReference(ctx context.Context, ref string) (*domain.Order, error) {
	row, err := r.q.GetOrderByReference(ctx, ref)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}
	return mapOrderRow(&row, "", "", "", ""), nil
}

func (r *pgRepository) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus, paidAt *time.Time) (*domain.Order, error) {
	row, err := r.q.UpdateOrderStatus(ctx, sqlc.UpdateOrderStatusParams{
		ID:     id,
		Status: string(status),
		PaidAt: paidAt,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}
	return mapOrderRow(&row, "", "", "", ""), nil
}

func (r *pgRepository) ListExpiredPendingOrders(ctx context.Context, limit int32) ([]*domain.Order, error) {
	rows, err := r.q.ListExpiredPendingOrders(ctx, limit)
	if err != nil {
		return nil, err
	}
	orders := make([]*domain.Order, len(rows))
	for i, row := range rows {
		orders[i] = mapOrderRow(&row, "", "", "", "")
	}
	return orders, nil
}

func (r *pgRepository) InsertPaymentWebhook(ctx context.Context, provider, providerEventID string, payload []byte, valid bool, errStr *string) error {
	_, err := r.q.InsertPaymentWebhook(ctx, sqlc.InsertPaymentWebhookParams{
		Provider:        provider,
		ProviderEventID: providerEventID,
		Payload:         payload,
		SignatureValid:  valid,
		Error:           errStr,
	})
	return err
}

func (r *pgRepository) InsertSepayTransaction(ctx context.Context, tx *domain.SepayTransaction) (*domain.SepayTransaction, error) {
	row, err := r.q.InsertSepayTransaction(ctx, sqlc.InsertSepayTransactionParams{
		SepayID:         tx.SepayID,
		Gateway:         tx.Gateway,
		TransactionDate: tx.TransactionDate,
		AccountNumber:   tx.AccountNumber,
		SubAccount:      tx.SubAccount,
		Code:            tx.Code,
		Content:         tx.Content,
		TransferType:    tx.TransferType,
		TransferAmount:  tx.TransferAmount,
		ReferenceCode:   tx.ReferenceCode,
		Accumulated:     tx.Accumulated,
		OrderID:         tx.OrderID,
		MatchedAt:       tx.MatchedAt,
		UnmatchedReason: tx.UnmatchedReason,
	})
	if err != nil {
		return nil, err
	}
	return mapSepayTransactionRow(&row), nil
}

func (r *pgRepository) GetSepayTransactionBySepayID(ctx context.Context, sepayID int64) (*domain.SepayTransaction, error) {
	row, err := r.q.GetSepayTransactionBySepayID(ctx, sepayID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return mapSepayTransactionRow(&row), nil
}

func (r *pgRepository) UpdateSepayTransactionMatch(ctx context.Context, id uuid.UUID, orderID *uuid.UUID, matchedAt *time.Time, unmatchedReason *string) (*domain.SepayTransaction, error) {
	row, err := r.q.UpdateSepayTransactionMatch(ctx, sqlc.UpdateSepayTransactionMatchParams{
		ID:              id,
		OrderID:         orderID,
		MatchedAt:       matchedAt,
		UnmatchedReason: unmatchedReason,
	})
	if err != nil {
		return nil, err
	}
	return mapSepayTransactionRow(&row), nil
}

func (r *pgRepository) ListUnmatchedTransactions(ctx context.Context, limit, offset int32) ([]domain.UnmatchedTransaction, int64, error) {
	rows, err := r.q.ListUnmatchedTransactions(ctx, sqlc.ListUnmatchedTransactionsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountUnmatchedTransactions(ctx)
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.UnmatchedTransaction, len(rows))
	for i, r := range rows {
		items[i] = domain.UnmatchedTransaction{
			ID:              r.ID,
			SepayID:         r.SepayID,
			Gateway:         r.Gateway,
			TransactionDate: r.TransactionDate,
			AccountNumber:   r.AccountNumber,
			Content:         r.Content,
			TransferType:    r.TransferType,
			TransferAmount:  r.TransferAmount,
			UnmatchedReason: r.UnmatchedReason,
			CreatedAt:       r.CreatedAt,
		}
	}
	return items, total, nil
}

func (r *pgRepository) GetLastSeenSepayID(ctx context.Context) (int64, error) {
	return r.q.GetLastSeenSepayID(ctx)
}

func mapOrderRow(r *sqlc.BillingOrder, qrURL, bankCode, accountNum, holderName string) *domain.Order {
	return &domain.Order{
		ID:                r.ID,
		UserID:            r.UserID,
		Reference:         r.Reference,
		AmountVND:         r.AmountVnd,
		Status:            domain.OrderStatus(r.Status),
		SubjectKind:       r.SubjectKind,
		SubjectID:         r.SubjectID,
		QRURL:             qrURL,
		BankCode:          bankCode,
		AccountNumber:     accountNum,
		AccountHolderName: holderName,
		ExpiresAt:         r.ExpiresAt,
		PaidAt:            r.PaidAt,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}
}

func mapSepayTransactionRow(r *sqlc.BillingSepayTransaction) *domain.SepayTransaction {
	return &domain.SepayTransaction{
		ID:              r.ID,
		SepayID:         r.SepayID,
		Gateway:         r.Gateway,
		TransactionDate: r.TransactionDate,
		AccountNumber:   r.AccountNumber,
		SubAccount:      r.SubAccount,
		Code:            r.Code,
		Content:         r.Content,
		TransferType:    r.TransferType,
		TransferAmount:  r.TransferAmount,
		ReferenceCode:   r.ReferenceCode,
		Accumulated:     r.Accumulated,
		OrderID:         r.OrderID,
		MatchedAt:       r.MatchedAt,
		UnmatchedReason: r.UnmatchedReason,
		CreatedAt:       r.CreatedAt,
	}
}
