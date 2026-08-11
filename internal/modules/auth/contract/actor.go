package contract

import (
	"context"

	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// Actor is the authenticated caller, as resolved from an access token.
//
// It is an alias for `httpx.Actor`, not a copy. The type lives in the shared
// kernel because every module reads it and almost none may import this package
// — `auth` depends on `user`, so a `user` handler reaching for `auth.Actor`
// would be a dependency cycle as well as a boundary violation.
//
// The alias exists anyway because `auth/AGENT.md` §4 publishes `auth.Actor` as
// part of this module's contract, and `admin` (the one module that may import
// it) should be able to name it that way. An alias means there is one type with
// two names rather than two types that have to be kept in step.
type Actor = httpx.Actor

// TokenVerifier validates an access token and returns the caller it identifies.
//
// It is the surface the HTTP middleware consumes, published here so the
// composition root can wire the middleware without importing this module's
// service package.
//
// Verify returns an `apperr.Error` distinguishing the three outcomes a client
// acts on differently: `TOKEN_EXPIRED` (refresh and retry), `TOKEN_INVALID`
// (sign in again) and `SESSION_REVOKED` (this session was signed out).
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (Actor, error)
}
