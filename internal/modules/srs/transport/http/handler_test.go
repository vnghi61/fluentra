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

	"github.com/fluentra/fluentra/internal/modules/srs/contract"
	"github.com/fluentra/fluentra/internal/modules/srs/service"
	srshttp "github.com/fluentra/fluentra/internal/modules/srs/transport/http"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type fakeGuard struct{}

func (fakeGuard) Require(_ context.Context, _ string) error { return nil }

const stateReview = "review"

type fakeService struct {
	dueCountFn   func(ctx context.Context, userID uuid.UUID) (int, error)
	dueCardsFn   func(ctx context.Context, userID uuid.UUID, limit int32) ([]contract.ReviewCardSummary, error)
	answerCardFn func(
		ctx context.Context, userID, cardID uuid.UUID, grade string, elapsedMs int) (service.AnswerResult, error,
	)
	suspendCardFn     func(ctx context.Context, userID, cardID uuid.UUID) (contract.ReviewCardSummary, error)
	resetCardFn       func(ctx context.Context, userID, cardID uuid.UUID) (contract.ReviewCardSummary, error)
	completeSessionFn func(ctx context.Context, userID uuid.UUID, reviewed, correct int) (service.SessionResult, error)
	forecastFn        func(ctx context.Context, userID uuid.UUID, days int) ([]service.ForecastDay, error)
}

func (f *fakeService) DueCount(ctx context.Context, userID uuid.UUID) (int, error) {
	if f.dueCountFn != nil {
		return f.dueCountFn(ctx, userID)
	}
	return 0, nil
}

func (f *fakeService) DueCards(
	ctx context.Context, userID uuid.UUID, limit int32) ([]contract.ReviewCardSummary, error,
) {
	if f.dueCardsFn != nil {
		return f.dueCardsFn(ctx, userID, limit)
	}
	return nil, nil
}

func (f *fakeService) AnswerCard(
	ctx context.Context, userID, cardID uuid.UUID, grade string, elapsedMs int) (service.AnswerResult, error,
) {
	if f.answerCardFn != nil {
		return f.answerCardFn(ctx, userID, cardID, grade, elapsedMs)
	}
	return service.AnswerResult{}, nil
}

func (f *fakeService) SuspendCard(ctx context.Context, userID, cardID uuid.UUID) (contract.ReviewCardSummary, error) {
	if f.suspendCardFn != nil {
		return f.suspendCardFn(ctx, userID, cardID)
	}
	return contract.ReviewCardSummary{}, nil
}

func (f *fakeService) ResetCard(ctx context.Context, userID, cardID uuid.UUID) (contract.ReviewCardSummary, error) {
	if f.resetCardFn != nil {
		return f.resetCardFn(ctx, userID, cardID)
	}
	return contract.ReviewCardSummary{}, nil
}

func (f *fakeService) CompleteSession(
	ctx context.Context, userID uuid.UUID, reviewed, correct int) (service.SessionResult, error,
) {
	if f.completeSessionFn != nil {
		return f.completeSessionFn(ctx, userID, reviewed, correct)
	}
	return service.SessionResult{}, nil
}

func (f *fakeService) Forecast(ctx context.Context, userID uuid.UUID, days int) ([]service.ForecastDay, error) {
	if f.forecastFn != nil {
		return f.forecastFn(ctx, userID, days)
	}
	return nil, nil
}

func setupTestRouter(svc srshttp.SRSService) chi.Router {
	r := chi.NewRouter()
	h, err := srshttp.NewHandler(svc, fakeGuard{})
	if err != nil {
		panic(err)
	}
	h.Routes(r)
	return r
}

func withActor(req *http.Request, userID uuid.UUID) *http.Request {
	actor := httpx.Actor{
		UserID: userID,
		Role:   "user",
	}
	ctx := httpx.WithActor(req.Context(), actor)
	return req.WithContext(ctx)
}

func TestHTTP_GetDueCount(t *testing.T) {
	userID := uuid.New()
	svc := &fakeService{
		dueCountFn: func(_ context.Context, uid uuid.UUID) (int, error) {
			assert.Equal(t, userID, uid)
			return 5, nil
		},
	}

	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/reviews/due-count", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp srshttp.DueCountResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 5, resp.DueCount)
}

