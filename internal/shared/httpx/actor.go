package httpx

import (
	"context"

	"github.com/google/uuid"
)

// Actor is the authenticated caller, as resolved from the access token.
//
// It lives here rather than in the auth module because every module needs to
// read it and no module but `admin` may import `auth/contract` — `auth`
// depends on `user`, so a `user` handler reaching for `auth.Actor` would be a
// dependency cycle as well as a boundary violation. The shape matches the
// `auth.Actor` that `auth/AGENT.md` §4 specifies, so the middleware in P2.4
// populates this type directly.
//
// It sits beside WithRequestID for the same reason that does: both are
// request-scoped facts that middleware establishes once and handlers read.
type Actor struct {
	// UserID is the account the token was issued to. It is the only thing a
	// handler needs to answer "whose data is this?".
	UserID uuid.UUID

	// SessionID identifies the sign-in, so a single session can be revoked
	// without touching the others.
	SessionID uuid.UUID

	// Role is `admin` or `user`. There are exactly two.
	Role string

	// TokenID is the JWT `jti`, used for the logout denylist.
	TokenID string
}

type actorKey struct{}

// WithActor attaches the authenticated caller to a context. Only the auth
// middleware calls this in production; handler tests call it to stand in for
// the middleware.
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

// ActorFrom returns the authenticated caller.
//
// The second result is false for an unauthenticated request, and callers must
// check it: the zero Actor has a zero UserID, and a handler that used it
// without checking would query for the nil UUID rather than fail. Returning
// (Actor, bool) rather than a pointer is what makes forgetting that check a
// compile-time inconvenience instead of a silent data leak.
func ActorFrom(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorKey{}).(Actor)
	if !ok || actor.UserID == uuid.Nil {
		return Actor{}, false
	}
	return actor, true
}
