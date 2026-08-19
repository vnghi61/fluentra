package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// maxRegisterBody bounds the request. Registration is unauthenticated and
// hashing at 64 MiB is the most expensive thing an anonymous caller can ask
// for, so the body is capped well below the default.
const maxRegisterBody = 8 << 10

const (
	codeRequired = "REQUIRED"

	// The member names several decoders report violations against. They are
	// constants because the string appears in the payload tag, in the violation
	// and in the test that asserts the violation, and three spellings of one
	// field name is two chances to disagree.
	fieldEmail    = "email"
	fieldPassword = "password" // #nosec G101 -- a field name, not a credential
	fieldCode     = "code"
)

// maxUserAgent bounds the header before it is hashed. Nothing downstream reads
// the value, so the cap costs nothing and removes a caller's ability to make
// this path allocate as much as they like.
const maxUserAgent = 512

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

// challengeResponse matches components/auth.yaml#/Challenge.
//
// There is no `code` member and there never will be. The code goes to the email
// channel and nowhere else — a code in the response body would verify nothing,
// because whoever asked for it would already have it.
type challengeResponse struct {
	ChallengeID       string `json:"challenge_id"`
	Purpose           string `json:"purpose"`
	ExpiresAt         string `json:"expires_at"`
	ResendAfter       string `json:"resend_after"`
	AttemptsRemaining int    `json:"attempts_remaining"`
}

// verifiedResponse matches components/auth.yaml#/VerifiedChallenge.
type verifiedResponse struct {
	Purpose    string          `json:"purpose"`
	VerifiedAt string          `json:"verified_at"`
	Session    sessionResponse `json:"session"`
}

// sessionResponse matches components/auth.yaml#/AuthSession.
//
// This is the one place the access token is revealed. Everywhere else it is
// wrapped in secret.Redacted so that a struct reaching a log line or a test
// failure prints `[redacted]` rather than a working bearer credential.
type sessionResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	UserID      string `json:"user_id"`
	Role        string `json:"role"`
}

func toSessionResponse(session service.Session) sessionResponse {
	return sessionResponse{
		AccessToken: session.AccessToken.Reveal(),
		TokenType:   session.TokenType,
		ExpiresIn:   session.ExpiresIn,
		UserID:      session.UserID.String(),
		Role:        session.Role,
	}
}

// sessionSummaryResponse matches components/auth.yaml#/SessionSummary.
//
// There is no ip member and there never will be one: the row stores a keyed
// digest, and a response that carried the address would undo the reason for
// storing a digest at all.
type sessionSummaryResponse struct {
	ID          string  `json:"id"`
	Current     bool    `json:"current"`
	DeviceLabel *string `json:"device_label"`
	CreatedAt   string  `json:"created_at"`
	LastSeenAt  string  `json:"last_seen_at"`
}

// sessionListResponse matches components/auth.yaml#/SessionList.
type sessionListResponse struct {
	Sessions []sessionSummaryResponse `json:"sessions"`
}

func toSessionListResponse(sessions []service.SessionView) sessionListResponse {
	// Non-nil even when empty, so the field serialises as `[]` and not `null`.
	// The schema says array; a client written against it should not have to
	// handle a second spelling of "none".
	out := make([]sessionSummaryResponse, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, sessionSummaryResponse{
			ID:          session.ID.String(),
			Current:     session.Current,
			DeviceLabel: session.DeviceLabel,
			CreatedAt:   session.CreatedAt.UTC().Format(time.RFC3339),
			LastSeenAt:  session.LastSeenAt.UTC().Format(time.RFC3339),
		})
	}
	return sessionListResponse{Sessions: out}
}

func toChallengeResponse(issued service.Issued) challengeResponse {
	return challengeResponse{
		ChallengeID:       issued.Challenge.ID.String(),
		Purpose:           issued.Challenge.Purpose.String(),
		ExpiresAt:         issued.Challenge.ExpiresAt.UTC().Format(time.RFC3339),
		ResendAfter:       issued.Challenge.ResendAllowedAt().UTC().Format(time.RFC3339),
		AttemptsRemaining: issued.Challenge.AttemptsRemaining(),
	}
}

func toVerifiedResponse(verified service.Verification) verifiedResponse {
	return verifiedResponse{
		Purpose:    verified.Purpose.String(),
		VerifiedAt: verified.VerifiedAt.UTC().Format(time.RFC3339),
		Session:    toSessionResponse(verified.Session),
	}
}

// registerPayload is the wire shape. Every member is a pointer so that "absent"
// and "present but empty" are distinguishable — the schema marks three of them
// required, and an empty string is not the same failure as a missing field.
type registerPayload struct {
	Email       *string `json:"email"`
	Password    *string `json:"password"`
	DisplayName *string `json:"display_name"`
	Locale      *string `json:"locale"`
	Timezone    *string `json:"timezone"`
}

