package contract

import "encoding/json"

// ExplanationFrom decodes an authored explanation, in either spelling, and
// returns nil when there is nothing to show.
func ExplanationFrom(raw json.RawMessage) *AnswerExplanation {
	if len(raw) == 0 {
		return nil
	}
	var explanation AnswerExplanation
	if err := json.Unmarshal(raw, &explanation); err != nil || explanation.Empty() {
		return nil
	}
	return &explanation
}
