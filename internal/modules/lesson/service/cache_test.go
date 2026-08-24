package service_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
)

type memCache[T any] struct {
	mu        sync.RWMutex
	store     map[string]T
	delErr    error
	getErr    error
	setErr    error
	loadCalls atomic.Int64
	hitCalls  atomic.Int64
}

func newMemCache[T any]() *memCache[T] {
	return &memCache[T]{
		store: make(map[string]T),
	}
}

func (m *memCache[T]) Get(_ context.Context, key string) (T, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getErr != nil {
		var zero T
		return zero, m.getErr
	}
	val, ok := m.store[key]
	if !ok {
		var zero T
		return zero, errors.New("miss")
	}
	m.hitCalls.Add(1)
	return val, nil
}

func (m *memCache[T]) Set(_ context.Context, key string, val T, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setErr != nil {
		return m.setErr
	}
	m.store[key] = val
	return nil
}

func (m *memCache[T]) Delete(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.delErr != nil {
		return m.delErr
	}
	for _, k := range keys {
		delete(m.store, k)
	}
	return nil
}

func (m *memCache[T]) GetOrLoad(
	ctx context.Context, key string, ttl time.Duration, loader func(ctx context.Context) (T, error),
) (T, error) {
	if m.getErr == nil {
		m.mu.RLock()
		val, ok := m.store[key]
		m.mu.RUnlock()
		if ok {
			m.hitCalls.Add(1)
			return val, nil
		}
	}

	m.loadCalls.Add(1)
	val, err := loader(ctx)
	if err != nil {
		var zero T
		return zero, err
	}

	_ = m.Set(ctx, key, val, ttl)
	return val, nil
}

func (m *memCache[T]) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.store[key]
	return ok
}

type fakeEvents struct {
	mu     sync.Mutex
	events []string
}

func (f *fakeEvents) Write(_ context.Context, _ service.OutboxTx, _, event string, _ any) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
	return uuid.New(), nil
}

func TestCache_GetLessonDetail(t *testing.T) {
	t.Parallel()

	lessonID := uuid.New()
	contentID := uuid.New()
	lesson := &contract.Lesson{
		ID:               lessonID,
		UnitID:           uuid.New(),
		Position:         1,
		Title:            titleLesson1,
		SkillFocus:       "vocabulary",
		EstimatedMinutes: 15,
		Status:           statusPublished,
	}
	acts := []contract.Activity{
		{
			ID:               uuid.New(),
			LessonID:         lessonID,
			Position:         1,
			Kind:             kindVocab,
			ContentVersionID: contentID,
		},
	}
	repo := &fakeLessonRepo{
		lesson:     lesson,
		activities: acts,
	}
	contentReader := &countingContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			contentID: {ID: contentID, Status: statusPublished, Kind: kindVocab, CEFRLevel: "B1"},
		},
	}

	detailCache := newMemCache[*service.LessonDetailDTO]()
	svc := service.New(service.Deps{
		Repo:    repo,
		Content: contentReader,
		Caches: service.LessonCaches{
			Detail: detailCache,
		},
		Env: testEnv,
	})

	ctx := context.Background()
	userID := uuid.New()

	// First call - cache miss
	d1, err := svc.GetLessonDetail(ctx, lessonID, userID)
	if err != nil {
		t.Fatalf("GetLessonDetail first call: %v", err)
	}
	if d1.Title != titleLesson1 || len(d1.Activities) != 1 {
		t.Fatalf("unexpected detail: %+v", d1)
	}
	if contentReader.getManyVersionsCount.Load() != 1 {
		t.Errorf("content reader call count = %d, want 1", contentReader.getManyVersionsCount.Load())
	}
	queriesAfterFirst := repo.queryCounter.Load()

	// Second call - cache hit (0 additional repo queries or content calls)
	d2, err := svc.GetLessonDetail(ctx, lessonID, userID)
	if err != nil {
		t.Fatalf("GetLessonDetail second call: %v", err)
	}
	if d2.Title != titleLesson1 {
		t.Errorf("d2.Title = %q, want %s", d2.Title, titleLesson1)
	}
	if contentReader.getManyVersionsCount.Load() != 1 {
		t.Errorf("content reader call count on cache hit = %d, want 1", contentReader.getManyVersionsCount.Load())
	}
	// Only ListPrerequisitesByLessonID was queried (or 0 for detail)
	if repo.queryCounter.Load() > queriesAfterFirst+1 {
		t.Errorf("queries on cache hit = %d, want <= %d", repo.queryCounter.Load(), queriesAfterFirst+1)
	}
}

