package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
)

// TestGetLessonDetail_CarriesNoAnswer serialises the whole learner-facing lesson
// and looks for the answer in it.
//
// It is written against the response rather than against RedactForLearner
// because the function was already covered and already correct, and the leak
// still shipped: the answer key exists in two places — the content version's
// body and the activity's own `config` — and only the body was redacted. The
// renderer reads `config`, so the copy that reached the browser was the one
// nobody had touched. That was found by reading a real response from a running
// server, which is exactly what this test now does without one.
//
// Searching the encoded JSON, rather than asserting on named fields, is the
// point: a field this test does not know about cannot hide from it.
func TestGetLessonDetail_CarriesNoAnswer(t *testing.T) {
	t.Parallel()

	lessonID, unitID, versionID := uuid.New(), uuid.New(), uuid.New()

	// Both copies, authored the way cmd/seed authors them.
	activityConfig := json.RawMessage(`{
		"prompt": "What is the best word?",
		"options": [{"id":"opt_habit","text":"Habit"},{"id":"opt_damage","text":"Damage"}],
		"correct_option_id": "opt_habit"
	}`)
	versionBody := json.RawMessage(`{
		"prompt": "What is the best word?",
		"correct_answer": "habit",
		"acceptable": ["habit", "routine"]
	}`)

	activities := []contract.Activity{{
		ID:               uuid.New(),
		LessonID:         lessonID,
		Position:         1,
		Kind:             "vocab_multiple_choice",
		ContentVersionID: versionID,
		Config:           activityConfig,
		Weight:           1,
	}}

	repo := &fakeLessonRepo{
		lesson: &contract.Lesson{
			ID: lessonID, UnitID: unitID, Position: 1,
			Title: "Morning Routines", Status: statusPublished, Activities: activities,
		},
		activities: activities,
	}
	contentReader := &countingContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Kind: "vocab_multiple_choice", Status: statusPublished, Body: versionBody},
		},
	}

	svc := service.New(service.Deps{Repo: repo, Content: contentReader})

	detail, err := svc.GetLessonDetail(context.Background(), lessonID, uuid.Nil)
	if err != nil {
		t.Fatalf("GetLessonDetail: %v", err)
	}

	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}
	response := string(encoded)

	for _, forbidden := range []string{
		"correct_option_id",
		"correct_answer",
		"acceptable",
		`"habit"`,
		"routine",
	} {
		if strings.Contains(response, forbidden) {
			t.Errorf("the learner-facing lesson carries %q:\n%s", forbidden, response)
		}
	}

	// The exercise must still be answerable, or the fix broke the thing it was
	// protecting. The option ids stay — a learner has to be able to pick one,
	// and "opt_habit" among four is not an answer key.
	for _, required := range []string{
		"What is the best word?",
		"opt_habit",
		"opt_damage",
	} {
		if !strings.Contains(response, required) {
			t.Errorf("the learner-facing lesson lost %q, so there is nothing to answer:\n%s",
				required, response)
		}
	}
}
