package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/modules/user/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

var testNow = time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)

type harness struct {
	service *service.Service
	repo    *fakeRepo
	events  *recordingEvents
	pool    *fakeBeginner
	actor   uuid.UUID
}

// newHarness builds a service over fakes, with one active account already
// registered. Every test starts from an account that exists, because every
// operation here is "the caller acting on themselves".
func newHarness(t *testing.T) *harness {
	t.Helper()

	repo := newFakeRepo()
	events := &recordingEvents{}
	pool := &fakeBeginner{}

	actor := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345678")
	repo.users[actor] = domain.User{
		ID: actor, Email: "learner@example.com", Status: domain.StatusActive,
		CreatedAt: testNow, UpdatedAt: testNow,
	}
	repo.profiles[actor] = domain.Profile{
		UserID: actor, DisplayName: nameNghi, Timezone: timezoneHoChiMinh,
	}
	repo.preferences[actor] = domain.Preferences{
		UserID: actor, Locale: "en", Theme: domain.ThemeSystem, DailyGoalMinutes: 15,
		NotificationChannels: []domain.Channel{domain.ChannelInApp, domain.ChannelEmail},
	}
	repo.summaries[actor] = domain.Summary{
		ID: actor, DisplayName: nameNghi, Locale: "en", Timezone: timezoneHoChiMinh,
		Status: domain.StatusActive,
	}

	users := service.New(service.Deps{
		Pool: pool, Repo: repo, Events: events, Clock: clock.NewFake(testNow),
		NewID: func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
	})
	return &harness{service: users, repo: repo, events: events, pool: pool, actor: actor}
}

func stringPtr(value string) *string { return &value }

func TestGetAccount_ReturnsTheCallersOwnRecord(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	account, err := h.service.GetAccount(context.Background(), h.actor)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if account.User.ID != h.actor {
		t.Errorf("user id = %s, want %s", account.User.ID, h.actor)
	}
	if account.Profile.DisplayName != nameNghi {
		t.Errorf("display name = %q, want the seeded name", account.Profile.DisplayName)
	}
}

// TestGetAccount_UnknownActorIsNotFound covers the case a valid token for a
// deleted account produces. It is a 404 and not a 500: the row is gone, which
// is a state the system can be in.
func TestGetAccount_UnknownActorIsNotFound(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	_, err := h.service.GetAccount(context.Background(), uuid.New())
	if !apperr.Is(err, apperr.NotFound) {
		t.Fatalf("error = %v, want a not-found error", err)
	}
}

func TestUpdateProfile_WritesTheChangeAndExactlyOneEvent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	account, err := h.service.UpdateProfile(context.Background(), h.actor, domain.ProfileChange{
		DisplayName: stringPtr("Nghi Nguyen"),
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if account.Profile.DisplayName != "Nghi Nguyen" {
		t.Errorf("display name = %q, want Nghi Nguyen", account.Profile.DisplayName)
	}
	// Untouched fields must survive a partial update.
	if account.Profile.Timezone != timezoneHoChiMinh {
		t.Errorf("timezone = %q, want it unchanged", account.Profile.Timezone)
	}

	written := h.events.events()
	if len(written) != 1 {
		t.Fatalf("published %d events, want exactly 1", len(written))
	}
	if written[0].Aggregate != contract.Aggregate || written[0].Event != contract.EventProfileUpdated {
		t.Fatalf("event = %s/%s, want %s/%s",
			written[0].Aggregate, written[0].Event, contract.Aggregate, contract.EventProfileUpdated)
	}

	payload, ok := written[0].Payload.(contract.ProfileUpdated)
	if !ok {
		t.Fatalf("payload = %T, want contract.ProfileUpdated", written[0].Payload)
	}
	if payload.UserID != h.actor || payload.ActorID != h.actor {
		t.Errorf("payload identifies %s/%s, want both to be %s", payload.UserID, payload.ActorID, h.actor)
	}
	if len(payload.ChangedFields) != 1 || payload.ChangedFields[0] != "display_name" {
		t.Errorf("changed fields = %v, want [display_name]", payload.ChangedFields)
	}
	if payload.OccurredAt != testNow {
		t.Errorf("occurred_at = %s, want the injected clock's time %s", payload.OccurredAt, testNow)
	}
}

// TestUpdateProfile_ValidationHappensBeforeAnyWrite is what keeps a rejected
// request from costing a transaction, and more importantly from leaving an
// event behind for a change that never happened.
func TestUpdateProfile_ValidationHappensBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	_, err := h.service.UpdateProfile(context.Background(), h.actor, domain.ProfileChange{
		DisplayName: stringPtr("Fluentra Support"),
	})
	if err == nil {
		t.Fatal("an impersonating display name was accepted")
	}
	if h.pool.begins != 0 {
		t.Errorf("began %d transactions for a request that failed validation, want 0", h.pool.begins)
	}
	if len(h.events.events()) != 0 {
		t.Error("an event was published for a change that was rejected")
	}
}

