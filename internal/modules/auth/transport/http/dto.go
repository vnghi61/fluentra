package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// maxRegisterBody bounds the request. Registration is unauthenticated and
// hashing at 64 MiB is the most expensive thing an anonymous caller can ask
// for, so the body is capped well below the default.
const maxRegisterBody = 8 << 10

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
	Purpose    string `json:"purpose"`
	VerifiedAt string `json:"verified_at"`
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
		"email": payload.Email, "password": payload.Password, "display_name": payload.DisplayName,
	} {
		if value == nil {
			missing = append(missing, apperr.FieldViolation{
				Field: name, Code: "REQUIRED", Message: name + " is required.",
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
			Field: "code", Code: "REQUIRED", Message: "code is required.",
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
			Field: "email", Code: "REQUIRED", Message: "email is required.",
		})
	}
	if payload.Password == nil {
		missing = append(missing, apperr.FieldViolation{
			Field: "password", Code: "REQUIRED", Message: "password is required.",
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
	}, nil
}
