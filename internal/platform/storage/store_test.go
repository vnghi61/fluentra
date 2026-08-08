package storage_test

import (
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/platform/storage"
)

func TestBuildKey(t *testing.T) {
	date := time.Date(2026, time.August, 7, 10, 0, 0, 0, time.UTC)
	key := storage.BuildKey("user", "user_123", date, "asset_999", "png")

	expected := "user/user_123/2026/08/asset_999.png"
	if key != expected {
		t.Errorf("got %q, want %q", key, expected)
	}
}

func TestBuildKey_DefaultExtension(t *testing.T) {
	date := time.Date(2026, time.August, 7, 10, 0, 0, 0, time.UTC)
	key := storage.BuildKey("user", "user_123", date, "asset_999", "")

	expected := "user/user_123/2026/08/asset_999.bin"
	if key != expected {
		t.Errorf("got %q, want %q", key, expected)
	}
}
