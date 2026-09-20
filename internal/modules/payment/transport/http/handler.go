// Package http exposes the payment module over HTTP: the public SePay
// webhook, a learner's own order, and the back-office queues for refunds,
// payouts and transactions that matched nothing.
// Package http exposes the payment module over HTTP: the public SePay
// webhook, a learner's own order, and the back-office queues for refunds,
// payouts and transactions that matched nothing.
// Package http exposes the payment module over HTTP: the public SePay
// webhook, a learner's own order, and the back-office queues for refunds,
// payouts and transactions that matched nothing.
// Package http exposes the payment module over HTTP: the public SePay
// webhook, a learner's own order, and the back-office queues for refunds,
// payouts and transactions that matched nothing.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/payment/domain"
	"github.com/fluentra/fluentra/internal/modules/payment/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// Guard checks permissions for administrative operations.
type Guard interface {
	Require(ctx context.Context, permission string) error
}

// The permissions these routes enforce.
//
// They shipped behind `admin.dashboard`, which means "may open the back
// office" — so issuing a payout and reading a creator's bank account number sat
// behind the weakest permission the system has. payment/AGENT.md specified
// these two from the start.
const (
	permBillingRead   = "billing.read"
	permBillingManage = "billing.manage"
)

// PayoutAccountReader supplies creator bank account details for single payout inspection.
// BR-STUDIO-09: Used exclusively by admin detail view. Never exposed in list endpoints.
type PayoutAccountReader interface {
	GetPayoutAccount(
		ctx context.Context, creatorID uuid.UUID,
	) (bankCode, accountNumber, accountHolderName string, err error)
}

// Handler handles payment and SePay webhook HTTP requests.
type Handler struct {
	svc           service.Service
	guard         Guard
	accountReader PayoutAccountReader
}

// NewHandler creates a new payment HTTP handler.
func NewHandler(svc service.Service, guard Guard) *Handler {
	return &Handler{
		svc:   svc,
		guard: guard,
	}
}

// SetPayoutAccountReader configures the reader used to inspect creator bank accounts.
func (h *Handler) SetPayoutAccountReader(reader PayoutAccountReader) {
	h.accountReader = reader
}

// PublicRoutes mounts public routes such as SePay webhooks.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Post("/webhooks/payment/sepay", h.handleSepayWebhook)
}

// AuthenticatedRoutes mounts routes requiring user authentication.
func (h *Handler) AuthenticatedRoutes(r chi.Router) {
	r.Get("/me/orders/{id}", h.getOrder)
}

// AdminRoutes mounts staff/administrative routes.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/admin/payments/unmatched", h.listUnmatchedTransactions)
	r.Get("/admin/billing/refunds", h.listRefunds)
	r.Post("/admin/billing/refunds/{id}/sent", h.markRefundSent)
	r.Get("/admin/billing/payouts", h.listPayouts)
	r.Get("/admin/billing/payouts/{id}", h.getPayout)
	r.Post("/admin/billing/payouts/{id}/fulfill", h.fulfillPayout)
}

// ---------------------------------------------------------------- Webhook

// SepayWebhookResponse is the body SePay requires: it counts a delivery as
// successful only on a 200 or 201 carrying {"success": true}.
// SepayWebhookResponse is the body SePay requires: it counts a delivery as
// successful only on a 200 or 201 carrying {"success": true}.
// SepayWebhookResponse is the body SePay requires: it counts a delivery as
// successful only on a 200 or 201 carrying {"success": true}.
// SepayWebhookResponse is the body SePay requires: it counts a delivery as
// successful only on a 200 or 201 carrying {"success": true}.
type SepayWebhookResponse struct {
	Success bool `json:"success"`
}

func (h *Handler) handleSepayWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authHeader := r.Header.Get("Authorization")

	// Read raw body for storage/audit while enabling JSON decoding
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_BODY", "failed to read body"))
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var payload domain.SepayWebhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_JSON", "invalid sepay payload JSON"))
		return
	}

	// The address the request really came from.
	//
	// This read X-Forwarded-For directly, which anybody may set: the allowlist
	// could be walked past by sending one of SePay's own addresses, and behind
	// a real proxy the header is a comma-separated chain that never equalled a
	// single allowed IP, so it would have rejected every legitimate delivery
	// too. httpx.ClientIPResolver exists for this and honours the configured
	// trusted-proxy CIDRs.
	clientIP := httpx.ClientIP(ctx).String()

	if err := h.svc.HandleSepayWebhook(ctx, authHeader, clientIP, bodyBytes, &payload); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	// SePay strictly requires {"success": true} with 200/201 response.
	httpx.WriteJSON(w, r, http.StatusOK, SepayWebhookResponse{Success: true})
}

// ---------------------------------------------------------------- Orders

