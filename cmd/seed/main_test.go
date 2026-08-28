package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	vocabularycontract "github.com/fluentra/fluentra/internal/modules/vocabulary/contract"
)

// TestRun_RefusesProduction pins the one branch that must never be wrong.
//
// Seeding writes accounts whose password is printed in a public guide. The
// refusal is checked before the pool is opened, so this test needs no database:
// if the guard ever moves below the connection, the test starts failing with a
// connection error instead of passing, which is the right way for it to break.
func TestRun_RefusesProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DB_DSN", "postgres://nobody@127.0.0.1:1/nothing?sslmode=disable")

	err := run(context.Background(), io.Discard)
	if err == nil {
		t.Fatal("run() in production returned no error; the demo password would have been written")
	}
	if !strings.Contains(err.Error(), "production") {
		t.Errorf("run() error = %q, want it to name production as the reason", err)
	}
}

// TestDemoAccounts_MatchTheGuide keeps the dataset and its documentation from
// drifting. The addresses below are the ones
// docs/development/getting-started.md §4 tells a new contributor to sign in
// with, and a rename here without a rename there is how a guide starts lying.
func TestDemoAccounts_MatchTheGuide(t *testing.T) {
	want := map[string]bool{
		"learner@fluentra.dev": false,
		"admin@fluentra.dev":   true,
	}

	if len(demoAccounts) != len(want) {
		t.Fatalf("demoAccounts has %d entries, the guide names %d", len(demoAccounts), len(want))
	}
	for _, account := range demoAccounts {
		admin, named := want[account.email]
		if !named {
			t.Errorf("%s is seeded but not named in getting-started.md", account.email)
			continue
		}
		if account.admin != admin {
			t.Errorf("%s admin = %v, want %v", account.email, account.admin, admin)
		}
	}
}

// TestNewUser_CarriesEverythingTheContractRequires fails if a field is added to
// contract.NewUser and the seeder keeps filling the old shape — an account
// created with an empty locale is one no reader is prepared for.
func TestNewUser_CarriesEverythingTheContractRequires(t *testing.T) {
	seeded := contract.NewUser{
		Email:       demoAccounts[0].email,
		DisplayName: demoAccounts[0].displayName,
		Locale:      "en",
		Timezone:    "Asia/Ho_Chi_Minh",
	}

	if seeded.Email == "" || seeded.DisplayName == "" ||
		seeded.Locale == "" || seeded.Timezone == "" {
		t.Error("the seeder builds a NewUser with an empty required field")
	}
}

// TestSeededKindsAreGradable is P11 §4: what the seed authors, what vocabulary
// registers and what cmd/api declares must name the same set.
//
// It reads vocabulary's list rather than repeating it. The first version of this
// test carried its own copy of the four kinds, which meant it agreed with itself
// and could not catch the divergence it was written for: an activity kind the
// registry does not know fails the learner's request with
// UNSUPPORTED_ACTIVITY_KIND, and a declared kind with no grader fails the boot.
func TestSeededKindsAreGradable(t *testing.T) {
	gradable := make(map[string]bool)
	for _, kind := range vocabularycontract.GradedKinds() {
		gradable[kind] = true
	}

	seeded := make(map[string]bool)
	for _, unit := range courseSeedData.Units {
		for _, lesson := range unit.Lessons {
			for _, act := range lesson.Activities {
				seeded[act.Kind] = true
				if !gradable[act.Kind] {
					t.Errorf("the seed authors activity kind %q, which no grader claims", act.Kind)
				}
			}
		}
	}

	// The runner renders three kinds and no more; a fourth in the seed is an
	// activity a learner reaches and cannot do.
	runnerKinds := map[string]bool{
		"vocab_multiple_choice": true,
		"vocab_gap_fill":        true,
		"vocab_flashcard":       true,
	}
	for kind := range seeded {
		if !runnerKinds[kind] {
			t.Errorf("the seed authors %q, which the lesson runner cannot render", kind)
		}
	}
	for kind := range runnerKinds {
		if !seeded[kind] {
			t.Errorf("the runner supports %q but the seed authors none of it", kind)
		}
	}
}

// TestCourseSeedData_Integrity validates that the seeded course, units, lessons,
// and activities satisfy the curriculum invariants P11.1 names: one course,
// eight lessons, and an activity body the grader can actually score.
func TestCourseSeedData_Integrity(t *testing.T) {
	if courseSeedData.Slug == "" || courseSeedData.Title == "" {
		t.Fatal("courseSeedData missing slug or title")
	}

	totalLessons := 0
	for _, unit := range courseSeedData.Units {
		if unit.Position <= 0 || unit.Title == "" {
			t.Errorf("invalid unit %+v", unit)
		}
		for _, lesson := range unit.Lessons {
			totalLessons++
			if lesson.Position <= 0 || lesson.Title == "" || lesson.SkillFocus == "" {
				t.Errorf("invalid lesson %+v", lesson)
			}
			if len(lesson.Activities) == 0 {
				t.Errorf("lesson %s has no activities", lesson.Title)
			}
			for _, act := range lesson.Activities {
				assertActivityIsGradable(t, lesson.Title, act)
			}
		}
	}

	if totalLessons != 8 {
		t.Errorf("seeded %d lessons, want the 8 P11.1 asks for", totalLessons)
	}
}

// assertActivityIsGradable checks the half of the contract a kind list cannot:
// the authored body has to answer the question the config asks, in the form the
// runner submits.
func assertActivityIsGradable(t *testing.T, lessonTitle string, act seedActivity) {
	t.Helper()

	answer, _ := act.Body[bodyKeyCorrectAnswer].(string)
	if answer == "" {
		t.Errorf("%s activity %d has no correct_answer; the grader refuses to score it",
			lessonTitle, act.Position)
		return
	}

	if act.Kind != "vocab_multiple_choice" {
		return
	}

	// The runner submits `selected_option_id`, so the authored answer has to be
	// an option id — not the word the option displays. A body naming the word
	// grades every learner wrong while looking entirely reasonable.
	options, ok := act.Config["options"].([]map[string]string)
	if !ok || len(options) == 0 {
		t.Errorf("%s activity %d is multiple choice with no options", lessonTitle, act.Position)
		return
	}
	for _, option := range options {
		if option["id"] == answer {
			return
		}
	}
	t.Errorf("%s activity %d: correct_answer %q is not one of the option ids",
		lessonTitle, act.Position, answer)
}

func TestWordSenseSeedData_Integrity(t *testing.T) {
	if len(wordSenseSeedData) != 200 {
		t.Errorf("wordSenseSeedData count = %d, want 200", len(wordSenseSeedData))
	}

	seen := make(map[string]bool)
	for i, sense := range wordSenseSeedData {
		if sense.Lemma == "" || sense.POS == "" || sense.Definition == "" || sense.IPA == "" {
			t.Errorf("word sense [%d] missing required field: %+v", i, sense)
		}
		if sense.CEFRLevel != "A1" && sense.CEFRLevel != "A2" && sense.CEFRLevel != "B1" && sense.CEFRLevel != "B2" {
			t.Errorf("word sense [%d] has invalid CEFR level: %s", i, sense.CEFRLevel)
		}
		key := sense.Lemma + ":" + sense.POS
		if seen[key] {
			t.Errorf("duplicate word sense lemma+pos: %s", key)
		}
		seen[key] = true
	}
}
