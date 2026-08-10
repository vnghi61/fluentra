package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fluentra/fluentra/internal/shared/id"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTx is the transaction surface the writer needs. Its signature matches
// pgx.Tx exactly, so a real transaction satisfies it without an adapter — the
// whole point of the outbox is that the event and the business write commit
// together, and a writer that cannot take the caller's transaction cannot
// deliver that.
type DBTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// Writer writes events into ops.outbox_events inside the caller's transaction.
type Writer struct{}

// NewWriter creates an outbox writer helper.
func NewWriter() *Writer { return &Writer{} }

// Write inserts an outbox event using tx and returns the event id. Consumers
// deduplicate on that id, so callers that need to correlate should keep it.
//
// `event` may be given either bare ("profile_updated") or already qualified
// with the aggregate ("user.profile_updated"), and the qualified form is what
// every module's contract declares as its event constant — that constant is
// the wire value a consumer subscribes to, which is exactly what Topic()
// returns. Storing it verbatim in the `event` column would make Topic()
// produce "user.user.profile_updated", so the prefix is stripped here.
//
// That is not a hypothetical. It is what the column held until P1.5 wired the
// first consumer: every event published since P1.2 went out under a doubled
// topic, matched no subscriber, and — because an event with no handlers is
// accepted rather than retried — was marked published and discarded. Nothing
// failed, because nothing was listening. See TestTopicRoundTripsContractNames.
func (w *Writer) Write(ctx context.Context, tx DBTx, aggregate, event string, payload any) (uuid.UUID, error) {
	if tx == nil {
		return uuid.Nil, fmt.Errorf("outbox write requires an active transaction")
	}
	name := BareEventName(aggregate, event)
	if name == "" {
		return uuid.Nil, fmt.Errorf("outbox write requires an event name")
	}

	eventID, err := id.NewUUIDv7(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("generate outbox event id: %w", err)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal outbox payload: %w", err)
	}

	const query = `
		INSERT INTO ops.outbox_events (event_id, aggregate, event, payload)
		VALUES ($1, $2, $3, $4)
	`
	if _, err := tx.Exec(ctx, query, eventID, aggregate, name, payloadBytes); err != nil {
		return uuid.Nil, fmt.Errorf("insert outbox event: %w", err)
	}
	return eventID, nil
}

// BareEventName strips a leading `<aggregate>.` from an event name, so that
// Event.Topic() reassembling the two halves gives back what the caller meant.
//
// It is exported because it is the inverse of Topic(), and a test that asserts
// the round trip should be able to name both halves.
func BareEventName(aggregate, event string) string {
	if aggregate == "" {
		return event
	}
	return strings.TrimPrefix(event, aggregate+".")
}
