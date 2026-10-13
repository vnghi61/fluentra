package main

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

// defaultSeedConcurrency is how many workers write at once. A seed is bound by
// round trips, not by the database: against a remote production database each
// statement costs a network hop, and one worker took an hour over a catalogue
// that eight finish in minutes. Eight also stays inside a session pooler's
// connection budget.
const defaultSeedConcurrency = 8

// seedConcurrency is set from -concurrency. 1 is the old, sequential seed.
var seedConcurrency = defaultSeedConcurrency

// newSeedPool opens a pool with a connection for every worker, plus a few for
// the module calls that run beside them. pgxpool's default (the CPU count, at
// least four) would leave most workers waiting for a connection.
func newSeedPool(ctx context.Context, dsn string, workers int) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DB_DSN: %w", err)
	}
	if want := int32(max(workers, 1) + 4); want > cfg.MaxConns { //nolint:gosec // workers is a small flag value
		cfg.MaxConns = want
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return pool, nil
}

// maxSerializationRetries bounds how often one item is retried after Postgres
// cancels its transaction to keep concurrent ones serializable.
const maxSerializationRetries = 16

// maxAuthoringWorkers caps the workers that author through the content module:
// one. Its publish path runs under SERIALIZABLE isolation and writes the
// outbox, and on a fresh database concurrent authors conflicted until their
// retries ran out, with eight workers and again with three (measured
// 2026-10-13). The retry stays: it also covers a running API writing
// alongside the seed.
const maxAuthoringWorkers = 1

// withSerializationRetry runs fn again when its transaction lost a
// serialization conflict (SQLSTATE 40001) or a deadlock (40P01).
//
// The content module authors under SERIALIZABLE isolation. One worker never
// conflicts with itself, but eight authoring exam items at once do, and
// Postgres resolves that by cancelling one of them with "could not serialize
// access", which is an instruction to retry, not a failure. The cancelled
// transaction rolled back whole and the authoring is idempotent by slug, so a
// retry writes exactly what the first attempt would have.
func withSerializationRetry(ctx context.Context, fn func() error) error {
	for attempt := 0; ; attempt++ {
		err := fn()
		if err == nil || attempt >= maxSerializationRetries || !isSerializationFailure(err) {
			return err
		}
		// Jittered, so two workers that conflicted do not retry in lockstep and
		// conflict again.
		base := time.Duration(20*(attempt+1)) * time.Millisecond
		backoff := base + time.Duration(rand.Int64N(int64(base))) //nolint:gosec // jitter, not security
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// isSerializationFailure reports a conflict a retry resolves. The text check
// covers a module that wrapped the driver error with %v instead of %w.
func isSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001" || pgErr.Code == "40P01"
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 40001") || strings.Contains(msg, "SQLSTATE 40P01")
}

// forEachPartitioned runs fn over items on up to `workers` goroutines. Items
// with the same key always land on the same worker, in their original order,
// so a select-then-insert keyed by it never races itself. The first error
// cancels the rest and is returned.
func forEachPartitioned[T any](
	ctx context.Context,
	workers int,
	items []T,
	key func(T) string,
	fn func(ctx context.Context, index int, item T) error,
) error {
	if workers < 1 {
		workers = 1
	}
	if workers > len(items) {
		workers = len(items)
	}
	if workers <= 1 {
		for i, item := range items {
			if err := fn(ctx, i, item); err != nil {
				return err
			}
		}
		return nil
	}

	buckets := make([][]int, workers)
	for i, item := range items {
		h := fnv.New32a()
		_, _ = h.Write([]byte(key(item)))
		b := int(h.Sum32() % uint32(workers)) //nolint:gosec // workers is small and positive
		buckets[b] = append(buckets[b], i)
	}

	group, gctx := errgroup.WithContext(ctx)
	for _, bucket := range buckets {
		group.Go(func() error {
			for _, i := range bucket {
				if err := gctx.Err(); err != nil {
					return err
				}
				if err := fn(gctx, i, items[i]); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return group.Wait()
}
