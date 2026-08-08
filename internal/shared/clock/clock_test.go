package clock

import (
	"testing"
	"time"
)

func TestFake_SetAndAdvance(t *testing.T) {
	t.Parallel()
	initial := time.Date(2026, 8, 7, 10, 0, 0, 0, time.FixedZone("UTC+7", 7*60*60))
	fake := NewFake(initial)
	if got := fake.Now(); got.Location() != time.UTC || !got.Equal(initial) {
		t.Fatalf("initial = %v", got)
	}
	fake.Advance(time.Minute)
	fake.Set(initial.Add(2 * time.Hour))
	if got := fake.Now(); !got.Equal(initial.Add(2 * time.Hour)) {
		t.Fatalf("now = %v", got)
	}
}

func TestReal_NowIsUTC(t *testing.T) {
	t.Parallel()
	if got := (Real{}).Now(); got.Location() != time.UTC {
		t.Fatalf("location = %s", got.Location())
	}
}
