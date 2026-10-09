package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestFanout_SendsEachRecordToEveryHandlerThatAcceptsIt(t *testing.T) {
	var info, warn bytes.Buffer
	logger := slog.New(fanout{
		slog.NewJSONHandler(&info, nil),
		slog.NewJSONHandler(&warn, &slog.HandlerOptions{Level: slog.LevelWarn}),
	}).With("component", "test")

	logger.InfoContext(context.Background(), "started")
	logger.WarnContext(context.Background(), "slow")

	if got := strings.Count(info.String(), "\n"); got != 2 {
		t.Errorf("info handler got %d records, want 2: %s", got, info.String())
	}
	if !strings.Contains(warn.String(), `"msg":"slow"`) || strings.Contains(warn.String(), "started") {
		t.Errorf("warn handler = %s, want only the warning", warn.String())
	}
	if !strings.Contains(warn.String(), `"component":"test"`) {
		t.Errorf("attributes were not passed to every handler: %s", warn.String())
	}
}
