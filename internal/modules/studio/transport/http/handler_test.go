package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	paymentcontract "github.com/fluentra/fluentra/internal/modules/payment/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/modules/studio/service"
	studiohttp "github.com/fluentra/fluentra/internal/modules/studio/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type mockStudioService struct {
	profile       *domain.CreatorProfile
	payoutAccount *domain.PayoutAccount
	queueItems    []contract.ModerationQueueItem
	err           error
	// The recorded arguments prove the handler scopes reads to the actor and
	// resolves an omitted is_default, rather than trusting the request body.
	lastPayoutIsDefault  bool
	lastPayoutCreator    uuid.UUID
	lastProfileUser      uuid.UUID
	lastDraftDetailOwner uuid.UUID
	latestSubmission     *domain.Submission
}

func (m *mockStudioService) TakedownCourse(
	_ context.Context, _, courseID uuid.UUID, reason string,
) (*domain.Takedown, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.Takedown{ID: uuid.New(), CourseID: courseID, Reason: reason}, nil
}

func (m *mockStudioService) ReinstateCourse(
	_ context.Context, _, courseID uuid.UUID,
) (*domain.Takedown, error) {
	if m.err != nil {
		return nil, m.err
	}
	now := time.Now()
	return &domain.Takedown{ID: uuid.New(), CourseID: courseID, ReinstatedAt: &now}, nil
}

func (m *mockStudioService) SuspendCreator(
	_ context.Context, creatorID uuid.UUID, reason string,
) (*domain.CreatorProfile, error) {
	if m.err != nil {
		return nil, m.err
	}
	now := time.Now()
	return &domain.CreatorProfile{UserID: creatorID, SuspendedAt: &now, SuspendedReason: &reason}, nil
}

func (m *mockStudioService) ReinstateCreator(
	_ context.Context, creatorID uuid.UUID,
) (*domain.CreatorProfile, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.CreatorProfile{UserID: creatorID}, nil
}

func (m *mockStudioService) GetCreatorProfile(_ context.Context, userID uuid.UUID) (*domain.CreatorProfile, error) {
	m.lastProfileUser = userID
	return m.profile, m.err
}

func (m *mockStudioService) UpsertCreatorProfile(
	_ context.Context, userID uuid.UUID, bio, headline string,
) (*domain.CreatorProfile, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.CreatorProfile{
		UserID:         userID,
		Bio:            bio,
		Headline:       headline,
		PayoutEligible: false,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}, nil
}

func (m *mockStudioService) GetPayoutAccount(_ context.Context, creatorID uuid.UUID) (*domain.PayoutAccount, error) {
	m.lastPayoutCreator = creatorID
	return m.payoutAccount, m.err
}

func (m *mockStudioService) UpsertPayoutAccount(
	_ context.Context, creatorID uuid.UUID, bankCode, accountNumber, accountHolderName string, isDefault bool,
) (*domain.PayoutAccount, error) {
	m.lastPayoutCreator = creatorID
	m.lastPayoutIsDefault = isDefault
	if m.err != nil {
		return nil, m.err
	}
	return &domain.PayoutAccount{
		ID:                uuid.New(),
		CreatorID:         creatorID,
		BankCode:          bankCode,
		AccountNumber:     accountNumber,
		AccountHolderName: accountHolderName,
		IsDefault:         isDefault,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}, nil
}

