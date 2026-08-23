package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	"github.com/fluentra/fluentra/internal/modules/lesson/repository"
)

// errRow hands every Scan the injected error, which is how sqlc's `:one`
// methods surface a failed query.
type errRow struct{ err error }

func (r errRow) Scan(_ ...any) error { return r.err }

// errQuerier fails every statement with the same error, so a test can drive one
// repository method into its error branch without a database.
type errQuerier struct{ err error }

func (q errQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), q.err
}

func (q errQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, q.err
}

func (q errQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return errRow(q)
}

func repoFailingWith(err error) *repository.Repository {
	return repository.New(errQuerier{err: err})
}

// TestNoRowsBecomesADomainError is the "documented apperr code, not a 500"
// acceptance at the repository boundary: pgx.ErrNoRows must never escape as a
// bare error, because the HTTP layer turns anything unrecognised into a 500.
func TestNoRowsBecomesADomainError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func(context.Context, *repository.Repository) error
		want error
	}{
		{
			name: "GetCourseBySlug",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetCourseBySlug(ctx, "some-slug")
				return err
			},
			want: domain.ErrCourseNotFound,
		},
		{
			name: "GetPublishedCourseBySlug",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetPublishedCourseBySlug(ctx, "some-slug")
				return err
			},
			want: domain.ErrCourseNotFound,
		},
		{
			name: "GetCourseByID",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetCourseByID(ctx, uuid.New())
				return err
			},
			want: domain.ErrCourseNotFound,
		},
		{
			name: "GetUnitByID",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetUnitByID(ctx, uuid.New())
				return err
			},
			want: domain.ErrUnitNotFound,
		},
		{
			name: "GetLessonByID",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetLessonByID(ctx, uuid.New())
				return err
			},
			want: domain.ErrLessonNotFound,
		},
		{
			name: "GetPublishedLessonByID",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetPublishedLessonByID(ctx, uuid.New())
				return err
			},
			want: domain.ErrLessonNotFound,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := testCase.call(context.Background(), repoFailingWith(pgx.ErrNoRows))
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
		})
	}
}

// TestConstraintViolationsBecomeDomainErrors covers mapPgError. Every constraint
// the migration declares is a rule a caller can break, and each one has to come
// back as its documented code rather than a raw SQLSTATE.
func TestConstraintViolationsBecomeDomainErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		code       string
		constraint string
		want       error
	}{
		{"duplicate course slug", "23505", "uq_courses_slug", domain.ErrSlugAlreadyExists},
		{"duplicate unit position", "23505", "uq_course_units_course_position", domain.ErrInvalidPosition},
		{"duplicate lesson position", "23505", "uq_lessons_unit_position", domain.ErrInvalidPosition},
		{"duplicate activity position", "23505", "uq_activities_lesson_position", domain.ErrInvalidPosition},
		{"malformed slug", "23514", "ck_courses_slug_format", domain.ErrInvalidSlug},
		{"bad cefr_from", "23514", "ck_courses_cefr_from", domain.ErrInvalidCEFRLevel},
		{"bad cefr_to", "23514", "ck_courses_cefr_to", domain.ErrInvalidCEFRLevel},
		{"bad course status", "23514", "ck_courses_status", domain.ErrInvalidStatus},
		{"bad lesson status", "23514", "ck_lessons_status", domain.ErrInvalidStatus},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			pgErr := &pgconn.PgError{Code: testCase.code, ConstraintName: testCase.constraint}
			_, err := repoFailingWith(pgErr).CreateCourse(context.Background(), repository.CreateCourseParams{
				Slug: "some-slug", Title: "Some course", CEFRFrom: "B1", CEFRTo: "B2", Status: "draft",
			})
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
		})
	}
}

