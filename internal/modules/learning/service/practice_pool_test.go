package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/jackc/pgx/v5"
)


// poolMockRepo implements the practice pool methods for in-memory unit testing.
type poolMockRepo struct {
	*fakeLearningRepo
	mu                sync.Mutex
	dailySets         map[string]*domain.DailySet
	exposures         map[string]time.Time // key: userID:activityID -> first_served_at
	poolActivities    map[string][]domain.PoolActivity // key: level:kind
	poolLessonIDs     map[string]uuid.UUID // key: level:title
	hasActiveLearner  map[string]bool
	activitiesByID    map[uuid.UUID]domain.PoolActivity
}

func newPoolMockRepo() *poolMockRepo {
	return &poolMockRepo{
		fakeLearningRepo: newFakeRepo(),
		dailySets:        make(map[string]*domain.DailySet),
		exposures:        make(map[string]time.Time),
		poolActivities:   make(map[string][]domain.PoolActivity),
		poolLessonIDs:    make(map[string]uuid.UUID),
		hasActiveLearner: make(map[string]bool),
		activitiesByID:   make(map[uuid.UUID]domain.PoolActivity),
	}
}

func (r *poolMockRepo) WithTx(_ pgx.Tx) service.Repository {
	return r
}


func (r *poolMockRepo) GetPoolPracticeCourseID(_ context.Context) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (r *poolMockRepo) GetDailySet(_ context.Context, userID uuid.UUID, localDate time.Time) (*domain.DailySet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", userID, localDate.Format("2006-01-02"))
	if set, ok := r.dailySets[key]; ok {
		return set, nil
	}
	return nil, nil
}

func (r *poolMockRepo) CreateDailySet(_ context.Context, userID uuid.UUID, localDate time.Time, activityIDs []uuid.UUID) (*domain.DailySet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", userID, localDate.Format("2006-01-02"))
	set := &domain.DailySet{
		ID:          uuid.New(),
		UserID:      userID,
		LocalDate:   localDate,
		ActivityIDs: activityIDs,
		CreatedAt:   time.Now().UTC(),
	}
	r.dailySets[key] = set
	return set, nil
}

func (r *poolMockRepo) RecordItemExposure(_ context.Context, userID uuid.UUID, activityID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", userID, activityID)
	if _, ok := r.exposures[key]; !ok {
		r.exposures[key] = time.Now().UTC()
	}
	return nil
}

func (r *poolMockRepo) CountActivePoolActivitiesForSlot(_ context.Context, levelTitle string, kind string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", levelTitle, kind)
	return int64(len(r.poolActivities[key])), nil
}

func (r *poolMockRepo) ListPoolActivitiesForSlot(_ context.Context, levelTitle string, kind string) ([]domain.PoolActivity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", levelTitle, kind)
	items := r.poolActivities[key]
	cp := make([]domain.PoolActivity, len(items))
	copy(cp, items)
	return cp, nil
}

func (r *poolMockRepo) ListUnseenPoolActivitiesForSlot(_ context.Context, levelTitle string, kind string, userID uuid.UUID) ([]domain.PoolActivity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", levelTitle, kind)
	all := r.poolActivities[key]
	var unseen []domain.PoolActivity
	for _, act := range all {
		expKey := fmt.Sprintf("%s:%s", userID, act.ID)
		if _, exposed := r.exposures[expKey]; !exposed {
			unseen = append(unseen, act)
		}
	}
	return unseen, nil
}

func (r *poolMockRepo) ListSeenPoolActivitiesForSlotOldestFirst(_ context.Context, levelTitle string, kind string, userID uuid.UUID) ([]domain.PoolActivity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", levelTitle, kind)
	all := r.poolActivities[key]
	var seen []domain.PoolActivity
	for _, act := range all {
		expKey := fmt.Sprintf("%s:%s", userID, act.ID)
		if _, exposed := r.exposures[expKey]; exposed {
			seen = append(seen, act)
		}
	}
	return seen, nil
}

func (r *poolMockRepo) HasActiveUserWithFewUnseenItems(_ context.Context, levelTitle string, kind string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", levelTitle, kind)
	return r.hasActiveLearner[key], nil
}

