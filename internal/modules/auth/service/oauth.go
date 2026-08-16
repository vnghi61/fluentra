package service

import (
	"context"
	"crypto/hmac"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/contract"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service/oauth/google"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// DefaultOAuthStateTTL mirrors OAUTH_STATE_TTL in `.env.example`. Ten minutes is
// long enough to read a consent screen and short enough that an abandoned flow
// stops being a usable credential quickly (BR-AUTH-17).
const DefaultOAuthStateTTL = 10 * time.Minute

// OAuthRepo is the persistence this service needs.
type OAuthRepo interface {
	CreateOAuthState(
		ctx context.Context, id uuid.UUID, state, provider, nonce string,
		verifierHash []byte, redirectTo *string, now, expiresAt time.Time,
	) (domain.OAuthState, error)
	ConsumeOAuthState(ctx context.Context, state, provider string, now time.Time) (domain.OAuthState, bool, error)
	CreateOAuthIdentity(
		ctx context.Context, id, userID uuid.UUID, provider, subject string, emailHash []byte, now time.Time,
	) (domain.OAuthIdentity, error)
	FindOAuthIdentityBySubject(ctx context.Context, provider, subject string) (domain.OAuthIdentity, bool, error)
	FindOAuthIdentityByUser(ctx context.Context, userID uuid.UUID, provider string) (domain.OAuthIdentity, bool, error)
	DeleteOAuthIdentity(ctx context.Context, userID uuid.UUID, provider string) (bool, error)
	CountSignInMethods(ctx context.Context, userID uuid.UUID) (int, error)
	DeleteExpiredOAuthStates(ctx context.Context, cutoff time.Time) (int, error)

	WithTx(tx pgx.Tx) OAuthRepo
}

// OAuthProvider is the external half of the flow: where to send the browser,
// how to redeem what comes back, and how to verify what that returns.
//
// It is an interface so the five linking branches can be exercised against a
// fake that asserts whatever a test needs Google to assert. Every one of those
// branches is a policy decision, and a policy that can only be tested by
// standing up a real provider is a policy nobody tests.
type OAuthProvider interface {
	AuthorizationURL(state, nonce, challenge string) string
	Exchange(ctx context.Context, code, verifier string) (string, error)
	Verify(ctx context.Context, idToken, nonce string) (google.Identity, error)
}

// OAuthDeps are the service's collaborators.
type OAuthDeps struct {
	Pool     dbx.Beginner
	Repo     OAuthRepo
	Provider OAuthProvider
	Accounts Accounts
	Sessions Sessions
	Events   EventWriter
	Keys     domain.Keyring
	Clock    clock.Clock
	NewID    IDGenerator

	// StateTTL is how long an authorization request stays completable
	// (OAUTH_STATE_TTL). Zero means DefaultOAuthStateTTL.
	StateTTL time.Duration

	// Entropy draws the state and the nonce. Nil means crypto/rand.
	Entropy io.Reader
}

// OAuthService runs Google sign-in, linking and unlinking.
type OAuthService struct {
	pool     dbx.Beginner
	repo     OAuthRepo
	provider OAuthProvider
	accounts Accounts
	sessions Sessions
	events   EventWriter
	keys     domain.Keyring
	clock    clock.Clock
	ids      IDGenerator
	stateTTL time.Duration
	entropy  io.Reader
}

// NewOAuthService creates the service.
func NewOAuthService(deps OAuthDeps) *OAuthService {
	stateTTL := deps.StateTTL
	if stateTTL <= 0 {
		stateTTL = DefaultOAuthStateTTL
	}
	return &OAuthService{
		pool: deps.Pool, repo: deps.Repo, provider: deps.Provider,
		accounts: deps.Accounts, sessions: deps.Sessions, events: deps.Events,
		keys: deps.Keys, clock: deps.Clock, ids: deps.NewID,
		stateTTL: stateTTL, entropy: deps.Entropy,
	}
}

// Started is what the client needs to begin: one URL, and nothing else.
type Started struct {
	AuthorizationURL string
}

// CallbackInput is what came back from Google, plus what the request itself
// tells us about the device.
type CallbackInput struct {
	Code  string
	State string

	// ClientIP and UserAgent are digested onto the session row, exactly as the
	// login path digests them, and are never stored in the clear.
	ClientIP  string
	UserAgent string
}

// LinkedIdentity is a link as its owner sees it.
//
// The email comes from what Google just asserted, not from the row: the row
// stores a keyed digest, and there is nothing to render an address from. That
// is also why this is returned from the linking call and not from a read.
type LinkedIdentity struct {
	Provider string
	Email    string
	LinkedAt time.Time
}

// Start records an authorization request and returns where to send the browser.
//
// Three values are generated here and none of them is returned to the client:
//
//   - the `state`, which comes back on the redirect and proves this server
//     started the flow;
//   - the `nonce`, which comes back inside the ID token and proves the token was
//     minted for this flow rather than obtained somewhere else;
//   - the PKCE pair, whose challenge goes to Google and whose verifier is
//     derived from the state at callback time (see domain.Keyring.PKCEFor).
//
// The state and the nonce are separate values because they are checked in two
// different places by two different mechanisms. One value could not do both
// jobs: the redirect and the ID token travel by different routes, and a value
// that appeared in both would be checked twice against the same evidence.
func (s *OAuthService) Start(ctx context.Context, redirectTo string) (Started, error) {
	now := s.clock.Now().UTC()

	state, err := domain.NewOAuthState(s.entropy)
	if err != nil {
		return Started{}, err
	}
	nonce, err := domain.NewOAuthState(s.entropy)
	if err != nil {
		return Started{}, err
	}
	stateID, err := s.ids(ctx)
	if err != nil {
		return Started{}, fmt.Errorf("generate oauth state id: %w", err)
	}

	pkce := s.keys.PKCEFor(state)

	if _, err := s.repo.CreateOAuthState(ctx, stateID, state, domain.ProviderGoogle, nonce,
		domain.HashPKCEVerifier(pkce.Verifier), safeRedirect(redirectTo), now, now.Add(s.stateTTL)); err != nil {
		return Started{}, err
	}

	return Started{AuthorizationURL: s.provider.AuthorizationURL(state, nonce, pkce.Challenge)}, nil
}

// safeRedirect keeps an open redirect out of the row.
//
// Storing the destination server-side stops it being rewritten in flight, which
// is what the schema comment says it is for — but it does not make an attacker's
// own value safe, because the attacker is the one who called `start`. Only a
// same-site path survives: anything absolute, protocol-relative (`//evil.test`)
// or empty is dropped, and the flow lands on the default destination instead.
func safeRedirect(redirectTo string) *string {
	trimmed := strings.TrimSpace(redirectTo)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return nil
	}
	return &trimmed
}

