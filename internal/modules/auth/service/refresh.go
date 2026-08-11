package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/contract"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
	"github.com/fluentra/fluentra/internal/shared/secret"
)

// DefaultRefreshTTL mirrors REFRESH_TOKEN_TTL in `.env.example`. It is the idle
// window: a learner who does not open the app within it signs in again.
const DefaultRefreshTTL = 720 * time.Hour

// SignedIn is everything a successful sign-in or rotation produces.
//
// The refresh token is wrapped for the same reason the access token is: this
// struct passes through log lines and test failure messages on its way to the
// transport layer, and a refresh token printed once is a renewable credential
// leaked. The transport layer reveals it exactly once, into a Set-Cookie.
type SignedIn struct {
	Session          Session
	RefreshToken     secret.Redacted[string]
	RefreshExpiresAt time.Time
}

// StartInput opens a session for a caller who has just proved who they are.
//
// ClientIP and UserAgent are digested before they are stored and are never kept
// in the clear — an IP is personal data, and this table would otherwise be a
// movement log for every learner.
type StartInput struct {
	UserID    uuid.UUID
	ClientIP  string
	UserAgent string
}

// RefreshRepo is the persistence this service needs, declared here so the
// service is defined by what it uses rather than by what the repository offers.
type RefreshRepo interface {
	CreateSession(
		ctx context.Context, id, userID uuid.UUID, deviceLabel *string, ipHash, userAgentHash []byte, now time.Time,
	) error
	TouchSession(ctx context.Context, sessionID uuid.UUID, now time.Time) error
	RevokeSession(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error)
	CreateRefreshToken(
		ctx context.Context, id uuid.UUID, tokenHash []byte, familyID, sessionID uuid.UUID, now, expiresAt time.Time,
	) (domain.RefreshToken, error)
	ClaimRefreshToken(ctx context.Context, tokenHash []byte, now time.Time) (domain.SessionToken, bool, error)
	FindRefreshToken(ctx context.Context, tokenHash []byte) (domain.SessionToken, bool, error)
	RevokeRefreshFamily(ctx context.Context, familyID uuid.UUID, now time.Time) (int, error)
	WithTx(tx pgx.Tx) RefreshRepo
}

// RefreshDeps are the service's collaborators.
type RefreshDeps struct {
	Pool   dbx.Beginner
	Repo   RefreshRepo
	Tokens Tokens
	Events EventWriter
	Keys   domain.Keyring
	Clock  clock.Clock
	NewID  IDGenerator

	// TTL is the idle window a new or rotated token gets. Zero means
	// DefaultRefreshTTL; a literal duration anywhere else in this module is a
	// review failure (this module's AGENT.md §12).
	TTL time.Duration

	// Entropy draws the token bytes. Nil means crypto/rand.
	Entropy io.Reader
}

// RefreshService issues, rotates and revokes refresh tokens.
type RefreshService struct {
	pool    dbx.Beginner
	repo    RefreshRepo
	tokens  Tokens
	events  EventWriter
	keys    domain.Keyring
	clock   clock.Clock
	ids     IDGenerator
	ttl     time.Duration
	entropy io.Reader
}

// NewRefreshService creates the service.
func NewRefreshService(deps RefreshDeps) *RefreshService {
	ttl := deps.TTL
	if ttl <= 0 {
		ttl = DefaultRefreshTTL
	}
	return &RefreshService{
		pool:    deps.Pool,
		repo:    deps.Repo,
		tokens:  deps.Tokens,
		events:  deps.Events,
		keys:    deps.Keys,
		clock:   deps.Clock,
		ids:     deps.NewID,
		ttl:     ttl,
		entropy: deps.Entropy,
	}
}

