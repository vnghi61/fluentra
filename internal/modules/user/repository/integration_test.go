//go:build integration

package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/modules/user/repository"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/id"
)

// repositoryDatabase is this package's own database, for the same reason the
// schema and query suites have theirs: the outbox, job and worker suites share
// the database TEST_DATABASE_URL names, truncating `ops` tables and asserting
// exact row counts, and a package writing beside them causes failures in tests
// it never touches.
const repositoryDatabase = "fluentra_user_repository_test"

// timezoneHoChiMinh is what seedAccount writes, so the assertions can name it
// rather than repeating the string.
const timezoneHoChiMinh = "Asia/Ho_Chi_Minh"

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

// deleteUser removes a seeded row once the test body has returned. It takes no
// context on purpose: by cleanup time the test's context can already be
// cancelled, and the row would survive into the next test.
func deleteUser(userID uuid.UUID) {
	_, _ = pool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
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

// seedAccount writes the three rows registration writes, and removes them
// afterwards so the tests stay independent of each other.
func seedAccount(ctx context.Context, t *testing.T, repo *repository.Repository, email string) uuid.UUID {
	t.Helper()

	userID := newID(ctx, t)
	if _, err := repo.CreateUser(ctx, userID, email, domain.StatusActive); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { deleteUser(userID) }) //nolint:contextcheck // deleteUser explains it

	profile := domain.Profile{DisplayName: "Nghi", Timezone: timezoneHoChiMinh}
	if _, err := repo.CreateProfile(ctx, newID(ctx, t), userID, profile); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if _, err := repo.CreatePreferences(ctx, newID(ctx, t), userID); err != nil {
		t.Fatalf("CreatePreferences: %v", err)
	}
	return userID
}

func TestRepository_UserRoundTripsThroughTheDomainShape(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedAccount(ctx, t, repo, "roundtrip@fluentra.test")

	user, err := repo.GetUser(ctx, userID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.ID != userID {
		t.Errorf("id = %s, want %s", user.ID, userID)
	}
	if user.Status != domain.StatusActive {
		t.Errorf("status = %q, want active", user.Status)
	}
	if user.EmailVerified() {
		t.Error("a new account reports its email as verified")
	}
	if user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
		t.Errorf("timestamps did not map: %+v", user)
	}
}

func TestRepository_UnknownUserIsANotFoundError(t *testing.T) {
	repo, ctx := newRepository(t)

	_, err := repo.GetUser(ctx, newID(ctx, t))
	if !apperr.Is(err, apperr.NotFound) {
		t.Fatalf("error = %v, want a not-found error", err)
	}
}

// TestRepository_DuplicateEmailBecomesAConflict is the citext unique
// constraint arriving as a typed error rather than a 500. The pair is the same
// one the schema test uses, because case is the part that gets forgotten.
func TestRepository_DuplicateEmailBecomesAConflict(t *testing.T) {
	repo, ctx := newRepository(t)
	seedAccount(ctx, t, repo, "Duplicate@fluentra.test")

	_, err := repo.CreateUser(ctx, newID(ctx, t), "duplicate@fluentra.test", domain.StatusActive)
	if !apperr.Is(err, apperr.Conflict) {
		t.Fatalf("error = %v, want a conflict", err)
	}
}

// TestRepository_UpdateProfileLeavesOmittedFieldsAlone is the COALESCE in the
// query, checked against the real planner rather than against the fake that
// imitates it in the service tests.
func TestRepository_UpdateProfileLeavesOmittedFieldsAlone(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedAccount(ctx, t, repo, "coalesce@fluentra.test")

	country := "VN"
	born := time.Date(1998, time.March, 4, 0, 0, 0, 0, time.UTC)
	if _, err := repo.UpdateProfile(ctx, userID, domain.ProfileChange{
		Country: &country, DateOfBirth: &born,
	}); err != nil {
		t.Fatalf("first update: %v", err)
	}

	renamed := "Nghi Nguyen"
	updated, err := repo.UpdateProfile(ctx, userID, domain.ProfileChange{DisplayName: &renamed})
	if err != nil {
		t.Fatalf("second update: %v", err)
	}

	if updated.DisplayName != renamed {
		t.Errorf("display name = %q, want %q", updated.DisplayName, renamed)
	}
	if updated.Country == nil || *updated.Country != country {
		t.Errorf("country = %v, want it untouched at %q", updated.Country, country)
	}
	if updated.Timezone != timezoneHoChiMinh {
		t.Errorf("timezone = %q, want it untouched", updated.Timezone)
	}
	if updated.DateOfBirth == nil || !updated.DateOfBirth.Equal(born) {
		t.Errorf("date of birth = %v, want it untouched at %s", updated.DateOfBirth, born)
	}
}

