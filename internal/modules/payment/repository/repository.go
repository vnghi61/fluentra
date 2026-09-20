package repository

import (
	"context"
	"errors"
	"fmt"
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
	// MarkOrderPaid moves an order to paid only from pending or expired, and
	// reports domain.ErrOrderNotPayable otherwise. See the query's comment.
	MarkOrderPaid(ctx context.Context, id uuid.UUID, paidAt time.Time) (*domain.Order, error)
	MarkOrderRefunded(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	CreateRefund(ctx context.Context, orderID uuid.UUID, amountVND int64, reason string, actorID uuid.UUID) (*domain.Refund, error)
	ListRefunds(ctx context.Context, status *string, limit, offset int32) ([]domain.Refund, int64, error)
	MarkRefundSent(ctx context.Context, id uuid.UUID) (*domain.Refund, error)
	ListExpiredPendingOrders(ctx context.Context, limit int32) ([]*domain.Order, error)

	InsertPaymentWebhook(ctx context.Context, provider, providerEventID string, payload []byte, valid bool, errStr *string) error
	InsertSepayTransaction(ctx context.Context, tx *domain.SepayTransaction) (*domain.SepayTransaction, error)
	GetSepayTransactionBySepayID(ctx context.Context, sepayID int64) (*domain.SepayTransaction, error)
	UpdateSepayTransactionMatch(ctx context.Context, id uuid.UUID, orderID *uuid.UUID, matchedAt *time.Time, unmatchedReason *string) (*domain.SepayTransaction, error)
	ListUnmatchedTransactions(ctx context.Context, limit, offset int32) ([]domain.UnmatchedTransaction, int64, error)
	GetLastSeenSepayID(ctx context.Context) (int64, error)

	CreatePayout(ctx context.Context, p *domain.Payout) (*domain.Payout, error)
	GetPayoutByID(ctx context.Context, id uuid.UUID) (*domain.Payout, error)
	UpdatePayoutStatus(
		ctx context.Context, id uuid.UUID, status domain.PayoutStatus,
		bankRef *string, actorID uuid.UUID, sentAt *time.Time,
	) (*domain.Payout, error)
	ListPayouts(ctx context.Context, status *string, limit, offset int32) ([]domain.Payout, int64, error)
	ListPayoutsByCreatorID(ctx context.Context, creatorID uuid.UUID, limit, offset int32) ([]domain.Payout, error)
	GetPendingPayoutTotalByCreatorID(ctx context.Context, creatorID uuid.UUID) (int64, error)
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

func (r *pgRepository) CreatePayout(ctx context.Context, p *domain.Payout) (*domain.Payout, error) {
	row, err := r.q.CreatePayout(ctx, sqlc.CreatePayoutParams{
		CreatorID: p.CreatorID,
		AmountVnd: p.AmountVND,
		ActorID:   p.ActorID,
	})
	if err != nil {
		return nil, err
	}
	return mapPayoutRow(&row), nil
}

func (r *pgRepository) GetPayoutByID(ctx context.Context, id uuid.UUID) (*domain.Payout, error) {
	row, err := r.q.GetPayoutByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPayoutNotFound
		}
		return nil, err
	}
	return mapPayoutRow(&row), nil
}

func (r *pgRepository) UpdatePayoutStatus(
	ctx context.Context,
	id uuid.UUID,
	status domain.PayoutStatus,
	bankRef *string,
	actorID uuid.UUID,
	sentAt *time.Time,
) (*domain.Payout, error) {
	row, err := r.q.UpdatePayoutStatus(ctx, sqlc.UpdatePayoutStatusParams{
		ID:            id,
		Status:        string(status),
		BankReference: bankRef,
		ActorID:       actorID,
		SentAt:        sentAt,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPayoutNotFound
		}
		return nil, err
	}
	return mapPayoutRow(&row), nil
}

