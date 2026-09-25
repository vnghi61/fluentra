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
