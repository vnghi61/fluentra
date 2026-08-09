//go:build integration

// Package user_test executes every sqlc query in db/queries/user against a real
// PostgreSQL.
//
// `sqlc generate` succeeding only proves the SQL parses against the schema
// sqlc was given. It does not prove the statement runs, that the parameter
// order matches, or that the scan targets line up with the returned columns —
// all three are things the generator will happily get wrong if the query is
// written in a way it mis-models. So every generated function is called here at
// least once.
package user_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	sqlcuser "github.com/fluentra/fluentra/internal/generated/user/sqlc"
	"github.com/fluentra/fluentra/internal/shared/id"
)

// queriesDatabase is this package's own database. It is not the one
// TEST_DATABASE_URL names: the outbox, job and worker suites run against that
// one, truncating shared `ops` tables and asserting exact row counts, and a
// package writing beside them is a source of failures in tests it never
// touches.
const queriesDatabase = "fluentra_user_queries_test"

var packagePool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, queriesDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", queriesDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", queriesDatabase, err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", queriesDatabase, err)
		os.Exit(1)
	}
	packagePool = pool

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

// hoChiMinh is the timezone the profile tests write. IANA names are validated
// by the service, not the column, so the tests use a real one.
const hoChiMinh = "Asia/Ho_Chi_Minh"

func queries(t *testing.T) (*sqlcuser.Queries, context.Context) {
	t.Helper()
	if packagePool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return sqlcuser.New(packagePool), context.Background()
}

// newID uses the project's own generator rather than uuid.New(), so the tests
// insert the same kind of identifier production does (UUIDv7, time-ordered).
func newID(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()
	generated, err := id.NewUUIDv7(ctx)
	if err != nil {
		t.Fatalf("generate id: %v", err)
	}
	return generated
}

