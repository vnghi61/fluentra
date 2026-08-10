package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/auth/contract"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Account is what this service needs to know about an existing address. It
// mirrors user/contract.Account; declaring it here is what lets the service be
// tested without the user module.
type Account struct {
	ID       uuid.UUID
	Verified bool
	Status   string
}

// NewAccount is the minimum needed to open one.
type NewAccount struct {
	Email       string
	DisplayName string
	Locale      string
	Timezone    string
}

// Accounts is the slice of the `user` module this service uses, declared by the
// consumer. The composition root adapts `user.Registrar` to it.
//
// Every method here replaces a query `auth` is not allowed to write: it owns no
// user table, and rule L2 forbids it reading one.
type Accounts interface {
	CreateAccount(ctx context.Context, account NewAccount) (uuid.UUID, error)
	FindByEmail(ctx context.Context, email string) (Account, bool, error)
	Recipient(ctx context.Context, userID uuid.UUID) (Contact, error)
	MarkEmailVerified(ctx context.Context, userID uuid.UUID) error
	PurgeUnverifiedBefore(ctx context.Context, cutoff time.Time) (int, error)
}

// Credentials is the password half of registration.
type Credentials interface {
	Create(ctx context.Context, id, userID uuid.UUID, passwordHash string) (domain.Credential, error)
	ReplaceHash(ctx context.Context, userID uuid.UUID, passwordHash string) (domain.Credential, error)

	WithTx(tx pgx.Tx) Credentials
}

// OutboxTx is the transaction surface the outbox writer needs.
type OutboxTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// EventWriter records an event in the same transaction as the write that caused
// it.
type EventWriter interface {
	Write(ctx context.Context, tx OutboxTx, aggregate, event string, payload any) (uuid.UUID, error)
}

// UnverifiedAccountTTL is how long an address may be claimed without being
// proved. After it, the account is swept and the address is free again —
// otherwise claiming addresses is a denial of service against the people who
// own them.
const UnverifiedAccountTTL = 7 * 24 * time.Hour

// RegisterService runs registration and email verification.
type RegisterService struct {
	pool        dbx.Beginner
	accounts    Accounts
	credentials Credentials
	challenges  *ChallengeService
	hasher      domain.Hasher
	policy      domain.Policy
	events      EventWriter
	clock       clock.Clock
	ids         IDGenerator
}

// RegisterDeps are the service's collaborators.
type RegisterDeps struct {
	Pool        dbx.Beginner
	Accounts    Accounts
	Credentials Credentials
	Challenges  *ChallengeService
	Hasher      domain.Hasher
	Policy      domain.Policy
	Events      EventWriter
	Clock       clock.Clock
	NewID       IDGenerator
}

// NewRegisterService creates the registration service.
func NewRegisterService(deps RegisterDeps) *RegisterService {
	return &RegisterService{
		pool: deps.Pool, accounts: deps.Accounts, credentials: deps.Credentials,
		challenges: deps.Challenges, hasher: deps.Hasher, policy: deps.Policy,
		events: deps.Events, clock: deps.Clock, ids: deps.NewID,
	}
}

// Registration is what a caller submits.
type Registration struct {
	Email       string
	Password    string
	DisplayName string
	Locale      string
	Timezone    string
}

// Register opens an account and issues a verification challenge.
//
// Three paths lead out of here and all three return the same shape, because the
// response is the only thing a caller probing for accounts can see:
//
//   - the address is new: create the account, store the credential, issue a
//     challenge, and write the outbox row that delivers the code;
//   - the address is claimed but not verified: replace the credential with the
//     submitted password and issue a fresh challenge. Reissuing the pending one
//     instead is an account-takeover path — see BR-AUTH-14 and the note on
//     registerUnverified;
//   - the address is verified: issue a real challenge that nobody will ever
//     receive a code for, and send its owner a warning instead.
//
// The third path issues a real challenge rather than inventing an id because
// the alternative leaks. A fabricated id returns CHALLENGE_NOT_FOUND from the
// verify operation while a real one returns OTP_INVALID, and that difference is
// exactly the oracle this whole path exists to close.
func (s *RegisterService) Register(ctx context.Context, request Registration) (Issued, error) {
	email := normaliseEmail(request.Email)

	if err := s.policy.Validate(ctx, request.Password, email); err != nil {
		return Issued{}, err
	}

	existing, found, err := s.accounts.FindByEmail(ctx, email)
	if err != nil {
		return Issued{}, err
	}

	switch {
	case found && existing.Verified:
		return s.registerVerified(ctx, existing, email, request)
	case found:
		return s.registerUnverified(ctx, existing, email, request)
	default:
		return s.registerNew(ctx, email, request)
	}
}

