package domain

import "encoding/json"

// HasProvenance reports whether a body was written by the generator.
//
// Machine-authored content carries a non-empty `_provenance` object (prompt
// version, model, AI request id); a human's draft does not. The review queue
// uses this to tell the two apart: a machine draft has no author to submit it
// for review, and requiring someone to press "submit" on output a machine just
// produced is the busywork the queue exists to remove.
func HasProvenance(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var decoded struct {
		Provenance map[string]any `json:"_provenance"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return false
	}
	return len(decoded.Provenance) > 0
}
