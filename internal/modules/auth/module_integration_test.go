//go:build integration

// Package auth_test exercises refresh rotation against a real PostgreSQL.
//
// It is an integration suite and not a unit one because the property under test
// is a property of the database. Reuse detection and the concurrent-refresh race
// are decided by what `UPDATE ... WHERE used_at IS NULL` does under two
// transactions, and a fake repository would answer whatever its author expected
// rather than what PostgreSQL does.
package auth_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/auth/contract"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/repository"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/id"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// moduleDatabase is this package's own, for the reason every suite in the
// repository has one: sharing TEST_DATABASE_URL means one package's truncate is
// another package's missing row, and the failure lands in a test that never
// touched the data.
const moduleDatabase = "fluentra_auth_module_test"

// signingKey is long enough for NewTokenService to accept it and is not a
// secret in any deployment.
const signingKey = "integration-test-jwt-signing-key-32b" // gitleaks:allow

// otpKey keys the IP digest. Any 32 bytes will do; the value is never asserted.
const otpKey = "integration-test-hmac-key-32-bytes--" // gitleaks:allow

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, moduleDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}

	created, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}
	pool = created

	code := m.Run()

	pool.Close()
	dropDatabase()
	os.Exit(code)
}

func createDatabase(base, name string) (string, func(), error) {
	maintenance, err := replaceDatabase(base, "postgres")
	if err != nil {
		return "", nil, err
	}
	admin, err := sql.Open("pgx", maintenance)
	if err != nil {
		return "", nil, fmt.Errorf("open maintenance database: %w", err)
	}
	defer func() { _ = admin.Close() }()

	ctx := context.Background()
	drop := fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", name)
	if _, err := admin.ExecContext(ctx, drop); err != nil {
		return "", nil, fmt.Errorf("drop stale %s: %w", name, err)
	}
	if _, err := admin.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %q", name)); err != nil {
		return "", nil, fmt.Errorf("create %s: %w", name, err)
	}

	dsn, err := replaceDatabase(base, name)
	if err != nil {
		return "", nil, err
	}
	return dsn, func() {
		cleanup, err := sql.Open("pgx", maintenance)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.ExecContext(context.Background(), drop)
	}, nil
}

func migrateUp(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	sources, err := migrations.Flattened()
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("flatten migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("create goose provider: %w", err)
	}
	defer func() { _ = provider.Close() }()
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func replaceDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

// refreshAdapter narrows *repository.Repository to service.RefreshRepo. It is
// the same four lines module.go needs, and for the same reason: Go has no
// covariant return types, so WithTx returning *Repository cannot satisfy an
// interface method returning service.RefreshRepo.
type refreshAdapter struct {
	*repository.Repository
}

func (a refreshAdapter) WithTx(tx pgx.Tx) service.RefreshRepo {
	return refreshAdapter{Repository: a.Repository.WithTx(tx)}
}

// eventWriter adapts shared/outbox to service.EventWriter, as module.go does.
type eventWriter struct {
	*outbox.Writer
}

func (w eventWriter) Write(
	ctx context.Context, tx service.OutboxTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	return w.Writer.Write(ctx, tx, aggregate, event, payload)
}

type refreshHarness struct {
	service *service.RefreshService
	clock   *clock.Fake
	userID  uuid.UUID
}

// refreshTTL is short enough to step over with the fake clock and long enough
// that nothing else in a test trips it.
const refreshTTL = 30 * 24 * time.Hour

var harnessNow = time.Date(2026, time.August, 11, 9, 0, 0, 0, time.UTC)

func newRefreshHarness(t *testing.T, email string) *refreshHarness {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	fake := clock.NewFake(harnessNow)

	tokens, err := service.NewTokenService(service.TokenDeps{
		Config: service.TokenConfig{
			SigningKey: []byte(signingKey),
			Issuer:     "fluentra-test",
			Audience:   "fluentra-api-test",
			AccessTTL:  service.DefaultAccessTTL,
		},
		Clock: fake,
		NewID: id.NewUUIDv7,
	})
	if err != nil {
		t.Fatalf("NewTokenService: %v", err)
	}

	keys, err := domain.NewKeyring([]byte(otpKey))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}

	refresh := service.NewRefreshService(service.RefreshDeps{
		Pool:   pool,
		Repo:   refreshAdapter{Repository: repository.New(pool)},
		Tokens: tokens,
		Events: eventWriter{Writer: outbox.NewWriter()},
		Keys:   keys,
		Clock:  fake,
		NewID:  id.NewUUIDv7,
		TTL:    refreshTTL,
	})

	return &refreshHarness{service: refresh, clock: fake, userID: seedUser(t, email)}
}

