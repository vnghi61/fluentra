package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/generated/lesson/sqlc"
	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// ActivityInputDTO aliases domain.ActivityInput.
type ActivityInputDTO = domain.ActivityInput

// CreateCourseParams holds the parameters to insert a course.
type CreateCourseParams struct {
	Slug           string
	Title          string
	Description    string
	CEFRFrom       string
	CEFRTo         string
	Status         string
	EstimatedHours int
}

// CreateUnitParams holds the parameters to insert a unit.
type CreateUnitParams struct {
	CourseID    uuid.UUID
	Position    int
	Title       string
	Description string
}

// CreateLessonParams holds the parameters to insert a lesson.
type CreateLessonParams struct {
	UnitID           uuid.UUID
	Position         int
	Title            string
	SkillFocus       string
	EstimatedMinutes int
	Status           string
}

// UpdateLessonParams holds the parameters to update a lesson.
type UpdateLessonParams struct {
	ID               uuid.UUID
	Title            string
	SkillFocus       string
	EstimatedMinutes int
	Status           string
}

// PrerequisiteItem carries a lesson prerequisite with the required lesson's title.
type PrerequisiteItem struct {
	LessonID            uuid.UUID
	RequiresLessonID    uuid.UUID
	MinScore            int
	RequiresLessonTitle string
}

// Repository handles database operations for the lesson module.
type Repository struct {
	queries *sqlc.Queries
}

// New creates a new lesson Repository over db, which is a *pgxpool.Pool in
// production and a pgx.Tx inside a transaction. The parameter is dbx.Querier
// rather than the concrete pool for the same reason content's is: it is the
// seam the error-mapping tests inject through.
func New(db dbx.Querier) *Repository {
	return &Repository{queries: sqlc.New(db)}
}

// WithTx returns a new Repository instance operating within tx.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{queries: r.queries.WithTx(tx)}
}

// ListPublishedCourses retrieves paginated published courses ordered by CEFR level and title.
// level is nil when the caller did not filter.
func (r *Repository) ListPublishedCourses(
	ctx context.Context, level *string, limit, offset int32,
) ([]*contract.Course, error) {
	rows, err := r.queries.ListPublishedCourses(ctx, sqlc.ListPublishedCoursesParams{
		Level:  level,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return ToContractCourses(rows), nil
}

// CountPublishedCourses returns the total number of published courses matching level.
func (r *Repository) CountPublishedCourses(ctx context.Context, level *string) (int64, error) {
	count, err := r.queries.CountPublishedCourses(ctx, level)
	if err != nil {
		return 0, mapPgError(err)
	}
	return count, nil
}

// GetPublishedCourseBySlug is the learner-facing read: a draft or archived
// course is not found, rather than found and hidden afterwards in Go.
func (r *Repository) GetPublishedCourseBySlug(ctx context.Context, slug string) (*contract.Course, error) {
	row, err := r.queries.GetPublishedCourseBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrCourseNotFound
		}
		return nil, mapPgError(err)
	}
	return ToContractCourse(row), nil
}

// GetPublishedLessonByID is the learner-facing read. It also requires the owning
// course to be published.
func (r *Repository) GetPublishedLessonByID(ctx context.Context, id uuid.UUID) (*contract.Lesson, error) {
	row, err := r.queries.GetPublishedLessonByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrLessonNotFound
		}
		return nil, mapPgError(err)
	}
	return ToContractLesson(row, nil), nil
}

// ListPublishedLessonsByCourseID lists the lessons a learner may see in a course.
func (r *Repository) ListPublishedLessonsByCourseID(
	ctx context.Context, courseID uuid.UUID,
) ([]*contract.Lesson, error) {
	rows, err := r.queries.ListPublishedLessonsByCourseID(ctx, courseID)
	if err != nil {
		return nil, mapPgError(err)
	}
	lessons := make([]*contract.Lesson, len(rows))
	for i, row := range rows {
		lessons[i] = ToContractLesson(row, nil)
	}
	return lessons, nil
}

// GetCourseBySlug fetches a course by its unique slug.
func (r *Repository) GetCourseBySlug(ctx context.Context, slug string) (*contract.Course, error) {
	row, err := r.queries.GetCourseBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrCourseNotFound
		}
		return nil, mapPgError(err)
	}
	return ToContractCourse(row), nil
}

