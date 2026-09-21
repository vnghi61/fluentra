package contract_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
)

// The authored field names this suite is about, named once.
const (
	keyCorrectAnswer   = "correct_answer"
	keyAcceptable      = "acceptable"
	keyCorrectOptionID = "correct_option_id"
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
	for _, key := range []string{keyCorrectOptionID, keyCorrectAnswer, keyAcceptable} {
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

	for _, leaked := range []string{keyCorrectAnswer, "alpha", keyAcceptable, "beta"} {
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

	const multipleChoice = `{
		"prompt": "p",
		"options": [{"id": "opt_habit", "text": "Habit"}],
		"correct_answer": "habit",
		"acceptable": ["habit", "opt_habit"],
		"correct_option_id": "opt_habit"
	}`

	cases := map[string]json.RawMessage{
		"vocab_multiple_choice": json.RawMessage(multipleChoice),
		"vocab_gap_fill": json.RawMessage(
			`{"prompt":"p","correct_answer":"habit","acceptable":["habit","routine"]}`),
		"vocab_flashcard": json.RawMessage(
			`{"prompt":"p","correct_answer":"habit","acceptable":["habit","good"]}`),
		// The activity config of a gap fill or a curriculum sentence transform,
		// which carried the word for the blank to every visitor.
		"vocab_gap_fill config": json.RawMessage(
			`{"prompt":"p","sentence_before":"I try to keep a daily","sentence_after":"of reading.","expected_answer":"habit"}`),
	}

	for kind, body := range cases {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			redacted := string(contract.RedactForLearner(body))
			leaks := []string{keyCorrectAnswer, keyAcceptable, keyCorrectOptionID, "expected_answer", `"habit"`, "routine"}
			for _, leaked := range leaks {
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

	if !strings.Contains(string(original.Body), keyCorrectAnswer) {
		t.Error("the original body was mutated; the grader would read a redacted copy")
	}
	if strings.Contains(string(redacted.Body), keyCorrectAnswer) {
		t.Errorf("the copy still carries the answer: %s", redacted.Body)
	}
	if contract.RedactVersionForLearner(nil) != nil {
		t.Error("a nil version must stay nil")
	}
}

func TestRedactForLearner_ReadingQuestionsArray(t *testing.T) {
	t.Parallel()

	body := json.RawMessage(`{
		"passage_title": "Planetary Geology",
		"passage": "Mars has red soil due to iron oxide minerals on its surface.",
		"questions": [
			{
				"id": "q1",
				"type": "multiple_choice",
				"prompt": "Why is Mars red?",
				"options": [{"id": "opt_iron", "text": "Iron oxide"}],
				"answer": "opt_iron",
				"correct_option_id": "opt_iron",
				"correct_answer": "iron oxide"
			},
			{
				"id": "q2",
				"type": "gap_fill",
				"prompt": "The red colour is due to ___ oxide.",
				"answer": "iron",
				"acceptable": ["iron"]
			}
		]
	}`)

	redacted := string(contract.RedactForLearner(body))

	for _, leaked := range []string{"correct_option_id", "correct_answer", `"answer"`} {
		if strings.Contains(redacted, leaked) {
			t.Errorf("%q survived redaction of reading questions array: %s", leaked, redacted)
		}
	}

	if !strings.Contains(redacted, "Planetary Geology") || !strings.Contains(redacted, "Why is Mars red?") {
		t.Errorf("passage/prompt did not survive: %s", redacted)
	}
}

func TestRedactForLearner_ListeningComprehension(t *testing.T) {
	t.Parallel()

	body := json.RawMessage(`{
		"title": "Airport Announcement",
		"audio_key": "media/audio/flight-101.opus",
		"voice": "en-US-Jenny",
		"script": "Attention passengers on flight 101 to London, boarding now.",
		"transcript": "Attention passengers on flight 101 to London, boarding now.",
		"questions": [
			{
				"id": "q1",
				"type": "multiple_choice",
				"prompt": "Which flight is boarding?",
				"options": [{"id": "opt_101", "text": "101"}, {"id": "opt_202", "text": "202"}],
				"correct_option_id": "opt_101",
				"correct_answer": "101",
				"acceptable": ["101"]
			}
		]
	}`)

	redacted := string(contract.RedactForLearner(body))

	leakable := []string{"script", "transcript", "correct_option_id", "correct_answer", "acceptable", "London"}
	for _, leaked := range leakable {
		if strings.Contains(redacted, leaked) {
			t.Errorf("%q survived redaction of listening comprehension body: %s", leaked, redacted)
		}
	}

	// Audio key and prompt must survive so the player knows what audio file to stream
	if !strings.Contains(
		redacted, "media/audio/flight-101.opus",
	) || !strings.Contains(redacted, "Which flight is boarding?") {
		t.Errorf("audio_key or prompt did not survive: %s", redacted)
	}
}

func TestRedactForLearner_StripsProvenance(t *testing.T) {
	t.Parallel()

	body := json.RawMessage(`{
		"prompt": "Choose the correct tense.",
		"options": [{"id": "A", "text": "have gone"}],
		"correct_option_id": "A",
		"_provenance": {
			"prompt_version": "item_generate.v1",
			"model": "gpt-4o-mini",
			"ai_request_id": "0199a1c2-3d4e-7f80-9abc-def01234567a"
		}
	}`)

	redacted := string(contract.RedactForLearner(body))
	if strings.Contains(redacted, "_provenance") || strings.Contains(redacted, "gpt-4o-mini") {
		t.Fatalf("_provenance survived redaction: %s", redacted)
	}
	if !strings.Contains(redacted, "Choose the correct tense.") {
		t.Fatalf("prompt was stripped: %s", redacted)
	}
}

func TestRedactForLearner_TOEICKinds(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body     string
		leaks    []string
		survives []string
	}{
		"photo_description": {
			body: `{
				"image_url": "https://example.com/photo1.jpg",
				"statements": [
					{"id": "A", "audio_url": "https://example.com/a.mp3", "transcript": "She is running"},
					{"id": "B", "audio_url": "https://example.com/b.mp3", "transcript": "She is reading"}
				],
				"key": "A",
				"correct_option_id": "A"
			}`,
			leaks:    []string{"key", "correct_option_id", "transcript", "She is running"},
			survives: []string{"image_url", "https://example.com/photo1.jpg", "https://example.com/a.mp3"},
		},
		"question_response": {
			body: `{
				"audio_url": "https://example.com/q.mp3",
				"transcript": "Where is the meeting?",
				"responses": [
					{"id": "A", "audio_url": "https://example.com/r1.mp3", "transcript": "In room 3"},
					{"id": "B", "audio_url": "https://example.com/r2.mp3", "transcript": "At 2 PM"}
				],
				"key": "A"
			}`,
			leaks:    []string{"key", "transcript", "Where is the meeting?", "In room 3"},
			survives: []string{"audio_url", "https://example.com/q.mp3", "https://example.com/r1.mp3"},
		},
		"mcq_gap": {
			body: `{
				"sentence": "The report must be completed _____ Friday.",
				"options": [
					{"id": "A", "text": "by"},
					{"id": "B", "text": "at"}
				],
				"key": "A",
				"correct_answer": "by"
			}`,
			leaks:    []string{"key", "correct_answer"},
			survives: []string{"sentence", "Friday", "options", "by", "at"},
		},
		"text_completion": {
			body: `{
				"passage": "Dear team, please note that [1] ... and [2] ...",
				"questions": [
					{"id": "1", "options": [{"id": "A", "text": "opt1"}], "key": "A", "correct_option_id": "A"}
				],
				"keys": ["A"]
			}`,
			leaks:    []string{"key", "keys", "correct_option_id"},
			survives: []string{"passage", "Dear team", "questions", "opt1"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			redacted := string(contract.RedactForLearner(json.RawMessage(tc.body)))
			for _, leak := range tc.leaks {
				if strings.Contains(redacted, `"`+leak+`"`) {
					t.Errorf("expected %q to be redacted from %s: %s", leak, name, redacted)
				}
			}
			for _, surv := range tc.survives {
				if !strings.Contains(redacted, surv) {
					t.Errorf("expected %q to survive in %s: %s", surv, name, redacted)
				}
			}
		})
	}
}
