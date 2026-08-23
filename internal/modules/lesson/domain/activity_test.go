package domain_test

import (
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
)

func TestIsValidPosition(t *testing.T) {
	t.Parallel()

	cases := map[int]bool{1: true, 10: true, 0: false, -1: false}
	for pos, want := range cases {
		if got := domain.IsValidPosition(pos); got != want {
			t.Errorf("IsValidPosition(%d) = %v, want %v", pos, got, want)
		}
	}
}

func TestIsValidActivityKind(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"quiz":                  true,
		"gap_fill":              true,
		"":                      false,
		strings.Repeat("k", 50): true,
		strings.Repeat("k", 51): false,
	}
	for kind, want := range cases {
		if got := domain.IsValidActivityKind(kind); got != want {
			t.Errorf("IsValidActivityKind(%d chars) = %v, want %v", len(kind), got, want)
		}
	}
}

func TestCalculateLessonDuration(t *testing.T) {
	t.Parallel()

	if got := domain.CalculateLessonDuration(nil); got != 0 {
		t.Errorf("got %d for empty activities, want 0", got)
	}

	acts := []domain.ActivityInput{
		{Weight: 2},
		{Weight: 3},
		{Weight: 0}, // default weight 1 -> 2 mins
	}
	// (2*2) + (3*2) + (1*2) = 4 + 6 + 2 = 12
	if got := domain.CalculateLessonDuration(acts); got != 12 {
		t.Errorf("got duration %d, want 12", got)
	}
}
