package contract_test

import (
	"encoding/json"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
)

func TestExplanationFrom_ReadsEitherSpellingAndDropsNothingToShow(t *testing.T) {
	generated := contract.ExplanationFrom(
		json.RawMessage(`{"explanation_en":"Past tense.","explanation_vi":"Thì quá khứ."}`),
	)
	if generated == nil || generated.Text != "Past tense." || generated.TextVi != "Thì quá khứ." {
		t.Errorf("generated spelling = %+v", generated)
	}
	if contract.ExplanationFrom(nil) != nil {
		t.Error("no explanation decoded as one")
	}
	if contract.ExplanationFrom(json.RawMessage(`{"text":"  "}`)) != nil {
		t.Error("a blank explanation decoded as one")
	}
	if contract.ExplanationFrom(json.RawMessage(`"not an object"`)) != nil {
		t.Error("a malformed explanation decoded as one")
	}
}
