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
