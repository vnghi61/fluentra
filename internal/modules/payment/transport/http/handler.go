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

// Handler handles payment and SePay webhook HTTP requests.
type Handler struct {
	svc   service.Service
	guard Guard
}

// NewHandler creates a new payment HTTP handler.
func NewHandler(svc service.Service, guard Guard) *Handler {
	return &Handler{
		svc:   svc,
		guard: guard,
	}
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
}

// ---------------------------------------------------------------- Webhook

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

	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	if err := h.svc.HandleSepayWebhook(ctx, authHeader, clientIP, bodyBytes, &payload); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	// SePay strictly requires {"success": true} with 200/201 response.
	httpx.WriteJSON(w, r, http.StatusOK, SepayWebhookResponse{Success: true})
}

// ---------------------------------------------------------------- Orders

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
		if err := h.guard.Require(ctx, "admin.dashboard"); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
	}

	limit := int32(50)
	offset := int32(0)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = int32(v)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}

	list, err := h.svc.ListUnmatchedTransactions(ctx, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, list)
}
