// Package service implements the business logic and curriculum workflows for the lesson module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// UnlockChecker answers whether a learner has met a lesson's prerequisites.
// learning implements it; lesson only calls it, so the interface is declared
// here rather than imported, and lesson does not depend on learning (Trap 2).
type UnlockChecker interface {
	IsUnlocked(ctx context.Context, userID, lessonID uuid.UUID) (bool, error)
}

// CreateCourseParams holds data to create a course in the repository.
type CreateCourseParams struct {
	Slug           string
	Title          string
	Description    string
	CEFRFrom       string
	CEFRTo         string
	Status         string
	EstimatedHours int
}

// CreateUnitParams holds data to create a unit in the repository.
type CreateUnitParams struct {
	CourseID    uuid.UUID
	Position    int
	Title       string
	Description string
}

// CreateLessonParams holds data to create a lesson in the repository.
type CreateLessonParams struct {
	UnitID           uuid.UUID
	Position         int
	Title            string
	SkillFocus       string
	EstimatedMinutes int
	Status           string
}

// UpdateLessonParams holds data to update a lesson.
type UpdateLessonParams struct {
	ID               uuid.UUID
	Title            string
	SkillFocus       string
	EstimatedMinutes int
	Status           string
}

// PrerequisiteItem models a prerequisite link with descriptive fields for lock reason.
type PrerequisiteItem struct {
	LessonID            uuid.UUID
	RequiresLessonID    uuid.UUID
	MinScore            int
	RequiresLessonTitle string
}

// Repository defines data access methods required by the lesson service.
type Repository interface {
	ListPublishedCourses(ctx context.Context, level *string, limit, offset int32) ([]*contract.Course, error)
	CountPublishedCourses(ctx context.Context, level *string) (int64, error)
	GetCourseBySlug(ctx context.Context, slug string) (*contract.Course, error)
	GetPublishedCourseBySlug(ctx context.Context, slug string) (*contract.Course, error)
	GetPublishedLessonByID(ctx context.Context, id uuid.UUID) (*contract.Lesson, error)
	ListPublishedLessonsByCourseID(ctx context.Context, courseID uuid.UUID) ([]*contract.Lesson, error)
	GetCourseByID(ctx context.Context, id uuid.UUID) (*contract.Course, error)
	CreateCourse(ctx context.Context, params CreateCourseParams) (*contract.Course, error)
	ListUnitsByCourseID(ctx context.Context, courseID uuid.UUID) ([]*contract.Unit, error)
	GetUnitByID(ctx context.Context, id uuid.UUID) (*contract.Unit, error)
	GetLessonByID(ctx context.Context, id uuid.UUID) (*contract.Lesson, error)
	ListLessonsByUnitID(ctx context.Context, unitID uuid.UUID) ([]*contract.Lesson, error)
	ListLessonsByCourseID(ctx context.Context, courseID uuid.UUID) ([]*contract.Lesson, error)
	UpdateLessonStatus(ctx context.Context, id uuid.UUID, status string) (*contract.Lesson, error)
	UpdateLessonDuration(ctx context.Context, id uuid.UUID, minutes int32) error
	ListActivitiesByLessonID(ctx context.Context, lessonID uuid.UUID) ([]contract.Activity, error)
	ListActivitiesByLessonIDs(ctx context.Context, lessonIDs []uuid.UUID) ([]contract.Activity, error)
	ReplaceActivities(
		ctx context.Context, lessonID uuid.UUID, activities []domain.ActivityInput,
	) ([]contract.Activity, error)
	ListPrerequisitesByLessonID(ctx context.Context, lessonID uuid.UUID) ([]PrerequisiteItem, error)
	ListPrerequisitesForLessons(ctx context.Context, lessonIDs []uuid.UUID) ([]PrerequisiteItem, error)
	ListAllPrerequisitesInCourse(ctx context.Context, courseID uuid.UUID) ([]domain.PrerequisiteEdge, error)
	AddPrerequisite(ctx context.Context, lessonID, requiresID uuid.UUID, minScore int32) error
	WithTx(tx pgx.Tx) Repository
}

