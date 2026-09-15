package contract_test

import (
	"encoding/json"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
)

// A generated item body spells its explanation explanation_en/explanation_vi.
// Decoding it into the grader's shape used to give an empty explanation, so a
// learner who chose a wrong answer was shown no reason at all.
func TestAnswerExplanation_ReadsTheGeneratedSpelling(t *testing.T) {
	var body struct {
		Explanation *contract.AnswerExplanation `json:"explanation"`
	}
	raw := `{"explanation": {"explanation_en": "Past simple: a finished past habit.", "explanation_vi": "Quá khứ đơn."}}`
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Explanation.Empty() {
		t.Fatal("the generated explanation decoded as empty")
	}
	if body.Explanation.Text != "Past simple: a finished past habit." || body.Explanation.TextVi != "Quá khứ đơn." {
		t.Errorf("explanation = %+v", body.Explanation)
	}
}

func TestAnswerExplanation_ReadsTheCachedSpellingAndWritesIt(t *testing.T) {
	var expl contract.AnswerExplanation
	if err := json.Unmarshal([]byte(`{"text": "Because.", "text_vi": "Vì."}`), &expl); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if expl.Text != "Because." || expl.TextVi != "Vì." {
		t.Errorf("explanation = %+v", expl)
	}
	encoded, err := json.Marshal(expl)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if string(encoded) != `{"text":"Because.","text_vi":"Vì."}` {
		t.Errorf("encoded = %s; the API shape must not change", encoded)
	}
}

func TestAnswerExplanation_EmptyWhenNothingToShow(t *testing.T) {
	var none *contract.AnswerExplanation
	if !none.Empty() {
		t.Error("nil explanation is not empty")
	}
	if !(&contract.AnswerExplanation{Text: "  "}).Empty() {
		t.Error("blank explanation is not empty")
	}
	if (&contract.AnswerExplanation{TextVi: "Vì."}).Empty() {
		t.Error("an explanation in one language is empty")
	}
}
