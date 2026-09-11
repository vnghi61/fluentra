package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/writing/contract"
	writinghttp "github.com/fluentra/fluentra/internal/modules/writing/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type fakeFeedbackReader struct {
	feedback map[string]*contract.WritingFeedback
}

func (f *fakeFeedbackReader) GetWritingFeedback(
	_ context.Context, attemptID, userID uuid.UUID,
) (*contract.WritingFeedback, error) {
	key := fmt.Sprintf("%s:%s", attemptID, userID)
	fb, ok := f.feedback[key]
	if !ok {
		return nil, apperr.New(apperr.NotFound, "FEEDBACK_NOT_FOUND", "writing feedback not found")
	}
	return fb, nil
}

func (f *fakeFeedbackReader) ListWritingSubmissions(
	_ context.Context, userID uuid.UUID, page, pageSize int,
) (*contract.WritingSubmissionList, error) {
	var items []contract.WritingSubmissionSummary
	for _, fb := range f.feedback {
		if fb.UserID == userID {
			items = append(items, contract.WritingSubmissionSummary{
				AttemptID:   fb.AttemptID,
				Status:      "graded",
				OverallBand: fb.OverallBand,
				Score:       fb.Score,
				CreatedAt:   fb.CreatedAt,
			})
		}
	}
	return &contract.WritingSubmissionList{
		Items:    items,
		Total:    int64(len(items)),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func withActor(r *http.Request, userID uuid.UUID) *http.Request {
	actor := httpx.Actor{
		UserID: userID,
		Role:   "user",
	}
	return r.WithContext(httpx.WithActor(r.Context(), actor))
}

func setupWritingRouter(reader contract.FeedbackReader) chi.Router {
	r := chi.NewRouter()
	handler := writinghttp.NewHandler(reader)
	handler.Routes(r)
	return r
}

func TestGetWritingFeedback_Success(t *testing.T) {
	attemptID := uuid.New()
	userID := uuid.New()

	expected := &contract.WritingFeedback{
		AttemptID:   attemptID,
		UserID:      userID,
		OverallBand: 6.5,
		Score:       72,
		Criteria: []contract.WritingCriterion{
			{
				Name:      "task_response",
				Band:      7.0,
				CommentEn: "Good ideas.",
				CommentVi: "Ý tốt.",
			},
		},
		Annotations: []contract.WritingAnnotation{
			{
				QuotedText:  "rapid advancement",
				StartOffset: 10,
				EndOffset:   27,
				CommentEn:   "Strong phrase.",
				CommentVi:   "Cụm từ hay.",
			},
		},
		FeedbackEn:    "Good essay overall.",
		FeedbackVi:    "Bài viết tổng thể tốt.",
		PromptVersion: "writing_grade.v2",
		Model:         "gpt-4o-mini",
		CreatedAt:     time.Now().UTC(),
	}

	reader := &fakeFeedbackReader{
		feedback: map[string]*contract.WritingFeedback{
			fmt.Sprintf("%s:%s", attemptID, userID): expected,
		},
	}

	router := setupWritingRouter(reader)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/writing/attempts/%s/feedback", attemptID), nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var actual contract.WritingFeedback
	err := json.NewDecoder(rec.Body).Decode(&actual)
	require.NoError(t, err)
	assert.Equal(t, attemptID, actual.AttemptID)
	assert.Equal(t, userID, actual.UserID)
	assert.Equal(t, 6.5, actual.OverallBand)
	assert.Equal(t, 72, actual.Score)
	assert.Len(t, actual.Criteria, 1)
	assert.Len(t, actual.Annotations, 1)
	assert.Equal(t, "writing_grade.v2", actual.PromptVersion)
}

func TestGetWritingFeedback_Unauthenticated(t *testing.T) {
	router := setupWritingRouter(&fakeFeedbackReader{})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/writing/attempts/%s/feedback", uuid.New()), nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetWritingFeedback_InvalidUUID(t *testing.T) {
	router := setupWritingRouter(&fakeFeedbackReader{})

	req := httptest.NewRequest(http.MethodGet, "/writing/attempts/not-a-uuid/feedback", nil)
	req = withActor(req, uuid.New())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetWritingFeedback_NotFound(t *testing.T) {
	router := setupWritingRouter(&fakeFeedbackReader{feedback: map[string]*contract.WritingFeedback{}})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/writing/attempts/%s/feedback", uuid.New()), nil)
	req = withActor(req, uuid.New())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetWritingFeedback_UnownedAttempt_ReturnsNotFound(t *testing.T) {
	attemptID := uuid.New()
	ownerID := uuid.New()
	strangerID := uuid.New()

	reader := &fakeFeedbackReader{
		feedback: map[string]*contract.WritingFeedback{
			fmt.Sprintf("%s:%s", attemptID, ownerID): {
				AttemptID: attemptID,
				UserID:    ownerID,
			},
		},
	}

	router := setupWritingRouter(reader)

	// stranger requests attempt feedback -> returns 404 (not found for stranger)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/writing/attempts/%s/feedback", attemptID), nil)
	req = withActor(req, strangerID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListWritingSubmissions_Success(t *testing.T) {
	userID := uuid.New()
	attempt1 := uuid.New()
	attempt2 := uuid.New()

	reader := &fakeFeedbackReader{
		feedback: map[string]*contract.WritingFeedback{
			fmt.Sprintf("%s:%s", attempt1, userID): {
				AttemptID:   attempt1,
				UserID:      userID,
				OverallBand: 7.0,
				Score:       78,
				CreatedAt:   time.Now().UTC(),
			},
			fmt.Sprintf("%s:%s", attempt2, userID): {
				AttemptID:   attempt2,
				UserID:      userID,
				OverallBand: 6.5,
				Score:       70,
				CreatedAt:   time.Now().UTC().Add(-time.Hour),
			},
		},
	}

	router := setupWritingRouter(reader)

	req := httptest.NewRequest(http.MethodGet, "/writing/submissions?page=1&page_size=10", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var actual contract.WritingSubmissionList
	err := json.NewDecoder(rec.Body).Decode(&actual)
	require.NoError(t, err)
	assert.Equal(t, int64(2), actual.Total)
	assert.Equal(t, 1, actual.Page)
	assert.Equal(t, 10, actual.PageSize)
	assert.Len(t, actual.Items, 2)
}

func TestListWritingSubmissions_Unauthenticated(t *testing.T) {
	router := setupWritingRouter(&fakeFeedbackReader{})

	req := httptest.NewRequest(http.MethodGet, "/writing/submissions", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
