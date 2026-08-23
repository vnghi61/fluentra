package service_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
)

// countingContentReader records every call to prove batched resolution (Trap 4).
const (
	statusDraft     = "draft"
	statusPublished = "published"
)

const (
	slugIELTSCore   = "ielts-core"
	titleCourse     = "Course"
	titleUnit1      = "Unit 1"
	titleLesson1    = "Lesson 1"
	titleLesson2    = "Lesson 2"
	codeInvalidCEFR = "INVALID_CEFR_LEVEL"
	reasonLesson1   = "Complete Unit 1 Lesson 1 first"
)

type countingContentReader struct {
	getVersionCount      atomic.Int64
	getManyVersionsCount atomic.Int64
	browseCount          atomic.Int64
	versions             map[uuid.UUID]*contentcontract.Version
}

func (c *countingContentReader) GetVersion(_ context.Context, id uuid.UUID) (*contentcontract.Version, error) {
	c.getVersionCount.Add(1)
	return c.versions[id], nil
}

func (c *countingContentReader) GetManyVersions(
	_ context.Context, ids []uuid.UUID,
) (map[uuid.UUID]*contentcontract.Version, error) {
	c.getManyVersionsCount.Add(1)
	result := make(map[uuid.UUID]*contentcontract.Version)
	for _, id := range ids {
		if ver, ok := c.versions[id]; ok {
			result[id] = ver
		}
	}
	return result, nil
}

func (c *countingContentReader) Browse(
	_ context.Context, _ contentcontract.BrowseFilter,
) ([]*contentcontract.Version, int, error) {
	c.browseCount.Add(1)
	return nil, 0, nil
}

// fakeLessonRepo implements service.Repository for unit tests.
type fakeLessonRepo struct {
	lesson       *contract.Lesson
	activities   []contract.Activity
	prereqs      []service.PrerequisiteItem
	courses      []*contract.Course
	units        []*contract.Unit
	edges        []domain.PrerequisiteEdge
	queryCounter atomic.Int64

	// what the last catalogue query was given, so a test can assert the clamp
	// and the level filter reach the query rather than stopping at the service.
	lastLevel  *string
	lastLimit  int32
	lastOffset int32
}

func (f *fakeLessonRepo) ListPublishedCourses(
	_ context.Context, level *string, limit, offset int32,
) ([]*contract.Course, error) {
	f.queryCounter.Add(1)
	f.lastLevel = level
	f.lastLimit = limit
	f.lastOffset = offset
	return f.publishedCourses(), nil
}

func (f *fakeLessonRepo) CountPublishedCourses(_ context.Context, _ *string) (int64, error) {
	f.queryCounter.Add(1)
	return int64(len(f.publishedCourses())), nil
}

