package lesson_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/fluentra/fluentra/internal/modules/lesson"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
)

type allowAllWiringGuard struct{}

func (allowAllWiringGuard) Require(_ context.Context, _ string) error { return nil }

func newWiredModule(t *testing.T) *lesson.Module {
	t.Helper()
	return lesson.New(lesson.Deps{Guard: allowAllWiringGuard{}})
}

func TestNewExposesTheReadContract(t *testing.T) {
	t.Parallel()

	mod := newWiredModule(t)
	if mod.Reader() == nil {
		t.Fatal("Reader() is nil; learning resolves lessons through it")
	}
	if mod.Service() == nil {
		t.Fatal("Service() is nil")
	}
}

func TestNewFailsClosedWithoutAGuard(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("lesson.New accepted a nil guard; admin authoring would be unprotected")
		}
	}()

	lesson.New(lesson.Deps{})
}

func TestRoutesMountTheDocumentedPaths(t *testing.T) {
	t.Parallel()

	mod := newWiredModule(t)

	learner := chi.NewRouter()
	mod.Routes(learner)

	admin := chi.NewRouter()
	mod.AdminRoutes(admin)

	wantLearner := map[string]bool{
		"GET /courses":        false,
		"GET /courses/{slug}": false,
		"GET /lessons/{id}":   false,
	}
	wantAdmin := map[string]bool{
		"POST /admin/courses":                false,
		"PUT /admin/lessons/{id}/activities": false,
		"POST /admin/lessons/{id}/publish":   false,
	}

	collect := func(router chi.Router, into map[string]bool, label string) {
		walkErr := chi.Walk(router, func(
			method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler,
		) error {
			key := method + " " + route
			if _, expected := into[key]; !expected {
				t.Errorf("%s router mounts an unexpected route: %s", label, key)
				return nil
			}
			into[key] = true
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s router: %v", label, walkErr)
		}
		for key, found := range into {
			if !found {
				t.Errorf("%s router is missing %s", label, key)
			}
		}
	}

	collect(learner, wantLearner, "learner")
	collect(admin, wantAdmin, "admin")
}

type fakeBus struct {
	subscribed map[string]eventbus.Handler
}

func (f *fakeBus) Subscribe(topic string, handler eventbus.Handler) error {
	if f.subscribed == nil {
		f.subscribed = make(map[string]eventbus.Handler)
	}
	f.subscribed[topic] = handler
	return nil
}

func (f *fakeBus) Publish(_ context.Context, _ eventbus.Message) error {
	return nil
}

func TestModule_Subscribe(t *testing.T) {
	t.Parallel()

	mod := newWiredModule(t)
	bus := &fakeBus{}

	if err := mod.Subscribe(bus); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	for _, topic := range []string{"content.archived", "lesson.published"} {
		handler, ok := bus.subscribed[topic]
		if !ok || handler == nil {
			t.Fatalf("expected subscription for %s", topic)
		}

		// A payload the module cannot decode must nack, not be swallowed:
		// the bus contract redelivers on error and acknowledges on nil.
		msg := eventbus.Message{Topic: topic, Payload: []byte("not-json")}
		if err := handler(context.Background(), msg); err == nil {
			t.Errorf("%s handler accepted an undecodable payload", topic)
		}

		// A well-formed payload with no id is nothing to act on, and must be
		// acknowledged rather than redelivered forever.
		if err := handler(context.Background(), eventbus.Message{
			Topic: topic, Payload: []byte("{}"),
		}); err != nil {
			t.Errorf("%s handler rejected an empty payload: %v", topic, err)
		}
	}
}