// TestRepository_QuietHoursSurviveTheTimeConversion is the mapping most likely
// to be wrong: a wall-clock time crosses the boundary as microseconds since
// midnight, and an off-by-one-hour bug there would be invisible in a unit test
// that never touches pgtype.
func TestRepository_QuietHoursSurviveTheTimeConversion(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedAccount(ctx, t, repo, "quiet@fluentra.test")

	wanted := domain.Preferences{
		UserID:               userID,
		Locale:               "vi",
		Theme:                domain.ThemeDark,
		DailyGoalMinutes:     30,
		NotificationChannels: []domain.Channel{domain.ChannelInApp, domain.ChannelPush},
		QuietHours: &domain.QuietHours{
			Start: domain.TimeOfDay{Hour: 22, Minute: 45},
			End:   domain.TimeOfDay{Hour: 7, Minute: 5},
		},
		AIProcessingOptOut: true,
	}
	if _, err := repo.ReplacePreferences(ctx, wanted); err != nil {
		t.Fatalf("ReplacePreferences: %v", err)
	}

	read, err := repo.GetPreferences(ctx, userID)
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if read.QuietHours == nil {
		t.Fatal("quiet hours came back nil")
	}
	if read.QuietHours.Start != wanted.QuietHours.Start || read.QuietHours.End != wanted.QuietHours.End {
		t.Errorf("quiet hours = %+v, want %+v", read.QuietHours, wanted.QuietHours)
	}
	if read.Theme != domain.ThemeDark || read.Locale != "vi" || !read.AIProcessingOptOut {
		t.Errorf("preferences = %+v, want the submitted values", read)
	}
	if len(read.NotificationChannels) != 2 {
		t.Errorf("channels = %v, want two", read.NotificationChannels)
	}
}

func TestRepository_ClearingQuietHoursStoresNoWindow(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedAccount(ctx, t, repo, "noquiet@fluentra.test")

	_, err := repo.ReplacePreferences(ctx, domain.Preferences{
		UserID: userID, Locale: "en", Theme: domain.ThemeSystem, DailyGoalMinutes: 15,
		NotificationChannels: []domain.Channel{domain.ChannelInApp},
	})
	if err != nil {
		t.Fatalf("ReplacePreferences: %v", err)
	}

	read, err := repo.GetPreferences(ctx, userID)
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if read.QuietHours != nil {
		t.Errorf("quiet hours = %+v, want nil", read.QuietHours)
	}
}

// TestRepository_ListSummariesReadsEveryAccountInOneStatement is the batched
// read against the real join. The service test counts the calls; this one
// checks the SQL returns what the caller needs.
func TestRepository_ListSummariesReadsEveryAccountInOneStatement(t *testing.T) {
	repo, ctx := newRepository(t)

	ids := make([]uuid.UUID, 0, 3)
	for index := range 3 {
		ids = append(ids, seedAccount(ctx, t, repo, fmt.Sprintf("summary%d@fluentra.test", index)))
	}
	ids = append(ids, newID(ctx, t)) // an account that does not exist

	summaries, err := repo.ListSummaries(ctx, ids)
	if err != nil {
		t.Fatalf("ListSummaries: %v", err)
	}
	if len(summaries) != 3 {
		t.Fatalf("returned %d summaries, want 3", len(summaries))
	}
	for _, summary := range summaries {
		if summary.DisplayName != "Nghi" {
			t.Errorf("display name = %q, want the joined profile value", summary.DisplayName)
		}
		if summary.Locale != domain.DefaultLocale {
			t.Errorf("locale = %q, want the joined preference value", summary.Locale)
		}
		if summary.Timezone != timezoneHoChiMinh {
			t.Errorf("timezone = %q, want the joined profile value", summary.Timezone)
		}
		if summary.Status != domain.StatusActive {
			t.Errorf("status = %q, want active", summary.Status)
		}
	}
}

// TestRepository_SummaryFallsBackWhenTheJoinsMiss covers the LEFT in the join.
// A user row with no profile must still render, with the column defaults,
// rather than disappearing from every list that includes it.
func TestRepository_SummaryFallsBackWhenTheJoinsMiss(t *testing.T) {
	repo, ctx := newRepository(t)

	userID := newID(ctx, t)
	if _, err := repo.CreateUser(ctx, userID, "bare@fluentra.test", domain.StatusActive); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { deleteUser(userID) }) //nolint:contextcheck // deleteUser explains it

	summary, err := repo.GetSummary(ctx, userID)
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if summary.ID != userID {
		t.Errorf("id = %s, want %s", summary.ID, userID)
	}
	if summary.Timezone != domain.DefaultTimezone {
		t.Errorf("timezone = %q, want the default %q", summary.Timezone, domain.DefaultTimezone)
	}
	if summary.Locale != domain.DefaultLocale {
		t.Errorf("locale = %q, want the default %q", summary.Locale, domain.DefaultLocale)
	}
}

func TestRepository_ExistsReflectsTheRow(t *testing.T) {
	repo, ctx := newRepository(t)
	userID := seedAccount(ctx, t, repo, "exists@fluentra.test")

	exists, err := repo.Exists(ctx, userID)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("Exists = false for a row that is present")
	}

	exists, err = repo.Exists(ctx, newID(ctx, t))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("Exists = true for a row that was never written")
	}
}
