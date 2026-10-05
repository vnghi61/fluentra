//go:build integration

// Package repository_test exercises the studio queries that carry the
// invariants unit tests cannot prove: the payout foreign key, the one-account
// upsert, the duplicate-slug mapping, Gate 2 persistence and the submission
// transaction.
//
// The unit tests run the service against an in-memory repository, which proves
// the orchestration and nothing about the SQL. Run with `make test-int`; the
// tests skip when TEST_DATABASE_URL is not set.
package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/modules/studio/repository"
)

const testDatabase = "fluentra_studio_repository_test"

var packagePool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, testDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", testDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", testDatabase, err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", testDatabase, err)
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

// newRepo returns a repository over the migrated database plus a fresh creator
// id. Each test owns its own user, so they can run in any order and the rows
// are removed with the user.
func newRepo(ctx context.Context, t *testing.T) (repository.Repository, uuid.UUID) {
	t.Helper()
	if packagePool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	var userID uuid.UUID
	email := fmt.Sprintf("studio-repo-%s@example.com", uuid.NewString())
	const insert = `INSERT INTO core.users (email, status) VALUES ($1, 'active') RETURNING id`
	if err := packagePool.QueryRow(ctx, insert, email).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = packagePool.Exec(ctx, `DELETE FROM core.users WHERE id = $1`, userID)
	})

	return repository.NewRepository(packagePool), userID
}

// TestPayoutAccount_FirstSaveCreatesTheProfile is the foreign key the Studio UI
// used to trip over: payout_accounts.creator_id references creator_profiles.
func TestPayoutAccount_FirstSaveCreatesTheProfile(t *testing.T) {
	ctx := context.Background()
	repo, creatorID := newRepo(ctx, t)

	if _, err := repo.GetCreatorProfile(ctx, creatorID); !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("fixture should start without a profile, got %v", err)
	}

	first, err := repo.UpsertPayoutAccount(ctx, creatorID, "MB", "0123456789", "NGUYEN VAN A", true)
	if err != nil {
		t.Fatalf("first payout save: %v", err)
	}
	profile, err := repo.GetCreatorProfile(ctx, creatorID)
	if err != nil {
		t.Fatalf("profile should exist after the first payout save: %v", err)
	}
	if profile.UserID != creatorID {
		t.Errorf("profile user_id = %s, want %s", profile.UserID, creatorID)
	}

	// A repeated save updates the one row, including a flip to non-default.
	second, err := repo.UpsertPayoutAccount(ctx, creatorID, "VCB", "1017588888", "NGUYEN VAN A", true)
	if err != nil {
		t.Fatalf("second payout save: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("repeated save created a second account: %s then %s", first.ID, second.ID)
	}
	var rows int
	if err := packagePool.QueryRow(ctx,
		`SELECT count(*) FROM studio.payout_accounts WHERE creator_id = $1`, creatorID).Scan(&rows); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected one account per creator, found %d", rows)
	}

	if _, err := repo.UpsertPayoutAccount(ctx, creatorID, "VCB", "1017588888", "NGUYEN VAN A", false); err != nil {
		t.Fatalf("explicit non-default save: %v", err)
	}
	account, err := repo.GetPayoutAccount(ctx, creatorID)
	if err != nil {
		t.Fatalf("get default account: %v", err)
	}
	if account != nil {
		t.Errorf("a non-default account is not the payout destination, got %+v", account)
	}
}

// TestPayoutAccount_PreservesExistingProfileFields: creating the profile for a
// payout must not reset biography, eligibility, trust or suspension.
func TestPayoutAccount_PreservesExistingProfileFields(t *testing.T) {
	ctx := context.Background()
	repo, creatorID := newRepo(ctx, t)

	if _, err := repo.UpsertCreatorProfile(ctx, creatorID, "Biography", "Headline"); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if _, err := repo.SetPayoutEligible(ctx, creatorID, true); err != nil {
		t.Fatalf("set payout eligible: %v", err)
	}
	before, err := repo.GetCreatorProfile(ctx, creatorID)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}

	if _, err := repo.UpsertPayoutAccount(ctx, creatorID, "MB", "0123456789", "NGUYEN VAN A", true); err != nil {
		t.Fatalf("payout save: %v", err)
	}

	after, err := repo.GetCreatorProfile(ctx, creatorID)
	if err != nil {
		t.Fatalf("read profile after payout save: %v", err)
	}
	if after.Bio != before.Bio || after.Headline != before.Headline || after.PayoutEligible != before.PayoutEligible {
		t.Errorf("payout save changed the profile: before %+v, after %+v", before, after)
	}
	if after.TrustedAt != nil || after.SuspendedAt != nil {
		t.Errorf("payout save invented trust or suspension: %+v", after)
	}
}

