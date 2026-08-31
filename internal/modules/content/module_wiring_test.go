package content_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/fluentra/fluentra/internal/modules/content"
)

// allowAllWiringGuard permits everything; these tests are about what New wires
// together, not about authorization, which handler tests cover.
type allowAllWiringGuard struct{}

func (allowAllWiringGuard) Require(_ context.Context, _ string) error { return nil }

// newWiredModule builds the module the way cmd/api does. A nil pool is safe
// because nothing here executes a query — construction must not touch the
// database, and a module that did would fail this test by panicking.
func newWiredModule(t *testing.T) *content.Module {
	t.Helper()
	return content.New(content.Deps{Guard: allowAllWiringGuard{}})
}

func TestNewExposesTheReadContract(t *testing.T) {
	t.Parallel()

	mod := newWiredModule(t)
	if mod.Reader() == nil {
		t.Fatal("Reader() is nil; lesson and learning resolve content through it")
	}
	if mod.Service() == nil {
		t.Fatal("Service() is nil")
	}
}

// TestNewFailsClosedWithoutAGuard is the composition-root half of the
// fail-closed rule: a module assembled without an authorizer must not start,
// because its admin authoring routes would be open.
func TestNewFailsClosedWithoutAGuard(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("content.New accepted a nil guard; admin authoring would be unprotected")
		}
	}()

	content.New(content.Deps{})
}

// TestRoutesMountTheDocumentedPaths pins the split between what a signed-in
// learner reaches and what only an administrator reaches. The learner router
// must never carry an authoring path.
func TestRoutesMountTheDocumentedPaths(t *testing.T) {
	t.Parallel()

	mod := newWiredModule(t)

	learner := chi.NewRouter()
	mod.Routes(learner)

	admin := chi.NewRouter()
	mod.AdminRoutes(admin)

	wantLearner := map[string]bool{
		"GET /content":        false,
		"GET /content/{slug}": false,
	}
	wantAdmin := map[string]bool{
		"POST /admin/content":              false,
		"PUT /admin/content/{id}/draft":    false,
		"POST /admin/content/{id}/submit":  false,
		"POST /admin/content/{id}/review":  false,
		"POST /admin/content/{id}/publish": false,
		"POST /admin/content/{id}/archive": false,
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

// TestNewAuthoringWithoutAGuard is the other half of the fail-closed rule.
//
// cmd/worker generates practice content and mounts no routes, so it has no
// guard to give. It called New anyway, which builds the handler, which fails
// closed — and the worker panicked on boot with GUARD_REQUIRED, three call
// frames from anything that mentioned HTTP.
//
// The consequence was not local to the worker. A worker that will not start
// drains no outbox, so no OTP email is ever sent: every E2E journey timed out
// waiting for one, and in production registration would stop working while the
// API looked perfectly healthy.
//
// This asserts the escape hatch exists and stays narrow — a module with no
// routes and no handler. TestNewFailsClosedWithoutAGuard above still holds for
// anything that does serve HTTP, and the two together are the rule: no guard,
// no routes.
func TestNewAuthoringWithoutAGuard(t *testing.T) {
	t.Parallel()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("NewAuthoring panicked without a guard, which is how cmd/worker calls it: %v", recovered)
		}
	}()

	mod := content.NewAuthoring(content.Deps{})
	if mod == nil {
		t.Fatal("NewAuthoring returned nil")
	}
	if mod.Author() == nil {
		t.Error("Author() is what the worker came for, and it is nil")
	}
}