func decodeRegisterRequest(request *http.Request) (service.Registration, error) {
	var payload registerPayload
	if err := decodeStrict(request, &payload); err != nil {
		return service.Registration{}, err
	}

	missing := make([]apperr.FieldViolation, 0, 3)
	for name, value := range map[string]*string{
		fieldEmail: payload.Email, fieldPassword: payload.Password, "display_name": payload.DisplayName,
	} {
		if value == nil {
			missing = append(missing, apperr.FieldViolation{
				Field: name, Code: codeRequired, Message: name + " is required.",
			})
		}
	}
	if len(missing) > 0 {
		// Sorted so the response is deterministic; map iteration is not.
		return service.Registration{}, validationFailed().WithFields(sortViolations(missing)...)
	}

	return service.Registration{
		Email:       strings.TrimSpace(*payload.Email),
		Password:    *payload.Password,
		DisplayName: strings.TrimSpace(*payload.DisplayName),
		Locale:      valueOr(payload.Locale, ""),
		Timezone:    valueOr(payload.Timezone, ""),
	}, nil
}

// verifyPayload is the wire shape for the code.
type verifyPayload struct {
	Code *string `json:"code"`
}

func decodeVerifyRequest(request *http.Request) (string, error) {
	var payload verifyPayload
	if err := decodeStrict(request, &payload); err != nil {
		return "", err
	}
	if payload.Code == nil {
		return "", validationFailed().WithFields(apperr.FieldViolation{
			Field: fieldCode, Code: codeRequired, Message: "code is required.",
		})
	}
	// The shape is not validated here. A wrong-shaped code still costs an
	// attempt (see ChallengeService.Verify), and rejecting it at the boundary
	// would hand an attacker a free way to keep a challenge alive.
	return *payload.Code, nil
}

// decodeStrict reads a bounded body and rejects unknown members, which is what
// `additionalProperties: false` in the schema means on this side.
func decodeStrict(request *http.Request, target any) error {
	if err := httpx.DecodeJSONLimit(request, target, maxRegisterBody); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxErr), errors.As(err, &typeErr):
			return apperr.New(apperr.BadRequest, "MALFORMED_BODY", "The request body could not be read.")
		default:
			return apperr.New(apperr.BadRequest, "MALFORMED_BODY", "The request body could not be read.")
		}
	}
	return nil
}

func validationFailed() *apperr.Error {
	return apperr.New(apperr.Validation, "VALIDATION_FAILED", "One or more request fields are invalid.")
}

func valueOr(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return strings.TrimSpace(*value)
}

// sortViolations orders by field name so two identical requests produce two
// identical responses.
func sortViolations(violations []apperr.FieldViolation) []apperr.FieldViolation {
	for i := 1; i < len(violations); i++ {
		for j := i; j > 0 && violations[j].Field < violations[j-1].Field; j-- {
			violations[j], violations[j-1] = violations[j-1], violations[j]
		}
	}
	return violations
}

// Compile-time proof that the purpose the schema enumerates is the one the
// domain produces. The schema's enum and this constant are the same four
// strings, and this is the cheapest place to notice if one of them moves.
var _ = domain.PurposeVerifyEmail

type loginPayload struct {
	Email          *string `json:"email"`
	Password       *string `json:"password"`
	RememberDevice *bool   `json:"remember_device"`
	DeviceID       *string `json:"device_id"`
}

func decodeLoginRequest(request *http.Request) (service.LoginInput, error) {
	var payload loginPayload
	if err := decodeStrict(request, &payload); err != nil {
		return service.LoginInput{}, err
	}

	missing := make([]apperr.FieldViolation, 0, 2)
	if payload.Email == nil {
		missing = append(missing, apperr.FieldViolation{
			Field: fieldEmail, Code: codeRequired, Message: "email is required.",
		})
	}
	if payload.Password == nil {
		missing = append(missing, apperr.FieldViolation{
			Field: fieldPassword, Code: codeRequired, Message: fieldPassword + " is required.",
		})
	}
	if len(missing) > 0 {
		return service.LoginInput{}, validationFailed().WithFields(sortViolations(missing)...)
	}

	clientIP := ""
	if address := httpx.ClientIP(request.Context()); address.IsValid() {
		clientIP = address.String()
	}

	remember := false
	if payload.RememberDevice != nil {
		remember = *payload.RememberDevice
	}

	return service.LoginInput{
		Email:          strings.TrimSpace(*payload.Email),
		Password:       *payload.Password,
		RememberDevice: remember,
		DeviceID:       valueOr(payload.DeviceID, ""),
		ClientIP:       clientIP,
		// Stored as a digest on the session row, never in the clear, and
		// truncated first: a user agent is attacker-controlled and unbounded,
		// and the digest is fixed-width either way.
		UserAgent: truncate(request.UserAgent(), maxUserAgent),
	}, nil
}

