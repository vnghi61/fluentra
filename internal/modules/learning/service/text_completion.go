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
	for i, loc := range blanks {
		standalone := blankStandsAlone(cand.Passage[:loc[0]], cand.Passage[loc[1]:])
		sentences := optionsAreSentences(cand.Questions[i].Options)
		// The text around the blank goes into the error: it reaches the log
		// and the model's retry note, which then see what was refused.
		around := blankContext(cand.Passage, loc[0], loc[1])
		switch {
		case standalone && !sentences:
			return fmt.Errorf("check 3 failed: blank (%d) stands between sentences but offers words: %q", i+1, around)
		case !standalone && sentences:
			return fmt.Errorf("check 3 failed: blank (%d) is inside a sentence but offers whole sentences: %q", i+1, around)
		}
	}
	return nil
}

// blankStandsAlone reports whether a blank sits between two sentences: what
// precedes it ends a sentence (or the text starts) and what follows starts a
// new one (or the text ends).
func blankStandsAlone(before, after string) bool {
	// A line break, or a salutation ("Dear customers,"), ends what came
	// before as surely as a full stop.
	lineBreak := strings.HasSuffix(strings.TrimRight(before, " \t"), "\n")
	before = strings.TrimSpace(before)
	after = strings.TrimSpace(after)
	endsSentence := before == "" || lineBreak || strings.HasSuffix(before, ".") || strings.HasSuffix(before, "!") ||
		strings.HasSuffix(before, "?") || strings.HasSuffix(before, ":") || endsSalutation(before)
	after = strings.TrimLeft(after, ".")
	after = strings.TrimSpace(after)
	if after == "" {
		return endsSentence
	}
	// A capital after the blank starts a new sentence only when it is an
	// ordinary word; a name ("(1) ___ Riverside Bank will open…") is a word
	// blank at the start of its own sentence.
	next := strings.FieldsFunc(after, func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' })
	if len(next) == 0 {
		return endsSentence
	}
	first := []rune(next[0])
	return endsSentence && unicode.IsUpper(first[0]) && sentenceStarters[strings.ToLower(next[0])]
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

// optionsAreSentences reports whether every option reads as a whole sentence.
func optionsAreSentences(options []candOption) bool {
	if len(options) == 0 {
		return false
	}
	for _, o := range options {
		if len(strings.Fields(o.Text)) < sentenceOptionWords {
			return false
		}
	}
	return true
}