// Callback completes a sign-in.
//
// The order of this function is its security property, and it is the order
// BR-AUTH-18 requires: **everything is checked before anything is written.** The
// state is spent, the code is redeemed, the ID token is verified in full — and
// only then does any row appear. A token that fails any check leaves no account,
// no identity and no session, because at the moment it fails there is nothing to
// undo.
//
// The five branches themselves live in domain.DecideLink, which is a pure
// function precisely so the policy can be read without reading this. What
// happens here is the part that touches rows.
func (s *OAuthService) Callback(ctx context.Context, input CallbackInput) (SignedIn, error) {
	identity, _, err := s.redeem(ctx, input.Code, input.State)
	if err != nil {
		return SignedIn{}, err
	}

	userID, err := s.resolve(ctx, identity)
	if err != nil {
		return SignedIn{}, err
	}

	return s.sessions.Start(ctx, StartInput{
		UserID:    userID,
		ClientIP:  input.ClientIP,
		UserAgent: input.UserAgent,
	})
}

// redeem consumes the state, exchanges the code and verifies the ID token.
//
// It is shared by Callback and Link because both directions need exactly this
// and must not differ in it: a link that skipped the nonce, or accepted a
// replayed state, would be a way into an account that the callback closes.
func (s *OAuthService) redeem(ctx context.Context, code, state string) (google.Identity, domain.OAuthState, error) {
	now := s.clock.Now().UTC()

	// One guarded UPDATE, the same shape as the refresh claim: asking "is this
	// state still usable?" and using it are one statement, so two callbacks
	// arriving together cannot both pass. A read followed by a write would let a
	// replayed state through under exactly the concurrency an attacker replaying
	// one would produce.
	row, spendable, err := s.repo.ConsumeOAuthState(ctx, state, domain.ProviderGoogle, now)
	if err != nil {
		return google.Identity{}, domain.OAuthState{}, err
	}
	if !spendable {
		// Forged, already used, or expired. Which one is not reported and not
		// distinguished — those are the three shapes a CSRF attempt takes — but
		// all three are recorded, because a run of them is the signal.
		return google.Identity{}, domain.OAuthState{}, s.reportStateRefusal(ctx, now)
	}

	pkce := s.keys.PKCEFor(row.State)

	// The recomputed verifier is checked against the digest the row stored. It
	// cannot disagree in normal operation — both come from the same state and
	// the same key — which is exactly why the check is worth making: if it ever
	// fails, the server key has been rotated or the derivation has changed, and
	// the alternative to failing here is an opaque `invalid_grant` from Google
	// that looks like the learner's fault.
	if !hmac.Equal(domain.HashPKCEVerifier(pkce.Verifier), row.PKCEVerifierHash) {
		slog.ErrorContext(ctx, "the derived pkce verifier does not match the stored digest",
			"module", "auth", "op", "redeem", "state_id", row.ID.String())
		return google.Identity{}, domain.OAuthState{}, domain.ErrOAuthStateInvalid
	}

	idToken, err := s.provider.Exchange(ctx, code, pkce.Verifier)
	if err != nil {
		return google.Identity{}, domain.OAuthState{}, exchangeFailure(ctx, err)
	}

	// The nonce comes from the row, never from the request. A nonce the caller
	// supplied would be a nonce the caller could match.
	identity, err := s.provider.Verify(ctx, idToken, row.Nonce)
	if err != nil {
		slog.WarnContext(ctx, "google id token failed verification",
			"module", "auth", "op", "redeem", "error", err)
		return google.Identity{}, domain.OAuthState{}, domain.ErrOAuthTokenInvalid
	}
	return identity, row, nil
}