// TestCreateCourseDraft_DuplicateSlugIsAConflict: the unique constraint must
// surface as the documented 409, not a raw database error.
func TestCreateCourseDraft_DuplicateSlugIsAConflict(t *testing.T) {
	ctx := context.Background()
	repo, ownerID := newRepo(ctx, t)

	draft := &domain.CourseDraft{
		OwnerID: ownerID, Title: "One", Slug: "same-slug", CEFRLevel: "B1", Structure: []byte(`{}`),
	}
	if _, err := repo.CreateCourseDraft(ctx, draft); err != nil {
		t.Fatalf("create draft: %v", err)
	}

	_, err := repo.CreateCourseDraft(ctx, &domain.CourseDraft{
		OwnerID: ownerID, Title: "Two", Slug: "same-slug", CEFRLevel: "B1",
		Structure: []byte(`{}`),
	})
	if !errors.Is(err, domain.ErrDraftSlugConflict) {
		t.Fatalf("duplicate slug should map to ErrDraftSlugConflict, got %v", err)
	}

	// Another creator may use the same slug.
	repoOther, otherID := newRepo(ctx, t)
	if _, err := repoOther.CreateCourseDraft(ctx, &domain.CourseDraft{
		OwnerID: otherID, Title: "Three", Slug: "same-slug", CEFRLevel: "B1",
		Structure: []byte(`{}`),
	}); err != nil {
		t.Fatalf("another creator's draft should not conflict: %v", err)
	}

	// A rename onto an owned slug conflicts too.
	second, err := repo.CreateCourseDraft(ctx, &domain.CourseDraft{
		OwnerID: ownerID, Title: "Second", Slug: "second-slug", CEFRLevel: "B1",
		Structure: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("create second draft: %v", err)
	}
	second.Slug = "same-slug"
	if _, err := repo.UpdateCourseDraft(ctx, second); !errors.Is(err, domain.ErrDraftSlugConflict) {
		t.Fatalf("rename onto an owned slug should conflict, got %v", err)
	}
}

// TestSubmitDraft_IsAtomicAndPersistsGate2 covers the transaction the service
// delegates to: one active submission, monotonic versions and the review
// decision stored with the row.
func TestSubmitDraft_IsAtomicAndPersistsGate2(t *testing.T) {
	ctx := context.Background()
	repo, ownerID := newRepo(ctx, t)

	draft, err := repo.CreateCourseDraft(ctx, &domain.CourseDraft{
		OwnerID: ownerID, Title: "Paid", Slug: "paid-course", CEFRLevel: "B1",
		Structure: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	reason := "the course is sold, so a person reads it"
	sub := &domain.Submission{
		DraftID: draft.ID, Status: domain.SubmissionStatusSubmitted,
		SubmittedBy: ownerID, Gate2Required: true, Gate2Reason: &reason,
	}
	created, err := repo.SubmitDraft(ctx, ownerID, sub)
	if err != nil {
		t.Fatalf("submit draft: %v", err)
	}
	if created.Version != 1 {
		t.Errorf("first submission version = %d, want 1", created.Version)
	}

	stored, err := repo.GetSubmissionByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("read submission: %v", err)
	}
	if !stored.Gate2Required || stored.Gate2Reason == nil || *stored.Gate2Reason != reason {
		t.Errorf("Gate 2 decision did not survive persistence: %+v", stored)
	}
	moved, err := repo.GetCourseDraftByID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("read draft: %v", err)
	}
	if moved.Status != domain.DraftStatusSubmitted {
		t.Errorf("draft status = %s, want submitted", moved.Status)
	}

	// A second active submission is refused, and the draft does not move.
	if _, err := repo.SubmitDraft(ctx, ownerID, &domain.Submission{
		DraftID: draft.ID, Status: domain.SubmissionStatusSubmitted, SubmittedBy: ownerID,
	}); !errors.Is(err, domain.ErrCannotSubmit) {
		t.Fatalf("expected ErrCannotSubmit on repeated submit, got %v", err)
	}
	var active int
	if err := packagePool.QueryRow(ctx,
		`SELECT count(*) FROM studio.submissions WHERE draft_id = $1`, draft.ID).Scan(&active); err != nil {
		t.Fatalf("count submissions: %v", err)
	}
	if active != 1 {
		t.Fatalf("expected one submission after a refused repeat, found %d", active)
	}

	// changes_requested reopens the draft, and the next version is 2.
	if _, err := repo.UpdateCourseDraftStatus(ctx, draft.ID, domain.DraftStatusChangesRequested); err != nil {
		t.Fatalf("reopen draft: %v", err)
	}
	second, err := repo.SubmitDraft(ctx, ownerID, &domain.Submission{
		DraftID: draft.ID, Status: domain.SubmissionStatusSubmitted, SubmittedBy: ownerID,
	})
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if second.Version != 2 {
		t.Errorf("second submission version = %d, want 2", second.Version)
	}

	latest, err := repo.GetLatestSubmissionByDraftID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("latest submission: %v", err)
	}
	if latest == nil || latest.ID != second.ID {
		t.Fatalf("latest submission = %+v, want %s", latest, second.ID)
	}

	// Another creator cannot submit this draft.
	_, strangerID := newRepo(ctx, t)
	if _, err := repo.SubmitDraft(ctx, strangerID, &domain.Submission{
		DraftID: draft.ID, Status: domain.SubmissionStatusSubmitted, SubmittedBy: strangerID,
	}); !errors.Is(err, domain.ErrDraftNotFound) {
		t.Fatalf("another creator's submit should be a 404, got %v", err)
	}
}

// TestGetLatestSubmissionByDraftID_NoRowsIsNil keeps the "no submission is not
// an error" contract the draft detail endpoint relies on.
func TestGetLatestSubmissionByDraftID_NoRowsIsNil(t *testing.T) {
	ctx := context.Background()
	repo, ownerID := newRepo(ctx, t)

	draft, err := repo.CreateCourseDraft(ctx, &domain.CourseDraft{
		OwnerID: ownerID, Title: "Empty", Slug: "empty-draft", CEFRLevel: "B1",
		Structure: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	latest, err := repo.GetLatestSubmissionByDraftID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("latest submission: %v", err)
	}
	if latest != nil {
		t.Errorf("a draft with no submissions should answer nil, got %+v", latest)
	}
}
