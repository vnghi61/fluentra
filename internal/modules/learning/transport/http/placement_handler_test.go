package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
)

func TestPlacementHandler_GetInvitation(t *testing.T) {
	userID := uuid.New()
	svc := &fakeLearningService{
		placementInvDTO: &domain.PlacementInvitationDTO{
			Eligible:       true,
			PoolSufficient: true,
		},
	}
	router, err := setupTestRouter(svc)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/placement/invitation", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var res domain.PlacementInvitationDTO
	err = json.Unmarshal(rec.Body.Bytes(), &res)
	require.NoError(t, err)
	assert.True(t, res.Eligible)
	assert.True(t, res.PoolSufficient)
}

func TestPlacementHandler_StartSession(t *testing.T) {
	userID := uuid.New()
	sessID := uuid.New()
	actID := uuid.New()

	svc := &fakeLearningService{
		startPlacementSess: &domain.PlacementSession{
			ID:                sessID,
			UserID:            userID,
			Status:            domain.PlacementSessionStatusInProgress,
			Stage:             domain.StageFastConvergence,
			CurrentActivityID: &actID,
		},
		startPlacementAct: &lessoncontract.ActivityHierarchy{
			ActivityID: actID,
			Kind:       "vocabulary",
		},
	}
	router, err := setupTestRouter(svc)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/placement/sessions", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.NotNil(t, body["session"])
	assert.NotNil(t, body["current_activity"])
}

func TestPlacementHandler_GetSession(t *testing.T) {
	userID := uuid.New()
	sessID := uuid.New()
	actID := uuid.New()

	svc := &fakeLearningService{
		getPlacementSess: &domain.PlacementSession{
			ID:                sessID,
			UserID:            userID,
			Status:            domain.PlacementSessionStatusInProgress,
			Stage:             domain.StageFastConvergence,
			CurrentActivityID: &actID,
		},
		getPlacementAct: &lessoncontract.ActivityHierarchy{
			ActivityID: actID,
			Kind:       "vocabulary",
		},
	}
	router, err := setupTestRouter(svc)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/placement/sessions/"+sessID.String(), nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.NotNil(t, body["session"])
	assert.NotNil(t, body["current_activity"])
}

func TestPlacementHandler_SubmitAnswer(t *testing.T) {
	userID := uuid.New()
	sessID := uuid.New()
	level := "B1"

	svc := &fakeLearningService{
		submitPlacementSess: &domain.PlacementSession{
			ID:          sessID,
			UserID:      userID,
			Status:      domain.PlacementSessionStatusCompleted,
			PlacedLevel: &level,
		},
		submitPlacementAct:  nil,
		submitPlacementDone: true,
	}
	router, err := setupTestRouter(svc)
	require.NoError(t, err)

	payload := []byte(`{"response":{"selected_option_id":"A"}}`)
	req := httptest.NewRequest(http.MethodPost, "/placement/sessions/"+sessID.String()+"/answers", bytes.NewReader(payload))
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, true, body["finished"])
}

func TestPlacementHandler_GetStartingPath(t *testing.T) {
	userID := uuid.New()
	courseID := uuid.New()
	svc := &fakeLearningService{
		pathDTO: &domain.StartingPathDTO{
			PlacedLevel:            "B1",
			RecommendedCourseID:    courseID,
			RecommendedCourseTitle: "Intermediate English",
		},
	}
	router, err := setupTestRouter(svc)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/me/path", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var res domain.StartingPathDTO
	err = json.Unmarshal(rec.Body.Bytes(), &res)
	require.NoError(t, err)
	assert.Equal(t, "B1", res.PlacedLevel)
	assert.Equal(t, courseID, res.RecommendedCourseID)
}

func TestPlacementHandler_GetWeeklyPlan(t *testing.T) {
	userID := uuid.New()
	now := time.Now().UTC()
	svc := &fakeLearningService{
		planDTO: &domain.WeeklyPlan{
			ID:            uuid.New(),
			UserID:        userID,
			WeekStartDate: now,
			PlacedLevel:   "B1",
			WeakestSkill:  "speaking",
			DailyTargets: []domain.DailyPlanTarget{
				{DayOfWeek: "Monday", TargetMinutes: 20, PrimarySkill: "speaking"},
			},
		},
	}
	router, err := setupTestRouter(svc)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/me/weekly-plan", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var res domain.WeeklyPlan
	err = json.Unmarshal(rec.Body.Bytes(), &res)
	require.NoError(t, err)
	assert.Equal(t, "B1", res.PlacedLevel)
	assert.Equal(t, "speaking", res.WeakestSkill)
}
