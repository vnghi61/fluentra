package lesson_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/fluentra/fluentra/internal/modules/lesson"
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
