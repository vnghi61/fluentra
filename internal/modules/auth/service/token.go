package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/secret"
)

// Access token defaults, mirroring the JWT_* and ACCESS_TOKEN_TTL keys in
// `.env.example`.
const (
	// DefaultAccessTTL is fifteen minutes (BR-AUTH-03). It is the revocation
	// window: a suspended account keeps working for at most this long unless
	// something denylists its token id, which is the trade ADR-0007 accepted in
	// exchange for no database read on the authentication path.
	DefaultAccessTTL = 15 * time.Minute

	// ClockLeeway tolerates disagreement between replicas. Sixty seconds is
	// generous for hosts running NTP and small next to the token's own life; the
	// alternative is learners intermittently signed out by a clock nobody is
	// watching.
	ClockLeeway = 60 * time.Second

	// MinSigningKeyLength refuses a key too short to be worth signing with. A
	// deployment that left JWT_SIGNING_KEY at a placeholder must fail to start,
	// not issue forgeable tokens.
	MinSigningKeyLength = 32

	// TokenTypeBearer is the only scheme this issues.
	TokenTypeBearer = "Bearer"
)

// signingMethod is fixed at HS256 and is also the only method accepted when
// verifying.
//
// Pinning it on the way in is what closes the algorithm-substitution attack: a
// parser that trusts the token's own `alg` header will happily verify a token
// whose header says `none`, or verify an RS256 token using the public key as an
// HMAC secret. The library defaults to trusting the header; jwt.WithValidMethods
// below is the opt-out.
var signingMethod = jwt.SigningMethodHS256

// Session is a signed-in caller: what the HTTP layer renders and what the
// client presents on the next request.
type Session struct {
	// AccessToken is wrapped because this struct passes through log lines and
	// test failure messages, and a bearer credential printed once is a bearer
	// credential leaked. The transport layer reveals it exactly once.
	AccessToken secret.Redacted[string]
	TokenType   string
	ExpiresIn   int
	UserID      uuid.UUID
	SessionID   uuid.UUID
	Role        string
}

// Roles resolves the role a token should carry, declared by the consumer so the
// token service can be tested without `rbac`.
type Roles interface {
	RoleOf(ctx context.Context, userID uuid.UUID) (string, error)
}

// Denylist records access tokens that must stop working before they expire.
//
// It is keyed by `jti` rather than by user or session, because that is the only
// identifier a token carries that is unique to it: denying a session id would
// require reading the token's session out of it first, which is the same lookup,
// and denying a user id would sign out every device they own.
type Denylist interface {
	// Deny records a token id until it would have expired anyway. Storing it
	// for longer wastes memory on a token no verifier would accept regardless.
	Deny(ctx context.Context, tokenID string, ttl time.Duration) error
	// IsDenied reports whether the token id has been denied. An error means the
	// question could not be answered — see the note in TokenService.Verify on
	// which way that fails.
	IsDenied(ctx context.Context, tokenID string) (bool, error)
}

// TokenConfig is the signing material and the claims that identify this issuer.
type TokenConfig struct {
	// SigningKey signs every new token and is tried first when verifying.
	SigningKey []byte
	// PreviousKey is accepted when verifying and never used to sign. It is what
	// makes key rotation not sign everybody out: deploy the new key as
	// SigningKey and the old one here, wait one access-token lifetime, then
	// drop it.
	PreviousKey []byte
	Issuer      string
	Audience    string
	AccessTTL   time.Duration
}

// TokenService mints and validates access tokens.
type TokenService struct {
	config   TokenConfig
	roles    Roles
	denylist Denylist
	clock    clock.Clock
	ids      IDGenerator
	parser   *jwt.Parser
}

// TokenDeps are the service's collaborators.
type TokenDeps struct {
	Config   TokenConfig
	Roles    Roles
	Denylist Denylist
	Clock    clock.Clock
	NewID    IDGenerator
}

