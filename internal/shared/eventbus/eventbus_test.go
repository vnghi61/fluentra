package eventbus_test

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/google/uuid"
)

func message(topic string) eventbus.Message {
	return eventbus.Message{ID: uuid.New(), Topic: topic, Payload: []byte(`{"user_id":"123"}`)}
}

func TestEventBus_DeliversToSubscriber(t *testing.T) {
	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())

	var received eventbus.Message
	var count int32
	if err := bus.Subscribe("user.registered", func(_ context.Context, m eventbus.Message) error {
		atomic.AddInt32(&count, 1)
		received = m
		return nil
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	sent := message("user.registered")
	if err := bus.Publish(context.Background(), sent); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("handler ran %d times, want 1", count)
	}
	if received.ID != sent.ID || string(received.Payload) != string(sent.Payload) {
		t.Errorf("handler received %#v, want %#v", received, sent)
	}
}

func TestEventBus_PublishWithoutSubscribersIsNotAnError(t *testing.T) {
	bus := eventbus.NewInProcessBus(nil)
	if err := bus.Publish(context.Background(), message("nobody.listening")); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

func TestEventBus_SlowHandlerDoesNotBlockOtherHandlers(t *testing.T) {
	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())

	var fast, slow int32
	_ = bus.Subscribe("test.event", func(context.Context, eventbus.Message) error {
		time.Sleep(100 * time.Millisecond)
		atomic.StoreInt32(&slow, 1)
		return nil
	})
	_ = bus.Subscribe("test.event", func(context.Context, eventbus.Message) error {
		atomic.StoreInt32(&fast, 1)
		return nil
	})

	start := time.Now()
	if err := bus.Publish(context.Background(), message("test.event")); err != nil {
		t.Fatalf("publish: %v", err)
	}
	elapsed := time.Since(start)

	if atomic.LoadInt32(&fast) != 1 || atomic.LoadInt32(&slow) != 1 {
		t.Error("expected both handlers to run")
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("publish took %v; handlers may be running sequentially", elapsed)
	}
}

// TestEventBus_OneFailingHandlerStillRunsTheOthers matters because the outbox
// retries the whole message: a handler that is skipped on the first delivery
// and run on the retry would see a different world.
func TestEventBus_OneFailingHandlerStillRunsTheOthers(t *testing.T) {
	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())

	failure := errors.New("database connection failed")
	var otherRan int32
	_ = bus.Subscribe("order.created", func(context.Context, eventbus.Message) error { return failure })
	_ = bus.Subscribe("order.created", func(context.Context, eventbus.Message) error {
		atomic.StoreInt32(&otherRan, 1)
		return nil
	})

	err := bus.Publish(context.Background(), message("order.created"))
	if err == nil {
		t.Fatal("expected the handler failure to surface so the outbox retries")
	}
	if !errors.Is(err, failure) {
		t.Errorf("error = %v, want it to wrap %v", err, failure)
	}
	if atomic.LoadInt32(&otherRan) != 1 {
		t.Error("the healthy handler was skipped because a sibling failed")
	}
}

// TestEventBus_PanickingHandlerIsContainedAndReported keeps one bad consumer
// from taking down the outbox publisher goroutine.
func TestEventBus_PanickingHandlerIsContainedAndReported(t *testing.T) {
	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())

	var healthyRan int32
	_ = bus.Subscribe("boom", func(context.Context, eventbus.Message) error { panic("kaboom") })
	_ = bus.Subscribe("boom", func(context.Context, eventbus.Message) error {
		atomic.StoreInt32(&healthyRan, 1)
		return nil
	})

	err := bus.Publish(context.Background(), message("boom"))
	if err == nil {
		t.Fatal("a panicking handler must be reported as a failure, not swallowed")
	}
	if atomic.LoadInt32(&healthyRan) != 1 {
		t.Error("the healthy handler did not run")
	}
}

func TestRegistry_RejectsInvalidRegistrations(t *testing.T) {
	registry := eventbus.NewRegistry()
	if err := registry.Register("", func(context.Context, eventbus.Message) error { return nil }); err == nil {
		t.Error("expected an error for an empty topic")
	}
	if err := registry.Register("topic", nil); err == nil {
		t.Error("expected an error for a nil handler")
	}
}

// TestEventBus_InterfaceHasNoInProcessSpecificMethod is the P0.12 acceptance:
// a broker implementation must satisfy EventBus unchanged.
func TestEventBus_InterfaceHasNoInProcessSpecificMethod(t *testing.T) {
	busType := reflect.TypeOf((*eventbus.EventBus)(nil)).Elem()
	if busType.NumMethod() != 2 {
		t.Fatalf("EventBus has %d methods; it should be Publish and Subscribe only", busType.NumMethod())
	}
	for _, name := range []string{"Publish", "Subscribe"} {
		if _, ok := busType.MethodByName(name); !ok {
			t.Errorf("EventBus is missing %s", name)
		}
	}
}