func (r *poolMockRepo) GetPoolLessonID(_ context.Context, levelTitle string, lessonTitle string) (uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", levelTitle, lessonTitle)
	if id, ok := r.poolLessonIDs[key]; ok {
		return id, nil
	}
	id := uuid.New()
	r.poolLessonIDs[key] = id
	return id, nil
}

func (r *poolMockRepo) ListActivitiesByIDs(_ context.Context, activityIDs []uuid.UUID) ([]domain.PoolActivity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []domain.PoolActivity
	for _, id := range activityIDs {
		if act, ok := r.activitiesByID[id]; ok {
			result = append(result, act)
		}
	}
	return result, nil
}

func (r *poolMockRepo) addPoolActivity(level, kind string, act domain.PoolActivity) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", level, kind)
	r.poolActivities[key] = append(r.poolActivities[key], act)
	r.activitiesByID[act.ID] = act
}

// fakeContentAuthor tracks published items.
type fakeContentAuthor struct {
	published map[string]uuid.UUID
}

func newFakeContentAuthor() *fakeContentAuthor {
	return &fakeContentAuthor{published: make(map[string]uuid.UUID)}
}

func (f *fakeContentAuthor) EnsurePublished(_ context.Context, spec contentcontract.AuthorSpec) (uuid.UUID, error) {
	if id, ok := f.published[spec.Slug]; ok {
		return id, nil
	}
	id := uuid.New()
	f.published[spec.Slug] = id
	return id, nil
}

// fakeLessonAuthor tracks courses, units, lessons and activities.
type fakeLessonAuthor struct {
	appendedActivities []lessoncontract.ActivitySpec
}

func newFakeLessonAuthor() *fakeLessonAuthor {
	return &fakeLessonAuthor{}
}

