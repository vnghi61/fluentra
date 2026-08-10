// Package http serves the auth module's public operations: opening an account
// and completing the email challenge that proves the address.
//
// Every operation here is unauthenticated, which is the one thing that makes
// this package different from the others. There is no actor in the context and
// no permission to check — these are how a caller acquires an identity, so
// requiring one would be circular. What replaces the guard is the rate limiting
// and the enumeration-safety in the service below.
package http

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
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

// Handler serves the auth module's HTTP operations.
type Handler struct {
	registration  Registration
	authenticator Authenticator
}

// NewHandler creates the handler.
func NewHandler(registration Registration, authenticator Authenticator) *Handler {
	return &Handler{registration: registration, authenticator: authenticator}
}

// Routes mounts this module's operations on router, relative to /api/v1.
func (h *Handler) Routes(router chi.Router) {
	router.Route("/auth", func(auth chi.Router) {
		auth.Post("/register", h.register)
		auth.Post("/login", h.login)
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
	httpx.WriteJSON(writer, request, http.StatusOK, map[string]any{
		"user_id":  result.UserID.String(),
		"verified": result.Verified,
	})
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
