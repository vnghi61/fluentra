package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// A development stack runs the mock provider. Its generator returned the same
// item on every call, the duplicate check kept one per slot, and a sitting needs
// two or three of each: the exam hub offered no exam on any developer's machine.
func TestTopUpExamPool_TheMockProviderFillsASittingAtEveryLevel(t *testing.T) {
	registry, err := ai.NewRegistry()
	require.NoError(t, err)

	f := newExamPoolFixture(t, clock.NewFake(time.Now()), uuid.New(), examPassingGraders(), ai.NewMockProvider(registry))
	require.NoError(t, f.svc.TopUpExamPool(context.Background()))

	for _, level := range []string{"A2", "B1", "B2"} {
		sections, err := f.svc.DrawExamSitting(context.Background(), uuid.New(), level)
		require.NoError(t, err, "a %s sitting could not be drawn from a mock-filled pool", level)
		assert.NotEmpty(t, sections, level)
	}
}