func (f *fakeLessonAuthor) EnsureCourse(_ context.Context, _ lessoncontract.CourseSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (f *fakeLessonAuthor) EnsureUnit(_ context.Context, _ lessoncontract.UnitSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (f *fakeLessonAuthor) EnsureLesson(_ context.Context, _ lessoncontract.LessonSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (f *fakeLessonAuthor) SyncActivities(_ context.Context, _ uuid.UUID, _ []lessoncontract.ActivitySpec) error {
	return nil
}

func (f *fakeLessonAuthor) AppendActivity(_ context.Context, _ uuid.UUID, act lessoncontract.ActivitySpec) (uuid.UUID, error) {
	f.appendedActivities = append(f.appendedActivities, act)
	return uuid.New(), nil
}

// fakeContentReader tracks versions.
type fakeContentReader struct {
	versions map[uuid.UUID]*contentcontract.Version
}

func newFakeContentReader() *fakeContentReader {
	return &fakeContentReader{versions: make(map[uuid.UUID]*contentcontract.Version)}
}

func (f *fakeContentReader) GetVersion(ctx context.Context, id uuid.UUID) (*contentcontract.Version, error) {
	if v, ok := contentcontract.TempVersionFromContext(ctx, id); ok {
		return v, nil
	}
	if v, ok := f.versions[id]; ok {
		return v, nil
	}
	return nil, nil
}

func (f *fakeContentReader) GetManyVersions(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*contentcontract.Version, error) {
	res := make(map[uuid.UUID]*contentcontract.Version)
	for _, id := range ids {
		if v, ok := f.versions[id]; ok {
			res[id] = v
		}
	}
	return res, nil
}

func (f *fakeContentReader) Browse(_ context.Context, _ contentcontract.BrowseFilter) ([]*contentcontract.Version, int, error) {
	return nil, 0, nil
}

// fakeGrader is a flexible test grader for practice kinds.
type testPracticeGrader struct {
	shouldPass bool
}

func (g *testPracticeGrader) Grade(_ context.Context, req learningcontract.GradeRequest) (learningcontract.GradeResult, error) {
	if g.shouldPass {
		return learningcontract.GradeResult{
			Score:   100,
			Correct: true,
		}, nil
	}
	return learningcontract.GradeResult{
		Score:   0,
		Correct: false,
	}, nil
}


// fakeAIClient handles TaskPracticeGenerate and TaskPracticeSolve.
type mockAIClient struct {
	generateOutput string
	generateErr    error
	solveOutput    string
	solveErr       error
}

func (m *mockAIClient) Complete(_ context.Context, req ai.Request) (ai.Response, error) {
	if req.Task == ai.TaskPracticeGenerate {
		if m.generateErr != nil {
			return ai.Response{}, m.generateErr
		}
		return ai.Response{Text: m.generateOutput}, nil
	}
	if req.Task == ai.TaskPracticeSolve {
		if m.solveErr != nil {
			return ai.Response{}, m.solveErr
		}
		return ai.Response{Text: m.solveOutput}, nil
	}
	return ai.Response{}, nil
}

// --------------------------------------------------------------------------
// Unit Tests
// --------------------------------------------------------------------------

func TestGetDailySet_SecondSetSharesNothingWithFirstWhileUnseenRemain(t *testing.T) {
	t.Parallel()

	repo := newPoolMockRepo()
	contentReader := newFakeContentReader()
	contentAuth := newFakeContentAuthor()
	lessonAuth := newFakeLessonAuthor()

	testClock := clock.NewFake(time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC))

	svc := service.New(service.Deps{
		Repo:          repo,
		Content:       contentReader,
		ContentAuthor: contentAuth,
		LessonAuthor:  lessonAuth,
		Clock:         testClock,
	})

	level := "B1"

	// Seed pool with 2 passages, 10 tense choice, 6 sentence transform items
	for i := 1; i <= 2; i++ {
		verID := uuid.New()
		body := json.RawMessage(fmt.Sprintf(`{"passage": "Passage %d", "questions": []}`, i))
		contentReader.versions[verID] = &contentcontract.Version{ID: verID, Kind: "reading_comprehension", Body: body}
		repo.addPoolActivity(level, "reading_comprehension", domain.PoolActivity{
			ID:               uuid.New(),
			Kind:             "reading_comprehension",
			ContentVersionID: verID,
			Config:           body,
		})
	}
	for i := 1; i <= 10; i++ {
		verID := uuid.New()
		body := json.RawMessage(fmt.Sprintf(`{"prompt": "Tense %d"}`, i))
		contentReader.versions[verID] = &contentcontract.Version{ID: verID, Kind: "grammar_tense_choice", Body: body}
		repo.addPoolActivity(level, "grammar_tense_choice", domain.PoolActivity{
			ID:               uuid.New(),
			Kind:             "grammar_tense_choice",
			ContentVersionID: verID,
			Config:           body,
		})
	}
	for i := 1; i <= 6; i++ {
		verID := uuid.New()
		body := json.RawMessage(fmt.Sprintf(`{"prompt": "Transform %d"}`, i))
		contentReader.versions[verID] = &contentcontract.Version{ID: verID, Kind: "grammar_sentence_transform", Body: body}
		repo.addPoolActivity(level, "grammar_sentence_transform", domain.PoolActivity{
			ID:               uuid.New(),
			Kind:             "grammar_sentence_transform",
			ContentVersionID: verID,
			Config:           body,
		})
	}

	userID := uuid.New()

	// Day 1: Build first set (1 reading + 5 tense + 3 transform = 9 activities)
	set1, err := svc.GetDailySet(context.Background(), userID, level)
	if err != nil {
		t.Fatalf("unexpected error getting day 1 daily set: %v", err)
	}
	if len(set1.Activities) != 9 {
		t.Fatalf("expected 9 activities in day 1 set, got %d", len(set1.Activities))
	}

	set1IDs := make(map[uuid.UUID]bool)
	for _, act := range set1.Activities {
		set1IDs[act.ID] = true
	}

	// Advance clock by 24 hours to Day 2
	testClock.Advance(24 * time.Hour)

	// Day 2: Build second set
	set2, err := svc.GetDailySet(context.Background(), userID, level)
	if err != nil {
		t.Fatalf("unexpected error getting day 2 daily set: %v", err)
	}
	if len(set2.Activities) != 9 {
		t.Fatalf("expected 9 activities in day 2 set, got %d", len(set2.Activities))
	}

	// Assert: No overlap between Day 1 and Day 2 while unseen items remain
	for _, act := range set2.Activities {
		if set1IDs[act.ID] {
			t.Errorf("expected zero overlap between day 1 and day 2, but activity %s appeared in both", act.ID)
		}
	}
}

func TestGetDailySet_SeenEverythingGetsDrawnFromOldestExposures(t *testing.T) {
	t.Parallel()

	repo := newPoolMockRepo()
	contentReader := newFakeContentReader()
	testClock := clock.NewFake(time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC))

	svc := service.New(service.Deps{
		Repo:          repo,
		Content:       contentReader,
		ContentAuthor: newFakeContentAuthor(),
		LessonAuthor:  newFakeLessonAuthor(),
		Clock:         testClock,
	})

	level := "B1"

	// Pool has exactly 1 passage, 5 tense choice, 3 transform (9 items total)
	for i := 1; i <= 1; i++ {
		verID := uuid.New()
		body := json.RawMessage(`{"passage": "Only Passage"}`)
		contentReader.versions[verID] = &contentcontract.Version{ID: verID, Kind: "reading_comprehension", Body: body}
		repo.addPoolActivity(level, "reading_comprehension", domain.PoolActivity{
			ID:               uuid.New(),
			Kind:             "reading_comprehension",
			ContentVersionID: verID,
			Config:           body,
		})
	}
	for i := 1; i <= 5; i++ {
		verID := uuid.New()
		body := json.RawMessage(fmt.Sprintf(`{"prompt": "Tense %d"}`, i))
		contentReader.versions[verID] = &contentcontract.Version{ID: verID, Kind: "grammar_tense_choice", Body: body}
		repo.addPoolActivity(level, "grammar_tense_choice", domain.PoolActivity{
			ID:               uuid.New(),
			Kind:             "grammar_tense_choice",
			ContentVersionID: verID,
			Config:           body,
		})
	}
	for i := 1; i <= 3; i++ {
		verID := uuid.New()
		body := json.RawMessage(fmt.Sprintf(`{"prompt": "Transform %d"}`, i))
		contentReader.versions[verID] = &contentcontract.Version{ID: verID, Kind: "grammar_sentence_transform", Body: body}
		repo.addPoolActivity(level, "grammar_sentence_transform", domain.PoolActivity{
			ID:               uuid.New(),
			Kind:             "grammar_sentence_transform",
			ContentVersionID: verID,
			Config:           body,
		})
	}

	userID := uuid.New()

	// Day 1: User sees all 9 items
	set1, err := svc.GetDailySet(context.Background(), userID, level)
	if err != nil {
		t.Fatalf("day 1 get daily set: %v", err)
	}
	if len(set1.Activities) != 9 {
		t.Fatalf("expected 9 activities, got %d", len(set1.Activities))
	}

	// Advance clock to Day 2
	testClock.Advance(24 * time.Hour)

	// Day 2: User has seen all 9 items; set must still be full (drawn from oldest exposures)
	set2, err := svc.GetDailySet(context.Background(), userID, level)
	if err != nil {
		t.Fatalf("day 2 get daily set: %v", err)
	}
	if len(set2.Activities) != 9 {
		t.Fatalf("expected 9 activities drawn from oldest exposures, got %d", len(set2.Activities))
	}
}

func TestGetDailySet_SameDayReturnsCachedSet(t *testing.T) {
	t.Parallel()

	repo := newPoolMockRepo()
	contentReader := newFakeContentReader()
	testClock := clock.NewFake(time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC))

	svc := service.New(service.Deps{
		Repo:          repo,
		Content:       contentReader,
		ContentAuthor: newFakeContentAuthor(),
		LessonAuthor:  newFakeLessonAuthor(),
		Clock:         testClock,
	})

	level := "B1"
	verID := uuid.New()
	body := json.RawMessage(`{"prompt": "Item 1"}`)
	contentReader.versions[verID] = &contentcontract.Version{ID: verID, Kind: "reading_comprehension", Body: body}
	repo.addPoolActivity(level, "reading_comprehension", domain.PoolActivity{
		ID:               uuid.New(),
		Kind:             "reading_comprehension",
		ContentVersionID: verID,
		Config:           body,
	})

	userID := uuid.New()

	set1, err := svc.GetDailySet(context.Background(), userID, level)
	if err != nil {
		t.Fatalf("get daily set 1: %v", err)
	}

	// Call again within same local date
	testClock.Advance(2 * time.Hour)
	set2, err := svc.GetDailySet(context.Background(), userID, level)
	if err != nil {
		t.Fatalf("get daily set 2: %v", err)
	}

	if set1.ID != set2.ID {
		t.Errorf("expected same daily set ID on same day, got %s and %s", set1.ID, set2.ID)
	}
}

