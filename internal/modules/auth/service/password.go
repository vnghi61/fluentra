package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/contract"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// ResetInput is what a caller presents on POST /auth/reset-password.
type ResetInput struct {
	ChallengeID uuid.UUID
	Code        string
	Password    string
}

// ChangeInput is what a signed-in caller presents on POST /auth/change-password.
type ChangeInput struct {
	CurrentPassword string
	NewPassword     string
}

// PasswordChanged is the outcome of a reset or a change.
type PasswordChanged struct {
	ChangedAt time.Time
	// SessionsRevoked is how many devices were signed out. A reset takes every
	// one; a change keeps the device it was made from.
	SessionsRevoked int
}

// PasswordAccounts is the slice of the `user` module this service needs,
// declared by the consumer and narrowed to two methods — the register service
// declares a wider one, and neither should be able to reach the other's.
type PasswordAccounts interface {
	FindByEmail(ctx context.Context, email string) (Account, bool, error)
	Recipient(ctx context.Context, userID uuid.UUID) (Contact, error)
}

// PasswordCredentials is the credential half.
type PasswordCredentials interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Credential, error)
	ReplaceHash(ctx context.Context, userID uuid.UUID, passwordHash string) (domain.Credential, error)
	WithTx(tx pgx.Tx) Credentials
}

// SessionEnder revokes the sessions a password change invalidates.
type SessionEnder interface {
	RevokeAll(ctx context.Context, userID uuid.UUID) (int, error)
	RevokeAllExcept(ctx context.Context, userID, keep uuid.UUID) (int, error)
}

// PasswordDeps are the service's collaborators.
type PasswordDeps struct {
	Pool        dbx.Beginner
	Accounts    PasswordAccounts
	Credentials PasswordCredentials
	Challenges  *ChallengeService
	Sessions    SessionEnder
	Hasher      domain.Hasher
	Policy      domain.Policy
	Events      EventWriter
	Clock       clock.Clock
	NewID       IDGenerator
}

// PasswordService runs the reset and change flows.
type PasswordService struct {
	pool        dbx.Beginner
	accounts    PasswordAccounts
	credentials PasswordCredentials
	challenges  *ChallengeService
	sessions    SessionEnder
	hasher      domain.Hasher
	policy      domain.Policy
	events      EventWriter
	clock       clock.Clock
	ids         IDGenerator
}

// NewPasswordService creates the service.
func NewPasswordService(deps PasswordDeps) *PasswordService {
	return &PasswordService{
		pool: deps.Pool, accounts: deps.Accounts, credentials: deps.Credentials,
		challenges: deps.Challenges, sessions: deps.Sessions, hasher: deps.Hasher,
		policy: deps.Policy, events: deps.Events, clock: deps.Clock, ids: deps.NewID,
	}
}

// Forgot issues a password_reset challenge, whether or not the address has an
// account behind it.
//
// **Both paths do the same work and return the same shape** (BR-AUTH-26). An
// address with no account still gets a real challenge written, with a real code
// that is never delivered anywhere — the same insert, the same limiter calls,
// the same response. The alternative, returning early for an unknown address,
// is an account-enumeration oracle in both the body and the timing, and it is
// the exact question an attacker holding a list of addresses is asking.
//
// The handle is returned to the caller and the code goes to the inbox, so
// neither party alone can complete the reset (BR-AUTH-11).
func (s *PasswordService) Forgot(ctx context.Context, email string) (Issued, error) {
	address := normaliseEmail(email)

	account, found, err := s.accounts.FindByEmail(ctx, address)
	if err != nil {
		return Issued{}, fmt.Errorf("find account by email: %w", err)
	}

	var userID *uuid.UUID
	if found {
		userID = &account.ID
	}

	var issued Issued
	err = dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		// Asking again kills the previous code, in the same transaction as the
		// new one is written. A learner who requests twice because the first
		// email was slow must use the second code — otherwise the first message
		// stays a way in for as long as its thirty minutes last, and an inbox is
		// exactly where an old reset email goes to be forgotten about.
		//
		// It runs for an unknown address too. It has nothing to burn there, but
		// it does the same work, and the whole design of this operation is that
		// the two paths are indistinguishable.
		if _, burnErr := s.challenges.SupersedeIn(ctx, tx, domain.PurposePasswordReset, address); burnErr != nil {
			return burnErr
		}

		var issueErr error
		issued, issueErr = s.challenges.IssueIn(ctx, tx, IssueRequest{
			Purpose: domain.PurposePasswordReset, Subject: address, UserID: userID,
		})
		if issueErr != nil {
			return issueErr
		}
		if !found {
			// No account, so no recipient and nothing to deliver. The challenge
			// exists anyway, which is what makes this path indistinguishable
			// from the other one.
			return nil
		}
		return s.requestDelivery(ctx, tx, account.ID, issued)
	})
	if err != nil {
		return Issued{}, err
	}

	// The code is dropped here for an unknown address, deliberately: it existed
	// so the challenge was real, and it is never delivered anywhere.
	if !found {
		issued.Code = domain.Code{}
		slog.InfoContext(ctx, "password reset requested for an address with no account",
			"module", "auth", "op", "Forgot")
	}
	return issued, nil
}

