package service

import (
	"encoding/json"
	"math/rand/v2"
	"regexp"
	"strings"
)

// The model puts the right answer in the same place: of twenty-two VSTEP
// Listening Part 1 drafts, seventeen had the key at B, and nothing downstream
// reorders options. withShuffledChoices deals each exam multiple-choice
// question's options in a random order and relabels them A, B, C…, so the key
// moves with its text.
//
// A question is left as written when reordering could change its meaning:
// options that point at other options ("both A and B", "all of the above"), an
// explanation that names a letter, labels that are not A, B, C… in order (True,
// False, Not Given), or a shared option list (matching).

// letterReference finds an option letter named in text: "option B", "(C)",
// "the answer is A", "A and B", "đáp án C". The letter is matched in capitals
// only, or every "is a" would count.
var letterReference = regexp.MustCompile(
	`(?:[Oo]ption|[Aa]nswer|[Cc]hoice|[Ss]tatement|[Rr]esponse|` +
		`[Đđ]áp án|[Pp]hương án|[Nn]hận định|[Cc]âu|[Ii]s|[Ww]as|là)\s+[A-D]\b` +
		`|\([A-D]\)|\b[A-D]\s+(?:and|or|và|hoặc)\s+[A-D]\b`)

// optionsField is where a question keeps its choices.
const optionsField = "options"

// crossReferences are option texts that only make sense in their place.
var crossReferences = []string{"all of the above", "none of the above", "both a and b", "above", "below"}

// withShuffledChoices reorders the options of every multiple-choice question in
// an exam item's body: the questions of a group, or the item's own options
// (TOEIC Parts 1, 2 and 5). A body it cannot read is returned unchanged.
func withShuffledChoices(body json.RawMessage) json.RawMessage {
	var item map[string]any
	if err := json.Unmarshal(body, &item); err != nil {
		return body
	}
	changed := false
	if questions, ok := item["questions"].([]any); ok {
		for _, q := range questions {
			if question, ok := q.(map[string]any); ok && shuffleQuestion(question) {
				changed = true
			}
		}
	}
	if shuffleQuestion(item) {
		changed = true
	}
	if !changed {
		return body
	}
	out, err := json.Marshal(item)
	if err != nil {
		return body
	}
	return out
}

// shuffleQuestion reorders one question's options in place and reports
// whether it did.
func shuffleQuestion(q map[string]any) bool {
	if kind, _ := q["type"].(string); kind != "" && kind != questionTypeChoice {
		return false
	}
	// TOEIC Part 1 writes its four statements under "statements", Part 2 its
	// three responses under "responses".
	var options []any
	ok := false
	for _, field := range []string{optionsField, "statements", "responses"} {
		if options, ok = q[field].([]any); ok {
			break
		}
	}
	key, _ := q["correct_option_id"].(string)
	if !ok || len(options) < 2 || key == "" || !reorderable(q, options) {
		return false
	}

	texts := make([]string, len(options))
	keyText := ""
	for i, o := range options {
		option, _ := o.(map[string]any)
		texts[i], _ = option["text"].(string)
		if option["id"] == key {
			keyText = texts[i]
		}
	}
	if keyText == "" {
		return false
	}
	//nolint:gosec // answer order, not a secret
	rand.Shuffle(len(texts), func(i, j int) { texts[i], texts[j] = texts[j], texts[i] })
	for i, o := range options {
		option, _ := o.(map[string]any)
		option["text"] = texts[i]
		if texts[i] == keyText {
			q["correct_option_id"] = option["id"]
		}
	}
	return true
}

// reorderable reports whether a question's options can move without changing
// what it asks: labels A, B, C… in order, distinct texts, none pointing at
// another, and an explanation that names no letter.
func reorderable(q map[string]any, options []any) bool {
	seen := make(map[string]bool, len(options))
	for i, o := range options {
		option, ok := o.(map[string]any)
		if !ok {
			return false
		}
		id, _ := option["id"].(string)
		text, _ := option["text"].(string)
		if id != string(rune('A'+i)) || text == "" || seen[text] {
			return false
		}
		seen[text] = true
		lower := strings.ToLower(text)
		for _, ref := range crossReferences {
			if strings.Contains(lower, ref) {
				return false
			}
		}
		if letterReference.MatchString(text) {
			return false
		}
	}
	explanation, _ := json.Marshal(q["explanation"])
	return !letterReference.Match(explanation)
}
