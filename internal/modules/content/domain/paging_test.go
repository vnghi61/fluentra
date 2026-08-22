package domain_test

import (
	"math"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
)

func TestNormaliseLimit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		limit int
		want  int32
	}{
		{"not supplied", 0, domain.DefaultLimit},
		{"negative", -1, domain.DefaultLimit},
		{"one", 1, 1},
		{"under the ceiling", 50, 50},
		{"at the ceiling", domain.MaxLimit, domain.MaxLimit},
		{"over the ceiling", domain.MaxLimit + 1, domain.MaxLimit},
		// The reason the clamp happens before the conversion and not after:
		// int32(4294967297) is 1, so a range check on the converted value sees
		// a legal page size and serves it.
		{"wraps to 1 if converted first", math.MaxUint32 + 2, domain.MaxLimit},
		{"max int", math.MaxInt32 + 1, domain.MaxLimit},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := domain.NormaliseLimit(testCase.limit); got != testCase.want {
				t.Errorf("NormaliseLimit(%d) = %d, want %d", testCase.limit, got, testCase.want)
			}
		})
	}
}

func TestNormaliseOffset(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		offset int
		want   int32
	}{
		{"not supplied", 0, 0},
		{"negative", -5, 0},
		{"ordinary", 40, 40},
		{"at the ceiling", math.MaxInt32, math.MaxInt32},
		{"over the ceiling", math.MaxInt32 + 1, math.MaxInt32},
		{"wraps to a negative if converted first", math.MaxInt32 + 100, math.MaxInt32},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := domain.NormaliseOffset(testCase.offset); got != testCase.want {
				t.Errorf("NormaliseOffset(%d) = %d, want %d", testCase.offset, got, testCase.want)
			}
		})
	}
}
