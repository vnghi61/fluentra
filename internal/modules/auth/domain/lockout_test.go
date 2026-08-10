package domain

import (
	"testing"
	"time"
)

func TestLockoutDuration_ExponentiallyBacksOffAndCaps(t *testing.T) {
	tests := []struct {
		previousLockouts int
		want             time.Duration
	}{
		{previousLockouts: 0, want: 15 * time.Minute},
		{previousLockouts: 1, want: 30 * time.Minute},
		{previousLockouts: 2, want: time.Hour},
		{previousLockouts: 10, want: 24 * time.Hour},
	}

	for _, test := range tests {
		t.Run(test.want.String(), func(t *testing.T) {
			if got := LockoutDuration(test.previousLockouts); got != test.want {
				t.Fatalf("LockoutDuration(%d) = %s, want %s", test.previousLockouts, got, test.want)
			}
		})
	}
}
