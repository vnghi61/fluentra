package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type countingRenderRequester struct{ calls int }

func (c *countingRenderRequester) RequestRender(context.Context) error {
	c.calls++
	return nil
}

func examPoolWithRenderRequester(t *testing.T, model ai.Client, render service.AudioRenderRequester) *service.Service {
	t.Helper()
	lessons := newFakePoolLessons()
	svc := service.New(service.Deps{
		Repo:              newFakePoolRepo(),
		Lesson:            lessons,
		LessonAuthor:      lessons,
		Content:           newFakeContentReader(),
		ContentAuthor:     &fakeContentAuthor{},
		Graders:           examPassingGraders(),
		AI:                model,
		Clock:             clock.NewFake(time.Now()),
		GeneratorAuthorID: uuid.New(),
		Synthesiser:       &fakeAudioSynthesiser{},
		AudioRender:       render,
	})
	require.NoError(t, svc.EnsureExamPoolStructure(context.Background()))
	return svc
}

// Listening items are unusable until their audio is rendered, and rendering runs
// in a GitHub workflow. The top-up asks for it as soon as a listening slot gains
// items, once per level, rather than leaving the items until the next hour.
func TestTopUpExamPool_AsksForAudioWhenListeningItemsArePublished(t *testing.T) {
	registry, err := ai.NewRegistry()
	require.NoError(t, err)
	render := &countingRenderRequester{}
	svc := examPoolWithRenderRequester(t, ai.NewMockProvider(registry), render)

	require.NoError(t, svc.TopUpExamPool(context.Background()))

	assert.Equal(t, 3, render.calls, "one request per level's listening slot")
}

func TestTopUpExamPool_AsksForNoAudioWhenNoListeningItemIsAdded(t *testing.T) {
	render := &countingRenderRequester{}
	svc := examPoolWithRenderRequester(t, &refusingModel{}, render)

	require.NoError(t, svc.TopUpExamPool(context.Background()))

	assert.Zero(t, render.calls)
}
