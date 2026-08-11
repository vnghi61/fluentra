package http

import (
	"net/http"
	"time"

	"github.com/fluentra/fluentra/internal/modules/auth/service"
)

// RefreshCookieName is the cookie the browser replays to the refresh operation.
const RefreshCookieName = "refresh_token" // #nosec G101 -- a cookie name, not a credential

// refreshCookiePath scopes the cookie to the operations that use it.
//
// It is the deployed prefix, not "/". A refresh token attached to every request
// the SPA makes is a refresh token in every access log, every proxy buffer and
// every crash report along the way, for no benefit — exactly one endpoint reads
// it.
const refreshCookiePath = "/api/v1/auth"

// CookieOptions is what the composition root decides about the cookie.
type CookieOptions struct {
	// Secure omits the flag only for local HTTP development, where a browser
	// discards a Secure cookie outright and the learner cannot sign in at all.
	// It defaults to false in the zero value, so callers must opt in — but
	// auth.New derives it from APP_ENV such that only "local" turns it off.
	Secure bool
}

// setRefreshCookie writes the rotated token.
//
// SameSite=Lax and not Strict: Strict withholds the cookie on a top-level
// navigation from another site, so a learner following a link into the app from
// their email would arrive signed out and have to sign in again. Lax still
// withholds it on the cross-site POST that CSRF needs, which is the attack the
// attribute exists for.
func (h *Handler) setRefreshCookie(writer http.ResponseWriter, signedIn service.SignedIn, now time.Time) {
	token := signedIn.RefreshToken.Reveal()
	if token == "" {
		return
	}

	// Max-Age rather than only Expires, and derived from the token's own expiry
	// so the browser and the database agree on when it dies. A cookie that
	// outlives its row is a request that is refused for reasons the client
	// cannot see.
	maxAge := int(signedIn.RefreshExpiresAt.Sub(now).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	// gosec G124 wants Secure to be a literal true. It is a field instead,
	// because a Secure cookie over local HTTP is discarded by the browser and
	// nobody can sign in — see CookieOptions. Every attribute the linter checks
	// for is set; one of them is set from configuration.
	http.SetCookie(writer, &http.Cookie{ //nolint:gosec // Secure is configuration, see above
		Name:     RefreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookies.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearRefreshCookie removes a cookie whose token is dead.
//
// It runs when a refresh is refused, so a browser holding a revoked token stops
// replaying it on every app launch. The attributes must match the ones the
// cookie was set with or the browser keeps the original alongside the deletion.
func (h *Handler) clearRefreshCookie(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{ //nolint:gosec // Secure is configuration, as in setRefreshCookie
		Name:     RefreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookies.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// presentedRefreshToken reads the cookie.
//
// A missing cookie is an empty string rather than an error: the service refuses
// it as TOKEN_INVALID like any other value that cannot be exchanged, and a
// separate code for "you sent no cookie" would tell a caller probing the
// endpoint that the cookie is what it wants.
func presentedRefreshToken(request *http.Request) string {
	cookie, err := request.Cookie(RefreshCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
