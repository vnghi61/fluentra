package outbox_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fluentra/fluentra/internal/shared/outbox"
)

type mockTx struct {
	execCalled bool
	sql        string
	args       []any
	fail       bool
}

func (m *mockTx) Exec(ctx context.Context, sql string, arguments ...any) (any, error) {
	m.execCalled = true
	m.sql = sql
	m.args = arguments
	if m.fail {
		return nil, errors.New("db error")
	}
	return nil, nil
}

func TestWriter_Write_Success(t *testing.T) {
	writer := outbox.NewWriter()
	tx := &mockTx{}

	err := writer.Write(context.Background(), tx, "user", "user.created", map[string]string{"id": "u123"})
	if err != nil {
		t.Fatalf("expected write to succeed, got: %v", err)
	}

	if !tx.execCalled {
		t.Error("expected DB Exec to be called")
	}

	if len(tx.args) != 3 {
		t.Errorf("expected 3 query args, got %d", len(tx.args))
	}
}

func TestWriter_Write_NilTxFails(t *testing.T) {
	writer := outbox.NewWriter()
	err := writer.Write(context.Background(), nil, "user", "user.created", map[string]string{"id": "u123"})
	if err == nil {
		t.Error("expected error when transaction is nil")
	}
}
