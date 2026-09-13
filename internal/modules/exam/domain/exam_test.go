package domain

import (
	"testing"
	"time"
)

func TestClampPracticeDuration(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{5, 10},
		{10, 10},
		{60, 60},
		{180, 180},
		{200, 180},
	}
	for _, tt := range tests {
		if got := ClampPracticeDuration(tt.input); got != tt.want {
			t.Errorf("ClampPracticeDuration(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestIsPastDeadline(t *testing.T) {
	deadline := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

	// Within deadline
	if IsPastDeadline(deadline, deadline.Add(-1*time.Second)) {
		t.Error("expected false 1s before deadline")
	}

	// Exactly at deadline
	if IsPastDeadline(deadline, deadline) {
		t.Error("expected false at deadline")
	}

	// 4s past deadline (within 5s grace period)
	if IsPastDeadline(deadline, deadline.Add(4*time.Second)) {
		t.Error("expected false within 5s grace period")
	}

	// 6s past deadline (exceeds grace period)
	if !IsPastDeadline(deadline, deadline.Add(6*time.Second)) {
		t.Error("expected true 6s past deadline")
	}
}

func TestRemainingSeconds(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	deadline := now.Add(45 * time.Second)

	if got := RemainingSeconds(deadline, now); got != 45 {
		t.Errorf("RemainingSeconds = %d, want 45", got)
	}

	// Past deadline returns 0
	if got := RemainingSeconds(deadline, now.Add(50*time.Second)); got != 0 {
		t.Errorf("RemainingSeconds past deadline = %d, want 0", got)
	}
}

func TestSectionDeadline(t *testing.T) {
	start := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

	s1 := SectionDeadline(start, 1)
	if s1.Sub(start) != 20*time.Minute {
		t.Errorf("section 1 = %v, want +20m", s1.Sub(start))
	}

	s2 := SectionDeadline(start, 2)
	if s2.Sub(start) != 45*time.Minute {
		t.Errorf("section 2 = %v, want +45m", s2.Sub(start))
	}

	s3 := SectionDeadline(start, 3)
	if s3.Sub(start) != 65*time.Minute {
		t.Errorf("section 3 = %v, want +65m", s3.Sub(start))
	}

	s4 := SectionDeadline(start, 4)
	if s4.Sub(start) != 75*time.Minute {
		t.Errorf("section 4 = %v, want +75m", s4.Sub(start))
	}
}

func TestEstimateCEFRBand(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{95.0, "C1"},
		{90.0, "C1"},
		{85.0, "B2"},
		{75.0, "B2"},
		{70.0, "B1"},
		{60.0, "B1"},
		{50.0, "A2"},
		{40.0, "A2"},
		{35.0, "A1"},
		{0.0, "A1"},
	}
	for _, tt := range tests {
		if got := EstimateCEFRBand(tt.score); got != tt.want {
			t.Errorf("EstimateCEFRBand(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}
