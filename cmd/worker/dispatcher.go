package main

import (
	"context"

	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// busDispatcher delivers outbox events to the in-process event bus. It lives in
// the composition root so neither shared package has to know about the other.
type busDispatcher struct {
	bus eventbus.EventBus
}

// Dispatch forwards one outbox event. The event id becomes the message id, so
// a consumer can deduplicate a redelivery — the outbox is at-least-once.
func (d busDispatcher) Dispatch(ctx context.Context, event outbox.Event) error {
	return d.bus.Publish(ctx, eventbus.Message{
		ID:      event.ID,
		Topic:   event.Topic(),
		Payload: event.Payload,
		Attempt: event.Attempt,
	})
}

var _ outbox.EventDispatcher = busDispatcher{}
