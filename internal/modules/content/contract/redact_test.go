package contract_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
)

func TestRedactForLearner_RemovesTheAnswer(t *testing.T) {
	t.Parallel()

	body := json.RawMessage(`{
		"prompt": "What is the best word?",
		"options": [{"id": "opt_habit", "text": "Habit"}],
		"correct_option_id": "opt_habit",
		"correct_answer": "habit",
		"acceptable": ["habit", "routine"]
	}`)

	redacted := contract.RedactForLearner(body)

	var decoded map[string]any
	if err := json.Unmarshal(redacted, &decoded); err != nil {
		t.Fatalf("redacted body is not JSON: %v", err)
	}
	for _, key := range []string{"correct_option_id", "correct_answer", "acceptable"} {
		if _, present := decoded[key]; present {
			t.Errorf("%q survived redaction", key)
		}
	}
	// What the renderer needs must still be there, or the fix breaks the lesson
	// it was protecting.
	if decoded["prompt"] != "What is the best word?" {
		t.Errorf("prompt did not survive: %v", decoded["prompt"])
	}
	if _, present := decoded["options"]; !present {
		t.Error("options did not survive; there would be nothing to choose from")
	}
}

// A nested body must not hide an answer one level down from a top-level scan.
func TestRedactForLearner_Recurses(t *testing.T) {
	t.Parallel()

	body := json.RawMessage(`{
		"sections": [
			{"prompt": "one", "correct_answer": "alpha"},
			{"prompt": "two", "nested": {"acceptable": ["beta"]}}
		]
	}`)

	redacted := string(contract.RedactForLearner(body))

	for _, leaked := range []string{"correct_answer", "alpha", "acceptable", "beta"} {
		if strings.Contains(redacted, leaked) {
			t.Errorf("%q survived redaction of a nested body: %s", leaked, redacted)
		}
	}
	if !strings.Contains(redacted, "one") || !strings.Contains(redacted, "two") {
		t.Errorf("prompts did not survive: %s", redacted)
	}
}

// The three kinds the seeded curriculum actually authors. This is the test that
// makes the denylist maintainable: a new kind that introduces a new answer field
// fails here rather than shipping a leak.
func TestRedactForLearner_SeededKinds(t *testing.T) {
	t.Parallel()

	cases := map[string]json.RawMessage{
		"vocab_multiple_choice": json.RawMessage(`{"prompt":"p","options":[{"id":"opt_habit","text":"Habit"}],"correct_answer":"habit","acceptable":["habit","opt_habit"],"correct_option_id":"opt_habit"}`),
		"vocab_gap_fill":        json.RawMessage(`{"prompt":"p","correct_answer":"habit","acceptable":["habit","routine"]}`),
		"vocab_flashcard":       json.RawMessage(`{"prompt":"p","correct_answer":"habit","acceptable":["habit","good"]}`),
	}

	for kind, body := range cases {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			redacted := string(contract.RedactForLearner(body))
			for _, leaked := range []string{"correct_answer", "acceptable", "correct_option_id", "\"habit\"", "routine"} {
				if strings.Contains(redacted, leaked) {
					t.Errorf("%s leaked %q: %s", kind, leaked, redacted)
				}
			}
		})
	}
}

// Content that does not parse is a fault to surface elsewhere. Blanking it here
// would turn an authoring bug into a silently empty exercise.
func TestRedactForLearner_LeavesUnparseableBodyAlone(t *testing.T) {
	t.Parallel()

	body := json.RawMessage(`not json`)
	if got := string(contract.RedactForLearner(body)); got != "not json" {
		t.Errorf("got %q, want the body unchanged", got)
	}
	if got := contract.RedactForLearner(nil); got != nil {
		t.Errorf("got %q, want nil", got)
	}
}

// The version handed to the renderer is a copy. Redacting in place would poison
// the batch-read and cached copy the grader reads next.
func TestRedactVersionForLearner_DoesNotMutateTheOriginal(t *testing.T) {
	t.Parallel()

	original := &contract.Version{
		Kind: "vocab_gap_fill",
		Body: json.RawMessage(`{"prompt":"p","correct_answer":"habit"}`),
	}

	redacted := contract.RedactVersionForLearner(original)

	if !strings.Contains(string(original.Body), "correct_answer") {
		t.Error("the original body was mutated; the grader would read a redacted copy")
	}
	if strings.Contains(string(redacted.Body), "correct_answer") {
		t.Errorf("the copy still carries the answer: %s", redacted.Body)
	}
	if contract.RedactVersionForLearner(nil) != nil {
		t.Error("a nil version must stay nil")
	}
}