// Start opens a session and issues the first token of a new family.
//
// The session id becomes the token's `sid` claim, which P2.4 put in the token
// before there was a row to point at precisely so that this card could add the
// row without changing the token format.
func (s *RefreshService) Start(ctx context.Context, input StartInput) (SignedIn, error) {
	now := s.clock.Now().UTC()

	sessionID, err := s.ids(ctx)
	if err != nil {
		return SignedIn{}, fmt.Errorf("generate session id: %w", err)
	}

	var ipHash, userAgentHash []byte
	if input.ClientIP != "" {
		ipHash = s.keys.SubjectHash(input.ClientIP)
	}
	if input.UserAgent != "" {
		userAgentHash = s.keys.SubjectHash(input.UserAgent)
	}

	// The label is derived here and stored, because this is the only moment the
	// raw user agent exists: the column beside it holds a digest, and no label
	// can be recovered from that afterwards. It is nil for a sign-in that had no
	// user agent to read, which the session list renders as an unnamed device.
	deviceLabel := domain.DeviceLabel(input.UserAgent)

	// A new sign-in is its own family: the family id is the first token's id, so
	// there is no separate identifier to keep in step with anything.
	var issued SignedIn
	err = s.inTx(ctx, func(ctx context.Context, _ pgx.Tx, repo RefreshRepo) error {
		if err := repo.CreateSession(
			ctx, sessionID, input.UserID, deviceLabel, ipHash, userAgentHash, now); err != nil {
			return err
		}
		token, err := s.mint(ctx, repo, uuid.Nil, sessionID, now)
		if err != nil {
			return err
		}
		issued = token
		return nil
	})
	if err != nil {
		return SignedIn{}, err
	}

	// The access token is minted outside the transaction on purpose. It is a
	// signature over data already committed, it touches no row, and holding a
	// transaction open across it would keep a connection busy for the duration
	// of a role lookup.
	session, err := s.tokens.Issue(ctx, input.UserID, sessionID)
	if err != nil {
		return SignedIn{}, err
	}
	issued.Session = session
	return issued, nil
}

// Rotate exchanges a refresh token for the next one, and detects reuse.
//
// The shape of this function is the security property. The claim is a single
// guarded UPDATE inside the transaction that writes the replacement, so the
// question "is this token still spendable?" and the act of spending it are one
// statement. A read followed by a write would pass every sequential test in the
// suite and issue two live tokens from one under concurrency — which is the
// exact state reuse detection exists to make impossible, arrived at by the
// server itself.
//
// Everything that is not a successful claim funnels into refuse, which decides
// between "this credential buys nothing" and "somebody is replaying a spent
// token, burn the family".
func (s *RefreshService) Rotate(ctx context.Context, presented string) (SignedIn, error) {
	digest, ok := domain.RefreshTokenDigest(presented)
	if !ok {
		// Not even the right shape. Refusing here keeps a scanner's traffic off
		// the database entirely.
		return SignedIn{}, domain.ErrTokenInvalid
	}

	now := s.clock.Now().UTC()

	var (
		claimed domain.SessionToken
		issued  SignedIn
		spent   bool
	)
	err := s.inTx(ctx, func(ctx context.Context, _ pgx.Tx, repo RefreshRepo) error {
		token, ok, err := repo.ClaimRefreshToken(ctx, digest, now)
		if err != nil {
			return err
		}
		if !ok {
			// Commit an empty transaction. Why the claim missed is decided
			// outside, where a second transaction can revoke a family without
			// racing the one that is still open here.
			return nil
		}
		spent, claimed = true, token

		next, err := s.mint(ctx, repo, token.FamilyID, token.SessionID, now)
		if err != nil {
			return err
		}
		issued = next

		// Evidence the learner was here, for the session list P2.6 builds.
		return repo.TouchSession(ctx, token.SessionID, now)
	})
	if err != nil {
		return SignedIn{}, err
	}
	if !spent {
		return SignedIn{}, s.refuse(ctx, digest, now)
	}

	session, err := s.tokens.Issue(ctx, claimed.UserID, claimed.SessionID)
	if err != nil {
		return SignedIn{}, err
	}
	issued.Session = session
	return issued, nil
}

