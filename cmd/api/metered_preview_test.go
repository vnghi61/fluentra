package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/learning"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	writingcontract "github.com/fluentra/fluentra/internal/modules/writing/contract"
	writingservice "github.com/fluentra/fluentra/internal/modules/writing/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type countingAIClient struct {
	calls atomic.Int32
}

func (c *countingAIClient) Complete(_ context.Context, _ ai.Request) (ai.Response, error) {
	c.calls.Add(1)
	return ai.Response{
		Text: `{"score": 85, "correct": true, "feedback": "Good essay"}`,
	}, nil
}

type mockContentReader struct {
	versions map[uuid.UUID]*contentcontract.Version
}

func (m *mockContentReader) GetVersion(_ context.Context, id uuid.UUID) (*contentcontract.Version, error) {
	return m.versions[id], nil
}

type mockLessonReader struct {
	hierarchy map[uuid.UUID]*lessoncontract.ActivityHierarchy
}

func (m *mockLessonReader) ResolveActivity(
	_ context.Context, id uuid.UUID,
) (*lessoncontract.ActivityHierarchy, error) {
	return m.hierarchy[id], nil
}

func (m *mockLessonReader) GetLesson(_ context.Context, _ uuid.UUID) (*lessoncontract.Lesson, error) {
	return nil, nil
}
func (m *mockLessonReader) ListLessons(_ context.Context, _ uuid.UUID) ([]*lessoncontract.Lesson, error) {
	return nil, nil
}
func (m *mockLessonReader) ListUnitsByCourseID(_ context.Context, _ uuid.UUID) ([]*lessoncontract.Unit, error) {
	return nil, nil
}
func (m *mockLessonReader) ListPrerequisitesForLessons(
	_ context.Context, _ []uuid.UUID,
) ([]lessoncontract.PrerequisiteItem, error) {
	return nil, nil
}
func (m *mockLessonReader) ListActivitiesByCourseIDs(
	_ context.Context, _ []uuid.UUID,
) (map[uuid.UUID][]uuid.UUID, error) {
	return nil, nil
}
func (m *mockLessonReader) NextLesson(
	_ context.Context, _ uuid.UUID, _ *uuid.UUID,
) (*lessoncontract.Lesson, error) {
	return nil, nil
}

type noopGuard struct{}

func (noopGuard) Require(_ context.Context, _ string) error {
	return nil
}

func TestGradePreview_WritingPrompt_RefusesAndMakesZeroAICalls(t *testing.T) {
	aiClient := &countingAIClient{}
	versionID := uuid.New()
	content := &mockContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {
				ID:   versionID,
				Body: []byte(`{"prompt":"Write about your dreams.","min_words":5}`),
			},
		},
	}
	writingGrader := writingservice.NewGrader(content, aiClient)

	activityID := uuid.New()
	lessonReader := &mockLessonReader{
		hierarchy: map[uuid.UUID]*lessoncontract.ActivityHierarchy{
			activityID: {
				ActivityID:       activityID,
				ContentVersionID: versionID,
				Kind:             writingcontract.KindWritingPrompt,
			},
		},
	}

	graders := map[string]learningcontract.ExerciseGrader{
		writingcontract.KindWritingPrompt: writingGrader,
	}

	learningMod := learning.New(learning.Deps{
		Clock:         clock.Real{},
		Lesson:        lessonReader,
		Graders:       graders,
		DeclaredKinds: []string{writingcontract.KindWritingPrompt},
		Guard:         noopGuard{},
	})

	router := httpx.NewRouter(httpx.RouterDependencies{
		Modules: func(r chi.Router) {
			learningMod.Routes(r)
		},
	})

	// 1. Send an unauthenticated preview grading request for writing_prompt
	body := bytes.NewReader(
		[]byte(`{"response":{"text_answer":"I dream of building wonderful software systems."}}`),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities/"+activityID.String()+"/grade", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Must return 401 Unauthorized with ACCOUNT_REQUIRED
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var problem map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, "ACCOUNT_REQUIRED", problem["code"])

	// Counting fake AI client records ZERO calls!
	assert.Equal(t, int32(0), aiClient.calls.Load())

	// 2. If grader is called directly without the guard, call count becomes 1
	res, err := writingGrader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"text_answer":"I dream of building wonderful software systems."}`),
	})
	require.NoError(t, err)
	assert.True(t, res.Correct)
	assert.Equal(t, int32(1), aiClient.calls.Load())
}
