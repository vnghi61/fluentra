package domain

import (
	"time"

	"github.com/google/uuid"
)

// OrderStatus defines the state of a billing order.
type OrderStatus string

// Order status constants.
const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusExpired   OrderStatus = "expired"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusRefunded  OrderStatus = "refunded"
)

// Order represents an order in the domain layer.
type Order struct {
	ID                uuid.UUID   `json:"id"`
	UserID            uuid.UUID   `json:"user_id"`
	Reference         string      `json:"reference"`
	AmountVND         int64       `json:"amount_vnd"`
	Status            OrderStatus `json:"status"`
	SubjectKind       string      `json:"subject_kind"`
	SubjectID         uuid.UUID   `json:"subject_id"`
	QRURL             string      `json:"qr_url"`
	BankCode          string      `json:"bank_code"`
	AccountNumber     string      `json:"account_number"`
	AccountHolderName string      `json:"account_holder_name"`
	ExpiresAt         time.Time   `json:"expires_at"`
	PaidAt            *time.Time  `json:"paid_at"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

// Refund is money owed back to a learner.
//
// It is a record of an obligation, not a transfer: SePay receives money and
// does not send it, so a refund is paid by an admin making a bank transfer and
// then marking the row sent. A refund with no row is a refund nobody will make,
// which is why RefundPurchase writes one before it revokes anything.
type Refund struct {
	ID        uuid.UUID  `json:"id"`
	OrderID   uuid.UUID  `json:"order_id"`
	AmountVND int64      `json:"amount_vnd"`
	Reason    string     `json:"reason"`
	ActorID   uuid.UUID  `json:"actor_id"`
	Status    string     `json:"status"`
	SentAt    *time.Time `json:"sent_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Refund statuses.
const (
	RefundStatusRequested = "requested"
	RefundStatusSent      = "sent"
	RefundStatusFailed    = "failed"
)

// SepayWebhookPayload is the payload SePay posts to our webhook endpoint.
type SepayWebhookPayload struct {
	ID              int64  `json:"id"`
	Gateway         string `json:"gateway"`
	TransactionDate string `json:"transactionDate"`
	AccountNumber   string `json:"accountNumber"`
	SubAccount      string `json:"subAccount"`
	Code            string `json:"code"`
	Content         string `json:"content"`
	TransferType    string `json:"transferType"`
	Description     string `json:"description"`
	TransferAmount  int64  `json:"transferAmount"`
	Accumulated     int64  `json:"accumulated"`
	ReferenceCode   string `json:"referenceCode"`
}

// SepayTransaction represents an incoming transaction logged from SePay.
type SepayTransaction struct {
	ID              uuid.UUID  `json:"id"`
	SepayID         int64      `json:"sepay_id"`
	Gateway         string     `json:"gateway"`
	TransactionDate time.Time  `json:"transaction_date"`
	AccountNumber   string     `json:"account_number"`
	SubAccount      string     `json:"sub_account"`
	Code            string     `json:"code"`
	Content         string     `json:"content"`
	TransferType    string     `json:"transfer_type"`
	TransferAmount  int64      `json:"transfer_amount"`
	ReferenceCode   string     `json:"reference_code"`
	Accumulated     int64      `json:"accumulated"`
	OrderID         *uuid.UUID `json:"order_id"`
	MatchedAt       *time.Time `json:"matched_at"`
	UnmatchedReason *string    `json:"unmatched_reason"`
	CreatedAt       time.Time  `json:"created_at"`
}

// UnmatchedTransaction is returned by the admin unmatched transactions query.
type UnmatchedTransaction struct {
	ID              uuid.UUID `json:"id"`
	SepayID         int64     `json:"sepay_id"`
	Gateway         string    `json:"gateway"`
	TransactionDate time.Time `json:"transaction_date"`
	AccountNumber   string    `json:"account_number"`
	Content         string    `json:"content"`
	TransferType    string    `json:"transfer_type"`
	TransferAmount  int64     `json:"transfer_amount"`
	UnmatchedReason *string   `json:"unmatched_reason"`
	CreatedAt       time.Time `json:"created_at"`
}

// UnmatchedTransactionsList represents the paginated list for admin review.
type UnmatchedTransactionsList struct {
	Items []UnmatchedTransaction `json:"items"`
	Total int64                  `json:"total"`
}

// PayoutStatus defines the lifecycle status of a payout.
type PayoutStatus string

// Payout status constants.
const (
	PayoutStatusPending PayoutStatus = "pending"
	PayoutStatusSent    PayoutStatus = "sent"
	PayoutStatusFailed  PayoutStatus = "failed"
)

// Payout represents a manual payout record to a creator.
type Payout struct {
	ID            uuid.UUID    `json:"id"`
	CreatorID     uuid.UUID    `json:"creator_id"`
	AmountVND     int64        `json:"amount_vnd"`
	Status        PayoutStatus `json:"status"`
	BankReference *string      `json:"bank_reference"`
	ActorID       uuid.UUID    `json:"actor_id"`
	SentAt        *time.Time   `json:"sent_at"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}