// requestDelivery writes the outbox row the mailer consumes, in the same
// transaction as the challenge (rule L4) so a rolled-back request cannot send a
// code for a challenge that does not exist.
func (s *PasswordService) requestDelivery(
	ctx context.Context, tx pgx.Tx, userID uuid.UUID, issued Issued,
) error {
	contact, err := s.accounts.Recipient(ctx, userID)
	if err != nil {
		return fmt.Errorf("read recipient: %w", err)
	}
	_, err = s.events.Write(ctx, tx, contract.Aggregate, contract.EventPasswordResetRequested,
		contract.PasswordResetRequested{
			ChallengeID: issued.Challenge.ID,
			UserID:      userID,
			Email:       contact.Email,
			DisplayName: contact.DisplayName,
			Locale:      contact.Locale,
			// Reveal, not String: domain.Code is a secret.Redacted, and its
			// String renders the redaction marker. This is the one place the
			// code is meant to escape — into the payload the mailer renders —
			// and getting it wrong ships an email that says "[redacted]".
			Code:       issued.Code.Reveal(),
			ExpiresAt:  issued.Challenge.ExpiresAt,
			OccurredAt: s.clock.Now(),
		})
	return err
}

// Reset consumes the challenge and replaces the password.
//
// Every session goes (BR-AUTH-05). A reset is what a learner reaches for when
// they believe somebody else is in their account, so leaving that somebody
// signed in would defeat the operation they just performed — and unlike a
// change, there is no session worth keeping, because the caller is not signed
// in to one.
//
// The policy is checked before the challenge is consumed. A learner whose new
// password fails the breach corpus should be able to try another one with the
// code they already have, rather than being sent back to their inbox for a
// second email because the first attempt spent the code.
func (s *PasswordService) Reset(ctx context.Context, input ResetInput) (PasswordChanged, error) {
	if err := s.policy.Validate(ctx, input.Password, ""); err != nil {
		return PasswordChanged{}, err
	}

	consumed, err := s.challenges.Verify(ctx, input.ChallengeID, input.Code)
	if err != nil {
		return PasswordChanged{}, err
	}
	if consumed.Purpose != domain.PurposePasswordReset {
		// A challenge of another purpose, consumed correctly and not ours to
		// act on. Saying so beats silently doing nothing with a spent code.
		return PasswordChanged{}, fmt.Errorf("reset password: purpose %q is not a reset", consumed.Purpose)
	}
	if consumed.UserID == nil {
		// The enumeration-safe path: a challenge issued against an address with
		// no account. It verifies like any other and then has nothing to change,
		// and the caller must not be able to tell that apart from a wrong code.
		return PasswordChanged{}, domain.ErrChallengeInvalidCode
	}

	changedAt, err := s.replace(ctx, *consumed.UserID, input.Password)
	if err != nil {
		return PasswordChanged{}, err
	}

	revoked, err := s.sessions.RevokeAll(ctx, *consumed.UserID)
	if err != nil {
		return PasswordChanged{}, err
	}

	slog.InfoContext(ctx, "password reset completed",
		"module", "auth", "op", "Reset", "user_id", consumed.UserID.String(), "sessions_revoked", revoked)

	return PasswordChanged{ChangedAt: changedAt, SessionsRevoked: revoked}, nil
}

// Change replaces the password for a signed-in caller.
//
// The current password is required even though the caller holds a valid token.
// The token proves the session was opened by somebody who knew the password; it
// does not prove the person holding it now does, and without the check a token
// taken from an unlocked laptop is enough to lock its owner out of their own
// account.
//
// Every other session goes and this one stays (BR-AUTH-05). Signing the learner
// out of the device they are standing at, immediately after they did the
// responsible thing, teaches them not to do it again.
func (s *PasswordService) Change(
	ctx context.Context, actor httpx.Actor, input ChangeInput,
) (PasswordChanged, error) {
	credential, err := s.credentials.GetByUserID(ctx, actor.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrCredentialNotFound) {
			// A Google-only account has no password to change. It is the same
			// refusal a wrong one gets: which of the two it was is information
			// about the account, and the caller already knows if they have one.
			return PasswordChanged{}, errInvalidCredentials()
		}
		return PasswordChanged{}, err
	}

	verification, err := credential.Verify(s.hasher, input.CurrentPassword)
	if err != nil || !verification.Matches {
		return PasswordChanged{}, errInvalidCredentials()
	}

	if err := s.policy.Validate(ctx, input.NewPassword, ""); err != nil {
		return PasswordChanged{}, err
	}

	changedAt, err := s.replace(ctx, actor.UserID, input.NewPassword)
	if err != nil {
		return PasswordChanged{}, err
	}

	revoked, err := s.sessions.RevokeAllExcept(ctx, actor.UserID, actor.SessionID)
	if err != nil {
		return PasswordChanged{}, err
	}

	slog.InfoContext(ctx, "password changed",
		"module", "auth", "op", "Change", "user_id", actor.UserID.String(), "sessions_revoked", revoked)

	return PasswordChanged{ChangedAt: changedAt, SessionsRevoked: revoked}, nil
}

// replace hashes the new password and writes it, and announces the change.
//
// The write and the event are one transaction (rule L4): a notification saying
// "your password was changed" for a change that rolled back is worse than no
// notification, because the recipient's next move is to panic about an account
// nothing happened to.
func (s *PasswordService) replace(ctx context.Context, userID uuid.UUID, password string) (time.Time, error) {
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return time.Time{}, err
	}

	changedAt := s.clock.Now().UTC()
	err = dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, replaceErr := s.credentials.WithTx(tx).ReplaceHash(ctx, userID, hash); replaceErr != nil {
			return replaceErr
		}
		_, writeErr := s.events.Write(ctx, tx, contract.Aggregate, contract.EventPasswordChanged,
			contract.PasswordChanged{UserID: userID, OccurredAt: changedAt})
		return writeErr
	})
	if err != nil {
		return time.Time{}, err
	}
	return changedAt, nil
}

// errInvalidCredentials is the one refusal Change returns for a password that
// does not match, and for an account that has none. Telling those apart would
// say whether the account is Google-only.
func errInvalidCredentials() error {
	return apperr.New(apperr.Unauthenticated, "INVALID_CREDENTIALS", "That password is not correct.")
}
