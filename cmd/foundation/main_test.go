package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestFoundationCLI_ArgumentValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("missing node and all flags", func(t *testing.T) {
		var out bytes.Buffer
		err := run(ctx, []string{}, &out)
		if err == nil {
			t.Fatal("expected error when neither -node nor -all is specified")
		}
		if !strings.Contains(err.Error(), "must specify either -node CODE or -all") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("unknown node flag", func(t *testing.T) {
		var out bytes.Buffer
		err := run(ctx, []string{"-unknown-flag"}, &out)
		if err == nil {
			t.Fatal("expected error on unknown flag")
		}
	})
}

func TestFoundationCLI_ExerciseKindsForNode(t *testing.T) {
	cases := []struct {
		namespace string
		code      string
		wantLen   int
	}{
		{"grammar", "PRESENT_PERFECT", 3},
		{"vocabulary", "TECH_TERMS", 3},
		{"pattern", "IT_TAKES_TIME", 3},
		{"pronunciation", "VOWELS", 3},
		{nsSkill, "READING", 3},
		{nsSkill, "LISTENING", 3},
		{nsSkill, "WRITING", 3},
		{nsSkill, "SPEAKING", 3},
		{nsSkill, "GENERAL", 3},
		{"other", "CUSTOM", 3},
	}

	for _, tc := range cases {
		kinds := exerciseKindsForNode(tc.namespace, tc.code)
		if len(kinds) != tc.wantLen {
			t.Errorf("exerciseKindsForNode(%q, %q) returned %d kinds, want %d",
				tc.namespace, tc.code, len(kinds), tc.wantLen)
		}
	}
}