func TestHTTP_GetDueCount_Unauthenticated(t *testing.T) {
	svc := &fakeService{}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/reviews/due-count", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHTTP_GetReviewSession(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	contentID := uuid.New()
	now := time.Now().UTC()

	svc := &fakeService{
		dueCardsFn: func(_ context.Context, uid uuid.UUID, _ int32) ([]contract.ReviewCardSummary, error) {
			assert.Equal(t, userID, uid)
			return []contract.ReviewCardSummary{
				{
					ID:               cardID,
					UserID:           uid,
					ContentVersionID: contentID,
					Skill:            "vocabulary",
					Stability:        3.5,
					Difficulty:       4.2,
					DueAt:            now,
					Reps:             3,
					Lapses:           0,
					State:            stateReview,
				},
			}, nil
		},
	}

	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/reviews/session?limit=10", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp srshttp.ReviewSessionResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Cards, 1)
	assert.Equal(t, cardID, resp.Cards[0].ID)
	assert.Equal(t, "vocabulary", resp.Cards[0].Skill)
}

func TestHTTP_AnswerCard(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	now := time.Now().UTC()

	svc := &fakeService{
		answerCardFn: func(_ context.Context, uid, cid uuid.UUID, grade string, elapsed int) (service.AnswerResult, error) {
			assert.Equal(t, userID, uid)
			assert.Equal(t, cardID, cid)
			assert.Equal(t, "good", grade)
			assert.Equal(t, 1500, elapsed)
			return service.AnswerResult{
				Card: contract.ReviewCardSummary{
					ID:        cid,
					UserID:    uid,
					Stability: 5.0,
					State:     stateReview,
				},
				NextDueAt:    now.AddDate(0, 0, 5),
				IntervalDays: 5,
			}, nil
		},
	}

	router := setupTestRouter(svc)

	body, _ := json.Marshal(srshttp.AnswerReviewRequest{
		Grade:     "good",
		ElapsedMs: 1500,
	})

	req := httptest.NewRequest(http.MethodPost, "/reviews/"+cardID.String()+"/answer", bytes.NewReader(body))
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp srshttp.AnswerReviewResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 5, resp.IntervalDays)
	assert.Equal(t, cardID, resp.Card.ID)
}

func TestHTTP_CompleteSession(t *testing.T) {
	userID := uuid.New()
	now := time.Now().UTC()

	svc := &fakeService{
		completeSessionFn: func(_ context.Context, uid uuid.UUID, reviewed, correct int) (service.SessionResult, error) {
			assert.Equal(t, userID, uid)
			assert.Equal(t, 10, reviewed)
			assert.Equal(t, 9, correct)
			return service.SessionResult{
				Reviewed:    reviewed,
				Correct:     correct,
				Minutes:     5,
				CompletedAt: now,
			}, nil
		},
	}

	router := setupTestRouter(svc)

	body, _ := json.Marshal(srshttp.CompleteReviewSessionRequest{
		Reviewed: 10,
		Correct:  9,
	})

	req := httptest.NewRequest(http.MethodPost, "/reviews/session/complete", bytes.NewReader(body))
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp srshttp.CompleteReviewSessionResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 10, resp.Reviewed)
	assert.Equal(t, 9, resp.Correct)
	assert.Equal(t, 5, resp.Minutes)
}

func TestHTTP_SuspendAndReset(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()

	svc := &fakeService{
		suspendCardFn: func(_ context.Context, uid, cid uuid.UUID) (contract.ReviewCardSummary, error) {
			return contract.ReviewCardSummary{ID: cid, UserID: uid, State: stateReview}, nil
		},
		resetCardFn: func(_ context.Context, uid, cid uuid.UUID) (contract.ReviewCardSummary, error) {
			return contract.ReviewCardSummary{ID: cid, UserID: uid, State: "new"}, nil
		},
	}

	router := setupTestRouter(svc)

	// Suspend
	req := httptest.NewRequest(http.MethodPost, "/reviews/"+cardID.String()+"/suspend", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Reset
	req = httptest.NewRequest(http.MethodPost, "/reviews/"+cardID.String()+"/reset", nil)
	req = withActor(req, userID)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHTTP_Forecast(t *testing.T) {
	userID := uuid.New()
	svc := &fakeService{}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/reviews/forecast", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp srshttp.ForecastResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Days)
}