// GetCourseByID fetches a course by its primary key UUID.
func (r *Repository) GetCourseByID(ctx context.Context, id uuid.UUID) (*contract.Course, error) {
	row, err := r.queries.GetCourseByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrCourseNotFound
		}
		return nil, mapPgError(err)
	}
	return ToContractCourse(row), nil
}

// CreateCourse inserts a new course record into the database.
func (r *Repository) CreateCourse(ctx context.Context, params CreateCourseParams) (*contract.Course, error) {
	row, err := r.queries.CreateCourse(ctx, sqlc.CreateCourseParams{
		Slug:           params.Slug,
		Title:          params.Title,
		Description:    params.Description,
		CefrFrom:       params.CEFRFrom,
		CefrTo:         params.CEFRTo,
		Status:         params.Status,
		EstimatedHours: int32(params.EstimatedHours), //nolint:gosec // bounded integer
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return ToContractCourse(row), nil
}

// ListUnitsByCourseID lists all units for a given course ordered by position.
func (r *Repository) ListUnitsByCourseID(ctx context.Context, courseID uuid.UUID) ([]*contract.Unit, error) {
	rows, err := r.queries.ListUnitsByCourseID(ctx, courseID)
	if err != nil {
		return nil, mapPgError(err)
	}
	return ToContractUnits(rows), nil
}

// GetUnitByID fetches a course unit by its ID.
func (r *Repository) GetUnitByID(ctx context.Context, id uuid.UUID) (*contract.Unit, error) {
	row, err := r.queries.GetUnitByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUnitNotFound
		}
		return nil, mapPgError(err)
	}
	return ToContractUnit(row), nil
}

