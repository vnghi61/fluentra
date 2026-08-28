package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

// TestGetManyByIDs_IssuesOneQueryForNIDs is the P1.2 acceptance criterion, and
// the reason GetManyByIDs exists at all. The assertion is on the call count,
// not on the result: a loop around GetByID returns exactly the same map and is
// exactly the thing this method exists to prevent.
func TestGetManyByIDs_IssuesOneQueryForNIDs(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	const population = 50
	ids := make([]uuid.UUID, 0, population)
	for index := range population {
		id := uuid.New()
		ids = append(ids, id)
		h.repo.summaries[id] = domain.Summary{
			ID: id, DisplayName: "Learner", Locale: "en", Timezone: "UTC", Status: domain.StatusActive,
		}
		_ = index
	}

	summaries, err := h.service.GetManyByIDs(context.Background(), ids)
	if err != nil {
		t.Fatalf("GetManyByIDs: %v", err)
	}
	if len(summaries) != population {
		t.Fatalf("returned %d summaries, want %d", len(summaries), population)
	}

	if got := h.repo.callCount("ListSummaries"); got != 1 {
		t.Errorf("ListSummaries was called %d times for %d ids, want 1", got, population)
	}
	if got := h.repo.callCount("GetSummary"); got != 0 {
		t.Errorf("GetSummary was called %d times; the batched path must not fall back to per-id reads", got)
	}
}

// TestGetManyByIDs_SkipsIDsThatDoNotExist is what makes the method usable for
// rendering a list. A deleted account must not fail the whole page.
func TestGetManyByIDs_SkipsIDsThatDoNotExist(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	missing := uuid.New()
	summaries, err := h.service.GetManyByIDs(context.Background(), []uuid.UUID{h.actor, missing})
	if err != nil {
		t.Fatalf("GetManyByIDs: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("returned %d summaries, want 1", len(summaries))
	}
	if _, present := summaries[missing]; present {
		t.Error("an id with no row appeared in the result")
	}
	if summaries[h.actor].DisplayName != nameNghi {
		t.Errorf("summary = %+v, want the existing account", summaries[h.actor])
	}
}

func TestGetManyByIDs_DeduplicatesAndIgnoresTheNilID(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	summaries, err := h.service.GetManyByIDs(context.Background(),
		[]uuid.UUID{h.actor, h.actor, uuid.Nil, h.actor})
	if err != nil {
		t.Fatalf("GetManyByIDs: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("returned %d summaries, want 1", len(summaries))
	}
	if got := h.repo.callCount("ListSummaries"); got != 1 {
		t.Errorf("ListSummaries was called %d times, want 1", got)
	}
}

// TestGetManyByIDs_EmptyInputTouchesNothing keeps the caller's degenerate case
// from becoming a pointless round trip.
func TestGetManyByIDs_EmptyInputTouchesNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	summaries, err := h.service.GetManyByIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetManyByIDs: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("returned %d summaries for no ids", len(summaries))
	}
	if got := h.repo.callCount("ListSummaries"); got != 0 {
		t.Errorf("ListSummaries was called %d times for an empty id list, want 0", got)
	}
}

func TestCreateUser_WritesAllThreeRowsInOneTransaction(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	id, err := h.service.CreateUser(context.Background(), newUserFixture())
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("CreateUser returned the nil uuid")
	}

	if h.pool.begins != 1 {
		t.Errorf("began %d transactions, want 1", h.pool.begins)
	}
	for _, method := range []string{"CreateUser", "CreateProfile", "CreatePreferences"} {
		if got := h.repo.callCount(method); got != 1 {
			t.Errorf("%s was called %d times, want 1", method, got)
		}
	}

	// An account with no preference row would make GET /me/preferences a 404
	// immediately after a successful registration.
	if _, err := h.service.GetPreferences(context.Background(), id); err != nil {
		t.Errorf("the new account has no preferences: %v", err)
	}
}

// A created account that holds no role is what shipped: the access token called
// it `user` because HighestRole of an empty set is `user`, while core.user_roles
// — the table the guard reads — had nothing in it. Harmless until Phase 2 gave
// the `user` role its first permission, and from then on every learner was
// refused the published catalogue.
func TestCreateUser_GrantsTheBaselineRole(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	id, err := h.service.CreateUser(context.Background(), newUserFixture())
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if len(h.roles.granted) != 1 || h.roles.granted[0] != id {
		t.Errorf("baseline role granted to %v, want exactly [%v]", h.roles.granted, id)
	}
}

// Failing loudly is the point. An account that exists, can sign in, and holds
// nothing is indistinguishable from a working one until the learner reads
// something they do not own — which is how this went unnoticed for a phase.
func TestCreateUser_FailsWhenTheBaselineRoleCannotBeGranted(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.roles.err = errors.New("rbac unavailable")

	if _, err := h.service.CreateUser(context.Background(), newUserFixture()); err == nil {
		t.Fatal("CreateUser reported success for an account that holds no role")
	}
}

func TestCreateUser_RejectsAnImpersonatingDisplayNameBeforeWriting(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	newUser := newUserFixture()
	newUser.DisplayName = "Fluentra Admin"
	if _, err := h.service.CreateUser(context.Background(), newUser); err == nil {
		t.Fatal("registration accepted an impersonating display name")
	}
	if h.pool.begins != 0 {
		t.Error("a rejected registration still opened a transaction")
	}
}

func TestCreateUser_DefaultsTheTimezoneRatherThanStoringBlank(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	newUser := newUserFixture()
	newUser.Timezone = ""
	id, err := h.service.CreateUser(context.Background(), newUser)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if got := h.repo.profiles[id].Timezone; got != domain.DefaultTimezone {
		t.Errorf("timezone = %q, want %q", got, domain.DefaultTimezone)
	}
}

func TestExists(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	exists, err := h.service.Exists(context.Background(), h.actor)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("Exists = false for an account that is present")
	}

	exists, err = h.service.Exists(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("Exists = true for an account that was never created")
	}
}

func TestGetByID_ReturnsTheRenderingSummary(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	summary, err := h.service.GetByID(context.Background(), h.actor)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if summary.ID != h.actor || summary.DisplayName != nameNghi {
		t.Errorf("summary = %+v, want the seeded account", summary)
	}
	// The summary is what other modules render. It must never carry the email.
	if summary.Status != string(domain.StatusActive) {
		t.Errorf("status = %q, want active", summary.Status)
	}
}