// oauthStartResponse matches components/auth.yaml#/OAuthStart.
//
// One member, and that is the whole design. The `state`, the `nonce` and the
// PKCE verifier are all generated for this flow and none of them appears here:
// a value the page can read is a value an attacker who can read the same page
// can replay, and the verifier in particular would defeat PKCE outright.
type oauthStartResponse struct {
	AuthorizationURL string `json:"authorization_url"`
}

// oauthIdentityResponse matches components/auth.yaml#/OAuthIdentity.
//
// The address is the one Google just asserted, not one read back from the row —
// the row stores a keyed digest, and there is nothing there to render.
type oauthIdentityResponse struct {
	Provider string `json:"provider"`
	Email    string `json:"email"`
	LinkedAt string `json:"linked_at"`
}

func toOAuthIdentityResponse(linked service.LinkedIdentity) oauthIdentityResponse {
	return oauthIdentityResponse{
		Provider: linked.Provider,
		Email:    linked.Email,
		LinkedAt: linked.LinkedAt.UTC().Format(time.RFC3339),
	}
}

// oauthCallbackPayload matches components/auth.yaml#/OAuthCallbackRequest.
type oauthCallbackPayload struct {
	Code  *string `json:"code"`
	State *string `json:"state"`
}

// maxRedirectTo bounds the `redirect_to` query parameter, matching the schema.
// The service drops anything that is not a same-site path; this stops an
// oversized one reaching it at all.
const maxRedirectTo = 512

func decodeOAuthCallbackRequest(request *http.Request) (service.CallbackInput, error) {
	var payload oauthCallbackPayload
	if err := decodeStrict(request, &payload); err != nil {
		return service.CallbackInput{}, err
	}

	missing := make([]apperr.FieldViolation, 0, 2)
	if payload.Code == nil {
		missing = append(missing, apperr.FieldViolation{
			Field: fieldCode, Code: codeRequired, Message: "code is required.",
		})
	}
	if payload.State == nil {
		missing = append(missing, apperr.FieldViolation{
			Field: "state", Code: codeRequired, Message: "state is required.",
		})
	}
	if len(missing) > 0 {
		return service.CallbackInput{}, validationFailed().WithFields(sortViolations(missing)...)
	}

	// Neither value's shape is checked here. A `state` this server did not issue
	// is refused by the store — which is also what records it — and rejecting a
	// malformed one at the boundary would mean the ones that never reach the
	// store are the ones nobody ever counts.
	clientIP := ""
	if address := httpx.ClientIP(request.Context()); address.IsValid() {
		clientIP = address.String()
	}

	return service.CallbackInput{
		Code:      *payload.Code,
		State:     *payload.State,
		ClientIP:  clientIP,
		UserAgent: truncate(request.UserAgent(), maxUserAgent),
	}, nil
}

// passwordChangedResponse matches components/auth.yaml#/PasswordChanged.
type passwordChangedResponse struct {
	ChangedAt       string `json:"changed_at"`
	SessionsRevoked int    `json:"sessions_revoked"`
}

func toPasswordChangedResponse(changed service.PasswordChanged) passwordChangedResponse {
	return passwordChangedResponse{
		ChangedAt:       changed.ChangedAt.UTC().Format(time.RFC3339),
		SessionsRevoked: changed.SessionsRevoked,
	}
}

type forgotPasswordPayload struct {
	Email *string `json:"email"`
}

func decodeForgotPasswordRequest(request *http.Request) (string, error) {
	var payload forgotPasswordPayload
	if err := decodeStrict(request, &payload); err != nil {
		return "", err
	}
	if payload.Email == nil {
		return "", validationFailed().WithFields(apperr.FieldViolation{
			Field: fieldEmail, Code: codeRequired, Message: "email is required.",
		})
	}
	return strings.TrimSpace(*payload.Email), nil
}

type resetPasswordPayload struct {
	ChallengeID *string `json:"challenge_id"`
	Code        *string `json:"code"`
	Password    *string `json:"password"`
}