func (h *refreshHarness) start(t *testing.T) service.SignedIn {
	t.Helper()
	signedIn, err := h.service.Start(context.Background(), service.StartInput{
		UserID:    h.userID,
		ClientIP:  "203.0.113.7",
		UserAgent: "integration-suite",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return signedIn
}

// seedUser writes the core.users row the session foreign-keys to, with plain
// SQL. Importing the user module's repository from here would be the boundary
// crossing rule L1 forbids, test or not.
func seedUser(t *testing.T, email string) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	userID, err := id.NewUUIDv7(ctx)
	if err != nil {
		t.Fatalf("generate user id: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.users (id, email, status) VALUES ($1, $2, 'active')`, userID, email); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
	})
	return userID
}

// ---------------------------------------------------------------- assertions

func assertCode(t *testing.T, err error, wanted string) {
	t.Helper()

	if err == nil {
		t.Fatalf("no error, want %s", wanted)
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want an *apperr.Error with code %s", err, wanted)
	}
	if appErr.Code != wanted {
		t.Fatalf("code = %q, want %q", appErr.Code, wanted)
	}
}

// liveTokensInFamily counts the rows a stolen token could still be exchanged
// for: neither spent nor revoked.
func liveTokensInFamily(t *testing.T, sessionID uuid.UUID) int {
	t.Helper()

	var live int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM core.refresh_tokens
		WHERE session_id = $1 AND used_at IS NULL AND revoked_at IS NULL`, sessionID).Scan(&live)
	if err != nil {
		t.Fatalf("count live tokens: %v", err)
	}
	return live
}

func revokedTokensInFamily(t *testing.T, sessionID uuid.UUID) int {
	t.Helper()

	var revoked int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM core.refresh_tokens
		WHERE session_id = $1 AND revoked_at IS NOT NULL`, sessionID).Scan(&revoked)
	if err != nil {
		t.Fatalf("count revoked tokens: %v", err)
	}
	return revoked
}

func sessionIsRevoked(t *testing.T, sessionID uuid.UUID) bool {
	t.Helper()

	var revokedAt *time.Time
	err := pool.QueryRow(context.Background(),
		`SELECT revoked_at FROM core.sessions WHERE id = $1`, sessionID).Scan(&revokedAt)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	return revokedAt != nil
}

// securityEvents reads the outbox rows this module wrote for a user. The
// aggregate prefix is stripped by the writer, so the stored event name is bare.
func securityEvents(t *testing.T, userID uuid.UUID) []contract.SecurityEvent {
	t.Helper()

	rows, err := pool.Query(context.Background(), `
		SELECT payload FROM ops.outbox_events
		WHERE aggregate = $1 AND event = $2
		ORDER BY event_id`, contract.Aggregate, outbox.BareEventName(contract.Aggregate, contract.EventSecurityEvent))
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	defer rows.Close()

	var found []contract.SecurityEvent
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			t.Fatalf("scan outbox payload: %v", err)
		}
		var event contract.SecurityEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("decode security event: %v", err)
		}
		if event.UserID == userID {
			found = append(found, event)
		}
	}
	return found
}

// --------------------------------------------------------------------- tests

