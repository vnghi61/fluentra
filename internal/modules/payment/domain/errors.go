// Package domain defines payment domain models, statuses, and sentinel errors.
package domain

import "github.com/fluentra/fluentra/internal/shared/apperr"

// Payment domain sentinel errors.
var (
	ErrOrderNotFound     = apperr.New(apperr.NotFound, "ORDER_NOT_FOUND", "order not found")
	ErrOrderExpired      = apperr.New(apperr.Conflict, "ORDER_EXPIRED", "order has expired")
	ErrInvalidWebhookKey = apperr.New(apperr.Unauthenticated, "INVALID_WEBHOOK_KEY", "invalid sepay webhook api key")
	ErrForbiddenSenderIP = apperr.New(apperr.Forbidden, "FORBIDDEN_SENDER_IP", "sender ip not allowed")
	ErrInvalidAmount     = apperr.New(apperr.Validation, "INVALID_AMOUNT", "invalid amount")
	ErrAmountMismatch    = apperr.New(
		apperr.Conflict, "AMOUNT_MISMATCH", "transfer amount does not match order amount",
	)
	ErrPayoutNotFound         = apperr.New(apperr.NotFound, "PAYOUT_NOT_FOUND", "payout not found")
	ErrPayoutAlreadyProcessed = apperr.New(apperr.Conflict, "PAYOUT_ALREADY_PROCESSED", "payout already sent or failed")
)

// ErrOrderNotPayable is returned when an order cannot move to paid because it
// is already paid, cancelled or refunded. It is not a failure of the payment:
// it means this money belongs to a human to look at, not to a state machine.
var ErrOrderNotPayable = apperr.New(
	apperr.Conflict, "ORDER_NOT_PAYABLE", "Order is not in a payable state")

// ErrRefundNotFound is returned when a refund row cannot be read.
var ErrRefundNotFound = apperr.New(
	apperr.NotFound, "REFUND_NOT_FOUND", "Refund not found")