func decodeResetPasswordRequest(request *http.Request) (service.ResetInput, error) {
	var payload resetPasswordPayload
	if err := decodeStrict(request, &payload); err != nil {
		return service.ResetInput{}, err
	}

	missing := make([]apperr.FieldViolation, 0, 3)
	for field, present := range map[string]bool{
		"challenge_id": payload.ChallengeID != nil,
		"code":         payload.Code != nil,
		fieldPassword:  payload.Password != nil,
	} {
		if !present {
			missing = append(missing, apperr.FieldViolation{
				Field: field, Code: codeRequired, Message: field + " is required.",
			})
		}
	}
	if len(missing) > 0 {
		return service.ResetInput{}, validationFailed().WithFields(sortViolations(missing)...)
	}

	// A malformed handle is the same refusal a wrong code gets. "That is not a
	// uuid" and "that code is wrong" must not be distinguishable, or the shape
	// of the handle becomes something to probe for (BR-AUTH-11).
	challengeID, err := uuid.Parse(*payload.ChallengeID)
	if err != nil {
		return service.ResetInput{}, domain.ErrChallengeInvalidCode
	}

	return service.ResetInput{
		ChallengeID: challengeID,
		Code:        strings.TrimSpace(*payload.Code),
		Password:    *payload.Password,
	}, nil
}

type changePasswordPayload struct {
	CurrentPassword *string `json:"current_password"`
	NewPassword     *string `json:"new_password"`
}

func decodeChangePasswordRequest(request *http.Request) (service.ChangeInput, error) {
	var payload changePasswordPayload
	if err := decodeStrict(request, &payload); err != nil {
		return service.ChangeInput{}, err
	}

	missing := make([]apperr.FieldViolation, 0, 2)
	if payload.CurrentPassword == nil {
		missing = append(missing, apperr.FieldViolation{
			Field: "current_password", Code: codeRequired, Message: "current_password is required.",
		})
	}
	if payload.NewPassword == nil {
		missing = append(missing, apperr.FieldViolation{
			Field: "new_password", Code: codeRequired, Message: "new_password is required.",
		})
	}
	if len(missing) > 0 {
		return service.ChangeInput{}, validationFailed().WithFields(sortViolations(missing)...)
	}

	return service.ChangeInput{
		CurrentPassword: *payload.CurrentPassword,
		NewPassword:     *payload.NewPassword,
	}, nil
}

// deviceResponse matches components/auth.yaml#/TrustedDevice.
//
// Both expiries are here because "stay signed in" is only defensible if it
// ends: a learner who can see the fixed date can reason about the risk, and one
// who cannot is being asked to take it on faith.
type deviceResponse struct {
	ID                string  `json:"id"`
	Current           bool    `json:"current"`
	Label             *string `json:"label"`
	TrustedAt         string  `json:"trusted_at"`
	LastSeenAt        string  `json:"last_seen_at"`
	IdleExpiresAt     string  `json:"idle_expires_at"`
	AbsoluteExpiresAt string  `json:"absolute_expires_at"`
}

// deviceListResponse matches components/auth.yaml#/TrustedDeviceList.
type deviceListResponse struct {
	Devices []deviceResponse `json:"devices"`
}

func toDeviceListResponse(devices []service.DeviceView) deviceListResponse {
	// Non-nil even when empty, so the field serialises as `[]` and not `null`.
	out := make([]deviceResponse, 0, len(devices))
	for _, device := range devices {
		out = append(out, deviceResponse{
			ID:                device.ID.String(),
			Current:           device.Current,
			Label:             device.Label,
			TrustedAt:         device.TrustedAt.UTC().Format(time.RFC3339),
			LastSeenAt:        device.LastSeenAt.UTC().Format(time.RFC3339),
			IdleExpiresAt:     device.IdleExpiresAt.UTC().Format(time.RFC3339),
			AbsoluteExpiresAt: device.AbsoluteExpiresAt.UTC().Format(time.RFC3339),
		})
	}
	return deviceListResponse{Devices: out}
}

// googleLinkStatusResponse is GoogleLinkStatus from the spec.
//
// `linked_at` is a pointer so an unlinked account serialises it as null rather
// than as a zero time, which a client would have to know to read as "none".
// There is no address here: the identity row keeps a hash of it, never the
// address, so the only way to return one would be to invent it.
type googleLinkStatusResponse struct {
	Linked    bool    `json:"linked"`
	LinkedAt  *string `json:"linked_at"`
	CanUnlink bool    `json:"can_unlink"`
}

func toGoogleLinkStatusResponse(state service.LinkState) googleLinkStatusResponse {
	if !state.Linked {
		// can_unlink is false when nothing is linked: there is no action to
		// offer, and reporting true would invite one.
		return googleLinkStatusResponse{}
	}

	linkedAt := state.Identity.LinkedAt.UTC().Format(time.RFC3339)
	return googleLinkStatusResponse{
		Linked:    true,
		LinkedAt:  &linkedAt,
		CanUnlink: state.CanUnlink,
	}
}