func TestDailySet_RedactionCarriesNoAnswers(t *testing.T) {
	t.Parallel()

	repo := newPoolMockRepo()
	contentReader := newFakeContentReader()
	testClock := clock.NewFake(time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC))

	svc := service.New(service.Deps{
		Repo:          repo,
		Content:       contentReader,
		ContentAuthor: newFakeContentAuthor(),
		LessonAuthor:  newFakeLessonAuthor(),
		Clock:         testClock,
	})

	level := "B1"
	verID := uuid.New()
	unredactedBody := json.RawMessage(`{
		"prompt": "Choose the best answer",
		"correct_option_id": "opt_secret",
		"correct_answer": "secret_text",
		"acceptable": ["secret_alt"],
		"explanation": {"text": "en expl", "text_vi": "vi expl"},
		"options": [{"id": "opt_1", "text": "Option A"}]
	}`)
	contentReader.versions[verID] = &contentcontract.Version{
		ID:        verID,
		Kind:      "grammar_tense_choice",
		Body:      unredactedBody,
		Status:    "published",
		CEFRLevel: level,
	}

	repo.addPoolActivity(level, "grammar_tense_choice", domain.PoolActivity{
		ID:               uuid.New(),
		Kind:             "grammar_tense_choice",
		ContentVersionID: verID,
		Config:           unredactedBody,
	})

	userID := uuid.New()
	dailySet, err := svc.GetDailySet(context.Background(), userID, level)
	if err != nil {
		t.Fatalf("get daily set: %v", err)
	}

	setJSON, err := json.Marshal(dailySet)
	if err != nil {
		t.Fatalf("marshal daily set: %v", err)
	}

	serialized := string(setJSON)
	for _, forbidden := range []string{"opt_secret", "secret_text", "secret_alt"} {
		if strings.Contains(serialized, forbidden) {
			t.Errorf("daily set serialized payload leaked forbidden answer: %q in %s", forbidden, serialized)
		}
	}
}