func (m *mockStudioService) CreateDraft(
	_ context.Context, ownerID uuid.UUID, req service.CreateDraftRequest,
) (*domain.CourseDraft, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.CourseDraft{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		Title:       req.Title,
		Slug:        req.Slug,
		Description: req.Description,
		CEFRLevel:   req.CEFRLevel,
		Status:      domain.DraftStatusDraft,
		Structure:   req.Structure,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (m *mockStudioService) UpdateDraft(
	_ context.Context, ownerID, draftID uuid.UUID, _ service.UpdateDraftRequest,
) (*domain.CourseDraft, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.CourseDraft{
		ID:        draftID,
		OwnerID:   ownerID,
		Title:     "Updated Title",
		Status:    domain.DraftStatusDraft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockStudioService) GetDraft(_ context.Context, ownerID, draftID uuid.UUID) (*domain.CourseDraft, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.CourseDraft{
		ID:        draftID,
		OwnerID:   ownerID,
		Title:     "Sample Draft",
		Status:    domain.DraftStatusDraft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockStudioService) GetDraftDetail(
	_ context.Context, ownerID, draftID uuid.UUID,
) (*domain.CourseDraft, *domain.Submission, error) {
	m.lastDraftDetailOwner = ownerID
	if m.err != nil {
		return nil, nil, m.err
	}
	return &domain.CourseDraft{
		ID:        draftID,
		OwnerID:   ownerID,
		Title:     "Sample Draft",
		Status:    domain.DraftStatusDraft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, m.latestSubmission, nil
}

func (m *mockStudioService) ListDrafts(
	_ context.Context, ownerID uuid.UUID, _, _ int,
) ([]*domain.CourseDraft, int64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	items := []*domain.CourseDraft{
		{
			ID:        uuid.New(),
			OwnerID:   ownerID,
			Title:     "Draft 1",
			Status:    domain.DraftStatusDraft,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	return items, 1, nil
}

func (m *mockStudioService) SubmitDraft(_ context.Context, ownerID, draftID uuid.UUID) (*domain.Submission, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.Submission{
		ID:          uuid.New(),
		DraftID:     draftID,
		Version:     1,
		Status:      domain.SubmissionStatusInReview,
		SubmittedBy: ownerID,
		SubmittedAt: time.Now(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (m *mockStudioService) ListModerationQueue(
	_ context.Context, _, _ int,
) ([]contract.ModerationQueueItem, int64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.queueItems, int64(len(m.queueItems)), nil
}

func (m *mockStudioService) ApproveSubmission(
	_ context.Context, reviewerID, submissionID uuid.UUID,
) (*domain.Submission, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.Submission{
		ID:          submissionID,
		Version:     1,
		Status:      domain.SubmissionStatusApproved,
		ReviewerID:  &reviewerID,
		SubmittedAt: time.Now(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (m *mockStudioService) RejectSubmission(
	_ context.Context, reviewerID, submissionID uuid.UUID, targetStatus, feedback string,
) (*domain.Submission, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.Submission{
		ID:          submissionID,
		Version:     1,
		Status:      targetStatus,
		ReviewerID:  &reviewerID,
		Feedback:    &feedback,
		SubmittedAt: time.Now(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (m *mockStudioService) ClaimCourse(_ context.Context, userID, courseID uuid.UUID) (*domain.Purchase, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.Purchase{
		ID:        uuid.New(),
		UserID:    userID,
		CourseID:  courseID,
		GrantedAt: time.Now(),
	}, nil
}

func (m *mockStudioService) PurchaseCourse(_ context.Context, userID, _ uuid.UUID) (*paymentcontract.Order, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &paymentcontract.Order{
		ID:        uuid.New(),
		UserID:    userID,
		Reference: "FLU12345",
		AmountVND: 99000,
	}, nil
}

func (m *mockStudioService) ListUserPurchases(
	_ context.Context, _ uuid.UUID, _, _ int,
) ([]*domain.Purchase, int64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return nil, 0, nil
}

func (m *mockStudioService) RefundPurchase(_ context.Context, _, _ uuid.UUID) error {
	return m.err
}

func (m *mockStudioService) GetEarnings(_ context.Context, _ uuid.UUID) (*domain.EarningsSummary, error) {
	if m.err != nil {
		return nil, m.err
	}
	masked := "******8888"
	bankCode := "VCB"
	holder := "TEST CREATOR"
	return &domain.EarningsSummary{
		AvailableBalanceVND:     1500000,
		LifetimeEarningsVND:     2000000,
		PendingPayoutVND:        500000,
		TotalPaidOutVND:         0,
		PayoutThresholdVND:      500000,
		CanRequestPayout:        true,
		PayoutAccountConfigured: true,
		PayoutBankCode:          &bankCode,
		PayoutAccountHolder:     &holder,
		PayoutMaskedAccount:     &masked,
	}, nil
}

func (m *mockStudioService) RequestPayout(
	_ context.Context, creatorID uuid.UUID, requestedAmount *int64,
) (*paymentcontract.Payout, error) {
	if m.err != nil {
		return nil, m.err
	}
	amt := int64(1000000)
	if requestedAmount != nil && *requestedAmount > 0 {
		amt = *requestedAmount
	}
	return &paymentcontract.Payout{
		ID:        uuid.New(),
		CreatorID: creatorID,
		AmountVND: amt,
		Status:    "pending",
		CreatedAt: time.Now(),
	}, nil
}

type mockGuard struct {
	allowed bool
}

func (g *mockGuard) Require(_ context.Context, _ string) error {
	if !g.allowed {
		return apperr.New(apperr.Forbidden, "FORBIDDEN", "Forbidden")
	}
	return nil
}

func setupRouter(svc *mockStudioService, guard *mockGuard) chi.Router {
	r := chi.NewRouter()
	handler := studiohttp.NewHandler(svc, guard)
	handler.Routes(r)
	handler.ModerationRoutes(r)
	return r
}

func TestStudioHandler_CreatorProfile(t *testing.T) {
	svc := &mockStudioService{}
	router := setupRouter(svc, &mockGuard{allowed: true})

	actorID := uuid.New()
	body := []byte(`{"bio":"Teacher","headline":"Expert"}`)
	req := httptest.NewRequest(http.MethodPost, "/studio/creator/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}

func TestStudioHandler_DraftLifecycle(t *testing.T) {
	svc := &mockStudioService{}
	router := setupRouter(svc, &mockGuard{allowed: true})
	actorID := uuid.New()

	// 1. Create Draft
	createBody := []byte(`{"title":"New Course","slug":"new-course","cefr_level":"B1"}`)
	reqCreate := httptest.NewRequest(http.MethodPost, "/studio/courses", bytes.NewReader(createBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate = reqCreate.WithContext(httpx.WithActor(reqCreate.Context(), httpx.Actor{UserID: actorID}))
	recCreate := httptest.NewRecorder()
	router.ServeHTTP(recCreate, reqCreate)

	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", recCreate.Code)
	}

	// 2. Submit Draft
	draftID := uuid.New()
	reqSubmit := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/studio/courses/%s/submit", draftID), nil)
	reqSubmit = reqSubmit.WithContext(httpx.WithActor(reqSubmit.Context(), httpx.Actor{UserID: actorID}))
	recSubmit := httptest.NewRecorder()
	router.ServeHTTP(recSubmit, reqSubmit)

	if recSubmit.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recSubmit.Code)
	}
}

func TestStudioHandler_ModerationRoutes(t *testing.T) {
	svc := &mockStudioService{}
	guard := &mockGuard{allowed: true}
	router := setupRouter(svc, guard)
	reviewerID := uuid.New()
	submissionID := uuid.New()

	// 1. List Queue
	reqList := httptest.NewRequest(http.MethodGet, "/moderation/courses", nil)
	reqList = reqList.WithContext(httpx.WithActor(reqList.Context(), httpx.Actor{UserID: reviewerID}))
	recList := httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list queue, got %d", recList.Code)
	}

	// 2. Approve Course
	reqApprove := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/moderation/courses/%s/approve", submissionID), nil)
	reqApprove = reqApprove.WithContext(httpx.WithActor(reqApprove.Context(), httpx.Actor{UserID: reviewerID}))
	recApprove := httptest.NewRecorder()
	router.ServeHTTP(recApprove, reqApprove)
	if recApprove.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for approve, got %d", recApprove.Code)
	}

	// 3. Reject Course
	rejectBody := []byte(`{"status":"changes_requested","feedback":"Please fix exercise 1"}`)
	rejectURL := fmt.Sprintf("/moderation/courses/%s/reject", submissionID)
	reqReject := httptest.NewRequest(http.MethodPost, rejectURL, bytes.NewReader(rejectBody))
	reqReject.Header.Set("Content-Type", "application/json")
	reqReject = reqReject.WithContext(httpx.WithActor(reqReject.Context(), httpx.Actor{UserID: reviewerID}))
	recReject := httptest.NewRecorder()
	router.ServeHTTP(recReject, reqReject)
	if recReject.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for reject, got %d", recReject.Code)
	}

	// 4. Forbidden Guard Test
	guard.allowed = false
	reqForbidden := httptest.NewRequest(http.MethodGet, "/moderation/courses", nil)
	reqForbidden = reqForbidden.WithContext(httpx.WithActor(reqForbidden.Context(), httpx.Actor{UserID: reviewerID}))
	recForbidden := httptest.NewRecorder()
	router.ServeHTTP(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when guard fails, got %d", recForbidden.Code)
	}
}

func TestStudioHandler_GetCreatorProfile(t *testing.T) {
	actorID := uuid.New()

	t.Run("returns the caller's profile", func(t *testing.T) {
		svc := &mockStudioService{profile: &domain.CreatorProfile{
			UserID: actorID, Bio: "Teacher", Headline: "Expert", PayoutEligible: true,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}}
		router := setupRouter(svc, &mockGuard{allowed: true})

		req := httptest.NewRequest(http.MethodGet, "/studio/creator/profile", nil)
		req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		if svc.lastProfileUser != actorID {
			t.Errorf("profile read was scoped to %s, want the actor %s", svc.lastProfileUser, actorID)
		}
	})

	t.Run("missing profile is a safe 404", func(t *testing.T) {
		svc := &mockStudioService{err: domain.ErrProfileNotFound}
		router := setupRouter(svc, &mockGuard{allowed: true})

		req := httptest.NewRequest(http.MethodGet, "/studio/creator/profile", nil)
		req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d: %s", rec.Code, rec.Body.String())
		}
		var problem apperr.Problem
		if err := json.NewDecoder(rec.Body).Decode(&problem); err != nil {
			t.Fatalf("decode problem: %v", err)
		}
		if problem.Code != "CREATOR_PROFILE_NOT_FOUND" {
			t.Errorf("problem code = %q, want CREATOR_PROFILE_NOT_FOUND", problem.Code)
		}
	})

	t.Run("nil profile is a 404, never a nil dereference", func(t *testing.T) {
		svc := &mockStudioService{}
		router := setupRouter(svc, &mockGuard{allowed: true})
		req := httptest.NewRequest(http.MethodGet, "/studio/creator/profile", nil)
		req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})
}

func TestStudioHandler_GetPayoutAccount(t *testing.T) {
	actorID := uuid.New()
	account := &domain.PayoutAccount{
		ID: uuid.New(), CreatorID: actorID, BankCode: "MB",
		AccountNumber: "0987654321", AccountHolderName: "NGUYEN VAN A",
		IsDefault: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	t.Run("returns the caller's default account", func(t *testing.T) {
		svc := &mockStudioService{payoutAccount: account}
		router := setupRouter(svc, &mockGuard{allowed: true})
		req := httptest.NewRequest(http.MethodGet, "/studio/creator/payout-account", nil)
		req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		if svc.lastPayoutCreator != actorID {
			t.Errorf("account read was scoped to %s, want the actor %s", svc.lastPayoutCreator, actorID)
		}
	})

	t.Run("no account is a 404 Problem Details", func(t *testing.T) {
		svc := &mockStudioService{}
		router := setupRouter(svc, &mockGuard{allowed: true})
		req := httptest.NewRequest(http.MethodGet, "/studio/creator/payout-account", nil)
		req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d: %s", rec.Code, rec.Body.String())
		}
		var problem apperr.Problem
		if err := json.NewDecoder(rec.Body).Decode(&problem); err != nil {
			t.Fatalf("decode problem: %v", err)
		}
		if problem.Code != "PAYOUT_ACCOUNT_NOT_FOUND" {
			t.Errorf("problem code = %q, want PAYOUT_ACCOUNT_NOT_FOUND", problem.Code)
		}
	})
}

func TestStudioHandler_PayoutAccountDefault(t *testing.T) {
	actorID := uuid.New()

	t.Run("omitted is_default follows the documented true", func(t *testing.T) {
		svc := &mockStudioService{}
		router := setupRouter(svc, &mockGuard{allowed: true})
		body := []byte(`{"bank_code":"MB","account_number":"0987654321","account_holder_name":"NGUYEN VAN A"}`)
		req := httptest.NewRequest(http.MethodPost, "/studio/creator/payout-account", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		if !svc.lastPayoutIsDefault {
			t.Error("an omitted is_default should be resolved to true")
		}
	})

	t.Run("explicit false is distinguishable", func(t *testing.T) {
		svc := &mockStudioService{}
		router := setupRouter(svc, &mockGuard{allowed: true})
		body := []byte(`{"bank_code":"MB","account_number":"0987654321",` +
			`"account_holder_name":"NGUYEN VAN A","is_default":false}`)
		req := httptest.NewRequest(http.MethodPost, "/studio/creator/payout-account", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		if svc.lastPayoutIsDefault {
			t.Error("an explicit false must not be treated as the default")
		}
	})
}

func TestStudioHandler_DuplicateSlugConflict(t *testing.T) {
	actorID := uuid.New()
	svc := &mockStudioService{err: domain.ErrDraftSlugConflict}
	router := setupRouter(svc, &mockGuard{allowed: true})

	createBody := []byte(`{"title":"New Course","slug":"new-course","cefr_level":"B1"}`)
	reqCreate := httptest.NewRequest(http.MethodPost, "/studio/courses", bytes.NewReader(createBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate = reqCreate.WithContext(httpx.WithActor(reqCreate.Context(), httpx.Actor{UserID: actorID}))
	recCreate := httptest.NewRecorder()
	router.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusConflict {
		t.Fatalf("create: expected 409 Conflict, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	draftID := uuid.New()
	updateBody := []byte(`{"slug":"taken"}`)
	reqUpdate := httptest.NewRequest(
		http.MethodPut, fmt.Sprintf("/studio/courses/%s", draftID), bytes.NewReader(updateBody),
	)
	reqUpdate.Header.Set("Content-Type", "application/json")
	reqUpdate = reqUpdate.WithContext(httpx.WithActor(reqUpdate.Context(), httpx.Actor{UserID: actorID}))
	recUpdate := httptest.NewRecorder()
	router.ServeHTTP(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusConflict {
		t.Fatalf("update: expected 409 Conflict, got %d: %s", recUpdate.Code, recUpdate.Body.String())
	}

	var problem apperr.Problem
	if err := json.NewDecoder(recCreate.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Code != "DRAFT_SLUG_EXISTS" {
		t.Errorf("problem code = %q, want DRAFT_SLUG_EXISTS", problem.Code)
	}
}

func TestStudioHandler_DraftDetailLatestSubmission(t *testing.T) {
	actorID := uuid.New()
	draftID := uuid.New()
	version := 3
	svc := &mockStudioService{latestSubmission: &domain.Submission{
		ID: uuid.New(), DraftID: draftID, Version: version,
		Status: domain.SubmissionStatusInReview, SubmittedBy: actorID,
		SubmittedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}}
	router := setupRouter(svc, &mockGuard{allowed: true})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/studio/courses/%s", draftID), nil)
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp studiohttp.CourseDraftResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode draft detail: %v", err)
	}
	if resp.LatestSubmission == nil || resp.LatestSubmission.Version != version {
		t.Fatalf("expected latest_submission version %d, got %+v", version, resp.LatestSubmission)
	}
	if svc.lastDraftDetailOwner != actorID {
		t.Errorf("detail read was scoped to %s, want the actor %s", svc.lastDraftDetailOwner, actorID)
	}

	// No submission: the optional field is absent, not null.
	svc.latestSubmission = nil
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	var raw map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode draft detail: %v", err)
	}
	if _, present := raw["latest_submission"]; present {
		t.Error("a draft with no submission must omit latest_submission")
	}
}

func TestStudioHandler_InternalErrorDoesNotLeakDetail(t *testing.T) {
	actorID := uuid.New()
	svc := &mockStudioService{err: errors.New(
		`pq: duplicate key value violates constraint "uq_payout" account 0987654321`)}
	router := setupRouter(svc, &mockGuard{allowed: true})

	req := httptest.NewRequest(http.MethodGet, "/studio/creator/payout-account", nil)
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, leak := range []string{"0987654321", "pq:", "uq_payout"} {
		if strings.Contains(body, leak) {
			t.Errorf("internal error response leaked %q: %s", leak, body)
		}
	}
}

func TestStudioHandler_EarningsAndPayouts(t *testing.T) {
	svc := &mockStudioService{}
	router := setupRouter(svc, &mockGuard{allowed: true})

	creatorID := uuid.New()

	// 1. GET /me/studio/earnings
	reqEarnings := httptest.NewRequest(http.MethodGet, "/me/studio/earnings", nil)
	reqEarnings = reqEarnings.WithContext(httpx.WithActor(reqEarnings.Context(), httpx.Actor{UserID: creatorID}))
	recEarnings := httptest.NewRecorder()
	router.ServeHTTP(recEarnings, reqEarnings)

	if recEarnings.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for earnings, got %d: %s", recEarnings.Code, recEarnings.Body.String())
	}

	var earningsResp studiohttp.CreatorEarningsSummaryResponse
	if err := json.NewDecoder(recEarnings.Body).Decode(&earningsResp); err != nil {
		t.Fatalf("decode earnings response: %v", err)
	}
	if earningsResp.AvailableBalanceVND != 1500000 {
		t.Errorf("expected available balance 1500000, got %d", earningsResp.AvailableBalanceVND)
	}
	if !earningsResp.CanRequestPayout {
		t.Errorf("expected can_request_payout = true")
	}

	// 2. POST /me/studio/payouts
	body := []byte(`{"amount_vnd": 1000000}`)
	reqPayout := httptest.NewRequest(http.MethodPost, "/me/studio/payouts", bytes.NewReader(body))
	reqPayout.Header.Set("Content-Type", "application/json")
	reqPayout = reqPayout.WithContext(httpx.WithActor(reqPayout.Context(), httpx.Actor{UserID: creatorID}))
	recPayout := httptest.NewRecorder()
	router.ServeHTTP(recPayout, reqPayout)

	if recPayout.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for payout, got %d: %s", recPayout.Code, recPayout.Body.String())
	}

	var payoutResp studiohttp.PayoutResponse
	if err := json.NewDecoder(recPayout.Body).Decode(&payoutResp); err != nil {
		t.Fatalf("decode payout response: %v", err)
	}
	if payoutResp.AmountVND != 1000000 {
		t.Errorf("expected payout amount 1000000, got %d", payoutResp.AmountVND)
	}
	if payoutResp.Status != "pending" {
		t.Errorf("expected payout status pending, got %s", payoutResp.Status)
	}
}
