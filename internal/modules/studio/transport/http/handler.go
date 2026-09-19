package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	paymentcontract "github.com/fluentra/fluentra/internal/modules/payment/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/modules/studio/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// Permissions required by studio and moderation operations.
const (
	PermModerationRead = "moderation.read"
	PermContentReview  = "content.review"
	PermContentPublish = "content.publish"
)

// Guard is the authorization interface required for moderation endpoints.
type Guard interface {
	Require(ctx context.Context, permission string) error
}

// StudioService describes use cases needed by HTTP handlers.
type StudioService interface {
	GetCreatorProfile(ctx context.Context, userID uuid.UUID) (*domain.CreatorProfile, error)
	UpsertCreatorProfile(ctx context.Context, userID uuid.UUID, bio, headline string) (*domain.CreatorProfile, error)
	GetPayoutAccount(ctx context.Context, creatorID uuid.UUID) (*domain.PayoutAccount, error)
	UpsertPayoutAccount(ctx context.Context, creatorID uuid.UUID, bankCode, accountNumber, accountHolderName string, isDefault bool) (*domain.PayoutAccount, error)

	CreateDraft(ctx context.Context, ownerID uuid.UUID, req service.CreateDraftRequest) (*domain.CourseDraft, error)
	UpdateDraft(ctx context.Context, ownerID, draftID uuid.UUID, req service.UpdateDraftRequest) (*domain.CourseDraft, error)
	GetDraft(ctx context.Context, ownerID, draftID uuid.UUID) (*domain.CourseDraft, error)
	ListDrafts(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]*domain.CourseDraft, int64, error)
	SubmitDraft(ctx context.Context, ownerID, draftID uuid.UUID) (*domain.Submission, error)

	ListModerationQueue(ctx context.Context, limit, offset int) ([]contract.ModerationQueueItem, int64, error)
	ApproveSubmission(ctx context.Context, reviewerID, submissionID uuid.UUID) (*domain.Submission, error)
	RejectSubmission(ctx context.Context, reviewerID, submissionID uuid.UUID, targetStatus, feedback string) (*domain.Submission, error)

	ClaimCourse(ctx context.Context, userID, courseID uuid.UUID) (*domain.Purchase, error)
	PurchaseCourse(ctx context.Context, userID, courseID uuid.UUID) (*paymentcontract.Order, error)
	ListUserPurchases(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Purchase, int64, error)
	RefundPurchase(ctx context.Context, userID, purchaseID uuid.UUID) error
}

// Handler serves HTTP endpoints for creator studio and moderation.
type Handler struct {
	svc   StudioService
	guard Guard
}

// NewHandler constructs a new Handler.
func NewHandler(svc StudioService, guard Guard) *Handler {
	return &Handler{
		svc:   svc,
		guard: guard,
	}
}

// Routes mounts creator-facing studio endpoints under the authenticated router.
func (h *Handler) Routes(router chi.Router) {
	router.Post("/studio/creator/profile", h.upsertCreatorProfile)
	router.Post("/studio/creator/payout-account", h.upsertPayoutAccount)

	router.Get("/studio/courses", h.listCourseDrafts)
	router.Post("/studio/courses", h.createCourseDraft)
	router.Get("/studio/courses/{id}", h.getCourseDraft)
	router.Put("/studio/courses/{id}", h.updateCourseDraft)
	router.Post("/studio/courses/{id}/submit", h.submitCourseDraft)

	// Commerce / Purchases (Step 6)
	router.Post("/courses/{id}/claim", h.claimCourse)
	router.Post("/courses/{id}/purchase", h.purchaseCourse)
	router.Get("/me/purchases", h.listPurchases)
	router.Post("/me/purchases/{id}/refund", h.refundPurchase)
}

// ModerationRoutes mounts staff moderation endpoints under the admin/authenticated router.
func (h *Handler) ModerationRoutes(router chi.Router) {
	router.Get("/moderation/courses", h.listModerationQueue)
	router.Post("/moderation/courses/{id}/approve", h.approveCourse)
	router.Post("/moderation/courses/{id}/reject", h.rejectCourse)
}

// ---------------------------------------------------------------- DTOs

