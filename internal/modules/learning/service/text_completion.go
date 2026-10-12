package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// A TOEIC Part 6 blank is either a missing word or phrase inside a sentence,
// or a whole missing sentence between two. Drafts kept offering single words
// (designers, collapses) for blanks that stood between sentences, where no
// word can fit, and a template in the prompt did not stop them. checkTextBlanks
// refuses a text whose blanks and options do not match.

// textBlank finds "(1) ___" and its neighbours.
var textBlank = regexp.MustCompile(`\((\d)\)\s*_{2,}`)

// sentenceOptionWords is the length from which an option counts as a whole
// sentence rather than a word or phrase.
const sentenceOptionWords = 5

// checkTextBlanks checks each blank of a text_completion body against the
// options its question offers.
func checkTextBlanks(body json.RawMessage) error {
	var cand readingComprehensionCand
	if err := json.Unmarshal(body, &cand); err != nil {
		return fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	blanks := textBlank.FindAllStringSubmatchIndex(cand.Passage, -1)
	if len(blanks) != len(cand.Questions) {
		return fmt.Errorf("check 3 failed: text has %d blanks for %d questions", len(blanks), len(cand.Questions))
	}
	if m := secondText.FindString(cand.Passage); m != "" {
		return fmt.Errorf("check 3 failed: a Part 6 set is one text, but this one has %q", m)
	}
	sentenceBlanks := 0
	for i, loc := range blanks {
		canTakeSentence, needsSentence := blankPosition(cand.Passage[:loc[0]], cand.Passage[loc[1]:])
		sentences := optionsAreSentences(cand.Questions[i].Options)
		if sentences {
			sentenceBlanks++
		}
		// The text around the blank goes into the error: it reaches the log
		// and the model's retry note, which then see what was refused.
		around := blankContext(cand.Passage, loc[0], loc[1])
		switch {
		case needsSentence && !sentences:
			return fmt.Errorf("check 3 failed: blank (%d) stands between sentences but offers words (%s): %q",
				i+1, optionTexts(cand.Questions[i].Options), around)
		case !canTakeSentence && sentences:
			return fmt.Errorf("check 3 failed: blank (%d) is inside a sentence but offers whole sentences: %q", i+1, around)
		}
	}
	if len(blanks) == textBlanks && sentenceBlanks != 1 {
		return fmt.Errorf("check 3 failed: %d blanks take a whole sentence; a Part 6 text has exactly one, "+
			"and its other three blanks are a word or phrase inside a sentence", sentenceBlanks)
	}
	return nil
}

// textBlanks is the number of blanks in a TOEIC Part 6 text.
const textBlanks = 4

// secondText finds the heading of a second text ("Text 2", "Review 2"):
// drafts split the set into four one-blank texts, which Part 6 never does.
var secondText = regexp.MustCompile(`\b(?:Text|Review|Letter|Message|Email|E-mail|Notice|Part)\s*2\b`)

// blankPosition says whether a blank can take a whole sentence (a sentence
// ended before it and what follows starts with a capital) and whether it
// needs one (what follows is an ordinary sentence opener, or nothing). A
// capital that may be a name ("(1) ___ Riverside Bank will open…") can take
// either a sentence or a word.
func blankPosition(before, after string) (canTakeSentence, needsSentence bool) {
	if !afterSentenceEnd(before) {
		return false, false
	}
	after = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(after), "."))
	next := strings.FieldsFunc(after, func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' })
	if len(next) == 0 {
		return true, true
	}
	if !unicode.IsUpper([]rune(next[0])[0]) {
		return false, false
	}
	return true, sentenceStarters[strings.ToLower(next[0])]
}

// afterSentenceEnd reports whether the text before a blank ends a sentence: a
// full stop and the like, a line break, a salutation, or the start of the text.
func afterSentenceEnd(before string) bool {
	// A line break, or a salutation ("Dear customers,"), ends what came
	// before as surely as a full stop.
	lineBreak := strings.HasSuffix(strings.TrimRight(before, " \t"), "\n")
	before = strings.TrimSpace(before)
	return before == "" || lineBreak || strings.HasSuffix(before, ".") || strings.HasSuffix(before, "!") ||
		strings.HasSuffix(before, "?") || strings.HasSuffix(before, ":") || endsSalutation(before)
}

// endsSalutation reports whether text ends with a letter's
// greeting: "Dear all," or "Hello team,".
func endsSalutation(before string) bool {
	if !strings.HasSuffix(before, ",") {
		return false
	}
	start := strings.LastIndexAny(before[:len(before)-1], ".!?:\n")
	last := strings.ToLower(strings.TrimSpace(before[start+1:]))
	for _, greeting := range []string{"dear ", "hello", "hi ", "to all", "good morning", "greetings"} {
		if strings.HasPrefix(last, greeting) {
			return true
		}
	}
	return false
}

// blankContext is the text around a blank, for an error message.
func blankContext(passage string, start, end int) string {
	from, to := max(0, start-60), min(len(passage), end+60)
	return strings.ReplaceAll(passage[from:to], "\n", " ")
}

// sentenceStarters are ordinary words that open a sentence; after a blank they
// show the blank stands on its own.
var sentenceStarters = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`the a an if please we our you your this these that those it its all any
		in for as they he she i there however also anyone each every thank to on at by with from many some
		most starting after before during when while because since although so but and or my me us them
		what which who how why where new staff customers employees guests members visitors
		unfortunately additionally finally meanwhile therefore`) {
		sentenceStarters[w] = true
	}
}

// optionsAreSentences reports whether every option reads as a whole sentence:
// five words or more, or a short one that starts with a capital and ends with
// a full stop ("See you there!").
func optionsAreSentences(options []candOption) bool {
	if len(options) == 0 {
		return false
	}
	for _, o := range options {
		text := strings.TrimSpace(o.Text)
		words := len(strings.Fields(text))
		short := words >= 2 && strings.ContainsAny(text[len(text)-1:], ".!?") && unicode.IsUpper([]rune(text)[0])
		if words < sentenceOptionWords && !short {
			return false
		}
	}
	return true
}

// optionTexts lists a question's options for an error message.
func optionTexts(options []candOption) string {
	texts := make([]string, 0, len(options))
	for _, o := range options {
		texts = append(texts, o.Text)
	}
	return strings.Join(texts, " / ")
}