// TestPresentingASpentRefreshTokenRevokesTheWholeFamilyAndTheSession is the
// single most important test in WP2, and it is written to fail against a
// read-then-write implementation as well as against no implementation at all.
//
// The assertion that carries the weight is the last one: the *legitimate*
// token, issued moments earlier and never presented by anybody, must stop
// working. That is the difference between detecting the theft and merely
// declining one request. A server that answers 401 to the replay and then
// happily keeps rotating the real client's token has told the attacker to try
// again and told nobody else anything.
func TestPresentingASpentRefreshTokenRevokesTheWholeFamilyAndTheSession(t *testing.T) {
	h := newRefreshHarness(t, "reuse@fluentra.test")
	ctx := context.Background()

	first := h.start(t)
	stolen := first.RefreshToken.Reveal()

	// The legitimate rotation. After it, `stolen` is spent and `second` is live.
	second, err := h.service.Rotate(ctx, stolen)
	if err != nil {
		t.Fatalf("the first rotation failed: %v", err)
	}
	if second.RefreshToken.Reveal() == stolen {
		t.Fatal("rotation returned the same token, so nothing rotated")
	}

	// The replay. Either the thief is a step behind the real client, or the real
	// client is a step behind the thief; the server cannot tell which, and does
	// not try.
	_, err = h.service.Rotate(ctx, stolen)
	assertCode(t, err, "SESSION_REVOKED")

	sessionID := first.Session.SessionID
	if live := liveTokensInFamily(t, sessionID); live != 0 {
		t.Errorf("%d refresh tokens in the family are still exchangeable, want 0", live)
	}
	if revoked := revokedTokensInFamily(t, sessionID); revoked == 0 {
		t.Error("no row in the family was marked revoked, so the revocation is invisible to an audit")
	}
	if !sessionIsRevoked(t, sessionID) {
		t.Error("the session survived the reuse, so its access tokens keep working for a full TTL")
	}

	events := securityEvents(t, h.userID)
	if len(events) != 1 {
		t.Fatalf("%d security events raised, want exactly 1", len(events))
	}
	if events[0].Kind != contract.SecurityKindRefreshReuse {
		t.Errorf("kind = %q, want %q", events[0].Kind, contract.SecurityKindRefreshReuse)
	}
	if events[0].Severity != contract.SeverityHigh {
		t.Errorf("severity = %q, want %q", events[0].Severity, contract.SeverityHigh)
	}

	// The one that matters. The honest client's live token is dead too.
	_, err = h.service.Rotate(ctx, second.RefreshToken.Reveal())
	assertCode(t, err, "SESSION_REVOKED")
}

// TestTwoConcurrentRefreshesWithOneTokenProduceExactlyOneWinner is the test a
// read-then-write implementation fails and every sequential test passes.
//
// Both callers hold the same valid token, as two tabs waking from sleep
// genuinely do. If the claim is a SELECT followed by an UPDATE, both read
// `used_at IS NULL`, both write, and one refresh token becomes two live ones --
// which is precisely the state reuse detection is supposed to make impossible.
// Exactly one caller must be served.
func TestTwoConcurrentRefreshesWithOneTokenProduceExactlyOneWinner(t *testing.T) {
	h := newRefreshHarness(t, "concurrent@fluentra.test")

	const callers = 8
	first := h.start(t)
	token := first.RefreshToken.Reveal()

	var (
		start   sync.WaitGroup
		done    sync.WaitGroup
		mu      sync.Mutex
		winners []service.SignedIn
		losers  []error
	)
	start.Add(1)
	done.Add(callers)

	for range callers {
		go func() {
			defer done.Done()
			start.Wait()

			signedIn, err := h.service.Rotate(context.Background(), token)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				losers = append(losers, err)
				return
			}
			winners = append(winners, signedIn)
		}()
	}

	start.Done()
	done.Wait()

	if len(winners) != 1 {
		t.Fatalf("%d of %d concurrent refreshes succeeded, want exactly 1", len(winners), callers)
	}
	if len(losers) != callers-1 {
		t.Fatalf("%d callers failed, want %d", len(losers), callers-1)
	}
	for _, err := range losers {
		assertCode(t, err, "SESSION_REVOKED")
	}

	// Seven losers, one theft. Every loser reaches the reuse path, but only the
	// one that actually revoked the family files a report — otherwise a replay
	// storm arrives on the dashboard as seven incidents and the person triaging
	// it counts thefts instead of reading them.
	if events := securityEvents(t, h.userID); len(events) != 1 {
		t.Errorf("%d security events raised for one incident, want exactly 1", len(events))
	}
}

// TestARefreshTokenOneMillisecondPastExpiryIsRefused pins the boundary, and
// pins that an idle timeout is not a theft.
//
// Expiry must not revoke the family: a learner who closed the laptop for a
// month is not an attacker, and raising a security event for every one of them
// would bury the events that matter.
func TestARefreshTokenOneMillisecondPastExpiryIsRefused(t *testing.T) {
	h := newRefreshHarness(t, "expired@fluentra.test")
	ctx := context.Background()

	signedIn := h.start(t)

	h.clock.Advance(refreshTTL - time.Millisecond)
	if _, err := h.service.Rotate(ctx, signedIn.RefreshToken.Reveal()); err != nil {
		t.Fatalf("a token one millisecond inside its window was refused: %v", err)
	}

	next := h.start(t)
	h.clock.Advance(refreshTTL + time.Millisecond)

	_, err := h.service.Rotate(ctx, next.RefreshToken.Reveal())
	assertCode(t, err, "TOKEN_INVALID")

	if sessionIsRevoked(t, next.Session.SessionID) {
		t.Error("an idle timeout revoked the session, which makes every dormant learner look like a theft")
	}
	if len(securityEvents(t, h.userID)) != 0 {
		t.Error("an expired token raised a security event")
	}
}

