package service

import (
	"encoding/json"
	"testing"
)

// Seventeen of twenty-two VSTEP Part 1 drafts had the key at B. Over many
// shuffles the key must land in every position, and always on its own text.
func TestShuffledChoicesMoveTheKeyWithItsText(t *testing.T) {
	t.Parallel()

	body := json.RawMessage(`{"script": "…", "questions": [{"id": "q1", "type": "multiple_choice",
		"prompt": "Where is the talk?",
		"options": [{"id": "A", "text": "Room 1"}, {"id": "B", "text": "the main hall"},
			{"id": "C", "text": "the café"}, {"id": "D", "text": "the east wing"}],
		"correct_option_id": "B",
		"explanation": {"explanation_en": "The speaker says it is a talk in the main hall."}}]}`)

	positions := map[string]int{}
	for range 200 {
		var item readingComprehensionCand
		if err := json.Unmarshal(withShuffledChoices(body), &item); err != nil {
			t.Fatal(err)
		}
		q := item.Questions[0]
		for _, o := range q.Options {
			if o.ID == q.CorrectOptionID && o.Text != "the main hall" {
				t.Fatalf("key %s now points at %q", q.CorrectOptionID, o.Text)
			}
		}
		if q.Options[0].ID != "A" || q.Options[3].ID != "D" {
			t.Fatalf("labels are no longer A-D in order: %+v", q.Options)
		}
		positions[q.CorrectOptionID]++
	}
	for _, id := range []string{"A", "B", "C", "D"} {
		if positions[id] == 0 {
			t.Errorf("in 200 shuffles the key never landed at %s: %v", id, positions)
		}
	}
}

// TOEIC Part 1 keeps its choices under "statements"; they are shuffled too.
func TestShuffledChoicesReachPhotoStatements(t *testing.T) {
	t.Parallel()

	body := json.RawMessage(`{"prompt": "Look at the photograph.", "statements": [
		{"id": "A", "text": "A man is typing."}, {"id": "B", "text": "A man is sleeping."},
		{"id": "C", "text": "A man is running."}, {"id": "D", "text": "A man is cooking."}],
		"correct_option_id": "A"}`)
	moved := false
	for range 50 {
		var item struct {
			Statements      []candOption `json:"statements"`
			CorrectOptionID string       `json:"correct_option_id"`
		}
		if err := json.Unmarshal(withShuffledChoices(body), &item); err != nil {
			t.Fatal(err)
		}
		for _, s := range item.Statements {
			if s.ID == item.CorrectOptionID && s.Text != "A man is typing." {
				t.Fatalf("key %s points at %q", item.CorrectOptionID, s.Text)
			}
		}
		if item.CorrectOptionID != "A" {
			moved = true
		}
	}
	if !moved {
		t.Error("in 50 shuffles the Part 1 key never moved from A")
	}
}

func TestShuffledChoicesLeaveOrderThatCarriesMeaning(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"true/false/not given": `{"questions": [{"type": "true_false_not_given",
			"options": [{"id": "True", "text": "True"}, {"id": "False", "text": "False"},
				{"id": "Not Given", "text": "Not Given"}], "correct_option_id": "False"}]}`,
		"all of the above": `{"questions": [{"type": "multiple_choice",
			"options": [{"id": "A", "text": "tea"}, {"id": "B", "text": "coffee"},
				{"id": "C", "text": "juice"}, {"id": "D", "text": "All of the above"}], "correct_option_id": "D"}]}`,
		"explanation names a letter": `{"questions": [{"type": "multiple_choice",
			"options": [{"id": "A", "text": "tea"}, {"id": "B", "text": "coffee"},
				{"id": "C", "text": "juice"}, {"id": "D", "text": "water"}], "correct_option_id": "B",
			"explanation": {"explanation_en": "Option B is right because she orders coffee."}}]}`,
		"vietnamese names a letter": `{"questions": [{"type": "multiple_choice",
			"options": [{"id": "A", "text": "tea"}, {"id": "B", "text": "coffee"},
				{"id": "C", "text": "juice"}, {"id": "D", "text": "water"}], "correct_option_id": "B",
			"explanation": {"explanation_vi": "Đáp án B đúng vì cô ấy gọi cà phê."}}]}`,
	}
	for name, body := range cases {
		for range 20 {
			if got := string(withShuffledChoices(json.RawMessage(body))); got != body {
				t.Errorf("%s: reordered:\n%s", name, got)
				break
			}
		}
	}
}
