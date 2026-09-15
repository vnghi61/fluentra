package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// refusingModel is a chain whose every provider is out of quota.
type refusingModel struct{ calls int }

func (m *refusingModel) Complete(context.Context, ai.Request) (ai.Response, error) {
	m.calls++
	return ai.Response{}, fmt.Errorf("%w: all providers failed (429)", ai.ErrProvidersUnavailable)
}

// When every provider refuses, the run stops at the first item. It used to try
// every slot at every level, three times each, and one day of that was a
// thousand requests into refusals.
func TestTopUpExamPool_StopsWhenEveryProviderIsRefusing(t *testing.T) {
	model := &refusingModel{}
	f := newExamPoolFixture(t, clock.NewFake(time.Now()), uuid.New(), examPassingGraders(), model)

	require.NoError(t, f.svc.TopUpExamPool(context.Background()))

	// One item's attempts, not 3 levels × 6 slots × 5 items × 3 attempts.
	assert.LessOrEqual(t, model.calls, 3)
	assert.Positive(t, model.calls)
}
