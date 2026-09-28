package service

import (
	"encoding/json"
	"strings"
	"testing"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

// The format table stores a listening part's recording as {"genre", "speakers"}.
// Before it reached the prompt, a monologue part (IELTS Listening Part 2) came
// back as interviews and dialogues, and a two-speaker part as a monologue.
func TestExamFormatStatesTheRecordingsSpeakers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		stored    string
		want      []string
		wantNever string
	}{
		{
			name:      "monologue",
			stored:    `{"recording": {"genre": "social_monologue", "speakers": 1}, "plays": 1}`,
			want:      []string{"exactly one person", "no dialogue", "played 1 time"},
			wantNever: "turns",
		},
		{
			name:      "dialogue",
			stored:    `{"recording": {"genre": "educational_discussion", "speakers": 2}}`,
			want:      []string{"students and a tutor", "exactly 2 speakers", `"turns"`},
			wantNever: "monologue",
		},
		{
			name:   "unknown genre",
			stored: `{"recording": {"genre": "radio_phone_in", "speakers": 3}}`,
			want:   []string{"radio phone in", "exactly 3 speakers"},
		},
		{
			name: "speaking part",
			stored: `{"source": "IELTS Academic test format; Speaking Part 2, one minute to prepare",
				"allowed_types": ["speaking_task"], "speaking_seconds": 120, "preparation_seconds": 60}`,
			want: []string{"Part: IELTS Academic test format; Speaking Part 2", "60 seconds to prepare",
				"set speaking_time_seconds to 120"},
			wantNever: "typed",
		},
		{
			name:      "no recording",
			stored:    `{"option_count": 4}`,
			want:      []string{"Options per question: exactly 4."},
			wantNever: "recording is",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var c learningcontract.ExamPartConstraints
			if err := json.Unmarshal([]byte(tc.stored), &c); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got := examFormat(&c)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("format lacks %q:\n%s", w, got)
				}
			}
			if tc.wantNever != "" && strings.Contains(got, tc.wantNever) {
				t.Errorf("format should not mention %q:\n%s", tc.wantNever, got)
			}
			if strings.Contains(got, "\n\n") {
				t.Errorf("format has an empty line:\n%s", got)
			}
		})
	}
}

// IELTS and VSTEP passages came back at about 190 words; the part's length is
// stated and a passage well short of it is refused.
func TestPassageLengthIsStatedAndEnforced(t *testing.T) {
	t.Parallel()

	c := learningcontract.ExamPartConstraints{QuestionsPerGroup: 13, PassageMinWords: 700}
	if got := examFormat(&c); !strings.Contains(got, "at least 700 words") {
		t.Errorf("format lacks the passage length:\n%s", got)
	}

	passage := func(words int) json.RawMessage {
		body, _ := json.Marshal(readingComprehensionCand{Passage: strings.Repeat("word ", words)})
		return body
	}
	if err := checkPassageLength(passage(190), 700); err == nil {
		t.Error("a 190-word passage was accepted for a 700-word part")
	}
	if err := checkPassageLength(passage(640), 700); err != nil {
		t.Errorf("a passage a tenth short should pass: %v", err)
	}
}

// Without a drawn subject the model wrote six IELTS reading passages on two
// topics. An exam item gets one; a Foundation item and a photograph item, whose
// subject is already set, do not.
func TestBuildGenerateVarsDrawsATopicForExamItems(t *testing.T) {
	t.Parallel()

	known := make(map[string]bool, len(examTopics))
	for _, topic := range examTopics {
		known[topic] = true
	}

	exam := buildGenerateVars(learningcontract.GenerateRequest{
		Kind: "reading_comprehension", CEFRLevel: "B2",
		ExamConstraints: &learningcontract.ExamPartConstraints{QuestionsPerGroup: 13},
	}, nil, "")
	topic, _ := exam["Topic"].(string)
	if !known[topic] {
		t.Errorf("exam item topic = %q, want one of examTopics", topic)
	}

	foundation := buildGenerateVars(
		learningcontract.GenerateRequest{Kind: "grammar_tense_choice", CEFRLevel: "A2"}, nil, "")
	if _, ok := foundation["Topic"]; ok {
		t.Error("a Foundation item should not be given a topic")
	}

	photo := buildGenerateVars(learningcontract.GenerateRequest{
		Kind: "photo_description", CEFRLevel: "B1",
		ExamConstraints: &learningcontract.ExamPartConstraints{OptionCount: 4},
		Photo:           &learningcontract.PhotoRef{Description: "A man is typing."},
	}, nil, "")
	if _, ok := photo["Topic"]; ok {
		t.Error("a photograph item already has its subject; it should not be given a topic")
	}
}
