package storage_test

import (
	"errors"
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/platform/storage"
)

var buildDate = time.Date(2026, time.August, 7, 10, 0, 0, 0, time.UTC)

func TestBuildKey(t *testing.T) {
	t.Parallel()
	key, err := storage.BuildKey("user", "user_123", buildDate, "asset_999", "png")
	if err != nil {
		t.Fatalf("build key: %v", err)
	}
	if want := "user/user_123/2026/08/asset_999.png"; key != want {
		t.Errorf("got %q, want %q", key, want)
	}
}

func TestBuildKey_DefaultExtension(t *testing.T) {
	t.Parallel()
	key, err := storage.BuildKey("user", "user_123", buildDate, "asset_999", "")
	if err != nil {
		t.Fatalf("build key: %v", err)
	}
	if want := "user/user_123/2026/08/asset_999.bin"; key != want {
		t.Errorf("got %q, want %q", key, want)
	}
}

// TestBuildKey_IsDeterministic is the P0.9 acceptance: same inputs, same key.
// The time zone of the input must not change the result either.
func TestBuildKey_IsDeterministic(t *testing.T) {
	t.Parallel()
	first, err := storage.BuildKey("user", "u1", buildDate, "a1", "webp")
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := buildDate.In(time.FixedZone("UTC+7", 7*60*60))
	second, err := storage.BuildKey("user", "u1", elsewhere, "a1", "webp")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("key changed with the input time zone: %q vs %q", first, second)
	}
}

// TestBuildKey_RejectsSegmentsThatEscapeThePrefix is the security property:
// an owner id is attacker-influenced, and `..` or a slash in it would place the
// object under someone else's prefix.
func TestBuildKey_RejectsSegmentsThatEscapeThePrefix(t *testing.T) {
	t.Parallel()
	for name, segment := range map[string]string{
		"parent traversal":  "..",
		"nested traversal":  "../../etc",
		"embedded slash":    "user/../admin",
		"backslash":         `user\..\admin`,
		"absolute":          "/etc/passwd",
		"empty":             "",
		"null byte":         "user\x00",
		"leading dot alone": ".",
	} {
		if _, err := storage.BuildKey("user", segment, buildDate, "a1", "png"); !errors.Is(err, storage.ErrUnsafeKeySegment) {
			t.Errorf("%s: BuildKey(owner=%q) error = %v, want ErrUnsafeKeySegment", name, segment, err)
		}
	}
}

func TestBuildKey_RejectsUnsafeExtension(t *testing.T) {
	t.Parallel()
	if _, err := storage.BuildKey("user", "u1", buildDate, "a1", "png/../../x"); err == nil {
		t.Error("expected an unsafe extension to be rejected")
	}
}
