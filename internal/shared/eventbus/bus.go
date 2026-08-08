package eventbus

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// Message is one published event. It carries what a broker would carry, so a
// remote implementation delivers the same shape as the in-process one.
type Message struct {
	// ID is the deduplication key. Delivery is at-least-once in every
	// implementation, so a handler must be idempotent on it.
	ID      uuid.UUID
	Topic   string
	Payload json.RawMessage
	// Attempt starts at 0 for a first delivery and increases on redelivery.
	Attempt int
}

// Handler consumes one message. Returning nil acknowledges it; returning an
// error negatively acknowledges it and asks the publisher to redeliver. That
// is the ack contract — there is no separate Ack call to forget.
type Handler func(ctx context.Context, message Message) error

// EventBus is a broker-agnostic publish/subscribe interface. Nothing in it is
// specific to in-process delivery: a NATS or RabbitMQ implementation satisfies
// it unchanged.
type EventBus interface {
	Publish(ctx context.Context, message Message) error
	Subscribe(topic string, handler Handler) error
}
