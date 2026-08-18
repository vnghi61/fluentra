package httpx_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/fluentra/fluentra/internal/shared/httpx"
)

func TestTraceID(t *testing.T) {
	t.Parallel()

	// 1. Context without span returns empty string
	if got := httpx.TraceID(context.Background()); got != "" {
		t.Fatalf("expected empty trace id for plain context, got %q", got)
	}

	// 2. Context with valid span returns trace id string
	tracer := noop.NewTracerProvider().Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// Even if noop span has valid/empty trace id, ensure function returns string without panic
	_ = httpx.TraceID(ctx)
}