func TestTopUpPracticePool_BlindSolveDisagreementRejectsCandidate(t *testing.T) {
	t.Parallel()

	repo := newPoolMockRepo()
	contentReader := newFakeContentReader()
	contentAuthor := newFakeContentAuthor()
	lessonAuthor := newFakeLessonAuthor()

	graders := domain.NewGraderRegistry()
	_ = graders.Register("grammar_tense_choice", &testPracticeGrader{shouldPass: false}) // Grader rejects blind solve

	aiMock := &mockAIClient{
		generateOutput: `{
			"prompt": "She ___ to school every day.",
			"options": [
				{"id": "A", "text": "goes"},
				{"id": "B", "text": "go"},
				{"id": "C", "text": "going"},
				{"id": "D", "text": "gone"}
			],
			"correct_option_id": "A",
			"explanation": {
				"explanation_en": "Present simple third person singular.",
				"explanation_vi": "Thì hiện tại đơn ngôi thứ 3 số ít."
			}
		}`,
		solveOutput: `{"selected_option_id": "B"}`, // Disagreeing answer
	}

	svc := service.New(service.Deps{
		Repo:          repo,
		Content:       contentReader,
		ContentAuthor: contentAuthor,
		LessonAuthor:  lessonAuthor,
		Graders:       graders,
		AI:            aiMock,
	})

	err := svc.TopUpPracticePool(context.Background())
	if err != nil {
		t.Fatalf("unexpected top-up error: %v", err)
	}

	// Disagreeing item must NOT be appended to lesson or published
	if len(lessonAuthor.appendedActivities) > 0 {
		t.Errorf("expected 0 activities appended due to blind solve failure, got %d", len(lessonAuthor.appendedActivities))
	}
}

