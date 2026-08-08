package eventbus

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"context"
)

// InProcessBus implements EventBus by calling handlers directly.
type InProcessBus struct {
	registry *Registry
	tracer   trace.Tracer
}

// NewInProcessBus initializes an in-process event bus.
func NewInProcessBus(registry *Registry) *InProcessBus {
	if registry == nil {
		registry = NewRegistry()
	}
	return &InProcessBus{
		registry: registry,
		tracer:   otel.Tracer("fluentra.shared.eventbus"),
	}
}

// Subscribe registers an event handler for a topic.
func (b *InProcessBus) Subscribe(topic string, handler Handler) error {
	return b.registry.Register(topic, handler)
}

// Publish delivers a message to every handler for its topic. Handlers run
// concurrently so one slow handler does not hold up the others, and Publish
// waits for all of them: the caller is the outbox publisher, and it may only
// mark the event published once every handler has accepted it.
//
// A handler that fails does not stop the others, and its error is returned so
// the outbox retries the whole message. Handlers must therefore be idempotent
// on Message.ID.
func (b *InProcessBus) Publish(ctx context.Context, message Message) error {
	handlers := b.registry.GetHandlers(message.Topic)
	if len(handlers) == 0 {
		return nil
	}

	var wait sync.WaitGroup
	failures := make([]error, len(handlers))

	for index, handler := range handlers {
		wait.Add(1)
		go func(index int, handler Handler) {
			defer wait.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					failures[index] = fmt.Errorf("handler %d on %q panicked: %v", index, message.Topic, recovered)
				}
			}()

			handlerCtx, span := b.tracer.Start(ctx, "eventbus.handle "+message.Topic,
				trace.WithAttributes(
					attribute.String("event.topic", message.Topic),
					attribute.String("event_id", message.ID.String()),
					attribute.Int("handler.index", index),
				),
			)
			defer span.End()

			if err := handler(handlerCtx, message); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "handler failed")
				slog.ErrorContext(handlerCtx, "event handler failed",
					"topic", message.Topic, "event_id", message.ID.String(), "error", err)
				failures[index] = fmt.Errorf("handler %d on %q: %w", index, message.Topic, err)
			}
		}(index, handler)
	}

	wait.Wait()
	return errors.Join(failures...)
}

var _ EventBus = (*InProcessBus)(nil)
