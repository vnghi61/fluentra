package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// SessionCacheTTL is how long an ownership answer is cached (this module's
// AGENT.md §12). Five minutes is short next to a session's life and long enough
// to cover a learner revoking several devices in one sitting.
const SessionCacheTTL = 5 * time.Minute

// SessionView is one row of the caller's own device list.
type SessionView struct {
	ID          uuid.UUID
	Current     bool
	DeviceLabel *string
	CreatedAt   time.Time
	LastSeenAt  time.Time
}

// SessionRepo is the persistence the session service needs.
type SessionRepo interface {
	ListLiveSessions(ctx context.Context, userID uuid.UUID) ([]domain.Session, error)
	GetOwnedSession(ctx context.Context, sessionID, userID uuid.UUID) (domain.Session, bool, error)
	RevokeSession(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error)
	RevokeAllSessionsForUser(ctx context.Context, userID uuid.UUID, now time.Time) (int, error)
	RevokeOtherSessionsForUser(ctx context.Context, userID, keepSessionID uuid.UUID, now time.Time) (int, error)
	RevokeRefreshTokensForOtherSessions(
		ctx context.Context, userID, keepSessionID uuid.UUID, now time.Time,
	) (int, error)
	RevokeRefreshTokensBySession(ctx context.Context, sessionID uuid.UUID, now time.Time) (int, error)
	RevokeRefreshTokensForUser(ctx context.Context, userID uuid.UUID, now time.Time) (int, error)
	UntrustAllDevicesForUser(ctx context.Context, userID uuid.UUID, now time.Time) (int, error)
	WithTx(tx pgx.Tx) SessionRepo
}