func TestTopUpPracticePool_ThrottlingLimits(t *testing.T) {
	t.Parallel()

	repo := newPoolMockRepo()
	contentReader := newFakeContentReader()
	contentAuthor := newFakeContentAuthor()
	lessonAuthor := newFakeLessonAuthor()

	graders := domain.NewGraderRegistry()
	_ = graders.Register("grammar_tense_choice", &testPracticeGrader{shouldPass: true})
	_ = graders.Register("reading_comprehension", &testPracticeGrader{shouldPass: true})
	_ = graders.Register("grammar_sentence_transform", &testPracticeGrader{shouldPass: true})

	aiMock := &mockAIClient{
		generateOutput: `{
			"prompt": "She ___ home early yesterday.",
			"options": [
				{"id": "A", "text": "came"},
				{"id": "B", "text": "come"},
				{"id": "C", "text": "comes"},
				{"id": "D", "text": "coming"}
			],
			"correct_option_id": "A",
			"explanation": {
				"explanation_en": "Past simple.",
				"explanation_vi": "Quá khứ đơn."
			}
		}`,
		solveOutput: `{"selected_option_id": "A"}`,
	}

	svc := service.New(service.Deps{
		Repo:          repo,
		Content:       contentReader,
		ContentAuthor: contentAuthor,
		LessonAuthor:  lessonAuthor,
		Graders:       graders,
		AI:            aiMock,
	})

	// Baseline: initialize all 9 slots to 50 items with no active learner running low
	levels := []string{"A2", "B1", "B2"}
	kinds := []string{"reading_comprehension", "grammar_tense_choice", "grammar_sentence_transform"}
	for _, l := range levels {
		for _, k := range kinds {
			for i := 0; i < 50; i++ {
				repo.addPoolActivity(l, k, domain.PoolActivity{
					ID:   uuid.New(),
					Kind: k,
				})
			}
			repo.hasActiveLearner[fmt.Sprintf("%s:%s", l, k)] = false
		}
	}

	// Case 1: All slots at 50 and NO active learner with < 10 unseen -> toAdd is 0
	initialCount := len(lessonAuthor.appendedActivities)
	if err := svc.TopUpPracticePool(context.Background()); err != nil {
		t.Fatalf("top up practice pool: %v", err)
	}
	if len(lessonAuthor.appendedActivities) != initialCount {
		t.Errorf("expected 0 items added when slots at 50 have no active learners running low, got %d", len(lessonAuthor.appendedActivities)-initialCount)
	}

	// Case 2: One slot has an active learner running low (< 10 unseen) -> adds 5 items
	targetLevel := "A2"
	targetKind := "grammar_tense_choice"
	repo.hasActiveLearner[fmt.Sprintf("%s:%s", targetLevel, targetKind)] = true

	if err := svc.TopUpPracticePool(context.Background()); err != nil {
		t.Fatalf("top up practice pool: %v", err)
	}
	if len(lessonAuthor.appendedActivities) != initialCount+5 {
		t.Errorf("expected 5 items added when slot has learner running low, got %d", len(lessonAuthor.appendedActivities)-initialCount)
	}

	// Case 3: Slot reaches 200 items (cap) -> never adds anything even if learner is running low
	for i := len(repo.poolActivities[fmt.Sprintf("%s:%s", targetLevel, targetKind)]); i < 200; i++ {
		repo.addPoolActivity(targetLevel, targetKind, domain.PoolActivity{
			ID:   uuid.New(),
			Kind: targetKind,
		})
	}
	repo.hasActiveLearner[fmt.Sprintf("%s:%s", targetLevel, targetKind)] = true
	capCount := len(lessonAuthor.appendedActivities)

	if err := svc.TopUpPracticePool(context.Background()); err != nil {
		t.Fatalf("top up practice pool: %v", err)
	}
	if len(lessonAuthor.appendedActivities) != capCount {
		t.Errorf("expected 0 items added when slot is at 200 cap, got %d", len(lessonAuthor.appendedActivities)-capCount)
	}
}
