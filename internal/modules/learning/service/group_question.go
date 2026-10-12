package service

import (
	"errors"
	"fmt"
	"strings"
)

// Question types a passage or recording group may hold besides plain multiple
// choice (WO 22 D22-25). IELTS is mostly these.
const (
	questionTypeCompletion = "completion"
	questionTypeTFNG       = "true_false_not_given"
	questionTypeChoice     = "multiple_choice"
)

// questionKey is a group question's answer key, whichever field carries it: a
// choice names an option id, a typed question its key or correct answer.
func questionKey(q candQuestion) string {
	switch {
	case q.CorrectOptionID != "":
		return q.CorrectOptionID
	case q.Key != "":
		return q.Key
	default:
		return q.CorrectAnswer
	}
}

// isTypedQuestion reports a completion question: no options, a typed answer.
func isTypedQuestion(q candQuestion) bool {
	return q.Type == questionTypeCompletion && len(q.Options) == 0
}

// isFixedTFNG reports a true/false/not-given question served with the fixed
// three answers rather than options of its own.
func isFixedTFNG(q candQuestion) bool {
	return q.Type == questionTypeTFNG && len(q.Options) == 0
}

// checkGroupQuestion is the structure check for one question of a passage or
// recording. A completion needs a key and no options; a true/false/not-given
// without options needs one of its three answers as the key; anything else is a
// choice whose key is one of its options. exactOptions, when positive, is the
// option count a plain multiple-choice question must have.
func checkGroupQuestion(q candQuestion, exactOptions int) error {
	key := strings.TrimSpace(questionKey(q))
	switch {
	case isTypedQuestion(q):
		if key == "" {
			return errors.New("completion question has no key")
		}
		return nil
	case isFixedTFNG(q):
		switch normaliseText(key) {
		case "true", "false", "not given":
			return nil
		}
		return fmt.Errorf("true/false/not given key %q is not True, False or Not Given", key)
	}
	if exactOptions > 0 && (q.Type == "" || q.Type == questionTypeChoice) && len(q.Options) != exactOptions {
		return fmt.Errorf("expected %d options, got %d", exactOptions, len(q.Options))
	}
	if len(q.Options) < 2 {
		return fmt.Errorf("expected at least 2 options, got %d", len(q.Options))
	}
	return checkOptions(q.Options, key)
}

// checkTypedAnswerLimit enforces a part's answer word limit on its completion
// questions ("NO MORE THAN TWO WORDS"): a key the learner could not write within
// the limit is a broken item.
func checkTypedAnswerLimit(questions []candQuestion, maxWords int) error {
	if maxWords <= 0 {
		return nil
	}
	for i, q := range questions {
		if !isTypedQuestion(q) {
			continue
		}
		if n := len(strings.Fields(questionKey(q))); n > maxWords {
			return fmt.Errorf("check (exam structure) failed: question %d answer has %d words, the limit is %d",
				i+1, n, maxWords)
		}
	}
	return nil
}
