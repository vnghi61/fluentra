package contract

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Order represents an order for course purchase or subscription in the billing system.
type Order struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Reference         string
	AmountVND         int64
	Status            string // "pending", "paid", "expired", "cancelled", "refunded"
	SubjectKind       string // "course"
	SubjectID         uuid.UUID
	QRURL             string
	BankCode          string
	AccountNumber     string
	AccountHolderName string
	ExpiresAt         time.Time
	PaidAt            *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CreateOrderInput contains the parameters to create a new bank transfer order.
type CreateOrderInput struct {
	UserID      uuid.UUID
	SubjectKind string
	SubjectID   uuid.UUID
	AmountVND   int64
}

// OrderCreator creates bank transfer orders.
type OrderCreator interface {
	CreateOrder(ctx context.Context, in CreateOrderInput) (*Order, error)
}

// OrderReader reads order details.
type OrderReader interface {
	GetOrder(ctx context.Context, id uuid.UUID) (*Order, error)
}

// EventPaymentSucceeded is published when an order payment is successfully matched.
type EventPaymentSucceeded struct {
	UserID      uuid.UUID `json:"user_id"`
	OrderID     uuid.UUID `json:"order_id"`
	SubjectKind string    `json:"subject_kind"`
	SubjectID   uuid.UUID `json:"subject_id"`
	AmountVND   int64     `json:"amount_vnd"`
}

// Refund is money owed back to a learner, as the billing module records it.
type Refund struct {
	ID        uuid.UUID  `json:"id"`
	OrderID   uuid.UUID  `json:"order_id"`
	AmountVND int64      `json:"amount_vnd"`
	Reason    string     `json:"reason"`
	Status    string     `json:"status"`
	SentAt    *time.Time `json:"sent_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// RefundRecorder records that money is owed back, and marks the order refunded.
//
// It records an obligation rather than moving money, because SePay only
// receives. `studio` calls this when it revokes a purchase; an admin pays it
// and marks it sent.
type RefundRecorder interface {
	RecordRefund(ctx context.Context, orderID uuid.UUID, amountVND int64, reason string, actorID uuid.UUID) (*Refund, error)
}

// Payout represents a manual bank transfer payout to a creator.
type Payout struct {
	ID            uuid.UUID  `json:"id"`
	CreatorID     uuid.UUID  `json:"creator_id"`
	AmountVND     int64      `json:"amount_vnd"`
	Status        string     `json:"status"` // "pending", "sent", "failed"
	BankReference *string    `json:"bank_reference"`
	ActorID       uuid.UUID  `json:"actor_id"`
	SentAt        *time.Time `json:"sent_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CreatePayoutInput contains parameters to create a payout request.
type CreatePayoutInput struct {
	CreatorID uuid.UUID
	AmountVND int64
	ActorID   uuid.UUID
}

// PayoutManager defines operations for creator payouts.
type PayoutManager interface {
	CreatePayout(ctx context.Context, in CreatePayoutInput) (*Payout, error)
	GetPayout(ctx context.Context, id uuid.UUID) (*Payout, error)
	ListPayouts(ctx context.Context, status *string, limit, offset int) ([]Payout, int64, error)
	ListCreatorPayouts(ctx context.Context, creatorID uuid.UUID, limit, offset int) ([]Payout, error)
	GetPendingPayoutTotal(ctx context.Context, creatorID uuid.UUID) (int64, error)
	FulfillPayout(ctx context.Context, id uuid.UUID, bankReference string, actorID uuid.UUID) (*Payout, error)
}

// EventPayoutSent is published when a creator payout is fulfilled.
type EventPayoutSent struct {
	PayoutID      uuid.UUID `json:"payout_id"`
	CreatorID     uuid.UUID `json:"creator_id"`
	AmountVND     int64     `json:"amount_vnd"`
	BankReference string    `json:"bank_reference"`
}