// OutboxTx is the database transaction interface needed to write outbox events.
type OutboxTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// EventWriter writes domain events to the outbox.
type EventWriter interface {
	Write(ctx context.Context, tx OutboxTx, aggregate, event string, payload any) (uuid.UUID, error)
}

// Deps carries dependencies for constructing the lesson Service.
type Deps struct {
	Pool     *pgxpool.Pool
	Repo     Repository
	Content  contentcontract.Reader
	Unlocker UnlockChecker
	Events   EventWriter
	Clock    clock.Clock
	NewID    func() uuid.UUID
}

// Service orchestrates curriculum and lesson use cases.
type Service struct {
	pool     *pgxpool.Pool
	repo     Repository
	content  contentcontract.Reader
	unlocker UnlockChecker
	events   EventWriter
	clock    clock.Clock
	newID    func() uuid.UUID
}

// New creates a new lesson Service.
func New(deps Deps) *Service {
	clk := deps.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	idGen := deps.NewID
	if idGen == nil {
		idGen = func() uuid.UUID { return uuid.Must(uuid.NewV7()) }
	}

	return &Service{
		pool:     deps.Pool,
		repo:     deps.Repo,
		content:  deps.Content,
		unlocker: deps.Unlocker,
		events:   deps.Events,
		clock:    clk,
		newID:    idGen,
	}
}

// CourseSummaryDTO matches OpenAPI CourseSummary schema.
type CourseSummaryDTO struct {
	ID             uuid.UUID `json:"id"`
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	CEFRFrom       string    `json:"cefr_from"`
	CEFRTo         string    `json:"cefr_to"`
	Status         string    `json:"status"`
	EstimatedHours int       `json:"estimated_hours"`
}

// LessonSummaryDTO matches OpenAPI LessonSummary schema.
type LessonSummaryDTO struct {
	ID               uuid.UUID `json:"id"`
	UnitID           uuid.UUID `json:"unit_id"`
	Position         int       `json:"position"`
	Title            string    `json:"title"`
	SkillFocus       string    `json:"skill_focus"`
	EstimatedMinutes int       `json:"estimated_minutes"`
	Status           string    `json:"status"`
	Locked           bool      `json:"locked"`
	LockReason       *string   `json:"lock_reason"`
}

// CourseUnitDTO matches OpenAPI CourseUnit schema.
type CourseUnitDTO struct {
	ID          uuid.UUID          `json:"id"`
	CourseID    uuid.UUID          `json:"course_id"`
	Position    int                `json:"position"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Lessons     []LessonSummaryDTO `json:"lessons"`
}

// CourseDetailDTO matches OpenAPI CourseDetail schema.
type CourseDetailDTO struct {
	ID             uuid.UUID       `json:"id"`
	Slug           string          `json:"slug"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	CEFRFrom       string          `json:"cefr_from"`
	CEFRTo         string          `json:"cefr_to"`
	Status         string          `json:"status"`
	EstimatedHours int             `json:"estimated_hours"`
	Units          []CourseUnitDTO `json:"units"`
}

// LessonActivityDTO matches OpenAPI LessonActivity schema.
type LessonActivityDTO struct {
	ID               uuid.UUID                `json:"id"`
	LessonID         uuid.UUID                `json:"lesson_id"`
	Position         int                      `json:"position"`
	Kind             string                   `json:"kind"`
	ContentVersionID uuid.UUID                `json:"content_version_id"`
	Config           json.RawMessage          `json:"config"`
	Weight           int                      `json:"weight"`
	Content          *contentcontract.Version `json:"content,omitempty"`
}

// LessonDetailDTO matches OpenAPI LessonDetail schema.
type LessonDetailDTO struct {
	ID               uuid.UUID           `json:"id"`
	UnitID           uuid.UUID           `json:"unit_id"`
	Position         int                 `json:"position"`
	Title            string              `json:"title"`
	SkillFocus       string              `json:"skill_focus"`
	EstimatedMinutes int                 `json:"estimated_minutes"`
	Status           string              `json:"status"`
	Activities       []LessonActivityDTO `json:"activities"`
}

// CreateCourseInput carries arguments for creating a course.
type CreateCourseInput struct {
	Slug           string
	Title          string
	Description    string
	CEFRFrom       string
	CEFRTo         string
	EstimatedHours int
}

