// Package http serves the auth module's operations: acquiring an identity, and
// managing one already held.
//
// The acquiring half — register, verify, login, refresh — is unauthenticated,
// which is what makes this package different from the others: there is no actor
// in the context and no permission to check, because these are how a caller gets
// one and requiring it would be circular. What replaces the guard there is the
// rate limiting and the enumeration-safety in the service below.
//
// The managing half — logout, and listing and revoking sessions — needs the
// actor like any other protected operation, and starts by demanding one.
package http

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// Registration is the service surface these handlers need, declared here so the
// handler can be exercised against a fake.
type Registration interface {
	Register(ctx context.Context, request service.Registration) (service.Issued, error)
	VerifyEmail(ctx context.Context, challengeID uuid.UUID, code string) (service.Verification, error)
	Resend(ctx context.Context, challengeID uuid.UUID) (service.Issued, error)
}

// Authenticator is the login surface required by Handler.
type Authenticator interface {
	Login(ctx context.Context, input service.LoginInput) (service.LoginResult, error)
}

// Rotator is the refresh surface required by Handler.
type Rotator interface {
	Rotate(ctx context.Context, presented string) (service.SignedIn, error)
}

// Passwords is the reset and change surface required by Handler.
type Passwords interface {
	Forgot(ctx context.Context, email string) (service.Issued, error)
	Reset(ctx context.Context, input service.ResetInput) (service.PasswordChanged, error)
	Change(ctx context.Context, actor httpx.Actor, input service.ChangeInput) (service.PasswordChanged, error)
}

// Sessions is the session surface required by Handler.
type Sessions interface {
	List(ctx context.Context, actor httpx.Actor) ([]service.SessionView, error)
	Revoke(ctx context.Context, actor httpx.Actor, sessionID uuid.UUID) error
	Logout(ctx context.Context, actor httpx.Actor) error
}

// Handler serves the auth module's HTTP operations.
type Handler struct {
	registration  Registration
	authenticator Authenticator
	rotator       Rotator
	sessions      Sessions
	passwords     Passwords
	cookies       CookieOptions
	clock         clock.Clock
}

// NewHandler creates the handler.
func NewHandler(
	registration Registration, authenticator Authenticator,
	rotator Rotator, sessions Sessions, passwords Passwords, cookies CookieOptions,
) *Handler {
	return &Handler{
		registration:  registration,
		authenticator: authenticator,
		rotator:       rotator,
		sessions:      sessions,
		passwords:     passwords,
		cookies:       cookies,
		clock:         clock.Real{},
	}
}

// Routes mounts this module's operations on router, relative to /api/v1.
func (h *Handler) Routes(router chi.Router) {
	router.Route("/auth", func(auth chi.Router) {
		auth.Post("/register", h.register)
		auth.Post("/login", h.login)
		auth.Post("/refresh", h.refresh)
		auth.Post("/logout", h.logout)
		auth.Post("/forgot-password", h.forgotPassword)
		auth.Post("/reset-password", h.resetPassword)
		auth.Post("/change-password", h.changePassword)
		auth.Get("/sessions", h.listSessions)
		auth.Delete("/sessions/{id}", h.revokeSession)
		auth.Route("/challenges/{id}", func(challenge chi.Router) {
			challenge.Post("/verify", h.verify)
			challenge.Post("/resend", h.resend)
		})
	})
}