// CreateUnit inserts a new unit under a course.
func (r *Repository) CreateUnit(ctx context.Context, params CreateUnitParams) (*contract.Unit, error) {
	row, err := r.queries.CreateUnit(ctx, sqlc.CreateUnitParams{
		CourseID:    params.CourseID,
		Position:    int32(params.Position), //nolint:gosec // bounded integer
		Title:       params.Title,
		Description: params.Description,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return ToContractUnit(row), nil
}

// GetLessonByID fetches a lesson and its activities by ID.
func (r *Repository) GetLessonByID(ctx context.Context, id uuid.UUID) (*contract.Lesson, error) {
	row, err := r.queries.GetLessonByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrLessonNotFound
		}
		return nil, mapPgError(err)
	}
	activities, err := r.ListActivitiesByLessonID(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToContractLesson(row, activities), nil
}

// ListLessonsByUnitID returns all lessons belonging to a unit ordered by position.
func (r *Repository) ListLessonsByUnitID(ctx context.Context, unitID uuid.UUID) ([]*contract.Lesson, error) {
	rows, err := r.queries.ListLessonsByUnitID(ctx, unitID)
	if err != nil {
		return nil, mapPgError(err)
	}

	lessons := make([]*contract.Lesson, len(rows))
	for i, rRow := range rows {
		lessons[i] = ToContractLesson(rRow, nil)
	}
	return lessons, nil
}

// ListLessonsByCourseID returns all lessons across all units of a course.
func (r *Repository) ListLessonsByCourseID(ctx context.Context, courseID uuid.UUID) ([]*contract.Lesson, error) {
	rows, err := r.queries.ListLessonsByCourseID(ctx, courseID)
	if err != nil {
		return nil, mapPgError(err)
	}

	lessons := make([]*contract.Lesson, len(rows))
	for i, rRow := range rows {
		lessons[i] = ToContractLesson(rRow, nil)
	}
	return lessons, nil
}

// CreateLesson creates a new lesson in a unit.
func (r *Repository) CreateLesson(ctx context.Context, params CreateLessonParams) (*contract.Lesson, error) {
	row, err := r.queries.CreateLesson(ctx, sqlc.CreateLessonParams{
		UnitID:           params.UnitID,
		Position:         int32(params.Position), //nolint:gosec // bounded integer
		Title:            params.Title,
		SkillFocus:       params.SkillFocus,
		EstimatedMinutes: int32(params.EstimatedMinutes), //nolint:gosec // bounded integer
		Status:           params.Status,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return ToContractLesson(row, nil), nil
}

// UpdateLesson updates mutable fields of a lesson.
func (r *Repository) UpdateLesson(ctx context.Context, params UpdateLessonParams) (*contract.Lesson, error) {
	row, err := r.queries.UpdateLesson(ctx, sqlc.UpdateLessonParams{
		ID:               params.ID,
		Title:            params.Title,
		SkillFocus:       params.SkillFocus,
		EstimatedMinutes: int32(params.EstimatedMinutes), //nolint:gosec // bounded integer
		Status:           params.Status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrLessonNotFound
		}
		return nil, mapPgError(err)
	}
	return ToContractLesson(row, nil), nil
}

// UpdateLessonStatus updates the lifecycle status of a lesson.
func (r *Repository) UpdateLessonStatus(ctx context.Context, id uuid.UUID, status string) (*contract.Lesson, error) {
	row, err := r.queries.UpdateLessonStatus(ctx, sqlc.UpdateLessonStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrLessonNotFound
		}
		return nil, mapPgError(err)
	}
	return ToContractLesson(row, nil), nil
}

// UpdateLessonDuration updates the estimated minutes of a lesson.
func (r *Repository) UpdateLessonDuration(ctx context.Context, id uuid.UUID, minutes int32) error {
	_, err := r.queries.UpdateLessonDuration(ctx, sqlc.UpdateLessonDurationParams{
		ID:               id,
		EstimatedMinutes: minutes,
	})
	if err != nil {
		return mapPgError(err)
	}
	return nil
}

// ListActivitiesByLessonID returns all activities belonging to a single lesson.
func (r *Repository) ListActivitiesByLessonID(ctx context.Context, lessonID uuid.UUID) ([]contract.Activity, error) {
	rows, err := r.queries.ListActivitiesByLessonID(ctx, lessonID)
	if err != nil {
		return nil, mapPgError(err)
	}
	return ToContractActivities(rows), nil
}

// ListActivitiesByLessonIDs batch retrieves all activities for a list of lessons.
func (r *Repository) ListActivitiesByLessonIDs(
	ctx context.Context, lessonIDs []uuid.UUID,
) ([]contract.Activity, error) {
	if len(lessonIDs) == 0 {
		return []contract.Activity{}, nil
	}
	rows, err := r.queries.ListActivitiesByLessonIDs(ctx, lessonIDs)
	if err != nil {
		return nil, mapPgError(err)
	}
	return ToContractActivities(rows), nil
}

// ReplaceActivities replaces the activity list for a lesson.
func (r *Repository) ReplaceActivities(
	ctx context.Context, lessonID uuid.UUID, activities []ActivityInputDTO,
) ([]contract.Activity, error) {
	if err := r.queries.DeleteActivitiesByLessonID(ctx, lessonID); err != nil {
		return nil, mapPgError(err)
	}

	result := make([]contract.Activity, 0, len(activities))
	for _, act := range activities {
		cfg := act.Config
		if len(cfg) == 0 {
			cfg = json.RawMessage("{}")
		}

		created, err := r.queries.CreateActivity(ctx, sqlc.CreateActivityParams{
			LessonID:         lessonID,
			Position:         int32(act.Position), //nolint:gosec // bounded integer
			Kind:             act.Kind,
			ContentVersionID: act.ContentVersionID,
			Config:           []byte(cfg),
			Weight:           int32(act.Weight), //nolint:gosec // bounded integer
		})
		if err != nil {
			return nil, mapPgError(err)
		}
		result = append(result, ToContractActivity(created))
	}

	return result, nil
}

// ListPrerequisitesByLessonID lists all prerequisites for a specific lesson with prerequisite titles.
func (r *Repository) ListPrerequisitesByLessonID(ctx context.Context, lessonID uuid.UUID) ([]PrerequisiteItem, error) {
	rows, err := r.queries.ListPrerequisitesByLessonID(ctx, lessonID)
	if err != nil {
		return nil, mapPgError(err)
	}
	items := make([]PrerequisiteItem, len(rows))
	for i, row := range rows {
		items[i] = PrerequisiteItem{
			LessonID:            row.LessonID,
			RequiresLessonID:    row.RequiresLessonID,
			MinScore:            int(row.MinScore),
			RequiresLessonTitle: row.RequiresLessonTitle,
		}
	}
	return items, nil
}

// ListPrerequisitesForLessons batch retrieves prerequisites for a slice of lesson IDs.
func (r *Repository) ListPrerequisitesForLessons(
	ctx context.Context, lessonIDs []uuid.UUID,
) ([]PrerequisiteItem, error) {
	if len(lessonIDs) == 0 {
		return []PrerequisiteItem{}, nil
	}
	rows, err := r.queries.ListPrerequisitesForLessons(ctx, lessonIDs)
	if err != nil {
		return nil, mapPgError(err)
	}
	items := make([]PrerequisiteItem, len(rows))
	for i, row := range rows {
		items[i] = PrerequisiteItem{
			LessonID:            row.LessonID,
			RequiresLessonID:    row.RequiresLessonID,
			MinScore:            int(row.MinScore),
			RequiresLessonTitle: row.RequiresLessonTitle,
		}
	}
	return items, nil
}

// ListAllPrerequisitesInCourse lists all prerequisite edges among lessons in a given course.
func (r *Repository) ListAllPrerequisitesInCourse(
	ctx context.Context, courseID uuid.UUID,
) ([]domain.PrerequisiteEdge, error) {
	rows, err := r.queries.ListAllPrerequisitesInCourse(ctx, courseID)
	if err != nil {
		return nil, mapPgError(err)
	}
	return ToPrerequisiteEdges(rows), nil
}

// AddPrerequisite inserts a prerequisite edge between two lessons.
func (r *Repository) AddPrerequisite(ctx context.Context, lessonID, requiresID uuid.UUID, minScore int32) error {
	err := r.queries.AddPrerequisite(ctx, sqlc.AddPrerequisiteParams{
		LessonID:         lessonID,
		RequiresLessonID: requiresID,
		MinScore:         minScore,
	})
	if err != nil {
		return mapPgError(err)
	}
	return nil
}

// ListLessonIDsByContentVersionID returns unique lesson IDs that contain activities using the content version.
func (r *Repository) ListLessonIDsByContentVersionID(
	ctx context.Context, versionID uuid.UUID,
) ([]uuid.UUID, error) {
	if r.queries == nil {
		return nil, nil
	}
	rows, err := r.queries.ListLessonIDsByContentVersionID(ctx, versionID)
	if err != nil {
		return nil, mapPgError(err)
	}
	return rows, nil
}

// ResolveActivity returns the full hierarchy and metadata for an activity.
func (r *Repository) ResolveActivity(
	ctx context.Context, activityID uuid.UUID,
) (*contract.ActivityHierarchy, error) {
	if r.queries == nil {
		return nil, domain.ErrActivityNotFound
	}
	row, err := r.queries.ResolveActivityHierarchy(ctx, activityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrActivityNotFound
		}
		return nil, mapPgError(err)
	}
	return &contract.ActivityHierarchy{
		ActivityID:       row.ActivityID,
		LessonID:         row.LessonID,
		UnitID:           row.UnitID,
		CourseID:         row.CourseID,
		Kind:             row.ActivityKind,
		ContentVersionID: row.ContentVersionID,
		Config:           row.ActivityConfig,
		Weight:           int(row.ActivityWeight),
		LessonSkillFocus: row.LessonSkillFocus,
	}, nil
}

func mapPgError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			switch pgErr.ConstraintName {
			case "uq_courses_slug":
				return domain.ErrSlugAlreadyExists
			case "uq_course_units_course_position", "uq_lessons_unit_position", "uq_activities_lesson_position":
				return domain.ErrInvalidPosition
			}
		case "23514": // check_violation
			switch pgErr.ConstraintName {
			case "ck_courses_slug_format":
				return domain.ErrInvalidSlug
			case "ck_courses_cefr_from", "ck_courses_cefr_to":
				return domain.ErrInvalidCEFRLevel
			case "ck_courses_status", "ck_lessons_status":
				return domain.ErrInvalidStatus
			case "ck_activities_kind_length":
				return domain.ErrInvalidActivityKind
			case "ck_course_units_position_positive", "ck_lessons_position_positive", "ck_activities_position_positive":
				return domain.ErrInvalidPosition
			case "ck_lesson_prerequisites_no_self_ref":
				return domain.ErrPrerequisiteCycle
			}
		}
	}
	return err
}