func TestCache_GetCourseDetail_DynamicLockEvaluation(t *testing.T) {
	t.Parallel()

	courseID := uuid.New()
	unitID := uuid.New()
	lessonID := uuid.New()
	reqLessonID := uuid.New()

	course := &contract.Course{
		ID:             courseID,
		Slug:           "ielts-master",
		Title:          "IELTS Master",
		CEFRFrom:       "B2",
		CEFRTo:         "C1",
		Status:         statusPublished,
		EstimatedHours: 40,
	}
	unit := &contract.Unit{
		ID:       unitID,
		CourseID: courseID,
		Position: 1,
		Title:    titleUnit1,
	}
	lesson := &contract.Lesson{
		ID:               lessonID,
		UnitID:           unitID,
		Position:         2,
		Title:            titleLesson2,
		Status:           statusPublished,
		EstimatedMinutes: 20,
	}
	prereqs := []service.PrerequisiteItem{
		{
			LessonID:            lessonID,
			RequiresLessonID:    reqLessonID,
			MinScore:            80,
			RequiresLessonTitle: titleLesson1,
		},
	}

	repo := &fakeLessonRepo{
		courses: []*contract.Course{course},
		units:   []*contract.Unit{unit},
		lesson:  lesson,
		prereqs: prereqs,
	}

	treeCache := newMemCache[*service.CourseTreeData]()

	// User 1 is locked, User 2 is unlocked
	unlocker := &dynamicUnlocker{
		unlockedUsers: map[uuid.UUID]bool{},
	}

	svc := service.New(service.Deps{
		Repo:     repo,
		Unlocker: unlocker,
		Caches: service.LessonCaches{
			Tree: treeCache,
		},
		Env: testEnv,
	})

	ctx := context.Background()
	user1 := uuid.New()
	user2 := uuid.New()
	unlocker.unlockedUsers[user2] = true

	// User 1 fetches course detail -> cache miss, evaluates lock -> locked
	d1, err := svc.GetCourseDetail(ctx, "ielts-master", user1)
	if err != nil {
		t.Fatalf("user1 GetCourseDetail: %v", err)
	}
	if !d1.Units[0].Lessons[0].Locked {
		t.Errorf("user1 lesson should be locked")
	}

	queriesAfterFirst := repo.queryCounter.Load()

	// User 2 fetches course detail -> cache hit (0 DB queries to load tree), evaluates lock -> unlocked!
	d2, err := svc.GetCourseDetail(ctx, "ielts-master", user2)
	if err != nil {
		t.Fatalf("user2 GetCourseDetail: %v", err)
	}
	if d2.Units[0].Lessons[0].Locked {
		t.Errorf("user2 lesson should be unlocked")
	}

	if repo.queryCounter.Load() != queriesAfterFirst {
		t.Errorf("queries after cache hit = %d, want %d", repo.queryCounter.Load(), queriesAfterFirst)
	}
}

type dynamicUnlocker struct {
	unlockedUsers map[uuid.UUID]bool
}

func (d *dynamicUnlocker) IsUnlocked(
	_ context.Context, userID uuid.UUID, lessonIDs []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	res := make(map[uuid.UUID]bool, len(lessonIDs))
	for _, id := range lessonIDs {
		res[id] = d.unlockedUsers[userID]
	}
	return res, nil
}

func TestCache_CatalogueGenerationCounter_Invalidation(t *testing.T) {
	t.Parallel()

	courseID := uuid.New()
	unitID := uuid.New()
	lessonID := uuid.New()
	contentID := uuid.New()

	course := &contract.Course{
		ID:             courseID,
		Slug:           "ielts-gen-test",
		Title:          "IELTS Gen Test",
		CEFRFrom:       "B1",
		CEFRTo:         "B2",
		Status:         statusPublished,
		EstimatedHours: 20,
	}
	unit := &contract.Unit{
		ID:       unitID,
		CourseID: courseID,
		Position: 1,
		Title:    titleUnit1,
	}
	lesson := &contract.Lesson{
		ID:               lessonID,
		UnitID:           unitID,
		Position:         1,
		Title:            titleLesson1,
		Status:           statusDraft,
		EstimatedMinutes: 10,
	}
	acts := []contract.Activity{
		{
			ID:               uuid.New(),
			LessonID:         lessonID,
			Position:         1,
			Kind:             kindQuiz,
			ContentVersionID: contentID,
		},
	}

	repo := &fakeLessonRepo{
		courses:    []*contract.Course{course},
		units:      []*contract.Unit{unit},
		lesson:     lesson,
		activities: acts,
	}
	contentReader := &countingContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			contentID: {ID: contentID, Status: statusPublished, Kind: kindQuiz, CEFRLevel: "B1"},
		},
	}

	catCache := newMemCache[*service.CatalogueData]()
	genCache := newMemCache[int64]()
	detailCache := newMemCache[*service.LessonDetailDTO]()
	treeCache := newMemCache[*service.CourseTreeData]()
	events := &fakeEvents{}

	svc := service.New(service.Deps{
		Repo:    repo,
		Content: contentReader,
		Events:  events,
		Caches: service.LessonCaches{
			Catalogue: catCache,
			Gen:       genCache,
			Detail:    detailCache,
			Tree:      treeCache,
		},
		Env: testEnv,
	})

	ctx := context.Background()

	// 1. ListCourses populates catalogue cache (gen 1)
	courses1, total1, err := svc.ListCourses(ctx, nil, 10, 0)
	if err != nil {
		t.Fatalf("ListCourses: %v", err)
	}
	if len(courses1) != 1 || total1 != 1 {
		t.Fatalf("unexpected courses1: %+v", courses1)
	}

	queriesAfterFirst := repo.queryCounter.Load()

	// 2. Second ListCourses hits catalogue cache (0 DB queries)
	courses2, _, err := svc.ListCourses(ctx, nil, 10, 0)
	if err != nil {
		t.Fatalf("ListCourses second call: %v", err)
	}
	if len(courses2) != 1 {
		t.Fatalf("unexpected courses2: %+v", courses2)
	}
	if repo.queryCounter.Load() != queriesAfterFirst {
		t.Errorf("queries after cache hit = %d, want %d", repo.queryCounter.Load(), queriesAfterFirst)
	}

	// 3. Publish lesson -> bumps generation counter and synchronously invalidates detail and tree caches
	_, err = svc.PublishLesson(ctx, uuid.New(), lessonID)
	if err != nil {
		t.Fatalf("PublishLesson: %v", err)
	}

	// 4. Third ListCourses misses cache because generation changed!
	courses3, _, err := svc.ListCourses(ctx, nil, 10, 0)
	if err != nil {
		t.Fatalf("ListCourses third call: %v", err)
	}
	if len(courses3) != 1 {
		t.Fatalf("unexpected courses3: %+v", courses3)
	}
	if repo.queryCounter.Load() <= queriesAfterFirst {
		t.Errorf("expected DB query after generation bump, but queries = %d", repo.queryCounter.Load())
	}
}