// NewTokenService creates the token service.
//
// It returns an error rather than panicking on a short key so the composition
// root can report which key is wrong; `auth.New` turns that into a refusal to
// start.
func NewTokenService(deps TokenDeps) (*TokenService, error) {
	config := deps.Config
	if len(config.SigningKey) < MinSigningKeyLength {
		return nil, fmt.Errorf(
			"jwt signing key must be at least %d bytes, got %d", MinSigningKeyLength, len(config.SigningKey))
	}
	// A previous key is optional, but a short one is a mistake rather than a
	// deliberate absence — silently ignoring it would leave rotation half done.
	if len(config.PreviousKey) > 0 && len(config.PreviousKey) < MinSigningKeyLength {
		return nil, fmt.Errorf(
			"jwt previous key must be at least %d bytes, got %d", MinSigningKeyLength, len(config.PreviousKey))
	}
	if config.AccessTTL <= 0 {
		config.AccessTTL = DefaultAccessTTL
	}

	options := []jwt.ParserOption{
		jwt.WithValidMethods([]string{signingMethod.Alg()}),
		jwt.WithLeeway(ClockLeeway),
		jwt.WithExpirationRequired(),
		// The parser defaults to time.Now(), which would make this the one part
		// of the service that ignores the injected clock — issuing against a
		// fake clock and validating against the real one. The closure is
		// evaluated per parse, so advancing the clock moves validation with it.
		jwt.WithTimeFunc(func() time.Time { return deps.Clock.Now() }),
	}
	if config.Issuer != "" {
		options = append(options, jwt.WithIssuer(config.Issuer))
	}
	if config.Audience != "" {
		options = append(options, jwt.WithAudience(config.Audience))
	}

	return &TokenService{
		config:   config,
		roles:    deps.Roles,
		denylist: deps.Denylist,
		clock:    deps.Clock,
		ids:      deps.NewID,
		parser:   jwt.NewParser(options...),
	}, nil
}

// accessClaims is the whole of what a token says.
//
// There is no email, no display name and no locale, and that is the point
// (BR-AUTH-03). A JWT is base64, not encryption: every claim is readable by
// anyone holding the token, including whoever stole it. What is here is two
// opaque identifiers, a role name and the standard registered claims.
type accessClaims struct {
	SessionID string `json:"sid"`
	Role      string `json:"role"`

	jwt.RegisteredClaims
}

// Issue mints an access token for a signed-in caller.
//
// sessionID identifies the sign-in. Until P2.6 gives sessions a table, it is a
// fresh identifier per login that nothing else stores — the claim is in the
// token from the start so that adding the row later does not change the token
// format and invalidate everything issued before the deploy.
func (s *TokenService) Issue(ctx context.Context, userID, sessionID uuid.UUID) (Session, error) {
	role, err := s.roleOf(ctx, userID)
	if err != nil {
		return Session{}, err
	}

	tokenID, err := s.ids(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("generate token id: %w", err)
	}

	now := s.clock.Now().UTC()
	expiresAt := now.Add(s.config.AccessTTL)

	claims := accessClaims{
		SessionID: sessionID.String(),
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        tokenID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    s.config.Issuer,
			Audience:  audienceOf(s.config.Audience),
		},
	}

	signed, err := jwt.NewWithClaims(signingMethod, claims).SignedString(s.config.SigningKey)
	if err != nil {
		return Session{}, fmt.Errorf("sign access token: %w", err)
	}

	return Session{
		AccessToken: secret.New(signed),
		TokenType:   TokenTypeBearer,
		ExpiresIn:   int(s.config.AccessTTL.Seconds()),
		UserID:      userID,
		SessionID:   sessionID,
		Role:        role,
	}, nil
}

// Verify validates a token and returns the caller it identifies.
//
// The two failures a client must tell apart are separated deliberately
// (BR-AUTH-03 acceptance): TOKEN_EXPIRED means "refresh and retry", and
// TOKEN_INVALID means "this credential is not ours — sign in again". Collapsing
// them would make every expiry look like tampering and send learners back to
// the login form fifteen minutes into a session.
func (s *TokenService) Verify(ctx context.Context, raw string) (httpx.Actor, error) {
	claims, err := s.parse(raw)
	if err != nil {
		return httpx.Actor{}, err
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return httpx.Actor{}, domain.ErrTokenInvalid
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return httpx.Actor{}, domain.ErrTokenInvalid
	}
	if claims.ID == "" {
		// Without a token id there is nothing to denylist, so an explicit logout
		// could never revoke it. Refuse rather than accept a token that cannot
		// be taken away.
		return httpx.Actor{}, domain.ErrTokenInvalid
	}

	if denied, err := s.isDenied(ctx, claims.ID); err != nil {
		return httpx.Actor{}, err
	} else if denied {
		return httpx.Actor{}, domain.ErrSessionRevoked
	}

	return httpx.Actor{
		UserID:    userID,
		SessionID: sessionID,
		Role:      claims.Role,
		TokenID:   claims.ID,
	}, nil
}

// Revoke denylists a token for the remainder of its life. `POST /auth/logout`
// (P2.6) is what calls it.
func (s *TokenService) Revoke(ctx context.Context, actor httpx.Actor, expiresAt time.Time) error {
	if s.denylist == nil {
		return nil
	}
	ttl := expiresAt.Sub(s.clock.Now())
	if ttl <= 0 {
		// Already expired; no verifier would accept it anyway.
		return nil
	}
	return s.denylist.Deny(ctx, actor.TokenID, ttl+ClockLeeway)
}

