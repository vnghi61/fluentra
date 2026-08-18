package httpx_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/httpx"
)

func TestActorContext(t *testing.T) {
	t.Parallel()

	// 1. Unauthenticated context returns false
	ctx := context.Background()
	if actor, ok := httpx.ActorFrom(ctx); ok || actor.UserID != uuid.Nil {
		t.Fatalf("expected unauthenticated context to return false, got %#v", actor)
	}

	// 2. Context with zero UserID returns false
	ctxZero := httpx.WithActor(ctx, httpx.Actor{UserID: uuid.Nil, Role: "user"})
	if actor, ok := httpx.ActorFrom(ctxZero); ok || actor.UserID != uuid.Nil {
		t.Fatalf("expected zero UserID actor to return false, got %#v", actor)
	}

	// 3. Valid actor returns true and matches fields
	validID := uuid.New()
	sessID := uuid.New()
	expected := httpx.Actor{
		UserID:    validID,
		SessionID: sessID,
		Role:      "admin",
		TokenID:   "token-123",
	}
	ctxValid := httpx.WithActor(ctx, expected)
	got, ok := httpx.ActorFrom(ctxValid)
	if !ok {
		t.Fatalf("expected valid actor to be found in context")
	}
	if got != expected {
		t.Fatalf("got actor %#v, want %#v", got, expected)
	}
}