type CreatorProfileResponse struct {
	UserID         uuid.UUID `json:"user_id"`
	Bio            string    `json:"bio"`
	Headline       string    `json:"headline"`
	PayoutEligible bool      `json:"payout_eligible"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UpsertCreatorProfileRequest struct {
	Bio      string `json:"bio"`
	Headline string `json:"headline"`
}

type PayoutAccountResponse struct {
	ID                uuid.UUID `json:"id"`
	CreatorID         uuid.UUID `json:"creator_id"`
	BankCode          string    `json:"bank_code"`
	AccountNumber     string    `json:"account_number"`
	AccountHolderName string    `json:"account_holder_name"`
	IsDefault         bool      `json:"is_default"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type CreatePayoutAccountRequest struct {
	BankCode          string `json:"bank_code"`
	AccountNumber     string `json:"account_number"`
	AccountHolderName string `json:"account_holder_name"`
	IsDefault         bool   `json:"is_default"`
}

type CourseDraftResponse struct {
	ID              uuid.UUID       `json:"id"`
	OwnerID         uuid.UUID       `json:"owner_id"`
	Title           string          `json:"title"`
	Slug            string          `json:"slug"`
	Description     string          `json:"description"`
	CEFRLevel       string          `json:"cefr_level"`
	TopicTaxonomyID *uuid.UUID      `json:"topic_taxonomy_id,omitempty"`
	PriceVND        int64           `json:"price_vnd"`
	Status          string          `json:"status"`
	Structure       json.RawMessage `json:"structure"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type CourseDraftListResponse struct {
	Items []CourseDraftResponse `json:"items"`
	Total int64                 `json:"total"`
}

type CourseSubmissionResponse struct {
	ID                 uuid.UUID       `json:"id"`
	DraftID            uuid.UUID       `json:"draft_id"`
	Version            int             `json:"version"`
	Status             string          `json:"status"`
	SubmittedBy        uuid.UUID       `json:"submitted_by"`
	ReviewerID         *uuid.UUID      `json:"reviewer_id,omitempty"`
	Feedback           *string         `json:"feedback,omitempty"`
	VerificationReport json.RawMessage `json:"verification_report,omitempty"`
	SubmittedAt        time.Time       `json:"submitted_at"`
	ReviewedAt         *time.Time      `json:"reviewed_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type ModerationQueueListResponse struct {
	Items []contract.ModerationQueueItem `json:"items"`
	Total int64                          `json:"total"`
}

type ReviewSubmissionRequest struct {
	Feedback string `json:"feedback"`
	Status   string `json:"status"`
}

// ---------------------------------------------------------------- Creator Handlers

func (h *Handler) upsertCreatorProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required"))
		return
	}

	var req UpsertCreatorProfileRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	profile, err := h.svc.UpsertCreatorProfile(ctx, actor.UserID, req.Bio, req.Headline)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, CreatorProfileResponse{
		UserID:         profile.UserID,
		Bio:            profile.Bio,
		Headline:       profile.Headline,
		PayoutEligible: profile.PayoutEligible,
		CreatedAt:      profile.CreatedAt,
		UpdatedAt:      profile.UpdatedAt,
	})
}

func (h *Handler) upsertPayoutAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required"))
		return
	}

	var req CreatePayoutAccountRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	account, err := h.svc.UpsertPayoutAccount(ctx, actor.UserID, req.BankCode, req.AccountNumber, req.AccountHolderName, req.IsDefault)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, PayoutAccountResponse{
		ID:                account.ID,
		CreatorID:         account.CreatorID,
		BankCode:          account.BankCode,
		AccountNumber:     account.AccountNumber,
		AccountHolderName: account.AccountHolderName,
		IsDefault:         account.IsDefault,
		CreatedAt:         account.CreatedAt,
		UpdatedAt:         account.UpdatedAt,
	})
}

func (h *Handler) listCourseDrafts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required"))
		return
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil {
			offset = val
		}
	}

	drafts, total, err := h.svc.ListDrafts(ctx, actor.UserID, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	items := make([]CourseDraftResponse, len(drafts))
	for i, d := range drafts {
		items[i] = toCourseDraftResponse(d)
	}

	httpx.WriteJSON(w, r, http.StatusOK, CourseDraftListResponse{
		Items: items,
		Total: total,
	})
}

func (h *Handler) createCourseDraft(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required"))
		return
	}

	var req service.CreateDraftRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	draft, err := h.svc.CreateDraft(ctx, actor.UserID, req)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, toCourseDraftResponse(draft))
}

func (h *Handler) getCourseDraft(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required"))
		return
	}

	idStr := chi.URLParam(r, "id")
	draftID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid course draft ID"))
		return
	}

	draft, err := h.svc.GetDraft(ctx, actor.UserID, draftID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toCourseDraftResponse(draft))
}

func (h *Handler) updateCourseDraft(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required"))
		return
	}

	idStr := chi.URLParam(r, "id")
	draftID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid course draft ID"))
		return
	}

	var req service.UpdateDraftRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	draft, err := h.svc.UpdateDraft(ctx, actor.UserID, draftID, req)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toCourseDraftResponse(draft))
}

func (h *Handler) submitCourseDraft(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required"))
		return
	}

	idStr := chi.URLParam(r, "id")
	draftID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid course draft ID"))
		return
	}

	sub, err := h.svc.SubmitDraft(ctx, actor.UserID, draftID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toSubmissionResponse(sub))
}

// ---------------------------------------------------------------- Moderation Handlers

