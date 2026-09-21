package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
)

func TestValidateFoundationTopicBody(t *testing.T) {
	t.Parallel()

	validBody := domain.FoundationTopicBody{
		SchemaVersion: 1,
		Objective:     "Use the present perfect for experience and for unfinished time.",
		Explanation: domain.ExplanationLocales{
			EN: "The present perfect connects the past with the present.",
			VI: "Thì hiện tại hoàn thành kết nối quá khứ với hiện tại.",
		},
		Examples: []domain.TopicExample{
			{Text: "I have lived here for ten years.", Note: "unfinished time"},
		},
		Related: []string{codePastSimple, "PRESENT_PERFECT_CONTINUOUS"},
		CommonMistakes: []domain.CommonMistake{
			{
				Wrong: "I have seen him yesterday.",
				Right: "I saw him yesterday.",
				Why:   "Yesterday specifies a finished past time.",
			},
		},
	}

	validJSON, err := json.Marshal(validBody)
	if err != nil {
		t.Fatalf("failed to marshal validBody: %v", err)
	}

	t.Run("valid body passes", func(t *testing.T) {
		if err := domain.ValidateFoundationTopicBody(validJSON); err != nil {
			t.Fatalf("expected valid body to pass: %v", err)
		}
	})

	t.Run("empty payload fails", func(t *testing.T) {
		if err := domain.ValidateFoundationTopicBody(nil); err == nil {
			t.Fatal("expected empty payload to fail")
		}
	})

	t.Run("unsupported schema version fails", func(t *testing.T) {
		badVersion := validBody
		badVersion.SchemaVersion = 2
		raw, _ := json.Marshal(badVersion)
		if err := domain.ValidateFoundationTopicBody(raw); err == nil {
			t.Fatal("expected schema_version 2 to fail")
		}
	})

	t.Run("missing objective fails", func(t *testing.T) {
		bad := validBody
		bad.Objective = ""
		raw, _ := json.Marshal(bad)
		if err := domain.ValidateFoundationTopicBody(raw); err == nil {
			t.Fatal("expected missing objective to fail")
		}
	})

	t.Run("missing vietnamese explanation fails", func(t *testing.T) {
		bad := validBody
		bad.Explanation.VI = ""
		raw, _ := json.Marshal(bad)
		if err := domain.ValidateFoundationTopicBody(raw); err == nil {
			t.Fatal("expected missing VI explanation to fail")
		}
	})

	t.Run("empty examples fails", func(t *testing.T) {
		bad := validBody
		bad.Examples = nil
		raw, _ := json.Marshal(bad)
		if err := domain.ValidateFoundationTopicBody(raw); err == nil {
			t.Fatal("expected empty examples to fail")
		}
	})

	t.Run("empty example text fails", func(t *testing.T) {
		bad := validBody
		bad.Examples = []domain.TopicExample{{Text: "  ", Note: ""}}
		raw, _ := json.Marshal(bad)
		if err := domain.ValidateFoundationTopicBody(raw); err == nil {
			t.Fatal("expected empty example text to fail")
		}
	})
}