// TestUnrecognisedErrorsAreWrappedNotSwallowed proves the mapping is a
// translation and not a catch-all: an unrelated failure must keep its identity
// so it can be logged and alerted on.
func TestUnrecognisedErrorsAreWrappedNotSwallowed(t *testing.T) {
	t.Parallel()

	boom := errors.New("connection reset by peer")
	repo := repoFailingWith(boom)

	_, err := repo.GetCourseBySlug(context.Background(), "some-slug")
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap the underlying failure", err)
	}
	if errors.Is(err, domain.ErrCourseNotFound) {
		t.Error("an unrelated failure was reported as a missing course")
	}

	// An unmapped constraint is not a missing row either.
	pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "some_index_nobody_mapped"}
	if _, err := repoFailingWith(pgErr).CreateCourse(
		context.Background(), repository.CreateCourseParams{Slug: "s", Title: "t"},
	); errors.Is(err, domain.ErrSlugAlreadyExists) {
		t.Error("an unmapped unique violation was reported as a duplicate slug")
	}
}

// TestListQueriesPropagateTheirError covers the `:many` path, which fails at
// Query rather than at Scan.
func TestListQueriesPropagateTheirError(t *testing.T) {
	t.Parallel()

	boom := errors.New("query failed")
	repo := repoFailingWith(boom)
	ctx := context.Background()
	level := "B1"

	if _, err := repo.ListPublishedCourses(ctx, &level, 20, 0); !errors.Is(err, boom) {
		t.Errorf("ListPublishedCourses error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListUnitsByCourseID(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("ListUnitsByCourseID error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListLessonsByUnitID(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("ListLessonsByUnitID error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListLessonsByCourseID(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("ListLessonsByCourseID error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListPublishedLessonsByCourseID(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("ListPublishedLessonsByCourseID error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListActivitiesByLessonID(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("ListActivitiesByLessonID error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListActivitiesByLessonIDs(ctx, []uuid.UUID{uuid.New()}); !errors.Is(err, boom) {
		t.Errorf("ListActivitiesByLessonIDs error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListPrerequisitesByLessonID(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("ListPrerequisitesByLessonID error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListPrerequisitesForLessons(ctx, []uuid.UUID{uuid.New()}); !errors.Is(err, boom) {
		t.Errorf("ListPrerequisitesForLessons error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListAllPrerequisitesInCourse(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("ListAllPrerequisitesInCourse error = %v, want the underlying failure", err)
	}
}

// TestSingleRowAndExecQueriesPropagateTheirError covers the remaining paths.
func TestSingleRowAndExecQueriesPropagateTheirError(t *testing.T) {
	t.Parallel()

	boom := errors.New("statement failed")
	repo := repoFailingWith(boom)
	ctx := context.Background()

	if _, err := repo.CountPublishedCourses(ctx, nil); !errors.Is(err, boom) {
		t.Errorf("CountPublishedCourses error = %v, want the underlying failure", err)
	}
	if _, err := repo.CreateUnit(ctx, repository.CreateUnitParams{Position: 1, Title: "Unit"}); !errors.Is(err, boom) {
		t.Errorf("CreateUnit error = %v, want the underlying failure", err)
	}
	_, err := repo.CreateLesson(ctx, repository.CreateLessonParams{Position: 1, Title: "Lesson"})
	if !errors.Is(err, boom) {
		t.Errorf("CreateLesson error = %v, want the underlying failure", err)
	}
	if _, err := repo.UpdateLesson(ctx, repository.UpdateLessonParams{Title: "Lesson"}); !errors.Is(err, boom) {
		t.Errorf("UpdateLesson error = %v, want the underlying failure", err)
	}
	if _, err := repo.UpdateLessonStatus(ctx, uuid.New(), "published"); !errors.Is(err, boom) {
		t.Errorf("UpdateLessonStatus error = %v, want the underlying failure", err)
	}
	if err := repo.UpdateLessonDuration(ctx, uuid.New(), 30); !errors.Is(err, boom) {
		t.Errorf("UpdateLessonDuration error = %v, want the underlying failure", err)
	}
	if err := repo.AddPrerequisite(ctx, uuid.New(), uuid.New(), 70); !errors.Is(err, boom) {
		t.Errorf("AddPrerequisite error = %v, want the underlying failure", err)
	}
}
