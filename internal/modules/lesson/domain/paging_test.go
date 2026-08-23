package domain_test

import (
	"math"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
)

func TestNormaliseLimit(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int32
	}{
		{"zero defaults", 0, domain.DefaultLimit},
		{"negative defaults", -5, domain.DefaultLimit},
		{"within range", 15, 15},
		{"upper boundary", domain.MaxLimit, domain.MaxLimit},
		{"exceeds max limit", 150, domain.MaxLimit},
		{"large value does not overflow", math.MaxInt64, domain.MaxLimit},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.NormaliseLimit(tc.input)
			if got != tc.expected {
				t.Errorf("NormaliseLimit(%d) = %d; want %d", tc.input, got, tc.expected)
			}
		})
	}
}

func TestNormaliseOffset(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int32
	}{
		{"zero", 0, 0},
		{"negative clamps to zero", -10, 0},
		{"positive offset", 50, 50},
		{"exceeds max int32 clamps to max int32", math.MaxInt64, math.MaxInt32},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.NormaliseOffset(tc.input)
			if got != tc.expected {
				t.Errorf("NormaliseOffset(%d) = %d; want %d", tc.input, got, tc.expected)
			}
		})
	}
}

func TestValidationHelpers(t *testing.T) {
	t.Run("slug validation", func(t *testing.T) {
		validSlugs := []string{"ielts-foundation", "course-123", "a-b-c", "english"}
		for _, s := range validSlugs {
			if !domain.IsValidSlug(s) {
				t.Errorf("expected slug %q to be valid", s)
			}
		}

		invalidSlugs := []string{"", "IELTS-foundation", "course_123", "-leading", "trailing-", "double--dash"}
		for _, s := range invalidSlugs {
			if domain.IsValidSlug(s) {
				t.Errorf("expected slug %q to be invalid", s)
			}
		}
	})

	t.Run("cefr validation", func(t *testing.T) {
		validLevels := []string{"A1", "A2", "B1", "B2", "C1", "C2"}
		for _, l := range validLevels {
			if !domain.IsValidCEFRLevel(l) {
				t.Errorf("expected CEFR level %q to be valid", l)
			}
		}

		invalidLevels := []string{"", "a1", "b2", "C3", "X1", "beginner"}
		for _, l := range invalidLevels {
			if domain.IsValidCEFRLevel(l) {
				t.Errorf("expected CEFR level %q to be invalid", l)
			}
		}
	})
}
