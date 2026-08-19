package httpx

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
)

const defaultRequestTimeout = 30 * time.Second

// PingDatabase is the minimal database surface used by the infrastructure ping endpoint.
type PingDatabase interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// PingCache is the minimal Redis surface used by the infrastructure ping endpoint.
type PingCache interface {
	Ping(context.Context) *redis.StatusCmd
}

// RouterDependencies supplies the infrastructure dependencies owned by the composition root.
type RouterDependencies struct {
	Database       PingDatabase
	Cache          PingCache
	Middleware     func(http.Handler) http.Handler
	CORS           func(http.Handler) http.Handler
	Health         http.HandlerFunc
	Ready          http.HandlerFunc
	Version        http.HandlerFunc
	RequestTimeout time.Duration
	// ClientIP resolves the address used for per-IP lockout and rate limiting.
	// Leaving it nil trusts no forwarding header, which is the safe default.
	ClientIP *ClientIPResolver

	// Modules mounts the business modules' operations under `/api/v1`, after
	// the standard middleware chain and beside `/ping`.
	//
	// It is a callback rather than a list of handlers because only the
	// composition root knows what a module needs — which guard wraps it, which
	// other module it was built from. This package supplies the mount point and
	// the middleware, and stays ignorant of everything above it.
	//
	// Nil serves the infrastructure endpoints alone, which is what every test
	// of this package wants.
	Modules func(chi.Router)
}

// NewRouter builds the API router and applies the standard HTTP middleware chain.
func NewRouter(deps RouterDependencies) http.Handler {
	requestTimeout := deps.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = defaultRequestTimeout
	}

	resolver := deps.ClientIP
	if resolver == nil {
		// No trusted proxies: the socket address is the client, and forwarding
		// headers are ignored.
		resolver, _ = NewClientIPResolver(nil)
	}

	router := chi.NewRouter()
	router.Use(resolver.Middleware)
	if deps.CORS != nil {
		// Outermost of the request-scoped concerns, so a preflight is answered
		// before it would be charged a rate-limit budget or asked for a token.
		router.Use(deps.CORS)
	}
	router.Use(middleware.Timeout(requestTimeout))
	if deps.Middleware != nil {
		router.Use(deps.Middleware)
	}
	if deps.Health != nil {
		router.Get("/health", deps.Health)
	}
	if deps.Ready != nil {
		router.Get("/ready", deps.Ready)
	}
	if deps.Version != nil {
		router.Get("/version", deps.Version)
	}
	router.Route("/api/v1", func(router chi.Router) {
		router.Get("/ping", pingHandler(deps.Database, deps.Cache))
		if deps.Modules != nil {
			deps.Modules(router)
		}
	})

	return router
}

func pingHandler(database PingDatabase, cache PingCache) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if database == nil || cache == nil {
			WriteProblem(writer, request, fmt.Errorf("ping dependencies are not configured"))
			return
		}

		ctx, databaseSpan := otel.Tracer("fluentra.api").Start(request.Context(), "pgx.query")
		var value int
		databaseErr := database.QueryRow(ctx, "SELECT 1").Scan(&value)
		databaseSpan.End()
		if databaseErr != nil || value != 1 {
			WriteProblem(writer, request, fmt.Errorf("ping database: %w", databaseErr))
			return
		}

		ctx, cacheSpan := otel.Tracer("fluentra.api").Start(ctx, "redis.ping")
		cacheErr := cache.Ping(ctx).Err()
		cacheSpan.End()
		if cacheErr != nil {
			WriteProblem(writer, request, fmt.Errorf("ping cache: %w", cacheErr))
			return
		}

		WriteJSON(writer, request, http.StatusOK, map[string]string{"status": "ok"})
	}
}
