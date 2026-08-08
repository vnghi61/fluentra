package outbox

import (
	"context"
	"encoding/json"
	"fmt"

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
func (w *Writer) Write(ctx context.Context, tx DBTx, aggregate, event string, payload any) (uuid.UUID, error) {
	if tx == nil {
		return uuid.Nil, fmt.Errorf("outbox write requires an active transaction")
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
	if _, err := tx.Exec(ctx, query, eventID, aggregate, event, payloadBytes); err != nil {
		return uuid.Nil, fmt.Errorf("insert outbox event: %w", err)
	}
	return eventID, nil
}