// registerNew is the ordinary path.
//
// The account is created first, in the `user` module's own transaction, and
// everything auth owns follows in a second one. That is not the single
// transaction the plan's sequence diagram draws, and the reason is rule L4: a
// transaction may not span two modules, and `user.Registrar` neither takes one
// nor should. What the second transaction still guarantees is the part that
// matters — the credential, the challenge and the outbox row commit together,
// so a failure anywhere in it sends no code.
//
// The cost is an account row with no credential if the second transaction
// fails. It is unverified, so it is swept after seven days, and a retry of the
// same registration takes the unverified path and completes it.
func (s *RegisterService) registerNew(ctx context.Context, email string, request Registration) (Issued, error) {
	userID, err := s.accounts.CreateAccount(ctx, NewAccount{
		Email:       email,
		DisplayName: request.DisplayName,
		Locale:      request.Locale,
		Timezone:    request.Timezone,
	})
	if err != nil {
		// Two registrations racing on one address: the loser sees the unique
		// constraint. Falling through to the unverified path makes the race
		// indistinguishable from arriving second, which is what it is.
		if apperr.Is(err, apperr.Conflict) {
			return s.registerRaceLoser(ctx, email, request)
		}
		return Issued{}, err
	}
	return s.issueVerification(ctx, userID, email, request, credentialCreate)
}

// registerUnverified handles an address that was claimed but never proved.
//
// The submitted password **replaces** the stored one. Reissuing the pending
// challenge and leaving the old credential in place would be an account
// takeover: somebody registers an address they do not own, its real owner
// registers it too, receives the code, verifies — and ends up with an account
// whose password the first party chose.
//
// Overwriting is safe in the other direction because an unverified account is
// worth nothing: whoever can do this could equally have registered the address
// first.
func (s *RegisterService) registerUnverified(
	ctx context.Context, existing Account, email string, request Registration,
) (Issued, error) {
	return s.issueVerification(ctx, existing.ID, email, request, credentialReplace)
}

// registerRaceLoser is registerUnverified for an account that appeared between
// the lookup and the insert.
func (s *RegisterService) registerRaceLoser(
	ctx context.Context, email string, request Registration,
) (Issued, error) {
	existing, found, err := s.accounts.FindByEmail(ctx, email)
	if err != nil {
		return Issued{}, err
	}
	if !found {
		// The conflicting row is gone again. Nothing sensible is left to do but
		// report the conflict the database saw.
		return Issued{}, domain.ErrEmailAlreadyRegistered
	}
	if existing.Verified {
		return s.registerVerified(ctx, existing, email, request)
	}
	return s.registerUnverified(ctx, existing, email, request)
}

