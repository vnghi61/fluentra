package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/studio/domain"
)

// failingQuerier answers every query with one error, so the repository's
// classification can be exercised without a database.
type failingQuerier struct{ err error }

func (f failingQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, f.err
}

func (f failingQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, f.err
}

func (f failingQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	return failingRow(f)
}

type failingRow struct{ err error }

func (r failingRow) Scan(...any) error { return r.err }

func TestIsDraftSlugConflict(t *testing.T) {
	slugViolation := &pgconn.PgError{Code: "23505", ConstraintName: draftSlugConstraint}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"owner and slug unique violation", slugViolation, true},
		{"wrapped owner and slug unique violation", fmt.Errorf("insert draft: %w", slugViolation), true},
		{"a different unique violation", &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}, false},
		{"a non-unique database failure", &pgconn.PgError{Code: "08006"}, false},
		{"not a database error", errors.New("connection refused"), false},
		{"nil", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDraftSlugConflict(tc.err); got != tc.want {
				t.Errorf("isDraftSlugConflict(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestCreateCourseDraftMapsDuplicateSlug(t *testing.T) {
	ctx := context.Background()
	conflict := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: draftSlugConstraint,
		Message:        "duplicate key value violates unique constraint \"uq_course_drafts_owner_slug\"",
	}
	repo := NewRepository(failingQuerier{err: conflict})

	_, err := repo.CreateCourseDraft(ctx, &domain.CourseDraft{})
	if !errors.Is(err, domain.ErrDraftSlugConflict) {
		t.Fatalf("duplicate slug should map to ErrDraftSlugConflict, got %v", err)
	}
	if err.Error() == conflict.Message {
		t.Error("the database detail must not be the returned error")
	}
}

func TestCreateCourseDraftLeavesOtherFailuresUnclassified(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(failingQuerier{err: &pgconn.PgError{Code: "08006", Message: "connection failure"}})

	_, err := repo.CreateCourseDraft(ctx, &domain.CourseDraft{})
	if err == nil {
		t.Fatal("expected the database failure to propagate")
	}
	if errors.Is(err, domain.ErrDraftSlugConflict) {
		t.Error("an unrelated database failure must stay a genuine server error")
	}
}

func TestUpdateCourseDraftMapsDuplicateSlug(t *testing.T) {
	ctx := context.Background()
	conflict := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "uq_course_drafts_owner_slug",
	}
	repo := NewRepository(failingQuerier{err: conflict})

	_, err := repo.UpdateCourseDraft(ctx, &domain.CourseDraft{})
	if !errors.Is(err, domain.ErrDraftSlugConflict) {
		t.Fatalf("duplicate slug on update should map to ErrDraftSlugConflict, got %v", err)
	}
}

func TestSubmitDraftRefusesWithoutTransactionSupport(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(failingQuerier{err: errors.New("no transactions")})

	if _, err := repo.SubmitDraft(ctx, uuid.Nil, &domain.Submission{}); err == nil {
		t.Fatal("expected a repository without transaction support to refuse a submit")
	}
}
