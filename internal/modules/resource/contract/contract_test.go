package contract_test

import (
	"testing"

	"github.com/fluentra/fluentra/internal/modules/resource/contract"
)

func TestClassification_LearningSuitableRequiresCurrentCompleteMetadata(t *testing.T) {
	t.Parallel()
	level, skill := "B1", "grammar"
	const currentPrompt = "resource_classify.v2"
	const nodeCode = "PRESENT_PERFECT"
	for _, tc := range []struct {
		name           string
		classification *contract.Classification
		known          bool
		suitable       bool
	}{
		{"missing classification", nil, false, false},
		{"old complete metadata", &contract.Classification{
			PromptVersion: "resource_classify.v1", CEFREstimate: &level, Skill: &skill, NodeCodes: []string{nodeCode},
		}, false, false},
		{"current complete metadata", &contract.Classification{
			PromptVersion: currentPrompt, CEFREstimate: &level, Skill: &skill, NodeCodes: []string{nodeCode},
		}, true, true},
		{"current unsuitable metadata", &contract.Classification{PromptVersion: currentPrompt}, true, false},
		{"missing CEFR", &contract.Classification{
			PromptVersion: currentPrompt, Skill: &skill, NodeCodes: []string{nodeCode},
		}, true, false},
		{"missing skill", &contract.Classification{
			PromptVersion: currentPrompt, CEFREstimate: &level, NodeCodes: []string{nodeCode},
		}, true, false},
		{"missing grounded nodes", &contract.Classification{
			PromptVersion: currentPrompt, CEFREstimate: &level, Skill: &skill,
		}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.classification.LearningSuitable()
			if (got != nil) != tc.known {
				t.Fatalf("suitability = %v, want known=%v", got, tc.known)
			}
			if got != nil && *got != tc.suitable {
				t.Fatalf("suitable = %v, want %v", *got, tc.suitable)
			}
		})
	}
}