// registerVerified is the enumeration-safe path.
//
// It issues a genuine challenge — so the verify and resend operations behave
// identically to a real registration — and writes a warning to the address
// instead of a code. Nobody can complete this challenge, because nobody is ever
// told its code.
func (s *RegisterService) registerVerified(
	ctx context.Context, existing Account, email string, _ Registration,
) (Issued, error) {
	var issued Issued
	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var issueErr error
		issued, issueErr = s.challenges.IssueIn(ctx, tx, IssueRequest{
			Purpose: domain.PurposeVerifyEmail, Subject: email, UserID: &existing.ID,
		})
		if issueErr != nil {
			return issueErr
		}
		_, writeErr := s.events.Write(ctx, tx, contract.Aggregate, contract.EventRegistrationAttempted,
			contract.RegistrationAttempted{
				UserID:     existing.ID,
				Email:      email,
				Locale:     defaultLocale(""),
				OccurredAt: s.clock.Now(),
			})
		return writeErr
	})
	if err != nil {
		return Issued{}, err
	}

	slog.InfoContext(ctx, "registration attempted against an already verified address",
		"module", "auth", "op", "Register", "user_id", existing.ID.String())

	// The code is dropped on the floor deliberately: it exists so the challenge
	// is real, and it is never delivered anywhere.
	issued.Code = domain.Code{}
	return issued, nil
}

// credentialWrite says whether the credential is being created or replaced.
type credentialWrite int

const (
	credentialCreate credentialWrite = iota
	credentialReplace
)

// issueVerification is the transaction auth owns: the credential, the challenge
// and the outbox row that delivers the code, committed together.
func (s *RegisterService) issueVerification(
	ctx context.Context, userID uuid.UUID, email string, request Registration, write credentialWrite,
) (Issued, error) {
	passwordHash, err := s.hasher.Hash(request.Password)
	if err != nil {
		return Issued{}, err
	}

	var issued Issued
	err = dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		credentials := s.credentials.WithTx(tx)
		if writeErr := s.writeCredential(ctx, credentials, userID, passwordHash, write); writeErr != nil {
			return writeErr
		}

		var issueErr error
		issued, issueErr = s.challenges.IssueIn(ctx, tx, IssueRequest{
			Purpose: domain.PurposeVerifyEmail, Subject: email, UserID: &userID,
		})
		if issueErr != nil {
			return issueErr
		}

		_, writeErr := s.events.Write(ctx, tx, contract.Aggregate, contract.EventVerificationRequested,
			contract.VerificationRequested{
				ChallengeID: issued.Challenge.ID,
				UserID:      userID,
				Email:       email,
				DisplayName: request.DisplayName,
				Locale:      defaultLocale(request.Locale),
				Code:        issued.Code.Reveal(),
				ExpiresAt:   issued.Challenge.ExpiresAt,
				OccurredAt:  s.clock.Now(),
			})
		return writeErr
	})
	if err != nil {
		return Issued{}, err
	}
	return issued, nil
}

func (s *RegisterService) writeCredential(
	ctx context.Context, credentials Credentials, userID uuid.UUID, passwordHash string, write credentialWrite,
) error {
	if write == credentialReplace {
		_, err := credentials.ReplaceHash(ctx, userID, passwordHash)
		if err == nil || !errors.Is(err, domain.ErrCredentialNotFound) {
			return err
		}
		// An account created but never given a credential — the orphan
		// registerNew's comment describes. Complete it rather than failing.
	}

	credentialID, err := s.ids(ctx)
	if err != nil {
		return err
	}
	_, err = credentials.Create(ctx, credentialID, userID, passwordHash)
	if apperr.Is(err, apperr.Conflict) {
		// Two retries racing. The row exists, which is what the caller wanted.
		_, err = credentials.ReplaceHash(ctx, userID, passwordHash)
	}
	return err
}

// Verification is what a successful verify returns.
type Verification struct {
	Purpose    domain.Purpose
	VerifiedAt time.Time
}

