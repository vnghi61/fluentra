// Package service implements the business logic and curriculum workflows for the lesson module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

const (
	detailTTL    = 1 * time.Hour
	treeTTL      = 1 * time.Hour
	catalogueTTL = 15 * time.Minute
	genTTL       = 24 * time.Hour

	cacheVersion = 1
)

// UnlockChecker answers whether a learner has met a lesson's prerequisites.
// learning implements it; lesson only calls it, so the interface is declared
// here rather than imported, and lesson does not depend on learning (Trap 2).
type UnlockChecker interface {
	IsUnlocked(ctx context.Context, userID uuid.UUID, lessonIDs []uuid.UUID) (map[uuid.UUID]bool, error)
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

// PrerequisiteItem models a prerequisite link with descriptive fields for lock reason.
type PrerequisiteItem = contract.PrerequisiteItem

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
	ListLessonIDsByContentVersionID(ctx context.Context, versionID uuid.UUID) ([]uuid.UUID, error)
	ReplaceActivities(
		ctx context.Context, lessonID uuid.UUID, activities []domain.ActivityInput,
	) ([]contract.Activity, error)
	ListPrerequisitesByLessonID(ctx context.Context, lessonID uuid.UUID) ([]PrerequisiteItem, error)
	ListPrerequisitesForLessons(ctx context.Context, lessonIDs []uuid.UUID) ([]PrerequisiteItem, error)
	ListAllPrerequisitesInCourse(ctx context.Context, courseID uuid.UUID) ([]domain.PrerequisiteEdge, error)
	AddPrerequisite(ctx context.Context, lessonID, requiresID uuid.UUID, minScore int32) error
	ResolveActivity(ctx context.Context, activityID uuid.UUID) (*contract.ActivityHierarchy, error)
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

// LessonCaches holds typed cache clients for the lesson service.
type LessonCaches struct {
	Detail    cache.Cache[*LessonDetailDTO]
	Tree      cache.Cache[*CourseTreeData]
	Catalogue cache.Cache[*CatalogueData]
	Gen       cache.Cache[int64]
}

// Deps carries dependencies for constructing the lesson Service.
type Deps struct {
	Pool     *pgxpool.Pool
	Repo     Repository
	Content  contentcontract.Reader
	Unlocker UnlockChecker
	Events   EventWriter
	Caches   LessonCaches
	Clock    clock.Clock
	NewID    func() uuid.UUID
	Env      string
}

// Service orchestrates curriculum and lesson use cases.
type Service struct {
	pool     *pgxpool.Pool
	repo     Repository
	content  contentcontract.Reader
	unlocker UnlockChecker
	events   EventWriter
	caches   LessonCaches
	clock    clock.Clock
	newID    func() uuid.UUID
	env      string
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
	env := deps.Env
	if env == "" {
		env = "unknown"
	}

	return &Service{
		pool:     deps.Pool,
		repo:     deps.Repo,
		content:  deps.Content,
		unlocker: deps.Unlocker,
		events:   deps.Events,
		caches:   deps.Caches,
		clock:    clk,
		newID:    idGen,
		env:      env,
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

// LessonTreeData models a lesson node in the static cached course tree.
type LessonTreeData struct {
	ID               uuid.UUID          `json:"id"`
	UnitID           uuid.UUID          `json:"unit_id"`
	Position         int                `json:"position"`
	Title            string             `json:"title"`
	SkillFocus       string             `json:"skill_focus"`
	EstimatedMinutes int                `json:"estimated_minutes"`
	Status           string             `json:"status"`
	Prereqs          []PrerequisiteItem `json:"prereqs"`
}

// UnitTreeData models a unit node in the static cached course tree.
type UnitTreeData struct {
	ID          uuid.UUID        `json:"id"`
	CourseID    uuid.UUID        `json:"course_id"`
	Position    int              `json:"position"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Lessons     []LessonTreeData `json:"lessons"`
}

// CourseTreeData models the static hierarchy of a course.
type CourseTreeData struct {
	Course CourseSummaryDTO `json:"course"`
	Units  []UnitTreeData   `json:"units"`
}

// CatalogueData holds cached catalogue items and count.
type CatalogueData struct {
	Courses []CourseSummaryDTO `json:"courses"`
	Total   int64              `json:"total"`
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

// CreateCourse handles creating a new course in draft status.
//
// Units and lessons have no equivalent here on purpose: P7.5 §3 leaves whether
// P11.1's seed calls the repository directly or the spec grows routes to the
// task that has the requirement.
func (s *Service) CreateCourse(
	ctx context.Context, _ uuid.UUID, input CreateCourseInput,
) (*contract.Course, error) {
	if !domain.IsValidCEFRLevel(input.CEFRFrom) || !domain.IsValidCEFRLevel(input.CEFRTo) {
		return nil, domain.ErrInvalidCEFRLevel
	}
	if !domain.IsValidSlug(input.Slug) {
		return nil, domain.ErrInvalidSlug
	}
	if !domain.IsValidTitle(input.Title) {
		return nil, domain.ErrInvalidTitle
	}

	course, err := s.repo.CreateCourse(ctx, CreateCourseParams{
		Slug:           input.Slug,
		Title:          input.Title,
		Description:    input.Description,
		CEFRFrom:       input.CEFRFrom,
		CEFRTo:         input.CEFRTo,
		Status:         "draft",
		EstimatedHours: input.EstimatedHours,
	})
	if err != nil {
		return nil, err
	}
	return course, nil
}

// ListCourses returns paginated published courses through the cache.
func (s *Service) ListCourses(
	ctx context.Context, level *string, limit, offset int,
) ([]CourseSummaryDTO, int64, error) {
	if level != nil && !domain.IsValidCEFRLevel(*level) {
		return nil, 0, domain.ErrInvalidCEFRLevel.WithInternal("level query parameter must be one of A1..C2")
	}

	normLimit := domain.NormaliseLimit(limit)
	normOffset := domain.NormaliseOffset(offset)

	levelKey := "all"
	if level != nil {
		levelKey = *level
	}

	gen := s.getCatalogueGeneration(ctx)
	filterHash := fmt.Sprintf("g%d_%s_%d_%d", gen, levelKey, normLimit, normOffset)
	catKey := cache.Key(s.env, "lesson", "catalogue", filterHash, cacheVersion)

	loader := func(loadCtx context.Context) (*CatalogueData, error) {
		courses, err := s.repo.ListPublishedCourses(loadCtx, level, normLimit, normOffset)
		if err != nil {
			return nil, err
		}

		total, err := s.repo.CountPublishedCourses(loadCtx, level)
		if err != nil {
			return nil, err
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

		return &CatalogueData{
			Courses: dtos,
			Total:   total,
		}, nil
	}

	var data *CatalogueData
	var err error
	if s.caches.Catalogue == nil {
		data, err = loader(ctx)
	} else {
		data, err = s.caches.Catalogue.GetOrLoad(ctx, catKey, catalogueTTL, loader)
	}
	if err != nil {
		return nil, 0, err
	}

	return data.Courses, data.Total, nil
}

func (s *Service) catalogueGenerationKey() string {
	return cache.Key(s.env, "lesson", "catalogue", "generation", cacheVersion)
}

// getCatalogueGeneration reads the counter folded into every catalogue key.
//
// It reads with Get rather than GetOrLoad on purpose. GetOrLoad writes the
// loaded value back asynchronously, so a read that missed could land its
// `generation = 1` write *after* a concurrent publish had already written
// `generation = 2` — resurrecting the pre-publish catalogue keys for the rest
// of their TTL. A read that never writes cannot lose that race.
//
// A miss and an outage are both answered with 1: the catalogue stays readable
// when Redis is down (Trap 3), and the 15 minute TTL bounds the staleness.
func (s *Service) getCatalogueGeneration(ctx context.Context) int64 {
	if s.caches.Gen == nil {
		return 1
	}
	gen, err := s.caches.Gen.Get(ctx, s.catalogueGenerationKey())
	if err != nil || gen < 1 {
		return 1
	}
	return gen
}

// bumpCatalogueGeneration makes every catalogue key of the previous generation
// unreachable at once (Trap 1). Two publishes racing here can settle on the
// same new value, which is harmless: the invariant this needs is only that the
// generation differs from the one the cached keys were written under.
func (s *Service) bumpCatalogueGeneration(ctx context.Context) {
	if s.caches.Gen == nil {
		return
	}
	current := s.getCatalogueGeneration(ctx)
	if err := s.caches.Gen.Set(ctx, s.catalogueGenerationKey(), current+1, genTTL); err != nil {
		slog.WarnContext(ctx, "failed to bump catalogue generation counter", "error", err)
	}
}

// GetCourseDetail returns full course curriculum hierarchy with unlock evaluations.
func (s *Service) GetCourseDetail(ctx context.Context, slug string, userID uuid.UUID) (*CourseDetailDTO, error) {
	treeKey := cache.Key(s.env, "lesson", "tree", slug, cacheVersion)

	tree, err := s.loadCourseTree(ctx, treeKey, slug)
	if err != nil {
		return nil, err
	}

	// Batch evaluate unlocking for all lessons with prerequisites
	var lessonsToCheck []uuid.UUID
	for _, u := range tree.Units {
		for _, l := range u.Lessons {
			if len(l.Prereqs) > 0 {
				lessonsToCheck = append(lessonsToCheck, l.ID)
			}
		}
	}

	var unlockedMap map[uuid.UUID]bool
	if len(lessonsToCheck) > 0 && s.unlocker != nil && userID != uuid.Nil {
		var unlockErr error
		unlockedMap, unlockErr = s.unlocker.IsUnlocked(ctx, userID, lessonsToCheck)
		if unlockErr != nil {
			return nil, fmt.Errorf("check unlock states: %w", unlockErr)
		}
	}

	unitDTOs := make([]CourseUnitDTO, len(tree.Units))
	for i, u := range tree.Units {
		lessonDTOs := make([]LessonSummaryDTO, len(u.Lessons))
		for j, l := range u.Lessons {
			locked := false
			var lockReason *string
			if len(l.Prereqs) > 0 {
				if s.unlocker != nil && userID != uuid.Nil {
					unlocked := unlockedMap[l.ID]
					if !unlocked {
						locked = true
						reason := lockReasonFor(l.Prereqs)
						lockReason = &reason
					}
				}
			}
			lessonDTOs[j] = LessonSummaryDTO{
				ID:               l.ID,
				UnitID:           l.UnitID,
				Position:         l.Position,
				Title:            l.Title,
				SkillFocus:       l.SkillFocus,
				EstimatedMinutes: l.EstimatedMinutes,
				Status:           l.Status,
				Locked:           locked,
				LockReason:       lockReason,
			}
		}
		unitDTOs[i] = CourseUnitDTO{
			ID:          u.ID,
			CourseID:    u.CourseID,
			Position:    u.Position,
			Title:       u.Title,
			Description: u.Description,
			Lessons:     lessonDTOs,
		}
	}

	return &CourseDetailDTO{
		ID:             tree.Course.ID,
		Slug:           tree.Course.Slug,
		Title:          tree.Course.Title,
		Description:    tree.Course.Description,
		CEFRFrom:       tree.Course.CEFRFrom,
		CEFRTo:         tree.Course.CEFRTo,
		Status:         tree.Course.Status,
		EstimatedHours: tree.Course.EstimatedHours,
		Units:          unitDTOs,
	}, nil
}

func (s *Service) loadCourseTree(ctx context.Context, treeKey, slug string) (*CourseTreeData, error) {
	loader := func(loadCtx context.Context) (*CourseTreeData, error) {
		course, err := s.repo.GetPublishedCourseBySlug(loadCtx, slug)
		if err != nil {
			return nil, err
		}

		units, err := s.repo.ListUnitsByCourseID(loadCtx, course.ID)
		if err != nil {
			return nil, err
		}

		lessons, err := s.repo.ListPublishedLessonsByCourseID(loadCtx, course.ID)
		if err != nil {
			return nil, err
		}

		lessonIDs := make([]uuid.UUID, len(lessons))
		for i, l := range lessons {
			lessonIDs[i] = l.ID
		}

		prereqs, err := s.repo.ListPrerequisitesForLessons(loadCtx, lessonIDs)
		if err != nil {
			return nil, err
		}

		return s.assembleTreeData(course, units, lessons, prereqs), nil
	}

	if s.caches.Tree == nil {
		return loader(ctx)
	}
	return s.caches.Tree.GetOrLoad(ctx, treeKey, treeTTL, loader)
}

func (s *Service) assembleTreeData(
	course *contract.Course, units []*contract.Unit, lessons []*contract.Lesson, prereqs []PrerequisiteItem,
) *CourseTreeData {
	prereqMap := make(map[uuid.UUID][]PrerequisiteItem)
	for _, p := range prereqs {
		prereqMap[p.LessonID] = append(prereqMap[p.LessonID], p)
	}

	unitLessons := make(map[uuid.UUID][]LessonTreeData)
	for _, l := range lessons {
		unitLessons[l.UnitID] = append(unitLessons[l.UnitID], LessonTreeData{
			ID:               l.ID,
			UnitID:           l.UnitID,
			Position:         l.Position,
			Title:            l.Title,
			SkillFocus:       l.SkillFocus,
			EstimatedMinutes: l.EstimatedMinutes,
			Status:           l.Status,
			Prereqs:          prereqMap[l.ID],
		})
	}

	unitTrees := make([]UnitTreeData, len(units))
	for i, u := range units {
		uLessons := unitLessons[u.ID]
		if uLessons == nil {
			uLessons = []LessonTreeData{}
		}
		unitTrees[i] = UnitTreeData{
			ID:          u.ID,
			CourseID:    u.CourseID,
			Position:    u.Position,
			Title:       u.Title,
			Description: u.Description,
			Lessons:     uLessons,
		}
	}

	return &CourseTreeData{
		Course: CourseSummaryDTO{
			ID:             course.ID,
			Slug:           course.Slug,
			Title:          course.Title,
			Description:    course.Description,
			CEFRFrom:       course.CEFRFrom,
			CEFRTo:         course.CEFRTo,
			Status:         course.Status,
			EstimatedHours: course.EstimatedHours,
		},
		Units: unitTrees,
	}
}

// GetLessonDetail returns the lesson with activities and resolved content versions.
func (s *Service) GetLessonDetail(ctx context.Context, lessonID, userID uuid.UUID) (*LessonDetailDTO, error) {
	prereqs, err := s.repo.ListPrerequisitesByLessonID(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	locked, lockReason, err := s.evaluateLock(ctx, userID, lessonID, prereqs)
	if err != nil {
		return nil, err
	}
	if locked {
		reason := "Prerequisites not met."
		if lockReason != nil {
			reason = *lockReason
		}
		return nil, domain.ErrLessonLocked.WithMeta("lock_reason", reason)
	}

	detailKey := cache.Key(s.env, "lesson", "detail", lessonID.String(), cacheVersion)
	return s.loadLessonDetail(ctx, detailKey, lessonID)
}

func (s *Service) loadLessonDetail(
	ctx context.Context, detailKey string, lessonID uuid.UUID,
) (*LessonDetailDTO, error) {
	loader := func(loadCtx context.Context) (*LessonDetailDTO, error) {
		lesson, lErr := s.repo.GetPublishedLessonByID(loadCtx, lessonID)
		if lErr != nil {
			return nil, lErr
		}

		activities, aErr := s.repo.ListActivitiesByLessonID(loadCtx, lessonID)
		if aErr != nil {
			return nil, aErr
		}

		versionIDs := make([]uuid.UUID, 0, len(activities))
		for _, act := range activities {
			if act.ContentVersionID != uuid.Nil {
				versionIDs = append(versionIDs, act.ContentVersionID)
			}
		}

		var versions map[uuid.UUID]*contentcontract.Version
		if len(versionIDs) > 0 && s.content != nil {
			var vErr error
			versions, vErr = s.content.GetManyVersions(loadCtx, versionIDs)
			if vErr != nil {
				return nil, fmt.Errorf("resolve activity content versions: %w", vErr)
			}
		}

		actDTOs := make([]LessonActivityDTO, len(activities))
		for i, act := range activities {
			actDTOs[i] = LessonActivityDTO{
				ID:               act.ID,
				LessonID:         act.LessonID,
				Position:         act.Position,
				Kind:             act.Kind,
				ContentVersionID: act.ContentVersionID,
				Config:           act.Config,
				Weight:           act.Weight,
				Content:          versions[act.ContentVersionID],
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

	if s.caches.Detail == nil {
		return loader(ctx)
	}
	return s.caches.Detail.GetOrLoad(ctx, detailKey, detailTTL, loader)
}

func (s *Service) execTx(
	ctx context.Context, fn func(ctx context.Context, txRepo Repository, tx OutboxTx) error,
) error {
	if s.pool == nil {
		return fn(ctx, s.repo, nil)
	}
	return dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		return fn(txCtx, txRepo, tx)
	})
}

// UpdateActivities replaces the activities for a lesson, enforcing BR-LESSON-04 & BR-LESSON-06.
func (s *Service) UpdateActivities(
	ctx context.Context, _, lessonID uuid.UUID, activities []domain.ActivityInput,
) ([]contract.Activity, error) {
	if len(activities) == 0 {
		return nil, domain.ErrEmptyActivities
	}
	if len(activities) > domain.MaxActivitiesPerLesson {
		return nil, domain.ErrTooManyActivities
	}

	for i, act := range activities {
		if !domain.IsValidPosition(act.Position) || act.Position != i+1 {
			return nil, domain.ErrInvalidPosition
		}
		if !domain.IsValidActivityKind(act.Kind) {
			return nil, domain.ErrInvalidActivityKind
		}
		if !domain.IsValidWeight(act.Weight) {
			return nil, domain.ErrInvalidWeight
		}
	}

	var result []contract.Activity
	err := s.execTx(ctx, func(ctx context.Context, txRepo Repository, _ OutboxTx) error {
		replaced, err := txRepo.ReplaceActivities(ctx, lessonID, activities)
		if err != nil {
			return err
		}
		result = replaced

		durMinutes := domain.CalculateLessonDuration(activities)
		return txRepo.UpdateLessonDuration(ctx, lessonID, durMinutes)
	})
	if err != nil {
		return nil, err
	}

	lesson, err := s.repo.GetLessonByID(ctx, lessonID)
	if err == nil && lesson != nil {
		unit, uErr := s.repo.GetUnitByID(ctx, lesson.UnitID)
		if uErr == nil && unit != nil {
			s.invalidateLessonCaches(ctx, lessonID, unit.CourseID)
		}
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
	if s.events == nil {
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
func (s *Service) PublishLesson(ctx context.Context, _, lessonID uuid.UUID) (*LessonDetailDTO, error) {
	lesson, err := s.repo.GetLessonByID(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if lesson.Status == "published" {
		return s.buildPublishedLessonDetail(ctx, lesson)
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
		updated, uErr := txRepo.UpdateLessonStatus(ctx, lessonID, "published")
		if uErr != nil {
			return uErr
		}
		publishedLesson = updated
		return s.writePublishedEvent(ctx, txRepo, tx, lesson)
	})
	if err != nil {
		return nil, err
	}

	unit, uErr := s.repo.GetUnitByID(ctx, lesson.UnitID)
	if uErr == nil && unit != nil {
		s.invalidateOnPublish(ctx, lessonID, unit.CourseID)
	}

	return s.buildPublishedLessonDetail(ctx, publishedLesson)
}

func (s *Service) buildPublishedLessonDetail(
	ctx context.Context, lesson *contract.Lesson,
) (*LessonDetailDTO, error) {
	activities, err := s.repo.ListActivitiesByLessonID(ctx, lesson.ID)
	if err != nil {
		return nil, err
	}

	versionIDs := make([]uuid.UUID, 0, len(activities))
	for _, act := range activities {
		if act.ContentVersionID != uuid.Nil {
			versionIDs = append(versionIDs, act.ContentVersionID)
		}
	}

	var versions map[uuid.UUID]*contentcontract.Version
	if len(versionIDs) > 0 && s.content != nil {
		var vErr error
		versions, vErr = s.content.GetManyVersions(ctx, versionIDs)
		if vErr != nil {
			return nil, fmt.Errorf("resolve activity content versions: %w", vErr)
		}
	}

	actDTOs := make([]LessonActivityDTO, len(activities))
	for i, act := range activities {
		actDTOs[i] = LessonActivityDTO{
			ID:               act.ID,
			LessonID:         act.LessonID,
			Position:         act.Position,
			Kind:             act.Kind,
			ContentVersionID: act.ContentVersionID,
			Config:           act.Config,
			Weight:           act.Weight,
			Content:          versions[act.ContentVersionID],
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

// invalidateLessonCaches drops the two keys a lesson's own content can stale:
// its detail, and the tree of the course carrying it. It deliberately does not
// touch the catalogue — a course summary carries no lesson, so an activities
// write cannot change it, and bumping the generation there would throw away
// every cached catalogue page for nothing.
func (s *Service) invalidateLessonCaches(ctx context.Context, lessonID, courseID uuid.UUID) {
	if s.caches.Detail != nil {
		detailKey := cache.Key(s.env, "lesson", "detail", lessonID.String(), cacheVersion)
		if err := s.caches.Detail.Delete(ctx, detailKey); err != nil {
			slog.WarnContext(ctx, "failed to invalidate lesson detail cache",
				"lesson_id", lessonID, "error", err)
		}
	}

	if s.caches.Tree != nil && courseID != uuid.Nil {
		course, err := s.repo.GetCourseByID(ctx, courseID)
		if err == nil && course != nil {
			treeKey := cache.Key(s.env, "lesson", "tree", course.Slug, cacheVersion)
			if err := s.caches.Tree.Delete(ctx, treeKey); err != nil {
				slog.WarnContext(ctx, "failed to invalidate course tree cache",
					"slug", course.Slug, "error", err)
			}
		}
	}
}

// invalidateOnPublish adds the catalogue to invalidateLessonCaches. Publishing
// is the one write that can change which courses the catalogue should list.
func (s *Service) invalidateOnPublish(ctx context.Context, lessonID, courseID uuid.UUID) {
	s.invalidateLessonCaches(ctx, lessonID, courseID)
	s.bumpCatalogueGeneration(ctx)
}

// HandleLessonPublished re-applies the invalidation a publish already performed
// in the process that served the request. It is the backstop the recorded
// decision names: a synchronous Delete that failed while Redis was briefly
// unreachable is only logged, so the publish still returns 200 (Trap 3), and
// this consumer clears the keys once the outbox event is dispatched. Bumping
// the catalogue generation a second time is harmless — it only has to differ
// from the generation the cached keys were written under.
func (s *Service) HandleLessonPublished(ctx context.Context, lessonID, courseID uuid.UUID) error {
	s.invalidateOnPublish(ctx, lessonID, courseID)
	return nil
}

// HandleContentArchived invalidates detail caches for all lessons referencing the archived content version.
func (s *Service) HandleContentArchived(ctx context.Context, versionID uuid.UUID) error {
	lessonIDs, err := s.repo.ListLessonIDsByContentVersionID(ctx, versionID)
	if err != nil {
		return err
	}

	if s.caches.Detail != nil && len(lessonIDs) > 0 {
		keys := make([]string, len(lessonIDs))
		for i, id := range lessonIDs {
			keys[i] = cache.Key(s.env, "lesson", "detail", id.String(), cacheVersion)
		}
		if err := s.caches.Detail.Delete(ctx, keys...); err != nil {
			slog.WarnContext(ctx, "failed to invalidate lesson detail caches on content archive",
				"version_id", versionID, "error", err)
		}
	}

	slog.InfoContext(ctx, "invalidated lesson caches for archived content version",
		"version_id", versionID, "lessons_count", len(lessonIDs))
	return nil
}

// AddPrerequisite adds a prerequisite relationship, enforcing DAG cycle detection (BR-LESSON-03).
func (s *Service) AddPrerequisite(ctx context.Context, _, lessonID, requiresLessonID uuid.UUID, minScore int) error {
	if lessonID == requiresLessonID {
		return domain.ErrPrerequisiteCycle
	}
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

// ResolveActivity implements contract.Reader.
func (s *Service) ResolveActivity(
	ctx context.Context, activityID uuid.UUID,
) (*contract.ActivityHierarchy, error) {
	return s.repo.ResolveActivity(ctx, activityID)
}

// ListPrerequisitesForLessons implements contract.Reader.
func (s *Service) ListPrerequisitesForLessons(
	ctx context.Context, lessonIDs []uuid.UUID,
) ([]contract.PrerequisiteItem, error) {
	return s.repo.ListPrerequisitesForLessons(ctx, lessonIDs)
}

// ListUnitsByCourseID implements contract.Reader.
func (s *Service) ListUnitsByCourseID(
	ctx context.Context, courseID uuid.UUID,
) ([]*contract.Unit, error) {
	return s.repo.ListUnitsByCourseID(ctx, courseID)
}

func (s *Service) evaluateLock(
	ctx context.Context, userID, lessonID uuid.UUID, prereqs []PrerequisiteItem,
) (bool, *string, error) {
	if len(prereqs) == 0 {
		return false, nil, nil
	}
	if s.unlocker == nil || userID == uuid.Nil {
		return false, nil, nil
	}

	unlockedMap, err := s.unlocker.IsUnlocked(ctx, userID, []uuid.UUID{lessonID})
	if err != nil {
		return false, nil, fmt.Errorf("check unlock state for lesson %s: %w", lessonID, err)
	}
	if unlockedMap != nil && unlockedMap[lessonID] {
		return false, nil, nil
	}

	reason := lockReasonFor(prereqs)
	return true, &reason, nil
}

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
