package service

import (
	"context"
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

// IELTS listening scripts came back at about 150 words with ten questions.
func TestListeningScriptLengthIsEnforced(t *testing.T) {
	t.Parallel()

	body := func(words int) json.RawMessage {
		raw, _ := json.Marshal(listeningCand{
			Title:  "A campus tour",
			Script: strings.Repeat("word ", words),
			Questions: []candQuestion{{
				ID: "q1", Type: "multiple_choice", Prompt: "Where does the tour start?",
				Options: []candOption{
					{ID: "A", Text: "the library"}, {ID: "B", Text: "the gym"},
					{ID: "C", Text: "the gate"}, {ID: "D", Text: "the canteen"},
				},
				CorrectOptionID: "C",
			}},
		})
		return raw
	}
	req := learningcontract.GenerateRequest{
		Kind:            kindListeningComprehension,
		ExamConstraints: &learningcontract.ExamPartConstraints{QuestionsPerGroup: 1, ScriptMinWords: 600},
	}
	s := &Service{}
	if _, err := s.prepareCandidateBody(context.Background(), req, body(150)); err == nil {
		t.Error("a 150-word script was accepted for a 600-word part")
	}
	if _, err := s.prepareCandidateBody(context.Background(), req, body(600)); err != nil {
		t.Errorf("a 600-word script was refused: %v", err)
	}
	if got := examFormat(req.ExamConstraints); !strings.Contains(got, "at least 600 words") {
		t.Errorf("format lacks the script length:\n%s", got)
	}

	// A part with no length still refuses a one-sentence "script".
	req.ExamConstraints.ScriptMinWords = 0
	if _, err := s.prepareCandidateBody(context.Background(), req, body(10)); err == nil {
		t.Error("a 10-word script was accepted")
	}
	if _, err := s.prepareCandidateBody(context.Background(), req, body(100)); err != nil {
		t.Errorf("a 100-word script was refused: %v", err)
	}
}

// A Part 1 body holds only the image URL; the blind solver, a text model, is
// told what the photograph shows, and nothing else is.
func TestSolverIsToldWhatThePhotographShows(t *testing.T) {
	t.Parallel()

	body := json.RawMessage(`{"image_url": "https://example.org/p.jpg", "correct_option_id": "B"}`)
	photo := &learningcontract.PhotoRef{Description: "A man is typing at a desk."}

	var got map[string]any
	if err := json.Unmarshal(withSolverPhotoDescription(body, photo), &got); err != nil {
		t.Fatal(err)
	}
	if desc, _ := got["photo_description"].(string); !strings.Contains(desc, "A man is typing at a desk.") {
		t.Errorf("solver body lacks the description: %v", got)
	}
	if string(withSolverPhotoDescription(body, nil)) != string(body) {
		t.Error("an item without a photograph was changed")
	}
}

// TOEIC Part 2 came back as general-knowledge quizzes; a TOEIC part draws
// only workplace and everyday subjects.
func TestTOEICDrawsWorkplaceTopics(t *testing.T) {
	t.Parallel()

	workplace := make(map[string]bool, len(workplaceTopics))
	for _, topic := range workplaceTopics {
		workplace[topic] = true
	}
	toeic := &learningcontract.ExamPartConstraints{Source: "ETS TOEIC L&R format"}
	for range 200 {
		if topic := examTopic(toeic); !workplace[topic] {
			t.Fatalf("a TOEIC part drew %q", topic)
		}
	}
	ielts := &learningcontract.ExamPartConstraints{Source: "IELTS Academic test format; Reading Passage 1"}
	for range 500 {
		if !workplace[examTopic(ielts)] {
			return
		}
	}
	t.Error("in 500 draws an IELTS part never drew beyond the workplace list")
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