// ListCourses returns paginated published courses.
func (s *Service) ListCourses(
	ctx context.Context, level *string, limit, offset int,
) ([]CourseSummaryDTO, int64, error) {
	if level != nil && !domain.IsValidCEFRLevel(*level) {
		return nil, 0, domain.ErrInvalidCEFRLevel.WithInternal("level query parameter must be one of A1..C2")
	}

	// Clamped in int space before the narrowing conversion. Both values arrive
	// from strconv.Atoi over the query string, so neither is known to fit int32
	// until the domain has bounded it.
	normLimit := domain.NormaliseLimit(limit)
	normOffset := domain.NormaliseOffset(offset)

	courses, err := s.repo.ListPublishedCourses(ctx, level, normLimit, normOffset)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.CountPublishedCourses(ctx, level)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]CourseSummaryDTO, len(courses))
	for i, c := range courses {
		dtos[i] = CourseSummaryDTO{
			ID:             c.ID,
			Slug:           c.Slug,
			Title:          c.Title,
			Description:    c.Description,
			CEFRFrom:       c.CEFRFrom,
			CEFRTo:         c.CEFRTo,
			Status:         c.Status,
			EstimatedHours: c.EstimatedHours,
		}
	}

	return dtos, total, nil
}

// GetCourseDetail returns full course curriculum hierarchy with unlock evaluations.
func (s *Service) GetCourseDetail(ctx context.Context, slug string, userID uuid.UUID) (*CourseDetailDTO, error) {
	// Published-only in SQL, not filtered in Go afterwards: a draft course must
	// be a 404 to a learner, and the version of this bug that filters after the
	// select is the one that passes tests and leaks in production.
	course, err := s.repo.GetPublishedCourseBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	units, err := s.repo.ListUnitsByCourseID(ctx, course.ID)
	if err != nil {
		return nil, err
	}

	lessons, err := s.repo.ListPublishedLessonsByCourseID(ctx, course.ID)
	if err != nil {
		return nil, err
	}

	lessonIDs := make([]uuid.UUID, len(lessons))
	for i, l := range lessons {
		lessonIDs[i] = l.ID
	}

	prereqs, err := s.repo.ListPrerequisitesForLessons(ctx, lessonIDs)
	if err != nil {
		return nil, err
	}

	prereqMap := make(map[uuid.UUID][]PrerequisiteItem)
	for _, p := range prereqs {
		prereqMap[p.LessonID] = append(prereqMap[p.LessonID], p)
	}

	// Group lessons by unit
	unitLessons := make(map[uuid.UUID][]LessonSummaryDTO)
	for _, l := range lessons {
		locked, lockReason, err := s.evaluateLock(ctx, userID, l.ID, prereqMap[l.ID])
		if err != nil {
			return nil, err
		}
		unitLessons[l.UnitID] = append(unitLessons[l.UnitID], LessonSummaryDTO{
			ID:               l.ID,
			UnitID:           l.UnitID,
			Position:         l.Position,
			Title:            l.Title,
			SkillFocus:       l.SkillFocus,
			EstimatedMinutes: l.EstimatedMinutes,
			Status:           l.Status,
			Locked:           locked,
			LockReason:       lockReason,
		})
	}

	unitDTOs := make([]CourseUnitDTO, len(units))
	for i, u := range units {
		unitDTOs[i] = CourseUnitDTO{
			ID:          u.ID,
			CourseID:    u.CourseID,
			Position:    u.Position,
			Title:       u.Title,
			Description: u.Description,
			Lessons:     unitLessons[u.ID],
		}
		if unitDTOs[i].Lessons == nil {
			unitDTOs[i].Lessons = []LessonSummaryDTO{}
		}
	}

	return &CourseDetailDTO{
		ID:             course.ID,
		Slug:           course.Slug,
		Title:          course.Title,
		Description:    course.Description,
		CEFRFrom:       course.CEFRFrom,
		CEFRTo:         course.CEFRTo,
		Status:         course.Status,
		EstimatedHours: course.EstimatedHours,
		Units:          unitDTOs,
	}, nil
}

