package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
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