func (f *fakeLessonRepo) publishedCourses() []*contract.Course {
	out := make([]*contract.Course, 0, len(f.courses))
	for _, c := range f.courses {
		if c.Status == statusPublished {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeLessonRepo) GetPublishedCourseBySlug(_ context.Context, slug string) (*contract.Course, error) {
	f.queryCounter.Add(1)
	for _, c := range f.courses {
		if c.Slug == slug && c.Status == statusPublished {
			return c, nil
		}
	}
	return nil, domain.ErrCourseNotFound
}

func (f *fakeLessonRepo) GetPublishedLessonByID(_ context.Context, _ uuid.UUID) (*contract.Lesson, error) {
	f.queryCounter.Add(1)
	if f.lesson == nil || f.lesson.Status != statusPublished {
		return nil, domain.ErrLessonNotFound
	}
	return f.lesson, nil
}

func (f *fakeLessonRepo) ListPublishedLessonsByCourseID(
	_ context.Context, _ uuid.UUID,
) ([]*contract.Lesson, error) {
	f.queryCounter.Add(1)
	if f.lesson == nil || f.lesson.Status != statusPublished {
		return nil, nil
	}
	return []*contract.Lesson{f.lesson}, nil
}

func (f *fakeLessonRepo) GetCourseBySlug(_ context.Context, slug string) (*contract.Course, error) {
	f.queryCounter.Add(1)
	for _, c := range f.courses {
		if c.Slug == slug {
			return c, nil
		}
	}
	return nil, nil
}

func (f *fakeLessonRepo) GetCourseByID(_ context.Context, id uuid.UUID) (*contract.Course, error) {
	f.queryCounter.Add(1)
	for _, c := range f.courses {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

func (f *fakeLessonRepo) CreateCourse(_ context.Context, params service.CreateCourseParams) (*contract.Course, error) {
	f.queryCounter.Add(1)
	c := &contract.Course{
		ID:             uuid.New(),
		Slug:           params.Slug,
		Title:          params.Title,
		Description:    params.Description,
		CEFRFrom:       params.CEFRFrom,
		CEFRTo:         params.CEFRTo,
		Status:         statusDraft,
		EstimatedHours: params.EstimatedHours,
	}
	f.courses = append(f.courses, c)
	return c, nil
}

func (f *fakeLessonRepo) ListUnitsByCourseID(_ context.Context, _ uuid.UUID) ([]*contract.Unit, error) {
	f.queryCounter.Add(1)
	return f.units, nil
}

func (f *fakeLessonRepo) GetUnitByID(_ context.Context, id uuid.UUID) (*contract.Unit, error) {
	f.queryCounter.Add(1)
	for _, u := range f.units {
		if u.ID == id {
			return u, nil
		}
	}
	return &contract.Unit{ID: id, CourseID: uuid.New()}, nil
}

func (f *fakeLessonRepo) CreateUnit(_ context.Context, _ service.CreateUnitParams) (*contract.Unit, error) {
	f.queryCounter.Add(1)
	return nil, nil
}

func (f *fakeLessonRepo) GetLessonByID(_ context.Context, _ uuid.UUID) (*contract.Lesson, error) {
	f.queryCounter.Add(1)
	return f.lesson, nil
}

func (f *fakeLessonRepo) ListLessonsByUnitID(_ context.Context, _ uuid.UUID) ([]*contract.Lesson, error) {
	f.queryCounter.Add(1)
	return []*contract.Lesson{f.lesson}, nil
}

func (f *fakeLessonRepo) ListLessonsByCourseID(_ context.Context, _ uuid.UUID) ([]*contract.Lesson, error) {
	f.queryCounter.Add(1)
	return []*contract.Lesson{f.lesson}, nil
}

func (f *fakeLessonRepo) CreateLesson(_ context.Context, _ service.CreateLessonParams) (*contract.Lesson, error) {
	f.queryCounter.Add(1)
	return nil, nil
}

func (f *fakeLessonRepo) UpdateLesson(_ context.Context, _ service.UpdateLessonParams) (*contract.Lesson, error) {
	f.queryCounter.Add(1)
	return nil, nil
}

func (f *fakeLessonRepo) UpdateLessonStatus(_ context.Context, _ uuid.UUID, status string) (*contract.Lesson, error) {
	f.queryCounter.Add(1)
	f.lesson.Status = status
	return f.lesson, nil
}

func (f *fakeLessonRepo) UpdateLessonDuration(_ context.Context, _ uuid.UUID, minutes int32) error {
	f.queryCounter.Add(1)
	f.lesson.EstimatedMinutes = int(minutes)
	return nil
}

func (f *fakeLessonRepo) ListActivitiesByLessonID(_ context.Context, _ uuid.UUID) ([]contract.Activity, error) {
	f.queryCounter.Add(1)
	return f.activities, nil
}

func (f *fakeLessonRepo) ListActivitiesByLessonIDs(_ context.Context, _ []uuid.UUID) ([]contract.Activity, error) {
	f.queryCounter.Add(1)
	return f.activities, nil
}

func (f *fakeLessonRepo) ReplaceActivities(
	_ context.Context, lessonID uuid.UUID, activities []domain.ActivityInput,
) ([]contract.Activity, error) {
	f.queryCounter.Add(1)
	acts := make([]contract.Activity, len(activities))
	for i, a := range activities {
		acts[i] = contract.Activity{
			ID:               uuid.New(),
			LessonID:         lessonID,
			Position:         a.Position,
			Kind:             a.Kind,
			ContentVersionID: a.ContentVersionID,
			Config:           a.Config,
			Weight:           a.Weight,
		}
	}
	f.activities = acts
	return acts, nil
}

func (f *fakeLessonRepo) ListPrerequisitesByLessonID(
	_ context.Context, _ uuid.UUID,
) ([]service.PrerequisiteItem, error) {
	f.queryCounter.Add(1)
	return f.prereqs, nil
}

func (f *fakeLessonRepo) ListPrerequisitesForLessons(
	_ context.Context, _ []uuid.UUID,
) ([]service.PrerequisiteItem, error) {
	f.queryCounter.Add(1)
	return f.prereqs, nil
}

func (f *fakeLessonRepo) ListAllPrerequisitesInCourse(
	_ context.Context, _ uuid.UUID,
) ([]domain.PrerequisiteEdge, error) {
	f.queryCounter.Add(1)
	return f.edges, nil
}

func (f *fakeLessonRepo) AddPrerequisite(_ context.Context, lessonID, requiresID uuid.UUID, _ int32) error {
	f.queryCounter.Add(1)
	f.edges = append(f.edges, domain.PrerequisiteEdge{
		LessonID:         lessonID,
		RequiresLessonID: requiresID,
	})
	return nil
}

func (f *fakeLessonRepo) WithTx(_ pgx.Tx) service.Repository {
	return f
}

// TestGetLessonDetail_BatchesContentResolution proves Trap 4:
// A lesson with 10 activities resolves content versions in ONE batch query via GetManyVersions,
// and does NOT invoke GetVersion in a loop.
func TestGetLessonDetail_BatchesContentResolution(t *testing.T) {
	lessonID := uuid.New()
	unitID := uuid.New()

	const numActivities = 10
	activities := make([]contract.Activity, numActivities)
	versions := make(map[uuid.UUID]*contentcontract.Version)

	for i := 0; i < numActivities; i++ {
		vID := uuid.New()
		activities[i] = contract.Activity{
			ID:               uuid.New(),
			LessonID:         lessonID,
			Position:         i + 1,
			Kind:             "multiple_choice",
			ContentVersionID: vID,
			Weight:           1,
		}
		versions[vID] = &contentcontract.Version{
			ID:     vID,
			Kind:   "multiple_choice",
			Status: statusPublished,
		}
	}

	repo := &fakeLessonRepo{
		lesson: &contract.Lesson{
			ID:         lessonID,
			UnitID:     unitID,
			Position:   1,
			Title:      "Batched Content Test Lesson",
			Status:     statusPublished,
			Activities: activities,
		},
		activities: activities,
	}

	contentReader := &countingContentReader{
		versions: versions,
	}

	svc := service.New(service.Deps{
		Repo:    repo,
		Content: contentReader,
	})

	ctx := context.Background()
	detail, err := svc.GetLessonDetail(ctx, lessonID, uuid.New())
	if err != nil {
		t.Fatalf("GetLessonDetail failed: %v", err)
	}

	if len(detail.Activities) != numActivities {
		t.Fatalf("got %d activities, want %d", len(detail.Activities), numActivities)
	}

	for i, act := range detail.Activities {
		if act.Content == nil {
			t.Errorf("activity %d has nil content", i)
		} else if act.Content.ID != act.ContentVersionID {
			t.Errorf("activity %d content ID mismatch: got %v, want %v", i, act.Content.ID, act.ContentVersionID)
		}
	}

	// Trap 4 assertion: exactly 1 call to GetManyVersions, 0 calls to GetVersion
	if got := contentReader.getManyVersionsCount.Load(); got != 1 {
		t.Errorf("GetManyVersions was called %d times; want exactly 1 (batched resolution)", got)
	}
	if got := contentReader.getVersionCount.Load(); got != 0 {
		t.Errorf("GetVersion was called %d times; want 0 (N+1 query failure!)", got)
	}
}
