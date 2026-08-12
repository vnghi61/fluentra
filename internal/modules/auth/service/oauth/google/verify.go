package google

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// signingMethod is fixed at RS256, which is what Google signs with, and is the
// only method accepted when verifying.
//
// Pinning it is what closes the algorithm-substitution attack, exactly as
// service/token.go pins HS256: a parser that trusts the token's own `alg` will
// verify a token whose header says `none`, or verify an HS256 token using the
// RSA public key as an HMAC secret — and that key is published, so anybody could
// forge one.
var signingMethod = jwt.SigningMethodRS256

// Identity is what a verified ID token asserts.
//
// Subject and not email is the identity. An address can be reassigned by a
// provider and a subject cannot.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// idTokenClaims is the subset this verifier reads.
type idTokenClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Nonce         string `json:"nonce"`

	jwt.RegisteredClaims
}

// Verifier checks Google's ID tokens.
type Verifier struct {
	jwks     *JWKS
	issuers  []string
	audience string
	now      func() time.Time
}

// VerifierOptions configures it.
type VerifierOptions struct {
	JWKS     *JWKS
	Issuer   string
	Audience string
	Now      func() time.Time
}

// NewVerifier creates the verifier.
func NewVerifier(options VerifierOptions) *Verifier {
	timeNow := options.Now
	if timeNow == nil {
		timeNow = time.Now
	}

	// Google issues both spellings and treats them as equivalent. Accepting one
	// only would reject half of Google's own tokens.
	issuers := []string{"https://accounts.google.com", "accounts.google.com"}
	if options.Issuer != "" {
		issuers = []string{options.Issuer}
	}

	return &Verifier{jwks: options.JWKS, issuers: issuers, audience: options.Audience, now: timeNow}
}

// Verify checks an ID token and returns what it asserts.
//
// Five checks, and **all five must pass before anything is written** anywhere
// (BR-AUTH-18): the signature against Google's published key, `iss`, `aud`,
// `exp`, and the `nonce` this server issued for this particular flow. A token
// failing any of them produces an error and no account, no identity and no
// session — there is no partial state, because nothing is created until the
// caller has this result in hand.
//
// The nonce is the one that is easy to leave out and the one that makes the
// others worth having. Without it a valid Google token obtained for some other
// application, or for an earlier flow, replays into this one.
func (v *Verifier) Verify(ctx context.Context, raw, expectedNonce string) (Identity, error) {
	if raw == "" || expectedNonce == "" {
		return Identity{}, errInvalidToken
	}

	var claims idTokenClaims
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{signingMethod.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithAudience(v.audience),
		jwt.WithTimeFunc(v.now),
	)

	_, err := parser.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
		keyID, _ := token.Header["kid"].(string)
		if keyID == "" {
			return nil, errors.New("id token carries no kid")
		}
		key, keyErr := v.jwks.KeyFor(ctx, keyID)
		if keyErr != nil {
			return nil, keyErr
		}
		return key, nil
	})
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %s", errInvalidToken, err)
	}

	if !v.issuedByGoogle(claims.Issuer) {
		return Identity{}, fmt.Errorf("%w: issuer %q", errInvalidToken, claims.Issuer)
	}

	// Constant-time is unnecessary — the nonce is not a secret being guessed,
	// it is a value this server already knows and is comparing against — but
	// the comparison must happen, and it must happen on every path.
	if claims.Nonce != expectedNonce {
		return Identity{}, fmt.Errorf("%w: nonce does not match this flow", errInvalidToken)
	}

	if claims.Subject == "" {
		return Identity{}, fmt.Errorf("%w: no subject", errInvalidToken)
	}

	return Identity{
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
	}, nil
}

func (v *Verifier) issuedByGoogle(issuer string) bool {
	for _, accepted := range v.issuers {
		if issuer == accepted {
			return true
		}
	}
	return false
}

// errInvalidToken is the sentinel every failure wraps. The five reasons collapse
// into one on the way out: which check a forged token tripped is information
// about our validation, and the caller's next move is the same for all of them.
var errInvalidToken = errors.New("google id token is not valid")

// ErrInvalidToken reports whether err is a token-validation failure.
func ErrInvalidToken(err error) bool { return errors.Is(err, errInvalidToken) }