// GetLessonDetail returns the lesson with activities and resolved content versions in 1 query (Trap 4).
func (s *Service) GetLessonDetail(ctx context.Context, lessonID, userID uuid.UUID) (*LessonDetailDTO, error) {
	// Published-only, and the query also requires the owning course to be
	// published — see GetPublishedLessonByID.
	lesson, err := s.repo.GetPublishedLessonByID(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	// Evaluate lock
	prereqs, err := s.repo.ListPrerequisitesByLessonID(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	locked, lockReason, err := s.evaluateLock(ctx, userID, lessonID, prereqs)
	if err != nil {
		return nil, err
	}
	if locked {
		// WithMeta, not fmt.Errorf("%w: ...") — httpx.WriteProblem runs the error
		// through apperr.Wrap, which returns the *apperr.Error it finds and
		// discards the wrapper text entirely. The reason has to travel on the
		// error itself to reach the client, and Problem.meta is where this API
		// already puts structured detail.
		reason := "Prerequisites not met."
		if lockReason != nil {
			reason = *lockReason
		}
		return nil, domain.ErrLessonLocked.WithMeta("lock_reason", reason)
	}

	activities, err := s.repo.ListActivitiesByLessonID(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	// Resolve content versions in ONE batch query (Trap 4)
	versionIDs := make([]uuid.UUID, 0, len(activities))
	for _, act := range activities {
		if act.ContentVersionID != uuid.Nil {
			versionIDs = append(versionIDs, act.ContentVersionID)
		}
	}

	var versions map[uuid.UUID]*contentcontract.Version
	if len(versionIDs) > 0 && s.content != nil {
		var err error
		versions, err = s.content.GetManyVersions(ctx, versionIDs)
		if err != nil {
			return nil, fmt.Errorf("resolve activity content versions: %w", err)
		}
	}

	actDTOs := make([]LessonActivityDTO, len(activities))
	for i, act := range activities {
		var ver *contentcontract.Version
		if versions != nil {
			ver = versions[act.ContentVersionID]
		}
		actDTOs[i] = LessonActivityDTO{
			ID:               act.ID,
			LessonID:         act.LessonID,
			Position:         act.Position,
			Kind:             act.Kind,
			ContentVersionID: act.ContentVersionID,
			Config:           act.Config,
			Weight:           act.Weight,
			Content:          ver,
		}
	}

	return &LessonDetailDTO{
		ID:               lesson.ID,
		UnitID:           lesson.UnitID,
		Position:         lesson.Position,
		Title:            lesson.Title,
		SkillFocus:       lesson.SkillFocus,
		EstimatedMinutes: lesson.EstimatedMinutes,
		Status:           lesson.Status,
		Activities:       actDTOs,
	}, nil
}

// CreateCourse implements admin course creation.
func (s *Service) CreateCourse(ctx context.Context, _ uuid.UUID, input CreateCourseInput) (*contract.Course, error) {
	if !domain.IsValidSlug(input.Slug) {
		return nil, domain.ErrInvalidSlug
	}
	if !domain.IsValidCEFRLevel(input.CEFRFrom) || !domain.IsValidCEFRLevel(input.CEFRTo) {
		return nil, domain.ErrInvalidCEFRLevel
	}
	if !domain.IsValidTitle(input.Title) {
		return nil, domain.ErrInvalidTitle
	}

	return s.repo.CreateCourse(ctx, CreateCourseParams{
		Slug:           input.Slug,
		Title:          input.Title,
		Description:    input.Description,
		CEFRFrom:       input.CEFRFrom,
		CEFRTo:         input.CEFRTo,
		Status:         "draft",
		EstimatedHours: input.EstimatedHours,
	})
}

// execTx runs fn in a serializable transaction if pool is present, or directly on repo in unit tests.
func (s *Service) execTx(
	ctx context.Context, fn func(ctx context.Context, txRepo Repository, tx OutboxTx) error,
) error {
	if s.pool == nil {
		return fn(ctx, s.repo, nil)
	}
	return dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, s.repo.WithTx(tx), tx)
	})
}

