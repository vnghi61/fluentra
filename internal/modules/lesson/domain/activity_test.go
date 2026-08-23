package domain_test

import (
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
)

func TestIsValidPosition(t *testing.T) {
	t.Parallel()

	cases := map[int]bool{
		1:                                 true,
		10:                                true,
		domain.MaxActivitiesPerLesson:     true,
		domain.MaxActivitiesPerLesson + 1: false,
		0:                                 false,
		-1:                                false,
	}
	for pos, want := range cases {
		if got := domain.IsValidPosition(pos); got != want {
			t.Errorf("IsValidPosition(%d) = %v, want %v", pos, got, want)
		}
	}
}

func TestIsValidActivityKind(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"quiz":     true,
		"gap_fill": true,
		"":         false,
		strings.Repeat("k", domain.MaxActivityKindLength):   true,
		strings.Repeat("k", domain.MaxActivityKindLength+1): false,
	}
	for kind, want := range cases {
		if got := domain.IsValidActivityKind(kind); got != want {
			t.Errorf("IsValidActivityKind(%d chars) = %v, want %v", len(kind), got, want)
		}
	}
}

func TestIsValidWeight(t *testing.T) {
	t.Parallel()

	cases := map[int]bool{
		0:                            true,
		10:                           true,
		domain.MaxActivityWeight:     true,
		domain.MaxActivityWeight + 1: false,
		-1:                           false,
		// The value that made the int32 in CalculateLessonDuration a lie.
		1 << 30: false,
	}
	for weight, want := range cases {
		if got := domain.IsValidWeight(weight); got != want {
			t.Errorf("IsValidWeight(%d) = %v, want %v", weight, got, want)
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

// The service rejects an out-of-range weight before this runs, but the int32
// return is only safe if the domain refuses to multiply an unbounded int by
// two. Without the clamp, 1<<30 wraps to a negative duration.
func TestCalculateLessonDuration_ClampsAnAbsurdWeight(t *testing.T) {
	t.Parallel()

	got := domain.CalculateLessonDuration([]domain.ActivityInput{{Weight: 1 << 30}})
	if want := int32(domain.MaxActivityWeight * 2); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

// The worst case the service can hand over must still fit the column.
func TestCalculateLessonDuration_MaximumListFitsInt32(t *testing.T) {
	t.Parallel()

	acts := make([]domain.ActivityInput, domain.MaxActivitiesPerLesson)
	for i := range acts {
		acts[i] = domain.ActivityInput{Weight: domain.MaxActivityWeight}
	}

	want := int32(domain.MaxActivitiesPerLesson * domain.MaxActivityWeight * 2)
	if got := domain.CalculateLessonDuration(acts); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}
