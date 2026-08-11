package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// bearerPrefix is the only authorization scheme this accepts, matched
// case-insensitively because RFC 7235 says the scheme token is case-insensitive
// and clients disagree about how to spell it.
const bearerPrefix = "bearer "

// TokenVerifier validates an access token and returns the caller it identifies.
// Declared here so the middleware can be tested without the token service.
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (httpx.Actor, error)
}

// Authenticate validates a bearer token when one is present and puts the caller
// in the request context.
//
// **A request with no Authorization header passes through unauthenticated.**
// That is what lets this be mounted over the whole API rather than threaded
// around the public operations: `/auth/register` and `/auth/login` are how a
// caller acquires a token in the first place, so requiring one there would be
// circular. Handlers that need a caller already ask for one — every `/me`
// handler starts with requireActor — so an unauthenticated request reaching a
// protected handler still becomes a 401, from the handler that knows it needs
// an actor rather than from middleware that has to guess.
//
// A header that **is** present and does not validate is rejected here, with the
// distinction between expired and invalid intact. Passing a bad token through
// as "unauthenticated" would turn a client whose token just expired into an
// anonymous caller, and it would learn about it as a confusing 404 on its own
// profile rather than as the 401 that tells it to refresh.
func Authenticate(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			raw, present := bearerToken(request)
			if !present {
				next.ServeHTTP(writer, request)
				return
			}

			actor, err := verifier.Verify(request.Context(), raw)
			if err != nil {
				httpx.WriteProblem(writer, request, err)
				return
			}
			next.ServeHTTP(writer, request.WithContext(httpx.WithActor(request.Context(), actor)))
		})
	}
}

// bearerToken extracts the credential from the Authorization header.
//
// A header that is present but not a bearer token — Basic, or a bare token with
// no scheme — reports present with an empty credential rather than absent. The
// caller then rejects it, which is right: somebody sent an authorization
// attempt and it was not one we accept, and treating that as "anonymous" would
// silently downgrade it.
func bearerToken(request *http.Request) (string, bool) {
	header := request.Header.Get("Authorization")
	if strings.TrimSpace(header) == "" {
		return "", false
	}
	if len(header) < len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return "", true
	}
	return strings.TrimSpace(header[len(bearerPrefix):]), true
}