func (h *Handler) listModerationQueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermModerationRead); err != nil {
			// Also allow content.review
			if err2 := h.guard.Require(ctx, PermContentReview); err2 != nil {
				httpx.WriteProblem(w, r, err)
				return
			}
		}
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil {
			offset = val
		}
	}

	items, total, err := h.svc.ListModerationQueue(ctx, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, ModerationQueueListResponse{
		Items: items,
		Total: total,
	})
}

func (h *Handler) approveCourse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermContentPublish); err != nil {
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
	submissionID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid submission ID"))
		return
	}

	sub, err := h.svc.ApproveSubmission(ctx, actor.UserID, submissionID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toSubmissionResponse(sub))
}

func (h *Handler) rejectCourse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermContentReview); err != nil {
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
	submissionID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid submission ID"))
		return
	}

	var req ReviewSubmissionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	sub, err := h.svc.RejectSubmission(ctx, actor.UserID, submissionID, req.Status, req.Feedback)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toSubmissionResponse(sub))
}

func toCourseDraftResponse(d *domain.CourseDraft) CourseDraftResponse {
	return CourseDraftResponse{
		ID:              d.ID,
		OwnerID:         d.OwnerID,
		Title:           d.Title,
		Slug:            d.Slug,
		Description:     d.Description,
		CEFRLevel:       d.CEFRLevel,
		TopicTaxonomyID: d.TopicTaxonomyID,
		PriceVND:        d.PriceVND,
		Status:          d.Status,
		Structure:       d.Structure,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

func toSubmissionResponse(s *domain.Submission) CourseSubmissionResponse {
	return CourseSubmissionResponse{
		ID:                 s.ID,
		DraftID:            s.DraftID,
		Version:            s.Version,
		Status:             s.Status,
		SubmittedBy:        s.SubmittedBy,
		ReviewerID:         s.ReviewerID,
		Feedback:           s.Feedback,
		VerificationReport: s.VerificationReport,
		SubmittedAt:        s.SubmittedAt,
		ReviewedAt:         s.ReviewedAt,
		CreatedAt:          s.CreatedAt,
		UpdatedAt:          s.UpdatedAt,
	}
}

// ---------------------------------------------------------------- Purchases & Claim Handlers

func (h *Handler) claimCourse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	idStr := chi.URLParam(r, "id")
	courseID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid course ID"))
		return
	}

	purchase, err := h.svc.ClaimCourse(ctx, actor.UserID, courseID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toCoursePurchaseResponse(purchase))
}

func (h *Handler) purchaseCourse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	idStr := chi.URLParam(r, "id")
	courseID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid course ID"))
		return
	}

	order, err := h.svc.PurchaseCourse(ctx, actor.UserID, courseID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, PurchaseOrderResponse{
		OrderID:           order.ID,
		Reference:         order.Reference,
		AmountVND:         order.AmountVND,
		QRURL:             order.QRURL,
		BankCode:          order.BankCode,
		AccountNumber:     order.AccountNumber,
		AccountHolderName: order.AccountHolderName,
		ExpiresAt:         order.ExpiresAt,
	})
}

func (h *Handler) listPurchases(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	limit := 20
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	offset := 0
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	purchases, total, err := h.svc.ListUserPurchases(ctx, actor.UserID, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	items := make([]CoursePurchaseResponse, len(purchases))
	for i, p := range purchases {
		items[i] = toCoursePurchaseResponse(p)
	}

	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}

func (h *Handler) refundPurchase(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	idStr := chi.URLParam(r, "id")
	purchaseID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid purchase ID"))
		return
	}

	if err := h.svc.RefundPurchase(ctx, actor.UserID, purchaseID); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		"success": true,
		"message": "Refund processed successfully",
	})
}

type CoursePurchaseResponse struct {
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	CourseID     uuid.UUID  `json:"course_id"`
	OrderID      *uuid.UUID `json:"order_id,omitempty"`
	PricePaidVND int64      `json:"price_paid_vnd"`
	GrantedAt    time.Time  `json:"granted_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	RevokeReason *string    `json:"revoke_reason,omitempty"`
}

type PurchaseOrderResponse struct {
	OrderID           uuid.UUID `json:"order_id"`
	Reference         string    `json:"reference"`
	AmountVND         int64     `json:"amount_vnd"`
	QRURL             string    `json:"qr_url"`
	BankCode          string    `json:"bank_code"`
	AccountNumber     string    `json:"account_number"`
	AccountHolderName string    `json:"account_holder_name"`
	ExpiresAt         time.Time `json:"expires_at"`
}

func toCoursePurchaseResponse(p *domain.Purchase) CoursePurchaseResponse {
	return CoursePurchaseResponse{
		ID:           p.ID,
		UserID:       p.UserID,
		CourseID:     p.CourseID,
		OrderID:      p.OrderID,
		PricePaidVND: p.PricePaidVND,
		GrantedAt:    p.GrantedAt,
		RevokedAt:    p.RevokedAt,
		RevokeReason: p.RevokeReason,
	}
}

