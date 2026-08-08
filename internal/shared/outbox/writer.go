package outbox

import (
	"context"
	"encoding/json"
	"fmt"
)

// DBTx defines the database transaction interface required by outbox writer.
type DBTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag any, err error)
}

// EventPayload contains data for an outbox event.
type EventPayload struct {
	Aggregate string
	Event     string
	Payload   any
}

// Writer writes events into ops.outbox_events inside the caller's transaction.
type Writer struct{}

// NewWriter creates an outbox writer helper.
func NewWriter() *Writer {
	return &Writer{}
}

// Write inserts an outbox event into ops.outbox_events using tx.
func (w *Writer) Write(ctx context.Context, tx DBTx, aggregate, event string, payload any) error {
	if tx == nil {
		return fmt.Errorf("outbox write requires an active transaction")
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}

	const query = `
		INSERT INTO ops.outbox_events (aggregate, event, payload)
		VALUES ($1, $2, $3)
	`
	if _, err := tx.Exec(ctx, query, aggregate, event, payloadBytes); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}