// TestAnUnknownRefreshTokenRevokesNothing is the other half of the same
// concern: a token that was never ours is a 401 and nothing more. Revoking on
// it would hand any caller a denial-of-service against a user they can name --
// except they cannot name one, which is exactly why there is nothing to revoke.
func TestAnUnknownRefreshTokenRevokesNothing(t *testing.T) {
	h := newRefreshHarness(t, "unknown@fluentra.test")
	ctx := context.Background()

	live := h.start(t)

	_, err := h.service.Rotate(ctx, "a-token-this-deployment-never-issued")
	assertCode(t, err, "TOKEN_INVALID")

	if sessionIsRevoked(t, live.Session.SessionID) {
		t.Error("an unknown token revoked a live session")
	}
	if _, err := h.service.Rotate(ctx, live.RefreshToken.Reveal()); err != nil {
		t.Errorf("the live session stopped working after an unrelated bad token: %v", err)
	}
}

// TestRotationKeepsTheSessionAndMovesItsLastSeen covers the ordinary path the
// three tests above are all deviations from: the same session, a new token, and
// evidence the learner was here.
func TestRotationKeepsTheSessionAndMovesItsLastSeen(t *testing.T) {
	h := newRefreshHarness(t, "rotation@fluentra.test")
	ctx := context.Background()

	first := h.start(t)
	before := sessionLastSeen(t, first.Session.SessionID)

	h.clock.Advance(time.Hour)
	second, err := h.service.Rotate(ctx, first.RefreshToken.Reveal())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	if second.Session.SessionID != first.Session.SessionID {
		t.Errorf("session id changed on rotation: %s -> %s", first.Session.SessionID, second.Session.SessionID)
	}
	if second.Session.UserID != h.userID {
		t.Errorf("user id = %s, want %s", second.Session.UserID, h.userID)
	}
	if !second.RefreshExpiresAt.After(first.RefreshExpiresAt) {
		t.Error("the idle window did not move forward, so an active learner is still signed out on schedule")
	}
	if after := sessionLastSeen(t, first.Session.SessionID); !after.After(before) {
		t.Errorf("last_seen_at did not move: %s -> %s", before, after)
	}

	// One rotation leaves exactly one exchangeable token behind.
	if live := liveTokensInFamily(t, first.Session.SessionID); live != 1 {
		t.Errorf("%d exchangeable tokens after one rotation, want 1", live)
	}
}

func sessionLastSeen(t *testing.T, sessionID uuid.UUID) time.Time {
	t.Helper()

	var lastSeen time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT last_seen_at FROM core.sessions WHERE id = $1`, sessionID).Scan(&lastSeen); err != nil {
		t.Fatalf("read last_seen_at: %v", err)
	}
	return lastSeen
}

// TestTheRefreshTokenIsNotStoredAndIsNotPrintedByAccident is BR-AUTH-10's
// spirit applied to the other credential this module hands out. The database
// must hold a digest and not the token, and the struct that carries it must not
// render it into a log line on the way to the transport layer.
func TestTheRefreshTokenIsNotStoredAndIsNotPrintedByAccident(t *testing.T) {
	h := newRefreshHarness(t, "redaction@fluentra.test")

	signedIn := h.start(t)
	raw := signedIn.RefreshToken.Reveal()

	var stored int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM core.refresh_tokens WHERE encode(token_hash, 'escape') = $1`,
		raw).Scan(&stored); err != nil {
		t.Fatalf("search for the plaintext token: %v", err)
	}
	if stored != 0 {
		t.Error("the refresh token is recoverable from the table it is stored in")
	}

	if rendered := fmt.Sprintf("%+v %v", signedIn, signedIn); strings.Contains(rendered, raw) {
		t.Errorf("formatting a SignedIn printed the refresh token: %s", rendered)
	}
}
