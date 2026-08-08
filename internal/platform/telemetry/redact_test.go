package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/shared/httpx"
)

func TestRedactingHandler_FailsClosed(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil), []string{"request_id"}))
	logger.Info("request completed", "request_id", "req-1", "email", "learner@example.com")
	line := output.String()
	if !strings.Contains(line, "req-1") || strings.Contains(line, "learner@example.com") || !strings.Contains(line, redactedValue) {
		t.Fatalf("unexpected log: %s", line)
	}
}

func TestRedactingHandler_RedactsGroupedAttributes(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil), []string{"request_id"}))
	logger.Info("request completed", slog.Group("request", "request_id", "req-1", "email", "learner@example.com"))
	line := output.String()
	if strings.Contains(line, "learner@example.com") || !strings.Contains(line, redactedValue) {
		t.Fatalf("unexpected log: %s", line)
	}
}

func TestRedactingHandler_AddsRequestIDFromContext(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil), []string{"request_id"}))
	logger.InfoContext(httpx.WithRequestID(context.Background(), "req-1"), "request completed")
	if !strings.Contains(output.String(), "req-1") {
		t.Fatalf("request ID missing from log: %s", output.String())
	}
}
