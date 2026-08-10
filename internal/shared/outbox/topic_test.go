package outbox_test

import (
	"context"
	"testing"

	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// storedEvent reads back the `aggregate` and `event` column values the writer
// sent, which is the only way to see which half of the topic landed where.
// recordingTx is the shared fake in outbox_test.go.
func storedEvent(t *testing.T, tx *recordingTx) (aggregate, event string) {
	t.Helper()
	if len(tx.calls) == 0 {
		t.Fatal("the writer sent no statement")
	}
	arguments := tx.calls[len(tx.calls)-1]
	if len(arguments) < 3 {
		t.Fatalf("the insert took %d arguments, want at least 3", len(arguments))
	}
	aggregate, _ = arguments[1].(string)
	event, _ = arguments[2].(string)
	return aggregate, event
}

// The halves of the one topic these tests name repeatedly.
const (
	aggregateUser       = "user"
	eventProfileUpdated = "profile_updated"
)

// TestTopicRoundTripsContractNames is the regression test for the bug P1.5
// found.
//
// Every module's contract declares its event constant fully qualified —
// `user.EventProfileUpdated = "user.profile_updated"` — because that constant
// is the wire value a consumer subscribes to, and that value is what Topic()
// produces. Storing it verbatim in the `event` column made Topic() return
// "user.user.profile_updated".
//
// Nothing failed at the time, and that is the part worth remembering. An event
// with no handlers is accepted rather than retried, so every event published
// between P1.2 and P1.5 was marked published and thrown away in silence. The
// first consumer is what made it visible.
func TestTopicRoundTripsContractNames(t *testing.T) {
	t.Parallel()

	// The literals are the contract constants of `user` and `rbac`. They are
	// written out rather than imported, because `shared` may not depend on a
	// module; a copy that drifts is caught by the wiring integration test in
	// cmd/api, which drives the real constants end to end.
	cases := []struct {
		aggregate string
		declared  string
		wantEvent string
		wantTopic string
	}{
		{aggregateUser, "user." + eventProfileUpdated, eventProfileUpdated, "user." + eventProfileUpdated},
		{aggregateUser, "user.preferences_updated", "preferences_updated", "user.preferences_updated"},
		{aggregateUser, "user.deleted", "deleted", "user.deleted"},
		{"rbac", "rbac.role_assigned", "role_assigned", "rbac.role_assigned"},
		{"rbac", "rbac.access_denied", "access_denied", "rbac.access_denied"},

		// A caller passing the bare name is equally correct and must lose
		// nothing.
		{aggregateUser, eventProfileUpdated, eventProfileUpdated, "user." + eventProfileUpdated},

		// Only the leading aggregate is stripped, and only once. A name that
		// merely starts with the same letters keeps them.
		{aggregateUser, "user.user_merged", "user_merged", "user.user_merged"},
		{aggregateUser, "users_exported", "users_exported", "user.users_exported"},
	}

	writer := outbox.NewWriter()
	for _, testCase := range cases {
		tx := &recordingTx{}
		if _, err := writer.Write(
			context.Background(), tx, testCase.aggregate, testCase.declared, map[string]any{},
		); err != nil {
			t.Fatalf("Write(%q, %q): %v", testCase.aggregate, testCase.declared, err)
		}

		aggregate, event := storedEvent(t, tx)
		if event != testCase.wantEvent {
			t.Errorf("Write(%q, %q) stored event %q, want %q",
				testCase.aggregate, testCase.declared, event, testCase.wantEvent)
		}

		delivered := outbox.Event{Aggregate: aggregate, Name: event}
		if got := delivered.Topic(); got != testCase.wantTopic {
			t.Errorf("Write(%q, %q) then Topic() = %q, want %q — a subscriber to the "+
				"contract constant would never match",
				testCase.aggregate, testCase.declared, got, testCase.wantTopic)
		}
	}
}

func TestWriteRefusesAnEventNameThatIsOnlyItsAggregate(t *testing.T) {
	t.Parallel()

	writer := outbox.NewWriter()
	tx := &recordingTx{}
	if _, err := writer.Write(context.Background(), tx, aggregateUser, "user.", map[string]any{}); err == nil {
		t.Error("an event name that is nothing but its aggregate was accepted")
	}
	if len(tx.calls) != 0 {
		t.Error("a row was written for an event with no name")
	}
}

func TestBareEventName(t *testing.T) {
	t.Parallel()

	cases := []struct{ aggregate, event, want string }{
		{aggregateUser, "user." + eventProfileUpdated, eventProfileUpdated},
		{aggregateUser, eventProfileUpdated, eventProfileUpdated},
		{"", "user." + eventProfileUpdated, "user." + eventProfileUpdated},
		{aggregateUser, "user.user." + eventProfileUpdated, "user." + eventProfileUpdated},
	}
	for _, testCase := range cases {
		if got := outbox.BareEventName(testCase.aggregate, testCase.event); got != testCase.want {
			t.Errorf("BareEventName(%q, %q) = %q, want %q",
				testCase.aggregate, testCase.event, got, testCase.want)
		}
	}
}