// TestUpdateProfile_NoEventWithoutTheWrite is rule L4 made observable: the
// event and the update share a transaction, so a failing commit must leave
// neither behind.
func TestUpdateProfile_NoEventWithoutTheWrite(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.repo.failOn["UpdateProfile"] = errors.New("constraint violation")

	_, err := h.service.UpdateProfile(context.Background(), h.actor, domain.ProfileChange{
		DisplayName: stringPtr("Nghi Nguyen"),
	})
	if err == nil {
		t.Fatal("a failing update reported success")
	}
	if len(h.events.events()) != 0 {
		t.Error("an event was published even though the update failed")
	}
	if h.pool.rollbacks == 0 {
		t.Error("the transaction was not rolled back")
	}
}

// TestUpdateProfile_FailingEventRollsBackTheWrite is the same guarantee from
// the other side. If the outbox insert fails, the profile change must not
// survive — otherwise the audit trail would be missing a write that happened.
func TestUpdateProfile_FailingEventRollsBackTheWrite(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.events.writeErr = errors.New("outbox unavailable")

	_, err := h.service.UpdateProfile(context.Background(), h.actor, domain.ProfileChange{
		DisplayName: stringPtr("Nghi Nguyen"),
	})
	if err == nil {
		t.Fatal("a failing outbox write reported success")
	}
	if h.pool.rollbacks == 0 {
		t.Error("the transaction was not rolled back after the outbox write failed")
	}
}

func TestUpdateProfile_SuspendedAccountCannotWrite(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	user := h.repo.users[h.actor]
	user.Status = domain.StatusSuspended
	h.repo.users[h.actor] = user

	_, err := h.service.UpdateProfile(context.Background(), h.actor, domain.ProfileChange{
		DisplayName: stringPtr("Nghi Nguyen"),
	})
	if !apperr.Is(err, apperr.Forbidden) {
		t.Fatalf("error = %v, want a forbidden error", err)
	}
	if h.pool.begins != 0 {
		t.Error("a suspended account still opened a write transaction")
	}
}

func TestReplacePreferences_StoresTheWholeSetAndPublishes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	stored, err := h.service.ReplacePreferences(context.Background(), h.actor, domain.Preferences{
		Locale:               "vi",
		Theme:                domain.ThemeDark,
		DailyGoalMinutes:     30,
		NotificationChannels: []domain.Channel{domain.ChannelPush, domain.ChannelInApp},
		AIProcessingOptOut:   true,
	})
	if err != nil {
		t.Fatalf("ReplacePreferences: %v", err)
	}
	if stored.Locale != "vi" || stored.Theme != domain.ThemeDark || !stored.AIProcessingOptOut {
		t.Errorf("stored = %+v, want the submitted values", stored)
	}
	// Stored in the declared order regardless of what the client sent.
	want := []domain.Channel{domain.ChannelInApp, domain.ChannelPush}
	if len(stored.NotificationChannels) != len(want) {
		t.Fatalf("channels = %v, want %v", stored.NotificationChannels, want)
	}
	for index := range want {
		if stored.NotificationChannels[index] != want[index] {
			t.Fatalf("channels = %v, want %v", stored.NotificationChannels, want)
		}
	}

	written := h.events.events()
	if len(written) != 1 || written[0].Event != contract.EventPreferencesUpdated {
		t.Fatalf("events = %+v, want one %s", written, contract.EventPreferencesUpdated)
	}
}

func TestReplacePreferences_RejectsAnInvalidSetBeforeWriting(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	_, err := h.service.ReplacePreferences(context.Background(), h.actor, domain.Preferences{
		Locale: "en", Theme: domain.ThemeDark, DailyGoalMinutes: 4,
		NotificationChannels: []domain.Channel{domain.ChannelInApp},
	})
	if !apperr.Is(err, apperr.Validation) {
		t.Fatalf("error = %v, want a validation error", err)
	}
	if h.pool.begins != 0 {
		t.Error("an invalid preference set opened a transaction")
	}
}
