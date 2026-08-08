package telemetry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

func TestRedactingHandler_FailsClosed(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil), []string{requestIDKey}))
	logger.Info("request completed", requestIDKey, "req-1", "email", "learner@example.com")
	line := output.String()
	if !strings.Contains(line, "req-1") ||
		strings.Contains(line, "learner@example.com") ||
		!strings.Contains(line, redactedValue) {
		t.Fatalf("unexpected log: %s", line)
	}
}

func TestRedactingHandler_RedactsGroupedAttributes(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil), []string{requestIDKey}))
	logger.Info("request completed",
		slog.Group("request", requestIDKey, "req-1", "email", "learner@example.com"))
	line := output.String()
	if strings.Contains(line, "learner@example.com") || !strings.Contains(line, redactedValue) {
		t.Fatalf("unexpected log: %s", line)
	}
}

// TestRedactingHandler_KeepsApplicationErrorsActionable is the regression test
// for logs rendering every error as [redacted], which made them useless.
func TestRedactingHandler_KeepsApplicationErrorsActionable(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil), DefaultAllowedLogKeys))

	appErr := apperr.New(apperr.Unavailable, "CACHE_UNAVAILABLE", "The cache is unavailable.").
		WithCause(errors.New("dial tcp 10.0.0.1:6379: connection refused"))
	logger.Error("cache degraded", "error", appErr, "module", "cache")

	line := output.String()
	if !strings.Contains(line, "CACHE_UNAVAILABLE") {
		t.Fatalf("error code missing from log: %s", line)
	}
	if strings.Contains(line, "10.0.0.1") {
		t.Fatalf("wrapped cause leaked into log: %s", line)
	}
}

// TestRedactingHandler_ReducesUnknownErrorsToTheirType proves an error carrying
// a DSN or an address never reaches the log verbatim.
func TestRedactingHandler_ReducesUnknownErrorsToTheirType(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil), DefaultAllowedLogKeys))

	logger.Error("pool failed", "error", errors.New("postgres://app:hunter2@db:5432/fluentra: refused"))

	line := output.String()
	if strings.Contains(line, "hunter2") || strings.Contains(line, "postgres://") {
		t.Fatalf("raw error string leaked into log: %s", line)
	}
	if !strings.Contains(line, "errorString") {
		t.Fatalf("error type missing from log: %s", line)
	}
}

func TestRedactingHandler_NamesContextErrors(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil), DefaultAllowedLogKeys))
	logger.Error("publisher stopped", "error", fmt.Errorf("run publisher: %w", context.Canceled))
	if !strings.Contains(output.String(), "context canceled") {
		t.Fatalf("context error not named: %s", output.String())
	}
}

// TestDefaultAllowedLogKeys_CoversGuidelineAttributes keeps the allowlist in
// step with the situation table in LOGGING_GUIDELINE.md §4.
func TestDefaultAllowedLogKeys_CoversGuidelineAttributes(t *testing.T) {
	t.Parallel()
	allowed := make(map[string]struct{}, len(DefaultAllowedLogKeys))
	for _, key := range DefaultAllowedLogKeys {
		allowed[key] = struct{}{}
	}
	for _, key := range []string{
		"user_id", "source", "email_hash", "ip_country", "reason", "family_id", "permission",
		"route", "limiter", "subject_hash", "content_id", "version", "actor_id", "submission_id",
		"task", "prompt_version", "provider", "duration_ms", "from_provider", "to_provider",
		"attempt", "module", "op", "kind", "max_attempts", "error_code", "job_id", "event_id",
		"config_digest",
	} {
		if _, ok := allowed[key]; !ok {
			t.Errorf("LOGGING_GUIDELINE.md requires attribute %q but it is not on the allowlist", key)
		}
	}
}

func TestRedactingHandler_AddsRequestIDFromContext(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil), []string{requestIDKey}))
	logger.InfoContext(httpx.WithRequestID(context.Background(), "req-1"), "request completed")
	if !strings.Contains(output.String(), "req-1") {
		t.Fatalf("request ID missing from log: %s", output.String())
	}
}