func TestCache_HandleContentArchived_InvalidatesDetail(t *testing.T) {
	t.Parallel()

	contentID := uuid.New()
	lesson1ID := uuid.New()
	lesson2ID := uuid.New()

	repo := &fakeLessonRepo{
		activities: []contract.Activity{
			{ID: uuid.New(), LessonID: lesson1ID, ContentVersionID: contentID},
			{ID: uuid.New(), LessonID: lesson2ID, ContentVersionID: contentID},
		},
	}

	detailCache := newMemCache[*service.LessonDetailDTO]()
	detailKey1 := fmt.Sprintf("fluentra:test:lesson:detail:%s:v1", lesson1ID)
	detailKey2 := fmt.Sprintf("fluentra:test:lesson:detail:%s:v1", lesson2ID)

	_ = detailCache.Set(context.Background(), detailKey1, &service.LessonDetailDTO{ID: lesson1ID}, time.Hour)
	_ = detailCache.Set(context.Background(), detailKey2, &service.LessonDetailDTO{ID: lesson2ID}, time.Hour)

	if !detailCache.Has(detailKey1) || !detailCache.Has(detailKey2) {
		t.Fatalf("keys should exist in cache before archive")
	}

	svc := service.New(service.Deps{
		Repo: repo,
		Caches: service.LessonCaches{
			Detail: detailCache,
		},
		Env: testEnv,
	})

	// Content archived event handler
	err := svc.HandleContentArchived(context.Background(), contentID)
	if err != nil {
		t.Fatalf("HandleContentArchived: %v", err)
	}

	if detailCache.Has(detailKey1) {
		t.Errorf("detailKey1 was not deleted on content archive")
	}
	if detailCache.Has(detailKey2) {
		t.Errorf("detailKey2 was not deleted on content archive")
	}
}

func TestCache_ResilienceOnCacheError(t *testing.T) {
	t.Parallel()

	lessonID := uuid.New()
	course := &contract.Course{
		ID:             uuid.New(),
		Slug:           "error-resilience",
		Title:          "Error Resilience",
		CEFRFrom:       "B1",
		CEFRTo:         "B2",
		Status:         statusPublished,
		EstimatedHours: 10,
	}
	lesson := &contract.Lesson{
		ID:               lessonID,
		UnitID:           uuid.New(),
		Position:         1,
		Title:            "Lesson Resilient",
		Status:           statusPublished,
		EstimatedMinutes: 10,
	}
	repo := &fakeLessonRepo{
		courses: []*contract.Course{course},
		lesson:  lesson,
	}

	failingDetailCache := newMemCache[*service.LessonDetailDTO]()
	failingDetailCache.getErr = errors.New("redis connection refused")
	failingDetailCache.setErr = errors.New("redis connection refused")

	svc := service.New(service.Deps{
		Repo: repo,
		Caches: service.LessonCaches{
			Detail: failingDetailCache,
		},
		Env: testEnv,
	})

	// When cache is down, GetLessonDetail falls through to repo and succeeds (Trap 3)
	detail, err := svc.GetLessonDetail(context.Background(), lessonID, uuid.New())
	if err != nil {
		t.Fatalf("GetLessonDetail should not fail on cache error: %v", err)
	}
	if detail.Title != "Lesson Resilient" {
		t.Errorf("detail.Title = %q, want Lesson Resilient", detail.Title)
	}
}
