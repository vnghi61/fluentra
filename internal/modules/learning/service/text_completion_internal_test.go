package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTextBlanksMatchTheirOptions(t *testing.T) {
	t.Parallel()

	words := []candOption{
		{ID: "A", Text: "attend"}, {ID: "B", Text: "attending"},
		{ID: "C", Text: "attended"}, {ID: "D", Text: "attendance"},
	}
	sentences := []candOption{
		{ID: "A", Text: "Please send your comments by Friday."},
		{ID: "B", Text: "The rent is higher than we expected."},
		{ID: "C", Text: "We look forward to settling in soon."},
		{ID: "D", Text: "The team visited twelve locations last month."},
	}
	body := func(passage string, opts ...[]candOption) json.RawMessage {
		cand := readingComprehensionCand{Passage: passage}
		for _, o := range opts {
			cand.Questions = append(cand.Questions, candQuestion{Type: questionTypeChoice, Options: o, CorrectOptionID: "A"})
		}
		raw, _ := json.Marshal(cand)
		return raw
	}

	cases := []struct {
		name    string
		body    json.RawMessage
		wantErr string
	}{
		{
			"word inside a sentence",
			body("All staff must (1) ___ the meeting. The room is booked. (2) ___", words, sentences), "",
		},
		{
			"word between sentences",
			body("The room is booked. (1) ___ If you have questions, call me.", words), "stands between sentences",
		},
		{"sentence inside a sentence", body("All staff must (1) ___ the meeting.", sentences), "inside a sentence"},
		{"blank count", body("All staff must (1) ___ the meeting.", words, words), "1 blanks for 2 questions"},
	}
	for _, tc := range cases {
		err := checkTextBlanks(tc.body)
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: unexpected error %v", tc.name, err)
		case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
			t.Errorf("%s: error %v, want %q", tc.name, err, tc.wantErr)
		}
	}
}
