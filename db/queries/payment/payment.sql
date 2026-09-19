-- name: CreateOrder :one
INSERT INTO billing.orders (
    user_id, reference, amount_vnd, status, subject_kind, subject_id, expires_at, updated_at
) VALUES ($1, $2, $3, 'pending', $4, $5, $6, now())
RETURNING id, user_id, reference, amount_vnd, status, subject_kind, subject_id, expires_at, paid_at, created_at, updated_at;

-- name: GetOrderByID :one
SELECT id, user_id, reference, amount_vnd, status, subject_kind, subject_id, expires_at, paid_at, created_at, updated_at
FROM billing.orders
WHERE id = $1;

-- name: GetOrderByReference :one
SELECT id, user_id, reference, amount_vnd, status, subject_kind, subject_id, expires_at, paid_at, created_at, updated_at
FROM billing.orders
WHERE reference = $1;

-- name: ListOrdersByUserID :many
SELECT id, user_id, reference, amount_vnd, status, subject_kind, subject_id, expires_at, paid_at, created_at, updated_at
FROM billing.orders
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateOrderStatus :one
UPDATE billing.orders
SET status = $2,
    paid_at = $3,
    updated_at = now()
WHERE id = $1
RETURNING id, user_id, reference, amount_vnd, status, subject_kind, subject_id, expires_at, paid_at, created_at, updated_at;

-- name: ListExpiredPendingOrders :many
SELECT id, user_id, reference, amount_vnd, status, subject_kind, subject_id, expires_at, paid_at, created_at, updated_at
FROM billing.orders
WHERE status = 'pending' AND expires_at <= now()
ORDER BY expires_at ASC
LIMIT $1;

-- name: InsertSepayTransaction :one
INSERT INTO billing.sepay_transactions (
    sepay_id, gateway, transaction_date, account_number, sub_account,
    code, content, transfer_type, transfer_amount, reference_code,
    accumulated, order_id, matched_at, unmatched_reason
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14
)
ON CONFLICT (sepay_id) DO UPDATE
SET gateway = EXCLUDED.gateway
RETURNING id, sepay_id, gateway, transaction_date, account_number, sub_account, code, content, transfer_type, transfer_amount, reference_code, accumulated, order_id, matched_at, unmatched_reason, created_at;

-- name: GetSepayTransactionBySepayID :one
SELECT id, sepay_id, gateway, transaction_date, account_number, sub_account, code, content, transfer_type, transfer_amount, reference_code, accumulated, order_id, matched_at, unmatched_reason, created_at
FROM billing.sepay_transactions
WHERE sepay_id = $1;

-- name: UpdateSepayTransactionMatch :one
UPDATE billing.sepay_transactions
SET order_id = $2,
    matched_at = $3,
    unmatched_reason = $4
WHERE id = $1
RETURNING id, sepay_id, gateway, transaction_date, account_number, sub_account, code, content, transfer_type, transfer_amount, reference_code, accumulated, order_id, matched_at, unmatched_reason, created_at;

-- name: ListUnmatchedTransactions :many
SELECT id, sepay_id, gateway, transaction_date, account_number, sub_account, code, content, transfer_type, transfer_amount, reference_code, accumulated, order_id, matched_at, unmatched_reason, created_at
FROM billing.sepay_transactions
WHERE matched_at IS NULL
ORDER BY transaction_date DESC
LIMIT $1 OFFSET $2;

-- name: CountUnmatchedTransactions :one
SELECT COUNT(*)
FROM billing.sepay_transactions
WHERE matched_at IS NULL;

-- name: GetLastSeenSepayID :one
SELECT COALESCE(MAX(sepay_id), 0)::bigint
FROM billing.sepay_transactions;

-- name: InsertPaymentWebhook :one
INSERT INTO billing.payment_webhooks (
    provider, provider_event_id, payload, signature_valid, error
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (provider, provider_event_id) DO NOTHING
RETURNING id, provider, provider_event_id, payload, signature_valid, received_at, processed_at, error;

-- name: UpdatePaymentWebhookStatus :one
UPDATE billing.payment_webhooks
SET processed_at = now(),
    error = $2
WHERE id = $1
RETURNING id, provider, provider_event_id, payload, signature_valid, received_at, processed_at, error;

-- name: ListUnprocessedWebhooks :many
SELECT id, provider, provider_event_id, payload, signature_valid, received_at, processed_at, error
FROM billing.payment_webhooks
WHERE processed_at IS NULL
ORDER BY received_at ASC
LIMIT $1;

-- -------------------------------------------------- billing.payouts

-- name: CreatePayout :one
INSERT INTO billing.payouts (
    creator_id, amount_vnd, status, actor_id, updated_at
) VALUES ($1, $2, 'pending', $3, now())
RETURNING id, creator_id, amount_vnd, status, bank_reference, actor_id, sent_at, created_at, updated_at;

-- name: GetPayoutByID :one
SELECT id, creator_id, amount_vnd, status, bank_reference, actor_id, sent_at, created_at, updated_at
FROM billing.payouts
WHERE id = $1;

-- name: UpdatePayoutStatus :one
UPDATE billing.payouts
SET status = $2,
    bank_reference = $3,
    actor_id = $4,
    sent_at = $5,
    updated_at = now()
WHERE id = $1
RETURNING id, creator_id, amount_vnd, status, bank_reference, actor_id, sent_at, created_at, updated_at;

-- name: ListPayouts :many
SELECT id, creator_id, amount_vnd, status, bank_reference, actor_id, sent_at, created_at, updated_at
FROM billing.payouts
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPayouts :one
SELECT COUNT(*)
FROM billing.payouts;

-- name: ListPayoutsByStatus :many
SELECT id, creator_id, amount_vnd, status, bank_reference, actor_id, sent_at, created_at, updated_at
FROM billing.payouts
WHERE status = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPayoutsByStatus :one
SELECT COUNT(*)
FROM billing.payouts
WHERE status = $1;

-- name: ListPayoutsByCreatorID :many
SELECT id, creator_id, amount_vnd, status, bank_reference, actor_id, sent_at, created_at, updated_at
FROM billing.payouts
WHERE creator_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetPendingPayoutTotalByCreatorID :one
SELECT COALESCE(SUM(amount_vnd), 0)::bigint
FROM billing.payouts
WHERE creator_id = $1 AND status = 'pending';

