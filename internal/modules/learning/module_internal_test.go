package learning

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/learning/repository"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

type dummyOutboxTx struct{}

func (dummyOutboxTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func TestRepositoryAdapter_WithTx(t *testing.T) {
	repo := repository.New(nil)
	adapter := repositoryAdapter{Repository: repo}
	adapted := adapter.WithTx(nil)
	if adapted == nil {
		t.Fatal("expected non-nil adapted repository")
	}
}

func TestOutboxWriter_Write(t *testing.T) {
	writer := outboxWriter{Writer: outbox.NewWriter()}
	ctx := context.Background()
	_, err := writer.Write(ctx, dummyOutboxTx{}, "test", "test.event", map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("unexpected error from outbox writer: %v", err)
	}
}
