package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/speaking/contract"
	speakinghttp "github.com/fluentra/fluentra/internal/modules/speaking/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type fakeSpeakingService struct {
	uploadIntentFn    func(ctx context.Context, userID uuid.UUID, contentType string) (*contract.UploadIntentResult, error)
	deleteRecordFn    func(ctx context.Context, attemptID, userID uuid.UUID) error
	getFeedbackFn     func(ctx context.Context, attemptID, userID uuid.UUID) (*contract.SpeakingFeedback, error)
	listSubmissionsFn func(
		ctx context.Context, userID uuid.UUID, page, pageSize int,
	) (*contract.SpeakingSubmissionList, error)
	getConsentFn    func(ctx context.Context, userID uuid.UUID) (*contract.SpeakingConsent, error)
	recordConsentFn func(ctx context.Context, userID uuid.UUID) (*contract.SpeakingConsent, error)
}

func (f *fakeSpeakingService) GetConsent(
	ctx context.Context, userID uuid.UUID,
) (*contract.SpeakingConsent, error) {
	if f.getConsentFn == nil {
		return &contract.SpeakingConsent{Consented: false}, nil
	}
	return f.getConsentFn(ctx, userID)
}

func (f *fakeSpeakingService) RecordConsent(
	ctx context.Context, userID uuid.UUID,
) (*contract.SpeakingConsent, error) {
	if f.recordConsentFn == nil {
		at := time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC)
		return &contract.SpeakingConsent{Consented: true, ConsentedAt: &at}, nil
	}
	return f.recordConsentFn(ctx, userID)
}

func (f *fakeSpeakingService) UploadIntent(
	ctx context.Context, userID uuid.UUID, contentType string,
) (*contract.UploadIntentResult, error) {
	if f.uploadIntentFn != nil {
		return f.uploadIntentFn(ctx, userID, contentType)
	}
	return nil, apperr.New(apperr.Internal, "NOT_IMPLEMENTED", "not implemented")
}

func (f *fakeSpeakingService) DeleteAttemptRecording(ctx context.Context, attemptID, userID uuid.UUID) error {
	if f.deleteRecordFn != nil {
		return f.deleteRecordFn(ctx, attemptID, userID)
	}
	return nil
}

func (f *fakeSpeakingService) GetSpeakingFeedback(
	ctx context.Context, attemptID, userID uuid.UUID,
) (*contract.SpeakingFeedback, error) {
	if f.getFeedbackFn != nil {
		return f.getFeedbackFn(ctx, attemptID, userID)
	}
	return nil, apperr.New(apperr.Internal, "NOT_IMPLEMENTED", "not implemented")
}