// exchangeFailure separates "Google said no" from "we could not ask".
//
// They are the same event to the code and completely different to the learner:
// one means start again, the other means this is us and it will pass.
func exchangeFailure(ctx context.Context, err error) error {
	if google.ErrUnavailable(err) {
		slog.ErrorContext(ctx, "google token endpoint unreachable",
			"module", "auth", "op", "redeem", "error", err)
		return domain.ErrOAuthProviderUnavailable
	}
	slog.WarnContext(ctx, "google rejected the authorization code",
		"module", "auth", "op", "redeem", "error", err)
	return domain.ErrOAuthTokenInvalid
}

// resolve turns a verified Google identity into the account to sign in, creating
// or linking one where the policy says to.
//
// Every refusal here happens before any write. The conflict branch in particular
// must leave nothing behind: the test that matters asserts `core.oauth_identities`
// is still empty afterwards, not merely that the call returned an error.
func (s *OAuthService) resolve(ctx context.Context, identity google.Identity) (uuid.UUID, error) {
	linked, identityKnown, err := s.repo.FindOAuthIdentityBySubject(ctx, domain.ProviderGoogle, identity.Subject)
	if err != nil {
		return uuid.Nil, err
	}

	email := normaliseEmail(identity.Email)

	// The local lookup happens for every branch, including the one that already
	// knows the account, because it is also how a suspended account is caught —
	// see checkUsable.
	local, localExists, err := s.accounts.FindByEmail(ctx, email)
	if err != nil {
		return uuid.Nil, err
	}

	switch domain.DecideLink(domain.LinkInput{
		ProviderEmailVerified: identity.EmailVerified,
		IdentityKnown:         identityKnown,
		LocalAccountExists:    localExists,
		LocalAccountVerified:  localExists && local.Verified,
	}) {
	case domain.LinkRefuseUnverifiedProvider:
		// Refused before the address is matched against anything. An address
		// Google will not vouch for is worth no more than one typed into a form
		// (BR-AUTH-15).
		return uuid.Nil, domain.ErrOAuthEmailUnverified

	case domain.LinkRefuseUnverified:
		// The account-takeover refusal (BR-AUTH-16). **No row is written.**
		// The learner completes an OTP on that address and comes back.
		slog.InfoContext(ctx, "refused a google sign-in matching an unverified local account",
			"module", "auth", "op", "resolve", "user_id", local.ID.String())
		return uuid.Nil, domain.ErrOAuthAccountConflict

	case domain.LinkSignIn:
		return linked.UserID, s.checkUsable(ctx, linked.UserID, local, localExists)

	case domain.LinkToVerified:
		if err := s.checkUsable(ctx, local.ID, local, localExists); err != nil {
			return uuid.Nil, err
		}
		if err := s.link(ctx, local.ID, identity); err != nil {
			return uuid.Nil, err
		}
		return local.ID, nil

	default:
		return s.createAccount(ctx, identity)
	}
}