func (r *pgRepository) ListPayouts(
	ctx context.Context,
	status *string,
	limit, offset int32,
) ([]domain.Payout, int64, error) {
	if status != nil && *status != "" {
		rows, err := r.q.ListPayoutsByStatus(ctx, sqlc.ListPayoutsByStatusParams{
			Status: *status,
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, 0, err
		}
		total, err := r.q.CountPayoutsByStatus(ctx, *status)
		if err != nil {
			return nil, 0, err
		}
		items := make([]domain.Payout, len(rows))
		for i, row := range rows {
			items[i] = *mapPayoutRow(&row)
		}
		return items, total, nil
	}

	rows, err := r.q.ListPayouts(ctx, sqlc.ListPayoutsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountPayouts(ctx)
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.Payout, len(rows))
	for i, row := range rows {
		items[i] = *mapPayoutRow(&row)
	}
	return items, total, nil
}

func (r *pgRepository) ListPayoutsByCreatorID(
	ctx context.Context,
	creatorID uuid.UUID,
	limit, offset int32,
) ([]domain.Payout, error) {
	rows, err := r.q.ListPayoutsByCreatorID(ctx, sqlc.ListPayoutsByCreatorIDParams{
		CreatorID: creatorID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]domain.Payout, len(rows))
	for i, row := range rows {
		items[i] = *mapPayoutRow(&row)
	}
	return items, nil
}

func (r *pgRepository) GetPendingPayoutTotalByCreatorID(ctx context.Context, creatorID uuid.UUID) (int64, error) {
	return r.q.GetPendingPayoutTotalByCreatorID(ctx, creatorID)
}

func mapPayoutRow(r *sqlc.BillingPayout) *domain.Payout {
	return &domain.Payout{
		ID:            r.ID,
		CreatorID:     r.CreatorID,
		AmountVND:     r.AmountVnd,
		Status:        domain.PayoutStatus(r.Status),
		BankReference: r.BankReference,
		ActorID:       r.ActorID,
		SentAt:        r.SentAt,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
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

// MarkOrderPaid moves an order to paid, and only from pending or expired.
func (r *pgRepository) MarkOrderPaid(ctx context.Context, id uuid.UUID, paidAt time.Time) (*domain.Order, error) {
	row, err := r.q.MarkOrderPaid(ctx, sqlc.MarkOrderPaidParams{ID: id, PaidAt: &paidAt})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No row updated means the WHERE clause refused the transition,
			// not that the order is missing.
			return nil, domain.ErrOrderNotPayable
		}
		return nil, fmt.Errorf("mark order paid: %w", err)
	}
	return mapOrderRow(&row, "", "", "", ""), nil
}

// MarkOrderRefunded moves a paid order to refunded.
func (r *pgRepository) MarkOrderRefunded(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	row, err := r.q.MarkOrderRefunded(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotPayable
		}
		return nil, fmt.Errorf("mark order refunded: %w", err)
	}
	return mapOrderRow(&row, "", "", "", ""), nil
}

// CreateRefund records that money is owed back on an order.
func (r *pgRepository) CreateRefund(
	ctx context.Context, orderID uuid.UUID, amountVND int64, reason string, actorID uuid.UUID,
) (*domain.Refund, error) {
	row, err := r.q.CreateRefund(ctx, sqlc.CreateRefundParams{
		OrderID:   orderID,
		AmountVnd: amountVND,
		Reason:    reason,
		ActorID:   actorID,
	})
	if err != nil {
		return nil, fmt.Errorf("create refund: %w", err)
	}
	return mapRefundRow(&row), nil
}

// ListRefunds lists refunds, newest first, optionally filtered by status.
func (r *pgRepository) ListRefunds(
	ctx context.Context, status *string, limit, offset int32,
) ([]domain.Refund, int64, error) {
	rows, err := r.q.ListRefundsByStatus(ctx, sqlc.ListRefundsByStatusParams{
		Status: status,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list refunds: %w", err)
	}
	total, err := r.q.CountRefundsByStatus(ctx, status)
	if err != nil {
		return nil, 0, fmt.Errorf("count refunds: %w", err)
	}
	out := make([]domain.Refund, 0, len(rows))
	for i := range rows {
		out = append(out, *mapRefundRow(&rows[i]))
	}
	return out, total, nil
}

// MarkRefundSent records that the bank transfer has been made.
func (r *pgRepository) MarkRefundSent(ctx context.Context, id uuid.UUID) (*domain.Refund, error) {
	row, err := r.q.MarkRefundSent(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRefundNotFound
		}
		return nil, fmt.Errorf("mark refund sent: %w", err)
	}
	return mapRefundRow(&row), nil
}

func mapRefundRow(r *sqlc.BillingRefund) *domain.Refund {
	return &domain.Refund{
		ID:        r.ID,
		OrderID:   r.OrderID,
		AmountVND: r.AmountVnd,
		Reason:    r.Reason,
		ActorID:   r.ActorID,
		Status:    r.Status,
		SentAt:    r.SentAt,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