func (f *fakeSpeakingService) ListSpeakingSubmissions(
	ctx context.Context, userID uuid.UUID, page, pageSize int,
) (*contract.SpeakingSubmissionList, error) {
	if f.listSubmissionsFn != nil {
		return f.listSubmissionsFn(ctx, userID, page, pageSize)
	}
	return &contract.SpeakingSubmissionList{
		Items:    []contract.SpeakingSubmissionSummary{},
		Total:    0,
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

func setupSpeakingRouter(svc speakinghttp.SpeakingService) chi.Router {
	r := chi.NewRouter()
	h := speakinghttp.NewHandler(svc)
	h.Routes(r)
	return r
}

func TestUploadIntent_Success(t *testing.T) {
	userID := uuid.New()
	svc := &fakeSpeakingService{
		uploadIntentFn: func(_ context.Context, uID uuid.UUID, ct string) (*contract.UploadIntentResult, error) {
			assert.Equal(t, userID, uID)
			assert.Equal(t, "audio/webm", ct)
			return &contract.UploadIntentResult{
				UploadURL:            "https://storage.local/upload",
				ObjectKey:            "recordings/" + uID.String() + "/abc.webm",
				ExpiresAt:            time.Now().Add(15 * time.Minute),
				DailyRecordingsUsed:  2,
				DailyRecordingsLimit: 30,
			}, nil
		},
	}

	router := setupSpeakingRouter(svc)

	body, _ := json.Marshal(map[string]string{
		"content_type": "audio/webm",
	})
	req := httptest.NewRequest(http.MethodPost, "/speaking/upload-intent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withActor(req, userID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp contract.UploadIntentResult
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "https://storage.local/upload", resp.UploadURL)
	assert.Equal(t, 2, resp.DailyRecordingsUsed)
	assert.Equal(t, 30, resp.DailyRecordingsLimit)
}

func TestUploadIntent_Unauthenticated(t *testing.T) {
	svc := &fakeSpeakingService{}
	router := setupSpeakingRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/speaking/upload-intent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeleteRecording_Success(t *testing.T) {
	userID := uuid.New()
	attemptID := uuid.New()

	var deletedAttempt, deletedUser uuid.UUID
	svc := &fakeSpeakingService{
		deleteRecordFn: func(_ context.Context, aID, uID uuid.UUID) error {
			deletedAttempt = aID
			deletedUser = uID
			return nil
		},
	}

	router := setupSpeakingRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/speaking/attempts/"+attemptID.String()+"/recording", nil)
	req = withActor(req, userID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, attemptID, deletedAttempt)
	assert.Equal(t, userID, deletedUser)
}

func TestGetFeedback_Success(t *testing.T) {
	userID := uuid.New()
	attemptID := uuid.New()

	svc := &fakeSpeakingService{
		getFeedbackFn: func(_ context.Context, aID, uID uuid.UUID) (*contract.SpeakingFeedback, error) {
			assert.Equal(t, attemptID, aID)
			assert.Equal(t, userID, uID)
			return &contract.SpeakingFeedback{
				AttemptID:  attemptID,
				UserID:     userID,
				Transcript: "Test transcript",
				FeedbackEn: "Good job",
			}, nil
		},
	}

	router := setupSpeakingRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/speaking/attempts/"+attemptID.String()+"/feedback", nil)
	req = withActor(req, userID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp contract.SpeakingFeedback
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "Test transcript", resp.Transcript)
}

func TestListSubmissions_Success(t *testing.T) {
	userID := uuid.New()
	attemptID := uuid.New()

	svc := &fakeSpeakingService{
		listSubmissionsFn: func(
			_ context.Context, uID uuid.UUID, page, pageSize int,
		) (*contract.SpeakingSubmissionList, error) {
			assert.Equal(t, userID, uID)
			assert.Equal(t, 2, page)
			assert.Equal(t, 5, pageSize)
			return &contract.SpeakingSubmissionList{
				Items: []contract.SpeakingSubmissionSummary{
					{
						AttemptID:    attemptID,
						Status:       "graded",
						OverallBand:  ptr(6.5),
						Score:        ptr(72),
						HasRecording: true,
						TaskType:     "read_aloud",
						FeedbackEn:   "Clear pace",
						CreatedAt:    time.Now().UTC(),
					},
				},
				Total:    1,
				Page:     page,
				PageSize: pageSize,
			}, nil
		},
	}

	router := setupSpeakingRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/speaking/submissions?page=2&page_size=5", nil)
	req = withActor(req, userID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp contract.SpeakingSubmissionList
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, 2, resp.Page)
	assert.Equal(t, 5, resp.PageSize)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, attemptID, resp.Items[0].AttemptID)
	assert.True(t, resp.Items[0].HasRecording)
	assert.Equal(t, "read_aloud", resp.Items[0].TaskType)
}

func TestListSubmissions_Unauthorized(t *testing.T) {
	svc := &fakeSpeakingService{}
	router := setupSpeakingRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/speaking/submissions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ptr returns a pointer to v. The summary carries the grade as a pointer
// because an attempt graded before the score was persisted has none, and this
// test needs to hand it one.
func ptr[T any](v T) *T { return &v }

func TestGetConsent_ReportsNotYetGiven(t *testing.T) {
	userID := uuid.New()
	router := setupSpeakingRouter(&fakeSpeakingService{})

	req := withActor(httptest.NewRequest(http.MethodGet, "/speaking/consent", nil), userID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body contract.SpeakingConsent
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.False(t, body.Consented)
	assert.Nil(t, body.ConsentedAt)
}

func TestRecordConsent_ReturnsTheTimestamp(t *testing.T) {
	userID := uuid.New()
	router := setupSpeakingRouter(&fakeSpeakingService{})

	req := withActor(httptest.NewRequest(http.MethodPost, "/speaking/consent", nil), userID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body contract.SpeakingConsent
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Consented)
	require.NotNil(t, body.ConsentedAt)
}

func TestConsent_RequiresAuthentication(t *testing.T) {
	router := setupSpeakingRouter(&fakeSpeakingService{})

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		req := httptest.NewRequest(method, "/speaking/consent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code, method)
	}
}