// checkUsable refuses to sign in to an account an administrator has disabled.
//
// It is best-effort in one direction, and that is worth being explicit about.
// The account is resolved by the address Google asserted, so an account whose
// Google address has since changed cannot be matched and its status cannot be
// read here — `Accounts` has no lookup by id, and adding one is a change to the
// `user` contract rather than to this card. The window is narrow (a learner who
// changed their Google address after being suspended) and the fallback is the
// one BR-AUTH-09 already relies on: suspension revokes every existing session,
// and this path cannot be reached by anyone who is not the account's owner.
// Filed in internal/modules/auth/TODO.md.
func (s *OAuthService) checkUsable(ctx context.Context, userID uuid.UUID, local Account, localExists bool) error {
	if !localExists || local.ID != userID {
		return nil
	}
	if local.Status == accountSuspended {
		slog.InfoContext(ctx, "refused a google sign-in to a suspended account",
			"module", "auth", "op", "checkUsable", "user_id", userID.String())
		return domain.ErrAccountSuspended
	}
	return nil
}

// accountSuspended is the status string `user` stores. It is a constant here for
// the reason the field names in the outbox payloads are: the value is a contract
// between two modules and two spellings of it is one bug.
const accountSuspended = "suspended"

// link attaches a verified identity to an account.
func (s *OAuthService) link(ctx context.Context, userID uuid.UUID, identity google.Identity) error {
	identityID, err := s.ids(ctx)
	if err != nil {
		return fmt.Errorf("generate oauth identity id: %w", err)
	}
	// The address is stored as a keyed digest for the reason every other address
	// in this schema is: an unkeyed hash of an email is reversible with a
	// wordlist, which would make this table an address book.
	_, err = s.repo.CreateOAuthIdentity(ctx, identityID, userID, domain.ProviderGoogle,
		identity.Subject, s.keys.SubjectHash(normaliseEmail(identity.Email)), s.clock.Now().UTC())
	return err
}

