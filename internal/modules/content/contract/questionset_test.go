package contract_test

import (
	"encoding/json"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
)

func TestGradeQuestionSet_AllCorrect(t *testing.T) {
	t.Parallel()

	questions := []contract.QuestionItem{
		{
			ID:              "q1",
			Type:            "multiple_choice",
			Prompt:          "What is the capital of France?",
			CorrectOptionID: "opt_paris",
			CorrectAnswer:   "Paris",
		},
		{
			ID:            "q2",
			Type:          "gap_fill",
			Prompt:        "Water boils at ___ degrees Celsius.",
			CorrectAnswer: "100",
			Acceptable:    []string{"100", "one hundred"},
		},
	}

	answers := map[string]string{
		"q1": "opt_paris",
		"q2": "one hundred",
	}

	result := contract.GradeQuestionSet(questions, answers, 100)

	if result.Score != 100 {
		t.Errorf("Score = %d, want 100", result.Score)
	}
	if !result.AllCorrect {
		t.Error("AllCorrect = false, want true")
	}
	if result.CorrectCount != 2 {
		t.Errorf("CorrectCount = %d, want 2", result.CorrectCount)
	}
	if len(result.ItemResults) != 2 {
		t.Fatalf("len(ItemResults) = %d, want 2", len(result.ItemResults))
	}
	if !result.ItemResults[0].Correct || !result.ItemResults[1].Correct {
		t.Error("expected both items to be marked correct")
	}
}

func TestGradeQuestionSet_PartialAndIncorrect(t *testing.T) {
	t.Parallel()

	questions := []contract.QuestionItem{
		{
			ID:              "q1",
			Type:            "multiple_choice",
			Prompt:          "Select True",
			CorrectOptionID: "opt_true",
		},
		{
			ID:            "q2",
			Type:          "gap_fill",
			Prompt:        "Type apple",
			CorrectAnswer: "apple",
		},
	}

	answers := map[string]string{
		"q1": "opt_true",
		"q2": "orange",
	}

	result := contract.GradeQuestionSet(questions, answers, 100)

	if result.Score != 50 {
		t.Errorf("Score = %d, want 50", result.Score)
	}
	if result.AllCorrect {
		t.Error("AllCorrect = true, want false")
	}
	if result.CorrectCount != 1 {
		t.Errorf("CorrectCount = %d, want 1", result.CorrectCount)
	}
	if !result.ItemResults[0].Correct {
		t.Error("q1 should be correct")
	}
	if result.ItemResults[1].Correct {
		t.Error("q2 should be incorrect")
	}
	if result.ItemResults[1].CorrectAnswer == nil || *result.ItemResults[1].CorrectAnswer != "apple" {
		t.Errorf("q2 CorrectAnswer = %v, want 'apple'", result.ItemResults[1].CorrectAnswer)
	}
}

func TestParseComprehensionResponse(t *testing.T) {
	t.Parallel()

	// Map format
	rawMap := json.RawMessage(`{"answers": {"q1": "a", "q2": "b"}}`)
	respMap := contract.ParseComprehensionResponse(rawMap)
	if respMap.Answers["q1"] != "a" || respMap.Answers["q2"] != "b" {
		t.Errorf("failed to parse answers map: %+v", respMap)
	}

	// Items array format
	rawItems := json.RawMessage(`{"items": [{"id": "q1", "answer": "a"}, {"id": "q2", "answer": "b"}]}`)
	respItems := contract.ParseComprehensionResponse(rawItems)
	if respItems.Answers["q1"] != "a" || respItems.Answers["q2"] != "b" {
		t.Errorf("failed to parse items array: %+v", respItems)
	}

	// Raw string format
	rawStr := json.RawMessage(`"just_an_answer"`)
	respStr := contract.ParseComprehensionResponse(rawStr)
	if respStr.Answer != "just_an_answer" {
		t.Errorf("failed to parse raw string: %+v", respStr)
	}
}
