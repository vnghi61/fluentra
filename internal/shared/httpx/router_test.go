package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type pingRow struct {
	value int
	err   error
}

func (r pingRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int) = r.value
	return nil
}

type pingDatabaseStub struct {
	query string
	row   pgx.Row
}

func (d *pingDatabaseStub) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	d.query = query
	return d.row
}

type pingCacheStub struct{ err error }

func (c pingCacheStub) Ping(context.Context) *redis.StatusCmd {
	return redis.NewStatusResult("PONG", c.err)
}

func TestRouterPing(t *testing.T) {
	database := &pingDatabaseStub{row: pingRow{value: 1}}
	router := NewRouter(RouterDependencies{
		Database: database,
		Cache:    pingCacheStub{},
		Middleware: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				next.ServeHTTP(writer, request.WithContext(WithRequestID(request.Context(), "request-123")))
			})
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if database.query != "SELECT 1" {
		t.Fatalf("database query = %q, want SELECT 1", database.query)
	}
	if got := response.Header().Get("X-Request-Id"); got != "request-123" {
		t.Fatalf("X-Request-Id = %q, want request-123", got)
	}
}

func TestRouterPingHidesInfrastructureErrors(t *testing.T) {
	router := NewRouter(RouterDependencies{
		Database: &pingDatabaseStub{row: pingRow{err: errors.New("postgres://user:secret@db is unavailable")}},
		Cache:    pingCacheStub{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("response leaked infrastructure detail: %s", response.Body.String())
	}
}

func TestRouterRegistersHealthEndpoints(t *testing.T) {
	router := NewRouter(RouterDependencies{Health: func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}