// UpdateActivities reorders/replaces the activity list for a lesson and recalculates duration (BR-LESSON-06).
func (s *Service) UpdateActivities(
	ctx context.Context, _, lessonID uuid.UUID, activities []domain.ActivityInput,
) ([]contract.Activity, error) {
	if len(activities) == 0 {
		return nil, domain.ErrEmptyActivities
	}

	// Validate positions continuous 1..N and non-empty content_version_id
	posSeen := make(map[int]bool)
	estimates := make([]domain.ActivityEstimate, len(activities))
	for i, act := range activities {
		if act.Position <= 0 || posSeen[act.Position] {
			return nil, domain.ErrInvalidPosition
		}
		posSeen[act.Position] = true
		if act.ContentVersionID == uuid.Nil {
			return nil, domain.ErrInvalidPosition
		}
		if len(act.Kind) == 0 {
			return nil, domain.ErrInvalidActivityKind
		}
		estimates[i] = domain.ActivityEstimate{Weight: act.Weight}
	}

	var result []contract.Activity
	err := s.execTx(ctx, func(ctx context.Context, txRepo Repository, _ OutboxTx) error {
		updated, err := txRepo.ReplaceActivities(ctx, lessonID, activities)
		if err != nil {
			return err
		}
		result = updated

		// Recalculate duration (BR-LESSON-06)
		newDuration := domain.CalculateLessonDuration(estimates)
		if err := txRepo.UpdateLessonDuration(ctx, lessonID, int32(newDuration)); err != nil { //nolint:gosec // bounded
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) validateActivityVersions(ctx context.Context, activities []contract.Activity) error {
	if s.content == nil {
		return nil
	}
	versionIDs := make([]uuid.UUID, len(activities))
	for i, act := range activities {
		versionIDs[i] = act.ContentVersionID
	}
	versions, err := s.content.GetManyVersions(ctx, versionIDs)
	if err != nil {
		return fmt.Errorf("check activity content versions: %w", err)
	}
	for _, vid := range versionIDs {
		ver, ok := versions[vid]
		if !ok || ver == nil || ver.Status != "published" {
			return domain.ErrActivityContentUnpublished
		}
	}
	return nil
}

func (s *Service) writePublishedEvent(
	ctx context.Context, txRepo Repository, tx OutboxTx, lesson *contract.Lesson,
) error {
	if s.events == nil || tx == nil {
		return nil
	}
	unit, err := txRepo.GetUnitByID(ctx, lesson.UnitID)
	if err != nil {
		return err
	}
	payload := contract.Published{
		LessonID:   lesson.ID,
		CourseID:   unit.CourseID,
		SkillFocus: lesson.SkillFocus,
		OccurredAt: s.clock.Now(),
	}
	if _, err := s.events.Write(ctx, tx, contract.Aggregate, contract.EventLessonPublished, payload); err != nil {
		return fmt.Errorf("write outbox event: %w", err)
	}
	return nil
}

// PublishLesson enforces BR-LESSON-02 (all activity content versions must be published and not archived).
func (s *Service) PublishLesson(ctx context.Context, _, lessonID uuid.UUID) (*contract.Lesson, error) {
	lesson, err := s.repo.GetLessonByID(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if lesson.Status == "published" {
		return lesson, nil
	}

	activities, err := s.repo.ListActivitiesByLessonID(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if len(activities) == 0 {
		return nil, domain.ErrEmptyActivities
	}

	if err := s.validateActivityVersions(ctx, activities); err != nil {
		return nil, err
	}

	var publishedLesson *contract.Lesson
	err = s.execTx(ctx, func(ctx context.Context, txRepo Repository, tx OutboxTx) error {
		updated, err := txRepo.UpdateLessonStatus(ctx, lessonID, "published")
		if err != nil {
			return err
		}
		publishedLesson = updated
		return s.writePublishedEvent(ctx, txRepo, tx, lesson)
	})
	if err != nil {
		return nil, err
	}

	return publishedLesson, nil
}

// AddPrerequisite adds a prerequisite relationship, enforcing DAG cycle detection (BR-LESSON-03).
func (s *Service) AddPrerequisite(ctx context.Context, _, lessonID, requiresLessonID uuid.UUID, minScore int) error {
	if lessonID == requiresLessonID {
		return domain.ErrPrerequisiteCycle
	}
	// Bounded here rather than annotated away below. ck_lesson_prerequisites_min_score
	// would reject 4294967346, but int32() truncates it to 50 first and the
	// database never sees the value the caller sent.
	if minScore < 0 || minScore > 100 {
		return domain.ErrInvalidMinScore
	}

	lesson, err := s.repo.GetLessonByID(ctx, lessonID)
	if err != nil {
		return err
	}

	unit, err := s.repo.GetUnitByID(ctx, lesson.UnitID)
	if err != nil {
		return err
	}

	existingEdges, err := s.repo.ListAllPrerequisitesInCourse(ctx, unit.CourseID)
	if err != nil {
		return err
	}

	if domain.WouldCreateCycle(existingEdges, lessonID, requiresLessonID) {
		return domain.ErrPrerequisiteCycle
	}

	return s.repo.AddPrerequisite(ctx, lessonID, requiresLessonID, int32(minScore))
}

// GetLesson implements contract.Reader.
func (s *Service) GetLesson(ctx context.Context, id uuid.UUID) (*contract.Lesson, error) {
	return s.repo.GetLessonByID(ctx, id)
}

// ListLessons implements contract.Reader.
func (s *Service) ListLessons(ctx context.Context, unitID uuid.UUID) ([]*contract.Lesson, error) {
	return s.repo.ListLessonsByUnitID(ctx, unitID)
}

// NextLesson implements contract.Reader.
func (s *Service) NextLesson(
	ctx context.Context, courseID uuid.UUID, currentLessonID *uuid.UUID,
) (*contract.Lesson, error) {
	lessons, err := s.repo.ListLessonsByCourseID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if len(lessons) == 0 {
		return nil, nil
	}

	if currentLessonID == nil {
		return lessons[0], nil
	}

	for i, l := range lessons {
		if l.ID == *currentLessonID {
			if i+1 < len(lessons) {
				return lessons[i+1], nil
			}
			return nil, nil // Last lesson in course
		}
	}

	return lessons[0], nil
}

// evaluateLock reports whether a lesson is locked for this learner, and the
// sentence the API hands the client when it is.
//
// A nil unlocker means learner progress does not exist yet: `learning` is WP8,
// and until it lands nothing can answer "has this learner finished the
// prerequisite". Reporting every lesson unlocked is the documented behaviour
// (see lesson/DECISIONS.md) and the one that lets P11.1's seed data and WP10's
// screens walk a course. Locking by default is not the safe choice here — it is
// not a security boundary, it is curriculum pacing, and it would make every
// lesson after the first unreachable for the whole of Phase 2.
func (s *Service) evaluateLock(
	ctx context.Context, userID, lessonID uuid.UUID, prereqs []PrerequisiteItem,
) (bool, *string, error) {
	if len(prereqs) == 0 {
		return false, nil, nil
	}
	if s.unlocker == nil || userID == uuid.Nil {
		return false, nil, nil
	}

	unlocked, err := s.unlocker.IsUnlocked(ctx, userID, lessonID)
	if err != nil {
		// Not swallowed into "locked": a learning outage is a 500, not a
		// curriculum decision the learner should read as a locked lesson.
		return false, nil, fmt.Errorf("check unlock state for lesson %s: %w", lessonID, err)
	}
	if unlocked {
		return false, nil, nil
	}

	reason := lockReasonFor(prereqs)
	return true, &reason, nil
}

// lockReasonFor names every prerequisite, not the first one.
//
// UnlockChecker answers with a bool, so this module cannot know *which*
// prerequisite is the unmet one. Naming prereqs[0] out of three would send a
// learner to a lesson they may already have finished; listing all of them is
// the honest sentence the bool supports. BR-LESSON-07 puts the rule here and
// the learner state in `learning`, and this is where that split lands.
func lockReasonFor(prereqs []PrerequisiteItem) string {
	titles := make([]string, 0, len(prereqs))
	for _, p := range prereqs {
		if p.RequiresLessonTitle != "" {
			titles = append(titles, p.RequiresLessonTitle)
		}
	}

	switch len(titles) {
	case 0:
		return "Complete the earlier lessons first"
	case 1:
		return fmt.Sprintf("Complete %s first", titles[0])
	case 2:
		return fmt.Sprintf("Complete %s and %s first", titles[0], titles[1])
	default:
		return fmt.Sprintf("Complete %s and %s first",
			strings.Join(titles[:len(titles)-1], ", "), titles[len(titles)-1])
	}
}