func (h *Handler) login(writer http.ResponseWriter, request *http.Request) {
	input, err := decodeLoginRequest(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	result, err := h.authenticator.Login(request.Context(), input)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	h.setRefreshCookie(writer, result.SignedIn, h.clock.Now())
	httpx.WriteJSON(writer, request, http.StatusOK, toSessionResponse(result.Session))
}

// refresh rotates the token in the cookie.
//
// There is no request body and no Authorization header: the credential is the
// cookie, which is why this operation is reachable without an actor in the
// context. A failure clears the cookie, so a browser holding a revoked token
// stops replaying it on every launch — and, after a reuse detection, stops
// re-reporting an incident that has already been filed.
func (h *Handler) refresh(writer http.ResponseWriter, request *http.Request) {
	signedIn, err := h.rotator.Rotate(request.Context(), presentedRefreshToken(request))
	if err != nil {
		h.clearRefreshCookie(writer)
		httpx.WriteProblem(writer, request, err)
		return
	}
	h.setRefreshCookie(writer, signedIn, h.clock.Now())
	httpx.WriteJSON(writer, request, http.StatusOK, toSessionResponse(signedIn.Session))
}

func (h *Handler) register(writer http.ResponseWriter, request *http.Request) {
	body, err := decodeRegisterRequest(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	issued, err := h.registration.Register(request.Context(), body)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusCreated, toChallengeResponse(issued))
}

func (h *Handler) verify(writer http.ResponseWriter, request *http.Request) {
	challengeID, err := challengeIDFrom(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	code, err := decodeVerifyRequest(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	verified, err := h.registration.VerifyEmail(request.Context(), challengeID, code)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	h.setRefreshCookie(writer, verified.SignedIn, h.clock.Now())
	httpx.WriteJSON(writer, request, http.StatusOK, toVerifiedResponse(verified))
}

func (h *Handler) resend(writer http.ResponseWriter, request *http.Request) {
	challengeID, err := challengeIDFrom(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	issued, err := h.registration.Resend(request.Context(), challengeID)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, toChallengeResponse(issued))
}

// logout signs the caller out of the device they are holding.
func (h *Handler) logout(writer http.ResponseWriter, request *http.Request) {
	actor, err := requireActor(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	if err := h.sessions.Logout(request.Context(), actor); err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	h.clearRefreshCookie(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listSessions(writer http.ResponseWriter, request *http.Request) {
	actor, err := requireActor(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	sessions, err := h.sessions.List(request.Context(), actor)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, toSessionListResponse(sessions))
}

// revokeSession signs one device out.
//
// The cookie is cleared only when the caller revoked the session they are
// holding. Clearing it unconditionally would sign somebody out of the device in
// their hand for tidying up a different one.
func (h *Handler) revokeSession(writer http.ResponseWriter, request *http.Request) {
	actor, err := requireActor(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	sessionID, err := sessionIDFrom(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	if err := h.sessions.Revoke(request.Context(), actor, sessionID); err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	if sessionID == actor.SessionID {
		h.clearRefreshCookie(writer)
	}
	writer.WriteHeader(http.StatusNoContent)
}

// forgotPassword starts a reset. It answers 202 whether or not the address has
// an account, which is the whole point of the operation (BR-AUTH-26).
func (h *Handler) forgotPassword(writer http.ResponseWriter, request *http.Request) {
	email, err := decodeForgotPasswordRequest(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	issued, err := h.passwords.Forgot(request.Context(), email)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusAccepted, toChallengeResponse(issued))
}

// resetPassword consumes the code and replaces the password.
//
// It does not sign the caller in, and it clears any refresh cookie the browser
// still holds. Every session was just revoked, so a cookie left in place would
// only be replayed once and refused.
func (h *Handler) resetPassword(writer http.ResponseWriter, request *http.Request) {
	input, err := decodeResetPasswordRequest(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	changed, err := h.passwords.Reset(request.Context(), input)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	h.clearRefreshCookie(writer)
	httpx.WriteJSON(writer, request, http.StatusOK, toPasswordChangedResponse(changed))
}

// changePassword replaces the password for a signed-in caller and keeps them
// signed in on this device, cookie and all.
func (h *Handler) changePassword(writer http.ResponseWriter, request *http.Request) {
	actor, err := requireActor(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	input, err := decodeChangePasswordRequest(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	changed, err := h.passwords.Change(request.Context(), actor, input)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, toPasswordChangedResponse(changed))
}

// requireActor is the guard the authenticated operations start with.
//
// The middleware lets a request with no Authorization header through
// unauthenticated so the public operations still work, which means the handler
// is where "this one needs a caller" is decided.
func requireActor(request *http.Request) (httpx.Actor, error) {
	actor, ok := httpx.ActorFrom(request.Context())
	if !ok || actor.UserID == uuid.Nil {
		return httpx.Actor{}, apperr.New(
			apperr.Unauthenticated, "TOKEN_INVALID", "This operation requires a signed-in caller.")
	}
	return actor, nil
}

// sessionIDFrom parses the path segment.
//
// A malformed uuid is the same 404 an unknown id gets, for the reason the
// operation exists to observe: "that is not a uuid" and "you have no such
// session" must not be distinguishable, or the shape of the id becomes something
// to probe for.
func sessionIDFrom(request *http.Request) (uuid.UUID, error) {
	sessionID, err := uuid.Parse(chi.URLParam(request, "id"))
	if err != nil {
		return uuid.Nil, apperr.New(apperr.NotFound, "RESOURCE_NOT_FOUND", "That session was not found.")
	}
	return sessionID, nil
}

// challengeIDFrom parses the path segment.
//
// A malformed uuid is a 404 rather than a 422. The id is the secret that gates
// the whole flow (BR-AUTH-11), and "that is not a valid uuid" versus "no such
// challenge" is a distinction only somebody probing would care about.
func challengeIDFrom(request *http.Request) (uuid.UUID, error) {
	challengeID, err := uuid.Parse(chi.URLParam(request, "id"))
	if err != nil {
		return uuid.Nil, apperr.New(
			apperr.NotFound, "CHALLENGE_NOT_FOUND", "That verification request was not found.")
	}
	return challengeID, nil
}