// SessionOwnerCache remembers which account a session belongs to.
//
// Ownership and not liveness, and the distinction is the reason this is safe to
// cache at all: the owner of a session row never changes, so a cached answer
// cannot go stale into something dangerous. Liveness *does* change, and it is
// deliberately not cached — every revocation is an UPDATE guarded on
// `revoked_at IS NULL`, so PostgreSQL remains the only thing that decides
// whether a session is still live. A cache that answered that question could
// resurrect a revoked session for up to its TTL, which is the failure this
// design will not accept for a five-minute saving.
//
// Entries are still deleted on revoke: the row is dead, and keeping an answer
// about it wastes memory on a question nobody will ask again.
type SessionOwnerCache interface {
	Get(ctx context.Context, key string) (uuid.UUID, error)
	Set(ctx context.Context, key string, value uuid.UUID, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// AccessRevoker stops an access token working before it expires.
//
// Declared by the consumer so the session service can be tested without a
// signing key, and narrowed to the one method it uses — TokenService also mints
// tokens, and nothing here should be able to.
type AccessRevoker interface {
	RevokeNow(ctx context.Context, actor httpx.Actor) error
}

// SessionDeps are the service's collaborators.
type SessionDeps struct {
	Pool   dbx.Beginner
	Repo   SessionRepo
	Tokens AccessRevoker
	Cache  SessionOwnerCache
	Clock  clock.Clock

	// Env namespaces the cache keys, so a staging deploy pointed at a shared
	// Redis cannot answer a production question.
	Env string
}

// SessionService lists and revokes sessions.
type SessionService struct {
	pool   dbx.Beginner
	repo   SessionRepo
	tokens AccessRevoker
	cache  SessionOwnerCache
	clock  clock.Clock
	env    string
}

// NewSessionService creates the service.
func NewSessionService(deps SessionDeps) *SessionService {
	return &SessionService{
		pool:   deps.Pool,
		repo:   deps.Repo,
		tokens: deps.Tokens,
		cache:  deps.Cache,
		clock:  deps.Clock,
		env:    deps.Env,
	}
}

// List returns the caller's own live sessions.
//
// There is no user id parameter beyond the actor's. The account is taken from
// the token, so there is no version of this operation that reads somebody
// else's list, and no path segment for a caller to change.
func (s *SessionService) List(ctx context.Context, actor httpx.Actor) ([]SessionView, error) {
	sessions, err := s.repo.ListLiveSessions(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}

	views := make([]SessionView, 0, len(sessions))
	for _, session := range sessions {
		views = append(views, SessionView{
			ID:          session.ID,
			Current:     session.ID == actor.SessionID,
			DeviceLabel: session.DeviceLabel,
			CreatedAt:   session.CreatedAt,
			LastSeenAt:  session.LastSeenAt,
		})
	}
	return views, nil
}

// ListForUser returns all live sessions for an account (used by GDPR export).
func (s *SessionService) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.Session, error) {
	return s.repo.ListLiveSessions(ctx, userID)
}

// Revoke signs one device out.
//
// A session belonging to another account is `RESOURCE_NOT_FOUND`, the same
// answer an id that never existed gets, and the two are indistinguishable
// because the ownership lookup puts both the id and the owner in one WHERE
// clause. A 403 here would confirm that the id names a real session and turn
// this operation into a way to enumerate them.
//
// Revoking an already-revoked session succeeds. The caller asked for a state,
// the state holds, and a client retrying after a dropped connection should not
// be told something went wrong.
func (s *SessionService) Revoke(ctx context.Context, actor httpx.Actor, sessionID uuid.UUID) error {
	owned, err := s.ownedBy(ctx, sessionID, actor.UserID)
	if err != nil {
		return err
	}
	if !owned {
		return errSessionNotFound()
	}

	now := s.clock.Now().UTC()
	if err := s.end(ctx, sessionID, now); err != nil {
		return err
	}

	// Only the caller's own token can be denylisted here: revoking some other
	// device does not put us in possession of its access token, and there is no
	// id to deny. That token stops working at its own expiry, within one
	// access-token lifetime, which is what the acceptance criterion asks for.
	if sessionID == actor.SessionID {
		s.denyAccessToken(ctx, actor)
	}
	return nil
}

// Logout signs the caller out of the device they are holding.
//
// It differs from Revoke in one way that matters: the access token is in hand,
// so its id is known and it can be denylisted immediately rather than left to
// expire.
func (s *SessionService) Logout(ctx context.Context, actor httpx.Actor) error {
	now := s.clock.Now().UTC()
	if err := s.end(ctx, actor.SessionID, now); err != nil {
		return err
	}
	s.denyAccessToken(ctx, actor)
	return nil
}

// RevokeAll ends every session an account has. It is what `contract.SessionRevoker`
// exposes to `user` (account deletion) and `admin` (suspension), and what P2.7
// will call on a password reset.
//
// It does not denylist any access token, because it is called by somebody other
// than the account's owner and holds none of them. Those stop working within one
// access-token lifetime — the window ADR-0007 accepted in exchange for keeping a
// datastore read off every request.
func (s *SessionService) RevokeAll(ctx context.Context, userID uuid.UUID) (int, error) {
	now := s.clock.Now().UTC()

	var (
		revoked  int
		sessions []domain.Session
	)
	err := s.inTx(ctx, func(ctx context.Context, repo SessionRepo) error {
		// Read before revoking, so the cache keys to drop are known. After the
		// UPDATE the rows are still there but the list query no longer returns
		// them.
		live, err := repo.ListLiveSessions(ctx, userID)
		if err != nil {
			return err
		}
		sessions = live

		if _, err := repo.RevokeRefreshTokensForUser(ctx, userID, now); err != nil {
			return err
		}
		// Every trusted device goes too (BR-AUTH-25). A device that stayed
		// trusted through a password reset would be a ninety-day window the
		// attacker keeps, which is the opposite of what the learner asked for.
		if _, err := repo.UntrustAllDevicesForUser(ctx, userID, now); err != nil {
			return err
		}
		revoked, err = repo.RevokeAllSessionsForUser(ctx, userID, now)
		return err
	})
	if err != nil {
		return 0, err
	}

	for _, session := range sessions {
		s.forget(ctx, session.ID)
	}
	return revoked, nil
}

// RevokeAllExcept ends every session an account has except one, and returns how
// many it ended.
//
// It is what a password change calls (BR-AUTH-05's "except, optionally, the
// current one"). A reset calls RevokeAll instead, because a reset is performed
// by somebody who is not signed in and there is no session worth keeping.
//
// The kept session's access token is not denylisted and does not need to be:
// the caller is still holding it and still entitled to it. The revoked ones are
// not denylisted either, for the reason RevokeAll gives — nobody here holds
// them, and they stop working within one access-token lifetime.
func (s *SessionService) RevokeAllExcept(ctx context.Context, userID, keep uuid.UUID) (int, error) {
	now := s.clock.Now().UTC()

	var (
		revoked  int
		sessions []domain.Session
	)
	err := s.inTx(ctx, func(ctx context.Context, repo SessionRepo) error {
		live, err := repo.ListLiveSessions(ctx, userID)
		if err != nil {
			return err
		}
		sessions = live

		if _, err := repo.RevokeRefreshTokensForOtherSessions(ctx, userID, keep, now); err != nil {
			return err
		}
		// A password change untrusts every device as well, including the one it
		// was made from: the learner keeps this session, not the standing
		// ninety-day permission attached to the browser it runs in.
		if _, err := repo.UntrustAllDevicesForUser(ctx, userID, now); err != nil {
			return err
		}
		revoked, err = repo.RevokeOtherSessionsForUser(ctx, userID, keep, now)
		return err
	})
	if err != nil {
		return 0, err
	}

	for _, session := range sessions {
		if session.ID != keep {
			s.forget(ctx, session.ID)
		}
	}
	return revoked, nil
}

// end revokes the refresh family and then the session, in one transaction.
//
// The order inside it is deliberate. The family is what can still produce new
// credentials, so it goes first; if the transaction failed between the two, a
// session marked live with no renewable token is a session that expires
// harmlessly, while a revoked session with a live family is one that renews
// itself forever. They commit together, so neither state is reachable — but the
// order is what makes the failure that cannot happen the harmless one anyway.
func (s *SessionService) end(ctx context.Context, sessionID uuid.UUID, now time.Time) error {
	err := s.inTx(ctx, func(ctx context.Context, repo SessionRepo) error {
		if _, err := repo.RevokeRefreshTokensBySession(ctx, sessionID, now); err != nil {
			return err
		}
		// The boolean is ignored on purpose: false means the session was
		// already revoked, which is the state the caller asked for.
		_, err := repo.RevokeSession(ctx, sessionID, now)
		return err
	})
	if err != nil {
		return err
	}
	s.forget(ctx, sessionID)
	return nil
}

// denyAccessToken stops the caller's token now, and does not fail the sign-out
// if it cannot.
//
// This is the same fail-open decision ADR-0007 already wrote down for reading
// the denylist, applied to writing it — and the reason it is safe here is that
// the durable half of the revocation has already committed to Postgres. The
// session is gone and the family cannot renew; what survives an unreachable
// Redis is one access token, for at most its remaining fifteen minutes.
// Refusing the logout instead would leave the learner staring at a failure for
// an operation that has, in every way that lasts, already succeeded.
func (s *SessionService) denyAccessToken(ctx context.Context, actor httpx.Actor) {
	if s.tokens == nil || actor.TokenID == "" {
		return
	}
	if err := s.tokens.RevokeNow(ctx, actor); err != nil {
		slog.WarnContext(ctx, "signed out, but the access token could not be denylisted",
			"module", "auth", "op", "Logout", "session_id", actor.SessionID.String(), "error", err)
	}
}

// ownedBy answers "does this session belong to this account", through the cache
// when it can.
//
// A cache failure is not an error: the answer is in Postgres either way, and an
// unreachable Redis must not stop a learner signing a device out.
func (s *SessionService) ownedBy(ctx context.Context, sessionID, userID uuid.UUID) (bool, error) {
	if s.cache != nil {
		if owner, err := s.cache.Get(ctx, s.cacheKey(sessionID)); err == nil {
			return owner == userID, nil
		}
	}

	session, found, err := s.repo.GetOwnedSession(ctx, sessionID, userID)
	if err != nil {
		return false, err
	}
	if !found {
		// Deliberately not cached. A negative answer here covers both "not
		// yours" and "does not exist", and caching it would let a session
		// created moments later be refused for five minutes.
		return false, nil
	}

	if s.cache != nil {
		if err := s.cache.Set(ctx, s.cacheKey(sessionID), session.UserID, SessionCacheTTL); err != nil {
			slog.WarnContext(ctx, "session owner cache unavailable, continuing without it",
				"module", "auth", "op", "Revoke", "error", err)
		}
	}
	return true, nil
}

// forget drops a cache entry for a session that is now dead.
func (s *SessionService) forget(ctx context.Context, sessionID uuid.UUID) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Delete(ctx, s.cacheKey(sessionID)); err != nil {
		slog.WarnContext(ctx, "could not drop the cached session owner",
			"module", "auth", "op", "Revoke", "session_id", sessionID.String(), "error", err)
	}
}

// cacheKey is the key this module's AGENT.md §12 already documents.
func (s *SessionService) cacheKey(sessionID uuid.UUID) string {
	return fmt.Sprintf("fluentra:%s:auth:session:%s:v1", s.env, sessionID)
}

// inTx runs fn in a READ COMMITTED transaction, for the reason set out on
// RefreshService.inTx: every write here is an UPDATE guarded on
// `revoked_at IS NULL`, so correctness is a row predicate rather than a
// snapshot, and SERIALIZABLE would only add retries for concurrent revocations
// of the same session — which resolve correctly and idempotently as they are.
func (s *SessionService) inTx(ctx context.Context, fn func(context.Context, SessionRepo) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(ctx, s.repo.WithTx(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// errSessionNotFound is the one refusal this service returns, and it is the same
// error for a session that does not exist and one that belongs to somebody else.
func errSessionNotFound() error {
	return apperr.New(apperr.NotFound, "RESOURCE_NOT_FOUND", "That session was not found.")
}

// ErrSessionNotFound reports whether err is that refusal, for callers that need
// to tell it apart from a failure.
func ErrSessionNotFound(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == "RESOURCE_NOT_FOUND"
}
