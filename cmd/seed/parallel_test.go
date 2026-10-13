package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type keyed struct {
	key   string
	order int
}

// Items sharing a key are written by one worker, in their original order: the
// vocabulary seed's sense lookup-then-insert depends on it.
func TestForEachPartitioned_KeepsAKeyOnOneWorkerInOrder(t *testing.T) {
	var items []keyed
	for i := range 200 {
		items = append(items, keyed{key: fmt.Sprintf("word-%d", i%13), order: i})
	}

	var mu sync.Mutex
	seen := map[string][]int{}
	visited := map[int]bool{}
	err := forEachPartitioned(context.Background(), 8, items,
		func(k keyed) string { return k.key },
		func(_ context.Context, index int, k keyed) error {
			mu.Lock()
			defer mu.Unlock()
			if index != k.order {
				return fmt.Errorf("index %d passed for item %d", index, k.order)
			}
			seen[k.key] = append(seen[k.key], k.order)
			visited[index] = true
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(visited) != len(items) {
		t.Fatalf("visited %d items, want %d", len(visited), len(items))
	}
	for key, orders := range seen {
		for i := 1; i < len(orders); i++ {
			if orders[i] <= orders[i-1] {
				t.Fatalf("key %s ran out of order: %v", key, orders)
			}
		}
	}
}

func TestForEachPartitioned_ReturnsTheFirstError(t *testing.T) {
	boom := errors.New("boom")
	items := []keyed{{key: "a"}, {key: "b"}, {key: "c"}, {key: "d"}}
	err := forEachPartitioned(context.Background(), 4, items,
		func(k keyed) string { return k.key },
		func(_ context.Context, _ int, k keyed) error {
			if k.key == "c" {
				return boom
			}
			return nil
		})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

func TestForEachPartitioned_OneWorkerIsSequential(t *testing.T) {
	var got []int
	items := []keyed{{key: "x", order: 0}, {key: "y", order: 1}, {key: "z", order: 2}}
	err := forEachPartitioned(context.Background(), 1, items,
		func(k keyed) string { return k.key },
		func(_ context.Context, i int, _ keyed) error {
			got = append(got, i)
			return nil
		})
	if err != nil || fmt.Sprint(got) != "[0 1 2]" {
		t.Fatalf("got %v, %v; want [0 1 2]", got, err)
	}
}

func TestWithSerializationRetry_RetriesOnlyConflicts(t *testing.T) {
	conflict := &pgconn.PgError{Code: "40001", Message: "could not serialize access"}

	calls := 0
	err := withSerializationRetry(context.Background(), func() error {
		calls++
		if calls < 3 {
			return fmt.Errorf("author item: %w", conflict)
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("conflicts: err=%v calls=%d, want nil after 3", err, calls)
	}

	// A conflict reported as text, by a module that wrapped with %v.
	calls = 0
	_ = withSerializationRetry(context.Background(), func() error {
		calls++
		if calls == 1 {
			return errors.New("transaction: ERROR: could not serialize access (SQLSTATE 40001)")
		}
		return nil
	})
	if calls != 2 {
		t.Fatalf("text conflict retried %d time(s), want 1", calls-1)
	}

	// Anything else is returned at once.
	calls = 0
	boom := errors.New("unique violation")
	err = withSerializationRetry(context.Background(), func() error { calls++; return boom })
	if !errors.Is(err, boom) || calls != 1 {
		t.Fatalf("other error: err=%v calls=%d, want boom after 1", err, calls)
	}
}
