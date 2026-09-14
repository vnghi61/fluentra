package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/listening/domain"
)

const optLondon = "opt_london"

func TestGrader_QuestionSet_AllCorrect(t *testing.T) {
	versionID := uuid.New()
	body := listeningBody{
		Script:          "Welcome to London. Mind the gap.",
		Voice:           "en_US-lessac-medium",
		DurationSeconds: 20,
		Questions: []contentcontract.QuestionItem{
			{
				ID:              "q1",
				Prompt:          "Where is the speaker welcoming you to?",
				CorrectOptionID: optLondon,
				Options: []contentcontract.QuestionOption{
					{ID: optLondon, Text: "London"},
					{ID: "opt_paris", Text: "Paris"},
				},
			},
			{
				ID:            "q2",
				Prompt:        "What should you mind?",
				CorrectAnswer: "the gap",
				Acceptable:    []string{"gap"},
			},
		},
	}

	rawBody, _ := json.Marshal(body)
	contentReader := &fakeContentReader{versions: map[uuid.UUID]*contentcontract.Version{
		versionID: {
			ID:   versionID,
			Kind: domain.KindListeningComprehension,
			Body: rawBody,
		},
	}}

	grader := NewGrader(contentReader)

	respJSON, _ := json.Marshal(map[string]any{
		"answers": map[string]string{
			"q1": optLondon,
			"q2": "the gap",
		},
	})

	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         respJSON,
	})
	if err != nil {
		t.Fatalf("unexpected grade error: %v", err)
	}

	if res.Score != 100 || !res.Correct {
		t.Fatalf("expected score 100, correct true, got %d, %v", res.Score, res.Correct)
	}
	if len(res.ItemResults) != 2 {
		t.Fatalf("expected 2 item results, got %d", len(res.ItemResults))
	}
	if len(res.ReviewItems) != 1 || res.ReviewItems[0].InitialGrade != "good" {
		t.Fatalf("expected 1 'good' review item, got %+v", res.ReviewItems)
	}
}

func TestGrader_QuestionSet_PartialScore(t *testing.T) {
	versionID := uuid.New()
	body := listeningBody{
		Script: "Audio script here",
		Questions: []contentcontract.QuestionItem{
			{
				ID:              "q1",
				Prompt:          "Question 1",
				CorrectOptionID: "a",
			},
			{
				ID:              "q2",
				Prompt:          "Question 2",
				CorrectOptionID: "b",
			},
		},
	}
	rawBody, _ := json.Marshal(body)
	contentReader := &fakeContentReader{versions: map[uuid.UUID]*contentcontract.Version{
		versionID: {
			ID:   versionID,
			Kind: domain.KindListeningComprehension,
			Body: rawBody,
		},
	}}

	grader := NewGrader(contentReader)
	respJSON, _ := json.Marshal(map[string]string{
		"q1": "a",
		"q2": "wrong",
	})

	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         respJSON,
	})
	if err != nil {
		t.Fatalf("unexpected grade error: %v", err)
	}

	if res.Score != 50 || res.Correct {
		t.Fatalf("expected score 50, correct false, got %d, %v", res.Score, res.Correct)
	}
	if len(res.ReviewItems) != 1 || res.ReviewItems[0].InitialGrade != "again" {
		t.Fatalf("expected 1 'again' review item, got %+v", res.ReviewItems)
	}
}

func TestRedactForLearner_ListeningScriptHidden(t *testing.T) {
	raw := []byte(`{
		"script": "Secret audio script that reveals answers",
		"transcript": "Secret audio transcript",
		"voice": "en_US-lessac-medium",
		"audio_object_key": "tts/en_US-lessac-medium/abc.mp3",
		"questions": [
			{
				"id": "q1",
				"prompt": "What did the speaker say?",
				"correct_option_id": "opt1",
				"options": [
					{"id": "opt1", "text": "Hello"},
					{"id": "opt2", "text": "Goodbye"}
				]
			}
		]
	}`)

	redacted := contentcontract.RedactForLearner(raw)

	redactedStr := string(redacted)
	if contains(redactedStr, "Secret audio script") {
		t.Fatal("redacted JSON leaked script")
	}
	if contains(redactedStr, "Secret audio transcript") {
		t.Fatal("redacted JSON leaked transcript")
	}
	if contains(redactedStr, "opt1") && contains(redactedStr, "correct_option_id") {
		t.Fatal("redacted JSON leaked correct_option_id")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && jsonSubstring(s, substr)))
}

func jsonSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
