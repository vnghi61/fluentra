package id

import (
	"context"
	"errors"
	"testing"
)

func TestNewIdentifiers(t *testing.T) {
	t.Parallel()
	uuid, err := NewUUIDv7(context.Background())
	if err != nil || uuid.Version() != 7 {
		t.Fatalf("UUIDv7 = %s, %v", uuid, err)
	}
	ulid, err := NewULID(context.Background())
	if err != nil || ulid.String() == "" {
		t.Fatalf("ULID = %s, %v", ulid, err)
	}
}

func TestNewIdentifiers_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewUUIDv7(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("UUID error = %v", err)
	}
	if _, err := NewULID(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("ULID error = %v", err)
	}
}
