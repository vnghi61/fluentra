package http_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	draft         *domain.CourseDraft
	submission    *domain.Submission
	queueItems    []contract.ModerationQueueItem
	err           error
}

func (m *mockStudioService) GetCreatorProfile(ctx context.Context, userID uuid.UUID) (*domain.CreatorProfile, error) {
	return m.profile, m.err
}

func (m *mockStudioService) UpsertCreatorProfile(ctx context.Context, userID uuid.UUID, bio, headline string) (*domain.CreatorProfile, error) {
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

func (m *mockStudioService) GetPayoutAccount(ctx context.Context, creatorID uuid.UUID) (*domain.PayoutAccount, error) {
	return m.payoutAccount, m.err
}

func (m *mockStudioService) UpsertPayoutAccount(ctx context.Context, creatorID uuid.UUID, bankCode, accountNumber, accountHolderName string, isDefault bool) (*domain.PayoutAccount, error) {
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

func (m *mockStudioService) CreateDraft(ctx context.Context, ownerID uuid.UUID, req service.CreateDraftRequest) (*domain.CourseDraft, error) {
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

func (m *mockStudioService) UpdateDraft(ctx context.Context, ownerID, draftID uuid.UUID, req service.UpdateDraftRequest) (*domain.CourseDraft, error) {
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

func (m *mockStudioService) GetDraft(ctx context.Context, ownerID, draftID uuid.UUID) (*domain.CourseDraft, error) {
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

func (m *mockStudioService) ListDrafts(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]*domain.CourseDraft, int64, error) {
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

func (m *mockStudioService) SubmitDraft(ctx context.Context, ownerID, draftID uuid.UUID) (*domain.Submission, error) {
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

func (m *mockStudioService) ListModerationQueue(ctx context.Context, limit, offset int) ([]contract.ModerationQueueItem, int64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.queueItems, int64(len(m.queueItems)), nil
}

func (m *mockStudioService) ApproveSubmission(ctx context.Context, reviewerID, submissionID uuid.UUID) (*domain.Submission, error) {
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

func (m *mockStudioService) RejectSubmission(ctx context.Context, reviewerID, submissionID uuid.UUID, targetStatus, feedback string) (*domain.Submission, error) {
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

func (m *mockStudioService) ClaimCourse(ctx context.Context, userID, courseID uuid.UUID) (*domain.Purchase, error) {
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

func (m *mockStudioService) PurchaseCourse(ctx context.Context, userID, courseID uuid.UUID) (*paymentcontract.Order, error) {
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

func (m *mockStudioService) ListUserPurchases(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Purchase, int64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return nil, 0, nil
}

func (m *mockStudioService) RefundPurchase(ctx context.Context, userID, purchaseID uuid.UUID) error {
	return m.err
}


type mockGuard struct {
	allowed bool
}

func (g *mockGuard) Require(ctx context.Context, permission string) error {
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
	reqReject := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/moderation/courses/%s/reject", submissionID), bytes.NewReader(rejectBody))
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
