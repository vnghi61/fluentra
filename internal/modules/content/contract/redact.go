package contract

import (
	"encoding/json"
)

// answerKeys are the body fields that decide whether an answer is right.
//
// They are authored alongside the question and stored in the same JSON body,
// which is convenient for the grader and wrong for the renderer: every
// learner-facing response that carried a body carried the answer with it. A
// learner opening a lesson received `correct_answer`, `acceptable` and
// `correct_option_id` for every activity before answering anything, and the
// only reason that was not an open scrape of the whole curriculum is that the
// endpoints needed an account. Opening them to anonymous callers removes that
// reason, so the fields have to go.
//
// A denylist rather than an allowlist because bodies are `json.RawMessage` and
// their shape belongs to whichever module authored the kind — there is no
// schema to allowlist against. That makes this a list that must grow when a new
// kind introduces a new answer field, which is what TestRedactForLearner
// enforces against the seeded curriculum: it asserts no accepted answer
// survives, so a new field that leaks fails the test rather than shipping.
var answerKeys = map[string]struct{}{
	"correct_answer":     {},
	"correct_answers":    {},
	"correct_option_id":  {},
	"correct_option_ids": {},
	// `vocab_match`: word option id against definition option id. Without this
	// row the whole matching key travelled to the renderer, which is the exact
	// leak the rest of this list exists to close.
	"correct_pairs": {},
	"acceptable":    {},
	"answer":        {},
	"answers":       {},
	"answer_key":    {},
	"solution":      {},
	"solutions":     {},
	"rubric":        {},
}

// RedactForLearner strips the answer from a content body.
//
// The result is what a learner may hold before they have answered. Grading runs
// on the server against the stored body, so nothing here costs the learner a
// working exercise: the runner needs the prompt and the options, and gets them.
// What it no longer gets is the answer, which it now learns from the grade
// response — after submitting, which is the point.
//
// Recursive, because a nested body would otherwise hide an answer one level
// down from a scan of the top-level keys.
//
// A body that is not an object, or not valid JSON, is returned unchanged: this
// removes fields it recognises and never invents a shape. Callers hand it
// authored content, and content that does not parse is a fault to surface
// elsewhere, not to silently blank here.
func RedactForLearner(body json.RawMessage) json.RawMessage {
	if len(body) == 0 {
		return body
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return body
	}
	cleaned, changed := redact(decoded)
	if !changed {
		return body
	}
	encoded, err := json.Marshal(cleaned)
	if err != nil {
		return body
	}
	return encoded
}

// redact walks a decoded body, reporting whether it removed anything.
func redact(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		changed := false
		out := make(map[string]any, len(typed))
		for key, inner := range typed {
			if _, forbidden := answerKeys[key]; forbidden {
				changed = true
				continue
			}
			cleaned, innerChanged := redact(inner)
			if innerChanged {
				changed = true
			}
			out[key] = cleaned
		}
		return out, changed
	case []any:
		changed := false
		out := make([]any, len(typed))
		for i, inner := range typed {
			cleaned, innerChanged := redact(inner)
			if innerChanged {
				changed = true
			}
			out[i] = cleaned
		}
		return out, changed
	default:
		return value, false
	}
}

// RedactVersionForLearner returns a copy of version with its body redacted.
//
// A copy, not a mutation: versions are handed out of a batch read and a cache,
// and redacting in place would poison the copy the grader reads next.
func RedactVersionForLearner(version *Version) *Version {
	if version == nil {
		return nil
	}
	clone := *version
	clone.Body = RedactForLearner(version.Body)
	return &clone
}
