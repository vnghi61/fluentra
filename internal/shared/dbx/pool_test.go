package dbx

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestInTx_Success(t *testing.T) {
	t.Parallel()
	tx := &fakeTx{}
	pool := &fakePool{transactions: []pgx.Tx{tx}}
	err := InTx(context.Background(), pool, func(_ context.Context, got pgx.Tx) error {
		if got != tx {
			t.Fatal("unexpected transaction")
		}
		return nil
	})
	if err != nil || tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("err=%v commits=%d rollbacks=%d", err, tx.commits, tx.rollbacks)
	}
}

func TestInTx_RetriesSerializationAndRollsBackCallbackError(t *testing.T) {
	t.Parallel()
	serialization := &pgconn.PgError{Code: "40001"}
	first, second := &fakeTx{commitErr: serialization}, &fakeTx{}
	pool := &fakePool{transactions: []pgx.Tx{first, second}}
	if err := InTx(context.Background(), pool, func(context.Context, pgx.Tx) error { return nil }); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if pool.calls != 2 || second.commits != 1 {
		t.Fatalf("calls=%d commits=%d", pool.calls, second.commits)
	}
	failed := &fakeTx{}
	err := InTx(context.Background(), &fakePool{transactions: []pgx.Tx{failed}}, func(context.Context, pgx.Tx) error { return errors.New("callback failed") })
	if err == nil || failed.rollbacks != 1 {
		t.Fatalf("err=%v rollbacks=%d", err, failed.rollbacks)
	}
}

func TestInTx_BeginAndFinalErrors(t *testing.T) {
	t.Parallel()
	if err := InTx(context.Background(), &fakePool{beginErr: errors.New("unavailable")}, func(context.Context, pgx.Tx) error { return nil }); err == nil {
		t.Fatal("expected begin error")
	}
	pool := &fakePool{transactions: []pgx.Tx{&fakeTx{commitErr: &pgconn.PgError{Code: "40001"}}, &fakeTx{commitErr: &pgconn.PgError{Code: "40001"}}, &fakeTx{commitErr: &pgconn.PgError{Code: "40001"}}}}
	if err := InTx(context.Background(), pool, func(context.Context, pgx.Tx) error { return nil }); err == nil || pool.calls != 3 {
		t.Fatalf("err=%v calls=%d", err, pool.calls)
	}
}

func TestNewPool_InvalidDSN(t *testing.T) {
	t.Parallel()
	if pool, err := NewPool(context.Background(), "postgres://%zz"); err == nil || pool != nil {
		t.Fatalf("pool=%v err=%v", pool, err)
	}
}

func TestSerializationErrorAndCancelledPool(t *testing.T) {
	t.Parallel()
	if isSerializationError(errors.New("ordinary")) {
		t.Fatal("ordinary error must not be retryable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if pool, err := NewPool(ctx, "postgres://localhost:5432/fluentra"); err == nil || pool != nil {
		t.Fatalf("pool=%v err=%v", pool, err)
	}
}

type fakePool struct {
	transactions []pgx.Tx
	beginErr     error
	calls        int
}

func (p *fakePool) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	p.calls++
	if p.beginErr != nil {
		return nil, p.beginErr
	}
	return p.transactions[p.calls-1], nil
}

type fakeTx struct {
	commitErr error
	commits   int
	rollbacks int
}

func (t *fakeTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *fakeTx) Commit(context.Context) error          { t.commits++; return t.commitErr }
func (t *fakeTx) Rollback(context.Context) error        { t.rollbacks++; return nil }
func (*fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (*fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (*fakeTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (*fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (*fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (*fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (*fakeTx) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }
func (*fakeTx) Conn() *pgx.Conn                                         { return nil }