// inTx runs fn in a READ COMMITTED transaction.
//
// It is not dbx.InTx, and the difference is deliberate. dbx.InTx opens every
// transaction SERIALIZABLE and retries a serialization failure three times,
// which is right for the read-decide-write shapes the rest of the codebase has
// and wrong for this one twice over.
//
// It is unnecessary, because nothing here decides anything from a snapshot: the
// claim's correctness is a row-level predicate evaluated by the UPDATE itself,
// and READ COMMITTED re-evaluates that predicate against the winner's committed
// row before deciding it matched nothing. That is exactly the behaviour reuse
// detection needs, and SERIALIZABLE cannot make it more true.
//
// It is also harmful. Every concurrent refresh of one token, and every
// concurrent revocation of one family, would collide on the same rows and burn
// retries -- so the isolation level would make the authentication path least
// available precisely during the replay storm it exists to detect. Three
// attempts is not a budget worth spending there.
//
// A tidier home for this would be an isolation-level option on dbx.InTx, which
// is a change to a shared package and outside this card.
func (s *RefreshService) inTx(ctx context.Context, fn func(context.Context, pgx.Tx, RefreshRepo) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(ctx, tx, s.repo.WithTx(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// mint writes one refresh token row and returns the value to hand out.
//
// familyID of uuid.Nil starts a new family rooted at this token, which is what
// a fresh sign-in wants; any other value continues an existing chain.
func (s *RefreshService) mint(
	ctx context.Context, repo RefreshRepo, familyID, sessionID uuid.UUID, now time.Time,
) (SignedIn, error) {
	tokenID, err := s.ids(ctx)
	if err != nil {
		return SignedIn{}, fmt.Errorf("generate refresh token id: %w", err)
	}
	if familyID == uuid.Nil {
		familyID = tokenID
	}

	raw, digest, err := domain.NewRefreshToken(s.entropy)
	if err != nil {
		return SignedIn{}, err
	}

	expiresAt := now.Add(s.ttl)
	if _, err := repo.CreateRefreshToken(ctx, tokenID, digest, familyID, sessionID, now, expiresAt); err != nil {
		return SignedIn{}, err
	}

	return SignedIn{
		RefreshToken:     secret.New(raw),
		RefreshExpiresAt: expiresAt,
	}, nil
}

// refuse decides why a claim matched nothing, and acts on it.
//
// The four reasons are not equivalent and only one of them is an incident:
//
//   - No such token, or one whose shape never existed here — TOKEN_INVALID. It
//     names no account, so there is nothing to revoke and no event to raise.
//     Revoking on an unknown value would hand any caller a way to sign out a
//     stranger by guessing, which is a denial of service with no authentication.
//   - Expired but never spent — TOKEN_INVALID. A learner who closed the laptop
//     for a month is not an attacker; raising an event for each one would bury
//     the events that matter.
//   - Already revoked — SESSION_REVOKED, and nothing more. The family is
//     already dead, so a second revocation writes nothing and a second event
//     would duplicate the first.
//   - Already spent — the incident. Two parties hold a single-use credential
//     and the server cannot tell which is the learner, so it trusts neither.
func (s *RefreshService) refuse(ctx context.Context, digest []byte, now time.Time) error {
	token, found, err := s.repo.FindRefreshToken(ctx, digest)
	if err != nil {
		return err
	}
	switch {
	case !found:
		return domain.ErrTokenInvalid
	case token.Spent():
		return s.handleReuse(ctx, token, now)
	case token.Revoked():
		return domain.ErrSessionRevoked
	case token.ExpiredAt(now):
		return domain.ErrTokenInvalid
	default:
		// The row is live and unspent, yet the claim did not match it. The one
		// way to reach here is a session revoked between the two statements,
		// which the claim's join checks and this read does not.
		return domain.ErrSessionRevoked
	}
}

// handleReuse burns the family a replayed token belongs to.
//
// The revocation, the session and the outbox row are one transaction (rule L4),
// so a crash cannot leave a family revoked with no record of why, or a security
// event describing a revocation that did not happen.
//
// The cost is real and worth stating: the legitimate learner is signed out
// alongside the thief, on every device sharing that session. Two honest tabs
// refreshing at the same instant land here too. The alternative — a grace
// window in which a spent token is tolerated — is a window an attacker with the
// stolen token simply refreshes inside, which is to say it is not an
// alternative. OAuth 2.0 BCP §4.14.2 makes the same call.
func (s *RefreshService) handleReuse(ctx context.Context, token domain.SessionToken, now time.Time) error {
	// Whether this call is the one that burnt the family, or the fourth caller
	// to arrive at a family already burnt. Both return the same refusal to the
	// client; only the first reports an incident. A replay storm is one theft,
	// and eight alerts for it is eight times the noise and none of the extra
	// information.
	first := false

	err := s.inTx(ctx, func(ctx context.Context, tx pgx.Tx, repo RefreshRepo) error {
		// Reset per attempt so a value from an abandoned transaction cannot be
		// read as this one's.
		first = false

		revokedTokens, err := repo.RevokeRefreshFamily(ctx, token.FamilyID, now)
		if err != nil {
			return err
		}
		revokedSession, err := repo.RevokeSession(ctx, token.SessionID, now)
		if err != nil {
			return err
		}
		if revokedTokens == 0 && !revokedSession {
			return nil
		}
		first = true

		_, err = s.events.Write(ctx, tx, contract.Aggregate, contract.EventSecurityEvent, contract.SecurityEvent{
			Kind:       contract.SecurityKindRefreshReuse,
			Severity:   contract.SeverityHigh,
			UserID:     token.UserID,
			SessionID:  token.SessionID,
			OccurredAt: now,
		})
		return err
	})
	if err != nil {
		return err
	}
	if first {
		slog.WarnContext(ctx, "refresh token reuse detected, family revoked",
			"module", "auth", "op", "Rotate",
			"session_id", token.SessionID.String(), "family_id", token.FamilyID.String())
	}

	return domain.ErrSessionRevoked
}
