package dbx

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is the minimal pgx query surface repositories consume.
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// NewPool creates and verifies a PostgreSQL connection pool.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping pgx pool: %w", err)
	}
	return pool, nil
}

// MaxConnsFloor is the smallest pool either binary will run with, and the
// cheapest fix for a class of failure that looks like a hang rather than an
// error.
//
// pgxpool defaults MaxConns to max(4, NumCPU). Seventeen cron jobs now run once
// at start-up, and each holds a pool connection for the whole of its advisory
// lock and its task -- so on a one-CPU instance the seventeen of them queue for
// four connections and none of them can finish. The scheduler's own semaphore is
// the other half of that fix; this is the half that stops the pool being the
// bottleneck for ordinary request traffic at the same moment.
const MaxConnsFloor = 25

// ResolveMaxConns decides the pool size from configuration and the DSN.
//
// Three rules, in order:
//
//   - A configured value wins, and is bounded. Converting an unchecked int to
//     the int32 the pool takes turns a fat-fingered 3000000000 into a negative
//     size, and the pool then refuses to open with a message naming neither the
//     key nor the value.
//   - A DSN that carries pool_max_conns is left alone, even when it is smaller
//     than the floor. An operator who wrote pool_max_conns=10 against a pooler
//     that allows ten had a reason, and silently tripling it is how one service
//     starves every other tenant of the same database.
//   - Otherwise the floor applies, because the default is the case that hangs.
func ResolveMaxConns(configured int, fromDSN int32, dsnSetsIt bool) (int32, error) {
	if configured > 0 {
		if configured > math.MaxInt32 {
			return 0, fmt.Errorf("db.max_conns %d is larger than a connection pool can express", configured)
		}
		return int32(configured), nil
	}
	if dsnSetsIt {
		return fromDSN, nil
	}
	if fromDSN < MaxConnsFloor {
		return MaxConnsFloor, nil
	}
	return fromDSN, nil
}

// DSNSetsMaxConns reports whether the connection string states a pool size.
//
// pgxpool.ParseConfig fills MaxConns either way, so the parsed config cannot
// tell "the operator asked for ten" from "nobody said, so you get four". The
// difference decides whether the floor applies, and the string is the only
// place it survives.
func DSNSetsMaxConns(dsn string) bool {
	return strings.Contains(dsn, "pool_max_conns")
}

// Beginner starts a pgx transaction. pgxpool.Pool implements this interface.
type Beginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

// TxFunc is a transactional callback.
type TxFunc func(context.Context, pgx.Tx) error

// InTx executes fn in a transaction and retries serialization conflicts up to three times.
func InTx(ctx context.Context, pool Beginner, fn TxFunc) error {
	const attempts = 3
	for attempt := 0; attempt < attempts; attempt++ {
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		err = fn(ctx, tx)
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if err == nil {
			return nil
		}
		if !isSerializationError(err) || attempt == attempts-1 {
			return fmt.Errorf("transaction: %w", err)
		}
	}
	return nil
}

func isSerializationError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "40001"
}