// RevokeNow denylists a token for the longest it could still be valid.
//
// It exists so a caller holding an actor does not have to know the access TTL
// in order to revoke: the actor carries a token id but not an expiry, and a
// caller that guessed the lifetime would either under-deny (leaving a window)
// or over-deny (wasting memory on a token no verifier would accept). The
// service that issued the token is the one that knows how long it lives.
func (s *TokenService) RevokeNow(ctx context.Context, actor httpx.Actor) error {
	return s.Revoke(ctx, actor, s.clock.Now().Add(s.config.AccessTTL))
}

// parse verifies the signature against the current key and then, only if the
// signature is what failed, against the previous one.
//
// Trying the current key first matters for more than speed: after a rotation
// almost every token presented is signed with the current key, and a parser that
// tried the retired key first would keep it hot long after it should have gone
// cold.
func (s *TokenService) parse(raw string) (accessClaims, error) {
	if raw == "" {
		return accessClaims{}, domain.ErrTokenInvalid
	}

	claims, err := s.parseWith(raw, s.config.SigningKey)
	if err == nil {
		return claims, nil
	}

	// Only a signature mismatch is worth retrying. An expired token signed with
	// the current key is expired under the previous key too, and retrying it
	// would report the wrong reason.
	if errors.Is(err, jwt.ErrTokenSignatureInvalid) && len(s.config.PreviousKey) > 0 {
		if previous, previousErr := s.parseWith(raw, s.config.PreviousKey); previousErr == nil {
			return previous, nil
		} else if !errors.Is(previousErr, jwt.ErrTokenSignatureInvalid) {
			return accessClaims{}, classify(previousErr)
		}
	}
	return accessClaims{}, classify(err)
}

func (s *TokenService) parseWith(raw string, key []byte) (accessClaims, error) {
	var claims accessClaims
	_, err := s.parser.ParseWithClaims(raw, &claims, func(*jwt.Token) (any, error) { return key, nil })
	if err != nil {
		return accessClaims{}, err
	}
	return claims, nil
}

// classify turns a parser failure into the code the client acts on.
//
// Expiry is checked **last**, not first, and that ordering is the point. The
// library joins every validation failure into one error, so a token that is
// both expired and signed with a key we do not hold matches ErrTokenExpired as
// well. Reporting that as TOKEN_EXPIRED would tell the client to refresh and
// retry a credential that was never ours — an instruction that can only fail,
// and one that hides a forgery attempt behind a routine-looking response.
//
// Everything that is not expiry collapses into TOKEN_INVALID. The asymmetry is
// deliberate: "expired" is a routine, actionable state, while the difference
// between a bad signature, a wrong audience and a malformed segment is
// information about our validation that a caller probing tokens should not get.
func classify(err error) error {
	switch {
	case errors.Is(err, jwt.ErrTokenSignatureInvalid),
		errors.Is(err, jwt.ErrTokenMalformed),
		errors.Is(err, jwt.ErrTokenUnverifiable),
		errors.Is(err, jwt.ErrTokenInvalidIssuer),
		errors.Is(err, jwt.ErrTokenInvalidAudience),
		errors.Is(err, jwt.ErrTokenNotValidYet):
		return domain.ErrTokenInvalid
	case errors.Is(err, jwt.ErrTokenExpired):
		return domain.ErrTokenExpired
	default:
		return domain.ErrTokenInvalid
	}
}

// isDenied consults the denylist, and lets the request through when it cannot.
//
// This is the one fail-open decision in token validation, and it is the one
// ADR-0007 already wrote down: "a revoked user can act for up to 15 minutes
// unless the denylist is consulted". Failing closed would turn a Redis outage
// into a total authentication outage for every signed-in learner, which is a
// larger harm than a logged-out token surviving to its natural expiry.
func (s *TokenService) isDenied(ctx context.Context, tokenID string) (bool, error) {
	if s.denylist == nil {
		return false, nil
	}
	denied, err := s.denylist.IsDenied(ctx, tokenID)
	if err != nil {
		slog.WarnContext(ctx, "token denylist unavailable, accepting the token until it expires",
			"module", "auth", "op", "Verify", "error", err)
		return false, nil
	}
	return denied, nil
}

// roleOf resolves the role for the token. A caller with no roles is a `user`:
// the catalogue has exactly two roles and every account holds at least the
// unprivileged one implicitly (BR-RBAC-02).
//
// A resolution failure is an error rather than a quiet fall back to `user`.
// Minting a token that says `user` for an administrator would silently strip
// their access, and the failure would surface as a permissions bug hours later
// rather than as a failed login now.
func (s *TokenService) roleOf(ctx context.Context, userID uuid.UUID) (string, error) {
	if s.roles == nil {
		return domain.RoleUser, nil
	}
	role, err := s.roles.RoleOf(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("resolve role for token: %w", err)
	}
	if role == "" {
		return domain.RoleUser, nil
	}
	return role, nil
}

func audienceOf(audience string) jwt.ClaimStrings {
	if audience == "" {
		return nil
	}
	return jwt.ClaimStrings{audience}
}