// createAccount opens an account for a Google identity with no local
// counterpart, already verified (BR-AUTH-19).
//
// It is already verified because Google has performed exactly the check the OTP
// would have, and sending a code to an address the provider just proved would be
// asking the learner to demonstrate something already demonstrated.
//
// Three writes, and they are not one transaction. `user` creates the account in
// its own, because rule L4 forbids a transaction spanning two modules — the same
// split registerNew documents. The cost here is worse than registerNew's and is
// worth stating: if the account is created and the verification mark fails, the
// retry finds an unverified local account and takes the conflict branch, so the
// learner is refused for the seven days it takes the sweep to remove it. Closing
// that needs `user.Registrar` to be able to open an account already verified,
// which is a change to another module's contract. Filed in TODO.md.
func (s *OAuthService) createAccount(ctx context.Context, identity google.Identity) (uuid.UUID, error) {
	email := normaliseEmail(identity.Email)

	userID, err := s.accounts.CreateAccount(ctx, NewAccount{
		Email: email,
		// Google's `name`, which is what the learner already sees on their own
		// account there. An empty one is left to the `user` module's default
		// rather than invented here.
		DisplayName: identity.Name,
	})
	if err != nil {
		return uuid.Nil, err
	}
	if err := s.accounts.MarkEmailVerified(ctx, userID); err != nil {
		return uuid.Nil, err
	}
	if err := s.link(ctx, userID, identity); err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// reportStateRefusal records the refusal and returns it.
//
// The event is written even though nothing else is, and that is the point: a
// refused state leaves no other trace, so without this row a campaign of them is
// invisible. It is `medium` rather than `high` because a single one is ordinary
// — a consent screen left open past its ten minutes produces one — and what
// matters is the rate.
//
// The refusal is returned whether or not the event could be written. Failing the
// sign-in because the audit trail was unavailable would turn a logging outage
// into an authentication outage, and the caller is being refused either way.
func (s *OAuthService) reportStateRefusal(ctx context.Context, now time.Time) error {
	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		// There is no user and no session to name. A forged state identifies
		// nobody, which is what makes it forged — and putting anything from the
		// request in here would let whoever sent it choose what gets stored in a
		// table nobody can UPDATE.
		_, writeErr := s.events.Write(ctx, tx, contract.Aggregate, contract.EventSecurityEvent,
			contract.SecurityEvent{
				Kind:       contract.SecurityKindOAuthStateInvalid,
				Severity:   contract.SeverityMedium,
				OccurredAt: now,
			})
		return writeErr
	})
	if err != nil {
		slog.ErrorContext(ctx, "could not record an oauth state refusal",
			"module", "auth", "op", "reportStateRefusal", "error", err)
	}
	return domain.ErrOAuthStateInvalid
}

// Link attaches a Google identity to the account already signed in.
//
// It runs the same redemption as the callback — same state, same PKCE, same five
// ID token checks — and then applies the two rules that only exist in this
// direction:
//
//   - **the address must be the one this account holds.** Linking a Google
//     account the owner cannot receive mail at would attach a second way in that
//     the account's owner does not control, which is the same takeover the
//     callback's conflict branch refuses, arrived at from the other side.
//   - **one Google account is one Fluentra account.** Enforced here for a clear
//     refusal and by uq_oauth_identities_subject underneath, which is what makes
//     it true under concurrency rather than merely usually true.
func (s *OAuthService) Link(ctx context.Context, actor httpx.Actor, input CallbackInput) (LinkedIdentity, error) {
	identity, _, err := s.redeem(ctx, input.Code, input.State)
	if err != nil {
		return LinkedIdentity{}, err
	}

	// Ordered as the callback orders it: an address Google will not vouch for is
	// refused before it is compared against anything (BR-AUTH-15).
	if !identity.EmailVerified {
		return LinkedIdentity{}, domain.ErrOAuthEmailUnverified
	}

	existing, known, err := s.repo.FindOAuthIdentityBySubject(ctx, domain.ProviderGoogle, identity.Subject)
	if err != nil {
		return LinkedIdentity{}, err
	}
	if known && existing.UserID != actor.UserID {
		return LinkedIdentity{}, domain.ErrOAuthAlreadyLinked
	}

	contact, err := s.accounts.Recipient(ctx, actor.UserID)
	if err != nil {
		return LinkedIdentity{}, err
	}
	if normaliseEmail(contact.Email) != normaliseEmail(identity.Email) {
		return LinkedIdentity{}, domain.ErrOAuthEmailMismatch
	}

	if known {
		// Already linked to this same account. Answering with the existing link
		// rather than a conflict makes a repeated request idempotent, which is
		// what a client retrying a timed-out call will produce.
		return LinkedIdentity{
			Provider: existing.Provider, Email: identity.Email, LinkedAt: existing.LinkedAt,
		}, nil
	}

	if err := s.link(ctx, actor.UserID, identity); err != nil {
		return LinkedIdentity{}, err
	}
	return LinkedIdentity{
		Provider: domain.ProviderGoogle, Email: identity.Email, LinkedAt: s.clock.Now().UTC(),
	}, nil
}

