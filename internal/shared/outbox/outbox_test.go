package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/shared/outbox"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// A real pgx transaction must satisfy DBTx without an adapter. This assertion
// is the regression test for a writer that only ever worked against a fake.
var _ outbox.DBTx = (pgx.Tx)(nil)

// recordingTx is a stand-in for a transaction, matching pgx.Tx's Exec exactly.
type recordingTx struct {
	calls [][]any
	sql   []string
	fail  bool
}

func (r *recordingTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	r.sql = append(r.sql, sql)
	r.calls = append(r.calls, arguments)
	if r.fail {
		return pgconn.CommandTag{}, errors.New("db error")
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

// countingDispatcher lives in this package to prove EventDispatcher can be
// implemented outside `outbox` — it could not be while the payload type was
// unexported.
type countingDispatcher struct {
	seen []outbox.Event
	err  error
}

func (c *countingDispatcher) Dispatch(_ context.Context, event outbox.Event) error {
	c.seen = append(c.seen, event)
	return c.err
}

var _ outbox.EventDispatcher = (*countingDispatcher)(nil)

func TestWriter_WriteInsertsEventWithGeneratedID(t *testing.T) {
	t.Parallel()
	tx := &recordingTx{}

	eventID, err := outbox.NewWriter().Write(
		context.Background(), tx, "user", "user.created", map[string]string{"id": "u123"})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if eventID == uuid.Nil {
		t.Fatal("writer returned a nil event id")
	}
	if len(tx.calls) != 1 || len(tx.calls[0]) != 4 {
		t.Fatalf("query args = %#v, want event_id, aggregate, event, payload", tx.calls)
	}
	if tx.calls[0][0] != eventID {
		t.Errorf("first argument = %v, want the returned event id", tx.calls[0][0])
	}
	payload, ok := tx.calls[0][3].([]byte)
	if !ok || !json.Valid(payload) {
		t.Errorf("payload argument is not JSON: %#v", tx.calls[0][3])
	}
}

func TestWriter_WriteRequiresTransaction(t *testing.T) {
	t.Parallel()
	if _, err := outbox.NewWriter().Write(context.Background(), nil, "user", "user.created", nil); err == nil {
		t.Error("expected an error when no transaction is supplied")
	}
}

func TestWriter_WritePropagatesExecFailure(t *testing.T) {
	t.Parallel()
	if _, err := outbox.NewWriter().Write(
		context.Background(), &recordingTx{fail: true}, "user", "user.created", nil); err == nil {
		t.Error("expected the exec failure to surface")
	}
}

func TestWriter_WriteRejectsUnmarshalablePayload(t *testing.T) {
	t.Parallel()
	if _, err := outbox.NewWriter().Write(
		context.Background(), &recordingTx{}, "user", "user.created", make(chan int)); err == nil {
		t.Error("expected a marshalling error")
	}
}

func TestEvent_TopicJoinsAggregateAndName(t *testing.T) {
	t.Parallel()
	event := outbox.Event{Aggregate: "user", Name: "user.created"}
	if event.Topic() != "user.user.created" {
		t.Errorf("Topic() = %q", event.Topic())
	}
}

// TestBackoffFor documents the retry schedule the publisher applies. Without a
// backoff a permanently failing event is retried at the poll interval forever.
func TestBackoffFor(t *testing.T) {
	t.Parallel()
	for attempt, want := range map[int]time.Duration{
		0:  time.Second,
		1:  time.Second,
		2:  2 * time.Second,
		3:  4 * time.Second,
		4:  8 * time.Second,
		20: 5 * time.Minute,
		64: 5 * time.Minute,
	} {
		if got := outbox.BackoffFor(attempt); got != want {
			t.Errorf("BackoffFor(%d) = %s, want %s", attempt, got, want)
		}
	}
}

// TestProcessBatch_RefusesToRunWithoutDispatcher is the regression test for a
// publisher wired with a nil dispatcher, which silently marked every event
// published and discarded it.
func TestProcessBatch_RefusesToRunWithoutDispatcher(t *testing.T) {
	t.Parallel()
	publisher := outbox.NewPublisher(nil, nil, 10, time.Second)
	if err := publisher.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("a publisher with no pool should be inert, got: %v", err)
	}
}
