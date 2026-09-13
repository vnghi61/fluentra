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

	"github.com/fluentra/fluentra/internal/modules/listening/domain"
	listeninghttp "github.com/fluentra/fluentra/internal/modules/listening/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type fakeListeningService struct {
	recordPlayFn    func(ctx context.Context, userID, versionID uuid.UUID, contextType string, contextID uuid.UUID) (*domain.PlayResult, error)
	getTranscriptFn func(ctx context.Context, userID, versionID, attemptID uuid.UUID) (string, error)
}

func (f *fakeListeningService) RecordPlay(ctx context.Context, userID, versionID uuid.UUID, contextType string, contextID uuid.UUID) (*domain.PlayResult, error) {
	if f.recordPlayFn != nil {
		return f.recordPlayFn(ctx, userID, versionID, contextType, contextID)
	}
	return nil, apperr.New(apperr.Internal, "NOT_IMPLEMENTED", "not implemented")
}

func (f *fakeListeningService) GetTranscript(ctx context.Context, userID, versionID, attemptID uuid.UUID) (string, error) {
	if f.getTranscriptFn != nil {
		return f.getTranscriptFn(ctx, userID, versionID, attemptID)
	}
	return "", apperr.New(apperr.Internal, "NOT_IMPLEMENTED", "not implemented")
}

func withActor(r *http.Request, userID uuid.UUID) *http.Request {
	actor := httpx.Actor{
		UserID: userID,
		Role:   "user",
	}
	return r.WithContext(httpx.WithActor(r.Context(), actor))
}

func setupRouter(svc listeninghttp.ListeningService) chi.Router {
	r := chi.NewRouter()
	handler := listeninghttp.NewHandler(svc)
	handler.Routes(r)
	return r
}

func TestRecordPlay_Success(t *testing.T) {
	userID := uuid.New()
	versionID := uuid.New()
	contextID := uuid.New()
	expiresAt := time.Now().Add(5 * time.Minute)

	svc := &fakeListeningService{
		recordPlayFn: func(ctx context.Context, uID, vID uuid.UUID, cType string, cID uuid.UUID) (*domain.PlayResult, error) {
			assert.Equal(t, userID, uID)
			assert.Equal(t, versionID, vID)
			assert.Equal(t, "exam", cType)
			assert.Equal(t, contextID, cID)
			return &domain.PlayResult{
				AudioURL:     "https://storage.local/audio.mp3",
				PlaysUsed:    1,
				PlaysAllowed: 1,
				ExpiresAt:    expiresAt,
			}, nil
		},
	}

	router := setupRouter(svc)

	body, _ := json.Marshal(map[string]any{
		"context_type": "exam",
		"context_id":   contextID.String(),
	})
	req := httptest.NewRequest(http.MethodPost, "/listening/items/"+versionID.String()+"/plays", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withActor(req, userID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp domain.PlayResult
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "https://storage.local/audio.mp3", resp.AudioURL)
	assert.Equal(t, 1, resp.PlaysUsed)
	assert.Equal(t, 1, resp.PlaysAllowed)
}

func TestRecordPlay_Unauthenticated(t *testing.T) {
	svc := &fakeListeningService{}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/listening/items/"+uuid.New().String()+"/plays", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRecordPlay_InvalidVersionID(t *testing.T) {
	svc := &fakeListeningService{}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/listening/items/not-a-uuid/plays", nil)
	req = withActor(req, uuid.New())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetTranscript_Success(t *testing.T) {
	userID := uuid.New()
	versionID := uuid.New()
	attemptID := uuid.New()

	svc := &fakeListeningService{
		getTranscriptFn: func(ctx context.Context, uID, vID, aID uuid.UUID) (string, error) {
			assert.Equal(t, userID, uID)
			assert.Equal(t, versionID, vID)
			assert.Equal(t, attemptID, aID)
			return "Full listening transcript here", nil
		},
	}

	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/listening/items/"+versionID.String()+"/transcript?attempt_id="+attemptID.String(), nil)
	req = withActor(req, userID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "Full listening transcript here", resp["script"])
}

func TestGetTranscript_MissingAttemptID(t *testing.T) {
	svc := &fakeListeningService{}
	router := setupRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/listening/items/"+uuid.New().String()+"/transcript", nil)
	req = withActor(req, uuid.New())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