// BillingOrderResponse is an order as the learner waiting to pay sees it:
// the amount, the reference to type into the transfer, and the QR that
// fills both in for them.
// BillingOrderResponse is an order as the learner waiting to pay sees it:
// the amount, the reference to type into the transfer, and the QR that
// fills both in for them.
// BillingOrderResponse is an order as the learner waiting to pay sees it:
// the amount, the reference to type into the transfer, and the QR that
// fills both in for them.
// BillingOrderResponse is an order as the learner waiting to pay sees it:
// the amount, the reference to type into the transfer, and the QR that
// fills both in for them.
type BillingOrderResponse struct {
	ID                uuid.UUID  `json:"id"`
	UserID            uuid.UUID  `json:"user_id"`
	Reference         string     `json:"reference"`
	AmountVND         int64      `json:"amount_vnd"`
	Status            string     `json:"status"`
	SubjectKind       string     `json:"subject_kind"`
	SubjectID         uuid.UUID  `json:"subject_id"`
	QRURL             string     `json:"qr_url"`
	BankCode          string     `json:"bank_code"`
	AccountNumber     string     `json:"account_number"`
	AccountHolderName string     `json:"account_holder_name"`
	ExpiresAt         time.Time  `json:"expires_at"`
	PaidAt            *time.Time `json:"paid_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required"))
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "invalid order id"))
		return
	}

	order, err := h.svc.GetOrder(ctx, id)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	// Ensure caller owns the order
	if order.UserID != actor.UserID {
		httpx.WriteProblem(w, r, domain.ErrOrderNotFound)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, BillingOrderResponse{
		ID:                order.ID,
		UserID:            order.UserID,
		Reference:         order.Reference,
		AmountVND:         order.AmountVND,
		Status:            string(order.Status),
		SubjectKind:       order.SubjectKind,
		SubjectID:         order.SubjectID,
		QRURL:             order.QRURL,
		BankCode:          order.BankCode,
		AccountNumber:     order.AccountNumber,
		AccountHolderName: order.AccountHolderName,
		ExpiresAt:         order.ExpiresAt,
		PaidAt:            order.PaidAt,
		CreatedAt:         order.CreatedAt,
		UpdatedAt:         order.UpdatedAt,
	})
}

// ---------------------------------------------------------------- Admin Unmatched

func (h *Handler) listUnmatchedTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.guard != nil {
		if err := h.guard.Require(ctx, permBillingRead); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
	}

	limit, offset := paginationFrom(r)

	list, err := h.svc.ListUnmatchedTransactions(ctx, pageInt32(limit), pageInt32(offset))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, list)
}

// ---------------------------------------------------------------- Admin Payouts

// PayoutItemResponse represents an item in the admin payout list.
type PayoutItemResponse struct {
	ID            uuid.UUID  `json:"id"`
	CreatorID     uuid.UUID  `json:"creator_id"`
	AmountVND     int64      `json:"amount_vnd"`
	Status        string     `json:"status"`
	BankReference *string    `json:"bank_reference"`
	SentAt        *time.Time `json:"sent_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

// AdminPayoutListResponse represents the paginated admin payout list.
type AdminPayoutListResponse struct {
	Items []PayoutItemResponse `json:"items"`
	Total int64                `json:"total"`
}

// FulfillPayoutRequestBody contains the bank reference and optional note to fulfill a payout.
type FulfillPayoutRequestBody struct {
	BankReference string  `json:"bank_reference"`
	Note          *string `json:"note"`
}

func (h *Handler) listPayouts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.guard != nil {
		if err := h.guard.Require(ctx, permBillingRead); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
	}

	limit, offset := paginationFrom(r)
	var statusPtr *string
	if s := r.URL.Query().Get("status"); s != "" {
		statusPtr = &s
	}

	payouts, total, err := h.svc.ListPayouts(ctx, statusPtr, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	items := make([]PayoutItemResponse, len(payouts))
	for i, p := range payouts {
		items[i] = PayoutItemResponse{
			ID:            p.ID,
			CreatorID:     p.CreatorID,
			AmountVND:     p.AmountVND,
			Status:        string(p.Status),
			BankReference: p.BankReference,
			SentAt:        p.SentAt,
			CreatedAt:     p.CreatedAt,
		}
	}

	httpx.WriteJSON(w, r, http.StatusOK, AdminPayoutListResponse{
		Items: items,
		Total: total,
	})
}

// PayoutDetailResponse represents the full details of a payout including creator bank account.
type PayoutDetailResponse struct {
	ID                uuid.UUID  `json:"id"`
	CreatorID         uuid.UUID  `json:"creator_id"`
	AmountVND         int64      `json:"amount_vnd"`
	Status            string     `json:"status"`
	BankReference     *string    `json:"bank_reference,omitempty"`
	SentAt            *time.Time `json:"sent_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	BankCode          *string    `json:"bank_code,omitempty"`
	AccountNumber     *string    `json:"account_number,omitempty"`
	AccountHolderName *string    `json:"account_holder_name,omitempty"`
}

// getPayout returns the creator's bank account, so it takes the permission
// that moves money rather than the one that reads it. BR-STUDIO-09: these
// details are never in a list response, and this is the only route that
// returns them at all.
func (h *Handler) getPayout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.guard != nil {
		if err := h.guard.Require(ctx, permBillingManage); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "invalid payout id"))
		return
	}

	p, err := h.svc.GetPayout(ctx, id)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	var bankCode, accNum, holder *string
	if h.accountReader != nil {
		bc, an, ah, err := h.accountReader.GetPayoutAccount(ctx, p.CreatorID)
		if err == nil && bc != "" {
			bankCode = &bc
			accNum = &an
			holder = &ah
		}
	}

	httpx.WriteJSON(w, r, http.StatusOK, PayoutDetailResponse{
		ID:                p.ID,
		CreatorID:         p.CreatorID,
		AmountVND:         p.AmountVND,
		Status:            string(p.Status),
		BankReference:     p.BankReference,
		SentAt:            p.SentAt,
		CreatedAt:         p.CreatedAt,
		BankCode:          bankCode,
		AccountNumber:     accNum,
		AccountHolderName: holder,
	})
}

func (h *Handler) fulfillPayout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.guard != nil {
		if err := h.guard.Require(ctx, permBillingManage); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
	}

	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required"))
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "invalid payout id"))
		return
	}

	var req FulfillPayoutRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_BODY", "failed to parse request body"))
		return
	}
	if req.BankReference == "" {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "BANK_REFERENCE_REQUIRED", "bank_reference is required"))
		return
	}

	p, err := h.svc.FulfillPayout(ctx, id, req.BankReference, actor.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, PayoutItemResponse{
		ID:            p.ID,
		CreatorID:     p.CreatorID,
		AmountVND:     p.AmountVND,
		Status:        string(p.Status),
		BankReference: p.BankReference,
		SentAt:        p.SentAt,
		CreatedAt:     p.CreatedAt,
	})
}

// ---------------------------------------------------------------- Refunds

// RefundItemResponse is one refund owed to a learner.
type RefundItemResponse struct {
	ID        uuid.UUID  `json:"id"`
	OrderID   uuid.UUID  `json:"order_id"`
	AmountVND int64      `json:"amount_vnd"`
	Reason    string     `json:"reason"`
	Status    string     `json:"status"`
	SentAt    *time.Time `json:"sent_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// AdminRefundListResponse is the paginated refund queue.
type AdminRefundListResponse struct {
	Items []RefundItemResponse `json:"items"`
	Total int64                `json:"total"`
}

// listRefunds is the queue of money owed back.
//
// SePay receives money and does not send it, so every refund is a bank
// transfer somebody makes by hand. This is the list of the ones still to make.
func (h *Handler) listRefunds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.guard != nil {
		if err := h.guard.Require(ctx, permBillingRead); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
	}

	limit, offset := paginationFrom(r)
	var statusPtr *string
	if s := r.URL.Query().Get("status"); s != "" {
		statusPtr = &s
	}

	refunds, total, err := h.svc.ListRefunds(ctx, statusPtr, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	items := make([]RefundItemResponse, len(refunds))
	for i, rf := range refunds {
		items[i] = RefundItemResponse{
			ID: rf.ID, OrderID: rf.OrderID, AmountVND: rf.AmountVND,
			Reason: rf.Reason, Status: rf.Status, SentAt: rf.SentAt, CreatedAt: rf.CreatedAt,
		}
	}
	httpx.WriteJSON(w, r, http.StatusOK, AdminRefundListResponse{Items: items, Total: total})
}

// markRefundSent records that the bank transfer has been made.
func (h *Handler) markRefundSent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.guard != nil {
		if err := h.guard.Require(ctx, permBillingManage); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "invalid refund id"))
		return
	}

	refund, err := h.svc.MarkRefundSent(ctx, id)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, RefundItemResponse{
		ID: refund.ID, OrderID: refund.OrderID, AmountVND: refund.AmountVND,
		Reason: refund.Reason, Status: refund.Status, SentAt: refund.SentAt, CreatedAt: refund.CreatedAt,
	})
}

// pageInt32 narrows an already-bounded page value. paginationFrom caps both,
// so this cannot overflow; the conversion is spelled out rather than inlined
// so that stays visible.
func pageInt32(value int) int32 {
	if value < 0 {
		return 0
	}
	if value > maxPageValue {
		return maxPageValue
	}
	return int32(value)
}

// maxPageValue bounds a page size and an offset. A limit of two billion is not
// a page anybody wants, and an int widened to int32 unchecked wraps negative.
const maxPageValue = 1_000_000

// paginationFrom reads limit and offset, bounded.
//
// Bounded rather than converted: an unbounded strconv.Atoi widened into int32
// was what gosec flagged across these handlers, and a limit of two billion is
// not a page anybody wants anyway.
func paginationFrom(r *http.Request) (limit, offset int) {
	limit, offset = 20, 0
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = min(v, 200)
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = min(v, 1_000_000)
	}
	return limit, offset
}