// createUser inserts a user through the generated query and removes it on
// cleanup. The cascade takes the profile, preferences and learning profile
// with it, which is itself worth exercising.
func createUser(ctx context.Context, t *testing.T, q *sqlcuser.Queries, email string) sqlcuser.CoreUser {
	t.Helper()
	user, err := q.CreateUser(ctx, sqlcuser.CreateUserParams{
		ID:     newID(ctx, t),
		Email:  email,
		Status: sqlcuser.CoreUserStatusActive,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { deleteUser(user.ID) }) //nolint:contextcheck // deleteUser explains it
	return user
}

// deleteUser removes a row once the test body has returned. It takes no
// context on purpose: by cleanup time the test's context can already be
// cancelled, and the row would survive into the next test.
func deleteUser(userID uuid.UUID) {
	_, _ = packagePool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
}

func TestUserQueries_CreateReadAndStatusTransitions(t *testing.T) {
	q, ctx := queries(t)

	user := createUser(ctx, t, q, "Reader@Fluentra.test")
	if user.Status != sqlcuser.CoreUserStatusActive {
		t.Fatalf("status = %q, want active", user.Status)
	}
	if user.EmailVerifiedAt != nil {
		t.Errorf("email_verified_at = %v, want nil on a fresh user", user.EmailVerifiedAt)
	}

	byID, err := q.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if byID.ID != user.ID {
		t.Errorf("GetUserByID returned %s, want %s", byID.ID, user.ID)
	}

	// Written mixed-case, read lower-case: the citext column, through sqlc.
	byEmail, err := q.GetUserByEmail(ctx, "reader@fluentra.test")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if byEmail.ID != user.ID {
		t.Errorf("GetUserByEmail matched %s, want %s", byEmail.ID, user.ID)
	}

	exists, err := q.UserExists(ctx, user.ID)
	if err != nil {
		t.Fatalf("UserExists: %v", err)
	}
	if !exists {
		t.Error("UserExists = false for a user that was just created")
	}
	absent, err := q.UserExists(ctx, newID(ctx, t))
	if err != nil {
		t.Fatalf("UserExists for an unknown id: %v", err)
	}
	if absent {
		t.Error("UserExists = true for an id that was never inserted")
	}

	suspended, err := q.UpdateUserStatus(ctx, sqlcuser.UpdateUserStatusParams{
		ID:     user.ID,
		Status: sqlcuser.CoreUserStatusSuspended,
	})
	if err != nil {
		t.Fatalf("UpdateUserStatus: %v", err)
	}
	if suspended.Status != sqlcuser.CoreUserStatusSuspended {
		t.Errorf("status = %q, want suspended", suspended.Status)
	}
	if !suspended.UpdatedAt.After(user.UpdatedAt) {
		t.Errorf("updated_at = %s, want later than %s", suspended.UpdatedAt, user.UpdatedAt)
	}
}

// TestUserQueries_MarkEmailVerifiedIsIdempotent covers the COALESCE in that
// query: the audit trail records the first verification, so a second call must
// not move the timestamp.
func TestUserQueries_MarkEmailVerifiedIsIdempotent(t *testing.T) {
	q, ctx := queries(t)
	user := createUser(ctx, t, q, "verify@fluentra.test")

	first, err := q.MarkUserEmailVerified(ctx, user.ID)
	if err != nil {
		t.Fatalf("MarkUserEmailVerified: %v", err)
	}
	if first.EmailVerifiedAt == nil {
		t.Fatal("email_verified_at is still nil after verification")
	}

	second, err := q.MarkUserEmailVerified(ctx, user.ID)
	if err != nil {
		t.Fatalf("MarkUserEmailVerified again: %v", err)
	}
	if second.EmailVerifiedAt == nil || !second.EmailVerifiedAt.Equal(*first.EmailVerifiedAt) {
		t.Errorf("email_verified_at moved from %v to %v on the second call",
			first.EmailVerifiedAt, second.EmailVerifiedAt)
	}
}

// TestUserQueries_ListByIDsReadsEveryRowInOneStatement is the batched read the
// P1.2 contract needs. Here it only has to return the right rows for N ids;
// proving the service issues one query for N ids belongs with the service.
func TestUserQueries_ListByIDsReadsEveryRowInOneStatement(t *testing.T) {
	q, ctx := queries(t)

	first := createUser(ctx, t, q, "batch1@fluentra.test")
	second := createUser(ctx, t, q, "batch2@fluentra.test")
	third := createUser(ctx, t, q, "batch3@fluentra.test")

	users, err := q.ListUsersByIDs(ctx, []uuid.UUID{first.ID, second.ID, third.ID, newID(ctx, t)})
	if err != nil {
		t.Fatalf("ListUsersByIDs: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("returned %d users, want 3 (the unknown id must simply be absent)", len(users))
	}
	found := map[uuid.UUID]bool{}
	for _, user := range users {
		found[user.ID] = true
	}
	for _, want := range []uuid.UUID{first.ID, second.ID, third.ID} {
		if !found[want] {
			t.Errorf("user %s missing from the batched read", want)
		}
	}
}

func TestProfileQueries_CreateReadAndPartialUpdate(t *testing.T) {
	q, ctx := queries(t)
	user := createUser(ctx, t, q, "profile@fluentra.test")

	country := "VN"
	born := pgtype.Date{Time: time.Date(1998, time.March, 4, 0, 0, 0, 0, time.UTC), Valid: true}
	profile, err := q.CreateProfile(ctx, sqlcuser.CreateProfileParams{
		ID:          newID(ctx, t),
		UserID:      user.ID,
		DisplayName: "Nghi",
		Country:     &country,
		Timezone:    hoChiMinh,
		DateOfBirth: born,
	})
	if err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if profile.AvatarAssetID != nil {
		t.Errorf("avatar_asset_id = %v, want nil before an avatar is uploaded", profile.AvatarAssetID)
	}

	read, err := q.GetProfileByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetProfileByUserID: %v", err)
	}
	if read.DisplayName != "Nghi" || read.Timezone != hoChiMinh {
		t.Errorf("read back %+v, want the values just written", read)
	}

	// Only the display name is supplied; everything else must survive. This is
	// what the COALESCE in UpdateProfile is for, and it is the failure mode a
	// partial-update query has: silently nulling the fields nobody sent.
	renamed := "Nghi Nguyen"
	updated, err := q.UpdateProfile(ctx, sqlcuser.UpdateProfileParams{
		UserID:      user.ID,
		DisplayName: &renamed,
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated.DisplayName != renamed {
		t.Errorf("display_name = %q, want %q", updated.DisplayName, renamed)
	}
	if updated.Country == nil || *updated.Country != country {
		t.Errorf("country = %v, want it untouched at %q", updated.Country, country)
	}
	if updated.Timezone != hoChiMinh {
		t.Errorf("timezone = %q, want it untouched", updated.Timezone)
	}
	if !updated.DateOfBirth.Valid || !updated.DateOfBirth.Time.Equal(born.Time) {
		t.Errorf("date_of_birth = %v, want it untouched at %v", updated.DateOfBirth, born.Time)
	}
}

func TestProfileQueries_ListByUserIDs(t *testing.T) {
	q, ctx := queries(t)

	first := createUser(ctx, t, q, "listprofile1@fluentra.test")
	second := createUser(ctx, t, q, "listprofile2@fluentra.test")
	for index, user := range []sqlcuser.CoreUser{first, second} {
		_, err := q.CreateProfile(ctx, sqlcuser.CreateProfileParams{
			ID:          newID(ctx, t),
			UserID:      user.ID,
			DisplayName: fmt.Sprintf("Learner %d", index),
			Timezone:    "UTC",
		})
		if err != nil {
			t.Fatalf("CreateProfile: %v", err)
		}
	}

	profiles, err := q.ListProfilesByUserIDs(ctx, []uuid.UUID{first.ID, second.ID})
	if err != nil {
		t.Fatalf("ListProfilesByUserIDs: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("returned %d profiles, want 2", len(profiles))
	}
}

func TestPreferenceQueries_DefaultsThenFullReplacement(t *testing.T) {
	q, ctx := queries(t)
	user := createUser(ctx, t, q, "prefs@fluentra.test")

	created, err := q.CreateUserPreferences(ctx, sqlcuser.CreateUserPreferencesParams{
		ID:     newID(ctx, t),
		UserID: user.ID,
	})
	if err != nil {
		t.Fatalf("CreateUserPreferences: %v", err)
	}
	// Registration inserts the row with no values, so the column defaults are
	// the product decision. Assert them here rather than in a comment.
	if created.Locale != "en" {
		t.Errorf("default locale = %q, want en", created.Locale)
	}
	if created.Theme != sqlcuser.CoreUiThemeSystem {
		t.Errorf("default theme = %q, want system", created.Theme)
	}
	if created.DailyGoalMinutes != 15 {
		t.Errorf("default daily_goal_minutes = %d, want 15", created.DailyGoalMinutes)
	}
	if created.AiProcessingOptOut {
		t.Error("ai_processing_opt_out defaults to true; it must default to false")
	}
	if len(created.NotificationChannels) != 2 {
		t.Errorf("default notification_channels = %v, want [in_app email]", created.NotificationChannels)
	}

	read, err := q.GetUserPreferences(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserPreferences: %v", err)
	}
	if read.ID != created.ID {
		t.Errorf("GetUserPreferences returned %s, want %s", read.ID, created.ID)
	}

	quietFrom := pgtype.Time{Microseconds: int64(22 * time.Hour / time.Microsecond), Valid: true}
	quietTo := pgtype.Time{Microseconds: int64(7 * time.Hour / time.Microsecond), Valid: true}
	replaced, err := q.ReplaceUserPreferences(ctx, sqlcuser.ReplaceUserPreferencesParams{
		UserID:               user.ID,
		Locale:               "vi",
		Theme:                sqlcuser.CoreUiThemeDark,
		DailyGoalMinutes:     30,
		NotificationChannels: []string{"push"},
		QuietHoursStart:      quietFrom,
		QuietHoursEnd:        quietTo,
		AiProcessingOptOut:   true,
	})
	if err != nil {
		t.Fatalf("ReplaceUserPreferences: %v", err)
	}
	if replaced.Locale != "vi" || replaced.Theme != sqlcuser.CoreUiThemeDark {
		t.Errorf("replaced = %+v, want locale vi and theme dark", replaced)
	}
	if !replaced.AiProcessingOptOut {
		t.Error("ai_processing_opt_out was not replaced")
	}
	if !replaced.QuietHoursStart.Valid || replaced.QuietHoursStart.Microseconds != quietFrom.Microseconds {
		t.Errorf("quiet_hours_start = %+v, want %+v", replaced.QuietHoursStart, quietFrom)
	}
}

func TestLearningProfileQueries_CreateReadAndReplace(t *testing.T) {
	q, ctx := queries(t)
	user := createUser(ctx, t, q, "learning@fluentra.test")

	declared := sqlcuser.CoreCefrLevelB1
	target := sqlcuser.CoreCefrLevelC1
	weekly := int32(180)
	created, err := q.CreateLearningProfile(ctx, sqlcuser.CreateLearningProfileParams{
		ID:                newID(ctx, t),
		UserID:            user.ID,
		DeclaredLevel:     &declared,
		TargetLevel:       &target,
		TargetExam:        sqlcuser.CoreTargetExamIelts,
		WeeklyMinutesGoal: &weekly,
		Motivations:       []string{"work", "study_abroad"},
	})
	if err != nil {
		t.Fatalf("CreateLearningProfile: %v", err)
	}
	if created.DeclaredLevel == nil || *created.DeclaredLevel != declared {
		t.Errorf("declared_level = %v, want %q", created.DeclaredLevel, declared)
	}

	read, err := q.GetLearningProfileByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetLearningProfileByUserID: %v", err)
	}
	if read.TargetExam != sqlcuser.CoreTargetExamIelts {
		t.Errorf("target_exam = %q, want ielts", read.TargetExam)
	}
	if len(read.Motivations) != 2 {
		t.Errorf("motivations = %v, want two entries", read.Motivations)
	}

	// A replacement that clears the optional fields must actually clear them.
	replaced, err := q.ReplaceLearningProfile(ctx, sqlcuser.ReplaceLearningProfileParams{
		UserID:      user.ID,
		TargetExam:  sqlcuser.CoreTargetExamNone,
		Motivations: []string{},
	})
	if err != nil {
		t.Fatalf("ReplaceLearningProfile: %v", err)
	}
	if replaced.DeclaredLevel != nil || replaced.TargetLevel != nil || replaced.WeeklyMinutesGoal != nil {
		t.Errorf("replaced = %+v, want the optional fields cleared", replaced)
	}
	if len(replaced.Motivations) != 0 {
		t.Errorf("motivations = %v, want empty", replaced.Motivations)
	}
}

// TestUserQueries_DeletingAUserCascadesToItsSatelliteTables is the ON DELETE
// CASCADE from the migration, checked from the Go side because that is where
// account erasure will run.
func TestUserQueries_DeletingAUserCascadesToItsSatelliteTables(t *testing.T) {
	q, ctx := queries(t)
	user := createUser(ctx, t, q, "cascade@fluentra.test")

	if _, err := q.CreateProfile(ctx, sqlcuser.CreateProfileParams{
		ID: newID(ctx, t), UserID: user.ID, DisplayName: "Cascade", Timezone: "UTC",
	}); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if _, err := q.CreateUserPreferences(ctx, sqlcuser.CreateUserPreferencesParams{
		ID: newID(ctx, t), UserID: user.ID,
	}); err != nil {
		t.Fatalf("CreateUserPreferences: %v", err)
	}

	if _, err := packagePool.Exec(ctx, `DELETE FROM core.users WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	if _, err := q.GetProfileByUserID(ctx, user.ID); err == nil {
		t.Error("profile survived the deletion of its user")
	}
	if _, err := q.GetUserPreferences(ctx, user.ID); err == nil {
		t.Error("preferences survived the deletion of their user")
	}
}
