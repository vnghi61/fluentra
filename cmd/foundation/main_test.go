package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
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

// The command assembles its modules without a database and without a guard.
//
// It did not: content.New fails closed on a nil guard, so `-all` panicked with
// GUARD_REQUIRED before writing a single draft — three call frames from
// anything that mentions HTTP, in a command that serves no HTTP at all. The
// worker had the same bug and grew content.NewAuthoring to fix it; this command
// was never given the same treatment because nothing ever assembled it in a
// test. This is that test.
func TestFoundationCLI_AssemblesWithoutAGuard(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("assembling the generator panicked: %v", r)
		}
	}()

	// No API key, so the AI client is the offline mock and nothing is called.
	generator, publisher := assembleGenerator(
		context.Background(), foundationCLIConfig{}, nil, uuid.New(),
	)
	if generator == nil {
		t.Fatal("assembleGenerator returned no generator")
	}
	// Auto-publish is off by default, so no node is published without a person.
	if publisher != nil {
		t.Fatal("with auto-publish off there must be no batch publisher")
	}
}

// The AI configuration reaches this command at all.
//
// config.Load only reads environment sections the caller has declared, and this
// command declared "app" and "db" but never "ai". Every AI_PROVIDER_* variable
// was dropped on the floor, initAIClient saw no key and returned the offline
// mock, and `-all` would have written six placeholder drafts per spine node
// into a real database while looking like it had worked.
func TestFoundationCLI_ReadsEveryAIProviderSlot(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://example/db")
	t.Setenv("AI_PROVIDER_1_NAME", "primary")
	t.Setenv("AI_PROVIDER_1_API_KEY", "key-1")
	// The working key in a later slot: reading only slot 1 left this unused.
	t.Setenv("AI_PROVIDER_3_NAME", "fallback")
	t.Setenv("AI_PROVIDER_3_API_KEY", "key-3")

	cfg, err := loadFoundationConfig(context.Background())
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}

	providers := cfg.aiProviders()
	if len(providers) != 2 {
		t.Fatalf("read %d providers, want 2: %+v", len(providers), providers)
	}
	if providers[0].Name != "primary" || providers[1].Name != "fallback" {
		t.Errorf("providers = %q, %q; want primary, fallback",
			providers[0].Name, providers[1].Name)
	}
}

// A slot with a name but no key is not a provider: handing it to ai.New would
// produce a client that fails on the first call, which reads as a broken model
// rather than as configuration nobody finished.
func TestFoundationCLI_IgnoresAProviderWithNoKey(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://example/db")
	t.Setenv("AI_PROVIDER_1_NAME", "named-but-unconfigured")

	cfg, err := loadFoundationConfig(context.Background())
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}
	if got := cfg.aiProviders(); len(got) != 0 {
		t.Fatalf("read %d providers, want none: %+v", len(got), got)
	}
}
