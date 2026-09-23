package service

import "testing"

// TestSampleSize pins the daily sample arithmetic: 2 %, at least five, never
// more than the day's total (WO 22 Stage A.5).
func TestSampleSize(t *testing.T) {
	cases := []struct {
		total int64
		want  int
	}{
		{0, 0},
		{1, 1},
		{4, 4},
		{5, 5},
		{100, 5},   // 2 % is 2, the floor lifts it to 5
		{250, 5},   // 2 % is exactly 5
		{1000, 20}, // 2 %
		{10000, 200},
	}
	for _, tc := range cases {
		if got := sampleSize(tc.total); got != tc.want {
			t.Errorf("sampleSize(%d) = %d, want %d", tc.total, got, tc.want)
		}
	}
}