// Unlink removes the link, unless it is the last way in (BR-AUTH-20).
//
// The count and the delete are one transaction because they are one decision.
// Read-then-delete would let two concurrent unlinks — or an unlink racing
// whatever future card removes a password — each see two methods and each remove
// one, leaving an account nobody can sign into. That is not a state any later
// operation can repair: `forgot-password` cannot help an account with no
// password, and the identity it would need is gone.
func (s *OAuthService) Unlink(ctx context.Context, actor httpx.Actor) error {
	return dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)

		_, linked, err := repo.FindOAuthIdentityByUser(ctx, actor.UserID, domain.ProviderGoogle)
		if err != nil {
			return err
		}
		if !linked {
			// Nothing to remove. 204 rather than a refusal, because the caller's
			// goal — not being linked — already holds, and this is the answer
			// that makes a retry safe.
			return nil
		}

		methods, err := repo.CountSignInMethods(ctx, actor.UserID)
		if err != nil {
			return err
		}
		if methods <= 1 {
			// The identity we are about to remove is the only way in. An account
			// created through Google has no password, so this would lock the
			// learner out with no recovery path at all.
			return domain.ErrLastSignInMethod
		}

		if _, err := repo.DeleteOAuthIdentity(ctx, actor.UserID, domain.ProviderGoogle); err != nil {
			return err
		}
		return nil
	})
}

// FindIdentityByUser returns the linked identity for provider if any.
func (s *OAuthService) FindIdentityByUser(
	ctx context.Context, userID uuid.UUID, provider string,
) (domain.OAuthIdentity, bool, error) {
	return s.repo.FindOAuthIdentityByUser(ctx, userID, provider)
}

// SweepOAuthStates removes authorization requests nobody came back for.
//
// Most rows in the table are abandoned consent screens, and an expired state is
// already refused, so this reclaims space rather than enforcing anything.
func (s *OAuthService) SweepOAuthStates(ctx context.Context) error {
	// Deleted well after expiry rather than at it, so a learner completing a
	// flow at the edge of the window meets "expired" — which is refused and
	// recorded — instead of "no such state", which is the same refusal with less
	// information behind it.
	cutoff := s.clock.Now().UTC().Add(-oauthStateSweepGrace)
	removed, err := s.repo.DeleteExpiredOAuthStates(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("sweep oauth states: %w", err)
	}
	if removed > 0 {
		slog.InfoContext(ctx, "swept abandoned oauth authorization requests",
			"module", "auth", "op", "SweepOAuthStates", "removed", removed)
	}
	return nil
}

// oauthStateSweepGrace is how long an expired state is kept before it is
// deleted.
const oauthStateSweepGrace = time.Hour

// Compile-time proof that the provider this module ships satisfies what the
// service asks for. Without it the mismatch surfaces in module.go as a wiring
// error at the composition root, which is further from the change that caused it.
var _ OAuthProvider = (*google.Provider)(nil)
