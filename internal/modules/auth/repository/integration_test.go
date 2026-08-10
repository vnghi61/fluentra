//go:build integration

package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/repository"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/id"
)

// repositoryDatabase is this package's own database. Every integration suite in
// the repository creates one: sharing TEST_DATABASE_URL means one package's
// truncate is another package's missing row, and the failures land in tests that
// never touched the data.
//
// It is not called repositoryDatabase, which would read better, because gosec's
// G101 heuristic flags any constant whose name contains "credential" as a
// hardcoded secret. Naming around the false positive is cheaper than a nolint
// that a reader then has to evaluate.
const repositoryDatabase = "fluentra_auth_repository_test"

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, repositoryDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", repositoryDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", repositoryDatabase, err)
		os.Exit(1)
	}

	created, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", repositoryDatabase, err)
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

func newRepository(t *testing.T) (*repository.Repository, context.Context) {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return repository.New(pool), context.Background()
}

func newID(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()
	generated, err := id.NewUUIDv7(ctx)
	if err != nil {
		t.Fatalf("generate id: %v", err)
	}
	return generated
}

// seedUser writes the core.users row the credential foreign-keys to. The auth
// module owns no user table, so the account is created with plain SQL rather
// than through the user module's repository — importing it would be exactly the
// boundary crossing rule L1 forbids, even from a test.
func seedUser(ctx context.Context, t *testing.T, email string) uuid.UUID {
	t.Helper()

	userID := newID(ctx, t)
	_, err := pool.Exec(ctx, `INSERT INTO core.users (id, email, status) VALUES ($1, $2, 'active')`, userID, email)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { deleteUser(userID) }) //nolint:contextcheck // deleteUser explains it
	return userID
}

// deleteUser removes a seeded account once the test body has returned, taking
// the credential with it through the cascade. It takes no context on purpose:
// by cleanup time the test's context can already be cancelled, and the row
// would survive into the next test.
func deleteUser(userID uuid.UUID) {
	_, _ = pool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
}

// cheapParams keep the suite fast. These tests are about the row, not about the
// derivation, and the production parameters are asserted in the domain suite.
var cheapParams = domain.HashParams{
	MemoryKiB: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
}

func mustHash(t *testing.T, hasher domain.Hasher, password string) string {
	t.Helper()
	encoded, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	return encoded
}

func TestRepository_CredentialRoundTripsAndStillVerifies(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedUser(ctx, t, "roundtrip@fluentra.test")
	hasher := domain.NewHasher(cheapParams)

	if _, err := repo.Create(ctx, newID(ctx, t), userID, mustHash(t, hasher, breachTestPassword)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	stored, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if stored.UserID != userID {
		t.Errorf("user id = %s, want %s", stored.UserID, userID)
	}

	result, err := stored.Verify(hasher, breachTestPassword)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Matches {
		t.Error("a hash that survived a round trip through Postgres no longer verifies")
	}
}

// TestRepository_TheHashIsNotPrintedByAccident is the secret.Redacted wrapper
// doing its job on a value that came out of the database. A %+v on a credential
// reaches a log line or a test failure message far more often than anyone
// intends.
func TestRepository_TheHashIsNotPrintedByAccident(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedUser(ctx, t, "redacted@fluentra.test")
	encoded := mustHash(t, domain.NewHasher(cheapParams), breachTestPassword)

	stored, err := repo.Create(ctx, newID(ctx, t), userID, encoded)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if rendered := fmt.Sprintf("%+v", stored); strings.Contains(rendered, encoded) {
		t.Errorf("formatting a credential printed the hash: %s", rendered)
	}
}

func TestRepository_UnknownAccountIsANotFoundError(t *testing.T) {
	repo, ctx := newRepository(t)

	_, err := repo.GetByUserID(ctx, newID(ctx, t))
	if !apperr.Is(err, apperr.NotFound) {
		t.Fatalf("error = %v, want a not-found error", err)
	}
}

func TestRepository_ASecondCredentialForOneAccountIsAConflict(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedUser(ctx, t, "conflict@fluentra.test")
	hasher := domain.NewHasher(cheapParams)

	if _, err := repo.Create(ctx, newID(ctx, t), userID, mustHash(t, hasher, breachTestPassword)); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err := repo.Create(ctx, newID(ctx, t), userID, mustHash(t, hasher, "another passphrase entirely"))
	if !apperr.Is(err, apperr.Conflict) {
		t.Fatalf("error = %v, want a conflict from uq_credentials_user", err)
	}
}

// TestRepository_RehashOnLoginReplacesTheStoredParameters is the acceptance
// criterion "a hash created with old parameters is transparently upgraded on
// successful login", end to end: an old hash goes in, the verifier asks for an
// upgrade, the upgrade is written, and the generated column agrees.
func TestRepository_RehashOnLoginReplacesTheStoredParameters(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedUser(ctx, t, "rehash@fluentra.test")

	old := domain.NewHasher(cheapParams)
	raised := cheapParams
	raised.Iterations = cheapParams.Iterations + 1
	current := domain.NewHasher(raised)

	before, err := repo.Create(ctx, newID(ctx, t), userID, mustHash(t, old, breachTestPassword))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if before.AlgoParams != "m=8,t=1,p=1" {
		t.Fatalf("algo_params = %q, want the parameters the hash was made with", before.AlgoParams)
	}

	stored, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	result, err := stored.Verify(current, breachTestPassword)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Matches || !result.NeedsRehash {
		t.Fatalf("verification = %+v, want a match that asks for a rehash", result)
	}

	after, err := repo.ReplaceHash(ctx, userID, mustHash(t, current, breachTestPassword))
	if err != nil {
		t.Fatalf("ReplaceHash: %v", err)
	}
	if after.AlgoParams != "m=8,t=2,p=1" {
		t.Errorf("algo_params = %q, want the raised parameters", after.AlgoParams)
	}

	// The upgrade is only transparent if the learner's password still works.
	upgraded, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID after rehash: %v", err)
	}
	result, err = upgraded.Verify(current, breachTestPassword)
	if err != nil {
		t.Fatalf("Verify after rehash: %v", err)
	}
	if !result.Matches {
		t.Error("the password stopped working after the rehash")
	}
	if result.NeedsRehash {
		t.Error("the rehashed credential still asks to be rehashed")
	}
}