// VerifyEmail consumes a challenge and, for a verify_email challenge, marks the
// address proved.
//
// It returns no tokens. Signing the learner in here — so verification is the
// last step rather than the second to last — is P2.4, which has the token
// service; the OpenAPI schema says so, so a client written against this version
// is not surprised by the field appearing.
func (s *RegisterService) VerifyEmail(ctx context.Context, challengeID uuid.UUID, code string) (
	Verification, error,
) {
	consumed, err := s.challenges.Verify(ctx, challengeID, code)
	if err != nil {
		return Verification{}, err
	}

	if consumed.Purpose != domain.PurposeVerifyEmail {
		// Another purpose's challenge, consumed correctly but not ours to act
		// on. P2.7 and P2.10 add their own handling; until then, saying so is
		// better than silently doing nothing.
		return Verification{}, fmt.Errorf("verify challenge: purpose %q has no handler yet", consumed.Purpose)
	}

	// The account comes from the challenge row. It cannot come from the subject:
	// that is stored as a keyed HMAC, which is irreversible by design and so
	// identifies nothing on its own.
	//
	// A verify_email challenge with no account is not a state registration can
	// produce — every path through Register issues one against an account it
	// just found or created. Treating it as an error rather than a no-op means a
	// future path that forgets to set it fails loudly here rather than silently
	// verifying nobody.
	if consumed.UserID == nil {
		return Verification{}, fmt.Errorf("verify challenge %s: no account is associated with it", consumed.ID)
	}
	if err := s.accounts.MarkEmailVerified(ctx, *consumed.UserID); err != nil {
		return Verification{}, err
	}

	verifiedAt := s.clock.Now()
	if consumed.ConsumedAt != nil {
		verifiedAt = *consumed.ConsumedAt
	}
	return Verification{Purpose: consumed.Purpose, VerifiedAt: verifiedAt}, nil
}

// PurgeUnverified removes accounts that claimed an address and never proved it.
// It is the scheduled half of BR-AUTH-14's promise that a claim is not
// permanent.
func (s *RegisterService) PurgeUnverified(ctx context.Context) error {
	cutoff := s.clock.Now().Add(-UnverifiedAccountTTL)
	removed, err := s.accounts.PurgeUnverifiedBefore(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("purge unverified accounts: %w", err)
	}
	if removed > 0 {
		slog.InfoContext(ctx, "swept accounts that never completed verification",
			"module", "auth", "op", "PurgeUnverified", "removed", removed, "cutoff", cutoff)
	}
	return nil
}

// normaliseEmail lowercases and trims. The column is citext so the database
// matches case-insensitively regardless, but the subject HMAC is computed in Go
// and has no such guarantee — two spellings would otherwise be two subjects and
// the per-subject issuance cap would apply to neither.
func normaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func defaultLocale(locale string) string {
	if locale == "" {
		return "en"
	}
	return locale
}

// Resend delivers a new code for an existing challenge.
//
// The recipient is looked up rather than remembered: the challenge row stores
// only a keyed digest of the subject, so the address has to come from the
// account the challenge belongs to. That is what `user.Registrar.Recipient` is
// for, and it is why the challenge carries a user id at all.
//
// The new code and the row that delivers it commit together. Splitting them
// would let a failure invalidate the learner's old code while sending no new
// one, leaving a live challenge nobody can complete.
func (s *RegisterService) Resend(ctx context.Context, challengeID uuid.UUID) (Issued, error) {
	var issued Issued
	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var resendErr error
		issued, resendErr = s.challenges.ResendIn(ctx, tx, challengeID)
		if resendErr != nil {
			return resendErr
		}

		if issued.Challenge.UserID == nil {
			return fmt.Errorf("resend challenge %s: no account is associated with it", challengeID)
		}
		contact, contactErr := s.accounts.Recipient(ctx, *issued.Challenge.UserID)
		if contactErr != nil {
			return contactErr
		}

		_, writeErr := s.events.Write(ctx, tx, contract.Aggregate, contract.EventVerificationRequested,
			contract.VerificationRequested{
				ChallengeID: issued.Challenge.ID,
				UserID:      *issued.Challenge.UserID,
				Email:       contact.Email,
				DisplayName: contact.DisplayName,
				Locale:      defaultLocale(contact.Locale),
				Code:        issued.Code.Reveal(),
				ExpiresAt:   issued.Challenge.ExpiresAt,
				OccurredAt:  s.clock.Now(),
			})
		return writeErr
	})
	if err != nil {
		return Issued{}, err
	}
	return issued, nil
}

// Contact is an account's mailing details, mirroring user/contract.Contact.
type Contact struct {
	Email       string
	DisplayName string
	Locale      string
}