func TestRepository_ReplacingTheHashOfAnUnknownAccountIsANotFoundError(t *testing.T) {
	repo, ctx := newRepository(t)

	_, err := repo.ReplaceHash(ctx, newID(ctx, t), mustHash(t, domain.NewHasher(cheapParams), breachTestPassword))
	if !apperr.Is(err, apperr.NotFound) {
		t.Fatalf("error = %v, want a not-found error", err)
	}
}

// TestSchema_RejectsAnythingThatIsNotAnArgon2idHash is the last line of defence
// against the worst bug this table could hold. It is asserted against the
// database rather than against the repository, because the constraint has to
// hold for a migration, a fixture and a psql session too.
func TestSchema_RejectsAnythingThatIsNotAnArgon2idHash(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedUser(ctx, t, "constraint@fluentra.test")

	rejected := map[string]string{
		"a plaintext password":  breachTestPassword,
		"an empty string":       "",
		"a bcrypt hash":         "$2y$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		"argon2i, not argon2id": "$argon2i$v=19$m=8,t=1,p=1$c29tZXNhbHQ$c29tZWtleQ",
	}
	for name, value := range rejected {
		t.Run(name, func(t *testing.T) {
			_, err := pool.Exec(ctx,
				`INSERT INTO core.credentials (id, user_id, password_hash) VALUES ($1, $2, $3)`,
				newID(ctx, t), userID, value)
			if err == nil {
				t.Fatalf("the database accepted %s", name)
			}
			if !strings.Contains(err.Error(), "ck_credentials_hash_is_argon2id") {
				t.Errorf("error = %v, want the CHECK constraint to have rejected it", err)
			}
		})
	}

	// The repository path is unaffected by the constraint for a real hash.
	if _, err := repo.Create(ctx, newID(ctx, t), userID,
		mustHash(t, domain.NewHasher(cheapParams), breachTestPassword)); err != nil {
		t.Fatalf("a genuine hash was rejected: %v", err)
	}
}

// TestSchema_DeletingTheAccountDeletesTheCredential is the ON DELETE CASCADE.
// A credential outliving its user would be a password with no account to belong
// to and no path that ever reads it again.
func TestSchema_DeletingTheAccountDeletesTheCredential(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedUser(ctx, t, "cascade@fluentra.test")

	if _, err := repo.Create(ctx, newID(ctx, t), userID,
		mustHash(t, domain.NewHasher(cheapParams), breachTestPassword)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM core.users WHERE id = $1`, userID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	if _, err := repo.GetByUserID(ctx, userID); !apperr.Is(err, apperr.NotFound) {
		t.Fatalf("error = %v, want the credential to have been cascaded away", err)
	}
}

// TestSchema_AlgoParamsCannotBeWrittenDirectly is what makes the column safe to
// trust. If it were an ordinary column, a caller could set it to something the
// hash does not say, and the rehash campaign would be sized from the lie.
func TestSchema_AlgoParamsCannotBeWrittenDirectly(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedUser(ctx, t, "generated@fluentra.test")

	if _, err := repo.Create(ctx, newID(ctx, t), userID,
		mustHash(t, domain.NewHasher(cheapParams), breachTestPassword)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err := pool.Exec(ctx, `UPDATE core.credentials SET algo_params = 'm=65536,t=3,p=2' WHERE user_id = $1`, userID)
	if err == nil {
		t.Fatal("algo_params accepted a direct write, so it is not a generated column")
	}
}
