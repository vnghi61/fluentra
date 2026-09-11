package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const (
	poolLevel          = "B1"
	poolKindReading    = "reading_comprehension"
	poolKindTense      = "grammar_tense_choice"
	poolKindTransform  = "grammar_sentence_transform"
	poolTitleReading   = "Reading Comprehension"
	poolTitleTense     = "Grammar Tense Choice"
	poolTitleTransform = "Grammar Sentence Transform"
)

var poolSlots = []struct{ kind, title string }{
	{kind: poolKindReading, title: poolTitleReading},
	{kind: poolKindTense, title: poolTitleTense},
	{kind: poolKindTransform, title: poolTitleTransform},
}

// --------------------------------------------------------------------------
// fakePoolRepo: learning's own tables — daily_sets and item_exposures
// --------------------------------------------------------------------------

type fakePoolRepo struct {
	*fakeLearningRepo

	poolMu    sync.Mutex
	sets      map[string]*domain.DailySet
	exposures map[string]time.Time
	tick      time.Time
	// raceWinner, when set, is a set another request stores just before this
	// request's insert, which then gets no row back.
	raceWinner []uuid.UUID
	// runningLow marks activities whose slot has an active learner running low.
	runningLow map[uuid.UUID]bool
}

func newFakePoolRepo() *fakePoolRepo {
	return &fakePoolRepo{
		fakeLearningRepo: newFakeRepo(),
		sets:             map[string]*domain.DailySet{},
		exposures:        map[string]time.Time{},
		tick:             time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		runningLow:       map[uuid.UUID]bool{},
	}
}

func dailySetKey(userID uuid.UUID, localDate time.Time) string {
	return userID.String() + "/" + localDate.Format("2006-01-02")
}

func exposureKey(userID, activityID uuid.UUID) string {
	return userID.String() + "/" + activityID.String()
}

func (r *fakePoolRepo) WithTx(_ pgx.Tx) service.Repository {
	return r
}

func (r *fakePoolRepo) GetDailySet(_ context.Context, userID uuid.UUID, localDate time.Time) (*domain.DailySet, error) {
	r.poolMu.Lock()
	defer r.poolMu.Unlock()
	return r.sets[dailySetKey(userID, localDate)], nil
}

// CreateDailySet mirrors ON CONFLICT DO NOTHING: nil when a set already exists.
func (r *fakePoolRepo) CreateDailySet(
	_ context.Context, userID uuid.UUID, localDate time.Time, activityIDs []uuid.UUID,
) (*domain.DailySet, error) {
	r.poolMu.Lock()
	defer r.poolMu.Unlock()
	key := dailySetKey(userID, localDate)
	if r.raceWinner != nil {
		r.sets[key] = &domain.DailySet{ID: uuid.New(), UserID: userID, LocalDate: localDate, ActivityIDs: r.raceWinner}
		r.raceWinner = nil
		return nil, nil
	}
	if _, exists := r.sets[key]; exists {
		return nil, nil
	}
	set := &domain.DailySet{ID: uuid.New(), UserID: userID, LocalDate: localDate, ActivityIDs: activityIDs}
	r.sets[key] = set
	return set, nil
}

func (r *fakePoolRepo) RecordItemExposure(_ context.Context, userID, activityID uuid.UUID) error {
	r.poolMu.Lock()
	defer r.poolMu.Unlock()
	r.tick = r.tick.Add(time.Minute)
	r.exposures[exposureKey(userID, activityID)] = r.tick
	return nil
}

func (r *fakePoolRepo) ListItemExposures(
	_ context.Context, userID uuid.UUID, activityIDs []uuid.UUID,
) (map[uuid.UUID]time.Time, error) {
	r.poolMu.Lock()
	defer r.poolMu.Unlock()
	out := map[uuid.UUID]time.Time{}
	for _, id := range activityIDs {
		if served, ok := r.exposures[exposureKey(userID, id)]; ok {
			out[id] = served
		}
	}
	return out, nil
}

func (r *fakePoolRepo) HasActiveLearnerRunningLow(_ context.Context, activityIDs []uuid.UUID, _ int) (bool, error) {
	r.poolMu.Lock()
	defer r.poolMu.Unlock()
	for _, id := range activityIDs {
		if r.runningLow[id] {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakePoolRepo) exposureCount() int {
	r.poolMu.Lock()
	defer r.poolMu.Unlock()
	return len(r.exposures)
}

// --------------------------------------------------------------------------
// fakePoolLessons: lesson's contract, both Reader and Author
// --------------------------------------------------------------------------

type fakePoolLessons struct {
	mu         sync.Mutex
	ids        map[string]uuid.UUID
	unitTitles map[uuid.UUID]string
	slotLesson map[string]uuid.UUID
	activities map[uuid.UUID][]lessoncontract.Activity
	byID       map[uuid.UUID]lessoncontract.Activity
	appended   []lessoncontract.ActivitySpec
}

func newFakePoolLessons() *fakePoolLessons {
	return &fakePoolLessons{
		ids:        map[string]uuid.UUID{},
		unitTitles: map[uuid.UUID]string{},
		slotLesson: map[string]uuid.UUID{},
		activities: map[uuid.UUID][]lessoncontract.Activity{},
		byID:       map[uuid.UUID]lessoncontract.Activity{},
	}
}

var (
	_ lessoncontract.Reader = (*fakePoolLessons)(nil)
	_ lessoncontract.Author = (*fakePoolLessons)(nil)
)

func (l *fakePoolLessons) stableID(key string) uuid.UUID {
	if id, ok := l.ids[key]; ok {
		return id
	}
	id := uuid.New()
	l.ids[key] = id
	return id
}

func (l *fakePoolLessons) EnsureCourse(_ context.Context, spec lessoncontract.CourseSpec) (uuid.UUID, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stableID("course/" + spec.Slug), nil
}

func (l *fakePoolLessons) EnsureUnit(_ context.Context, spec lessoncontract.UnitSpec) (uuid.UUID, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := l.stableID(fmt.Sprintf("unit/%s/%d", spec.CourseID, spec.Position))
	l.unitTitles[id] = spec.Title
	return id, nil
}

func (l *fakePoolLessons) EnsureLesson(_ context.Context, spec lessoncontract.LessonSpec) (uuid.UUID, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := l.stableID(fmt.Sprintf("lesson/%s/%d", spec.UnitID, spec.Position))
	l.slotLesson[l.unitTitles[spec.UnitID]+"/"+spec.Title] = id
	return id, nil
}

func (l *fakePoolLessons) SyncActivities(_ context.Context, _ uuid.UUID, _ []lessoncontract.ActivitySpec) error {
	return nil
}

func (l *fakePoolLessons) AppendActivity(
	_ context.Context, lessonID uuid.UUID, spec lessoncontract.ActivitySpec,
) (uuid.UUID, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.appended = append(l.appended, spec)
	return l.addLocked(lessonID, spec), nil
}

func (l *fakePoolLessons) addLocked(lessonID uuid.UUID, spec lessoncontract.ActivitySpec) uuid.UUID {
	activity := lessoncontract.Activity{
		ID:               uuid.New(),
		LessonID:         lessonID,
		Position:         len(l.activities[lessonID]) + 1,
		Kind:             spec.Kind,
		ContentVersionID: spec.ContentVersionID,
		Config:           spec.Config,
		Weight:           1,
	}
	l.activities[lessonID] = append(l.activities[lessonID], activity)
	l.byID[activity.ID] = activity
	return activity.ID
}

// seed adds an item to a slot without counting it as one the top-up generated.
func (l *fakePoolLessons) seed(
	t *testing.T, level, title, kind string, versionID uuid.UUID, body json.RawMessage,
) uuid.UUID {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	lessonID, ok := l.slotLesson[level+"/"+title]
	if !ok {
		t.Fatalf("no pool lesson for %s/%s; resolve the pool structure first", level, title)
	}
	return l.addLocked(lessonID, lessoncontract.ActivitySpec{Kind: kind, ContentVersionID: versionID, Config: body})
}

func (l *fakePoolLessons) appendedCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.appended)
}

func (l *fakePoolLessons) GetLesson(_ context.Context, id uuid.UUID) (*lessoncontract.Lesson, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return &lessoncontract.Lesson{ID: id, Activities: append([]lessoncontract.Activity(nil), l.activities[id]...)}, nil
}

func (l *fakePoolLessons) ResolveActivity(_ context.Context, id uuid.UUID) (*lessoncontract.ActivityHierarchy, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	activity, ok := l.byID[id]
	if !ok {
		return nil, fmt.Errorf("activity %s not found", id)
	}
	return &lessoncontract.ActivityHierarchy{
		ActivityID:       activity.ID,
		LessonID:         activity.LessonID,
		Kind:             activity.Kind,
		ContentVersionID: activity.ContentVersionID,
		Config:           activity.Config,
		Weight:           activity.Weight,
	}, nil
}

func (l *fakePoolLessons) ListLessons(context.Context, uuid.UUID) ([]*lessoncontract.Lesson, error) {
	return nil, nil
}

func (l *fakePoolLessons) ListUnitsByCourseID(context.Context, uuid.UUID) ([]*lessoncontract.Unit, error) {
	return nil, nil
}

func (l *fakePoolLessons) ListPrerequisitesForLessons(
	context.Context, []uuid.UUID,
) ([]lessoncontract.PrerequisiteItem, error) {
	return nil, nil
}

func (l *fakePoolLessons) ListActivitiesByCourseIDs(context.Context, []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	return map[uuid.UUID][]uuid.UUID{}, nil
}

func (l *fakePoolLessons) NextLesson(context.Context, uuid.UUID, *uuid.UUID) (*lessoncontract.Lesson, error) {
	return nil, nil
}

// --------------------------------------------------------------------------
// Content, graders and the model
// --------------------------------------------------------------------------

// fakeContentAuthor refuses an item with no owner, as content's EnsurePublished
// does. The fake this replaced accepted one, which is how a pool that could never
// publish a single item passed its tests.
type fakeContentAuthor struct {
	mu    sync.Mutex
	specs []contentcontract.AuthorSpec
}

func (f *fakeContentAuthor) EnsurePublished(_ context.Context, spec contentcontract.AuthorSpec) (uuid.UUID, error) {
	if spec.AuthorID == uuid.Nil {
		return uuid.Nil, errors.New("authored content needs an author")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.specs = append(f.specs, spec)
	return uuid.New(), nil
}

func (f *fakeContentAuthor) published() []contentcontract.AuthorSpec {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]contentcontract.AuthorSpec(nil), f.specs...)
}

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
	return f.versions[id], nil
}

func (f *fakeContentReader) GetManyVersions(
	_ context.Context, ids []uuid.UUID,
) (map[uuid.UUID]*contentcontract.Version, error) {
	res := make(map[uuid.UUID]*contentcontract.Version)
	for _, id := range ids {
		if v, ok := f.versions[id]; ok {
			res[id] = v
		}
	}
	return res, nil
}

func (f *fakeContentReader) Browse(
	_ context.Context, _ contentcontract.BrowseFilter,
) ([]*contentcontract.Version, int, error) {
	return nil, 0, nil
}

// solveOptionA is a blind solve that picks option A, the key keyGrader is built with.
const solveOptionA = `{"selected_option_id": "A"}`

// testPracticeGrader passes or fails every response.
type testPracticeGrader struct {
	shouldPass bool
}

func (g *testPracticeGrader) Grade(
	_ context.Context, _ learningcontract.GradeRequest,
) (learningcontract.GradeResult, error) {
	if g.shouldPass {
		return learningcontract.GradeResult{Score: 100, Correct: true}, nil
	}
	return learningcontract.GradeResult{}, nil
}

// keyGrader passes a response that selects its option and fails any other, so a
// blind solve can disagree with an item's own key.
type keyGrader struct {
	option string
}

func (g keyGrader) Grade(_ context.Context, req learningcontract.GradeRequest) (learningcontract.GradeResult, error) {
	var response struct {
		SelectedOptionID string `json:"selected_option_id"`
	}
	_ = json.Unmarshal(req.Response, &response)
	if response.SelectedOptionID == g.option {
		return learningcontract.GradeResult{Score: 100, Correct: true}, nil
	}
	return learningcontract.GradeResult{}, nil
}

// cannedPracticeAI answers practice_generate with a different prompt on each call,
// so candidates are not rejected as duplicates of each other, and practice_solve
// with a fixed answer.
type cannedPracticeAI struct {
	mu        sync.Mutex
	generated int
	solve     string
}

func (c *cannedPracticeAI) Complete(_ context.Context, req ai.Request) (ai.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch req.Task {
	case ai.TaskPracticeGenerate:
		c.generated++
		return ai.Response{Text: tenseChoiceItem(c.generated)}, nil
	case ai.TaskPracticeSolve:
		return ai.Response{Text: c.solve}, nil
	default:
		return ai.Response{}, nil
	}
}

func (c *cannedPracticeAI) generatedCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generated
}

func tenseChoiceItem(n int) string {
	return fmt.Sprintf(`{
		"prompt": "Item %d: she ___ home early yesterday.",
		"options": [
			{"id": "A", "text": "came"},
			{"id": "B", "text": "come"},
			{"id": "C", "text": "comes"},
			{"id": "D", "text": "coming"}
		],
		"correct_option_id": "A",
		"explanation": {"explanation_en": "Past simple.", "explanation_vi": "Quá khứ đơn."}
	}`, n)
}

// --------------------------------------------------------------------------
// Fixture
// --------------------------------------------------------------------------

type poolFixture struct {
	svc     *service.Service
	repo    *fakePoolRepo
	lessons *fakePoolLessons
	content *fakeContentReader
	authors *fakeContentAuthor
}

func newPoolFixture(
	t *testing.T, clk clock.Clock, author uuid.UUID, graders *domain.GraderRegistry, aiClient ai.Client,
) poolFixture {
	t.Helper()
	f := poolFixture{
		repo:    newFakePoolRepo(),
		lessons: newFakePoolLessons(),
		content: newFakeContentReader(),
		authors: &fakeContentAuthor{},
	}
	f.svc = service.New(service.Deps{
		Repo:              f.repo,
		Lesson:            f.lessons,
		LessonAuthor:      f.lessons,
		Content:           f.content,
		ContentAuthor:     f.authors,
		Graders:           graders,
		AI:                aiClient,
		Clock:             clk,
		GeneratorAuthorID: author,
	})
	if err := f.svc.EnsurePracticePoolStructure(context.Background()); err != nil {
		t.Fatalf("resolve practice pool: %v", err)
	}
	return f
}

// seed fills slots at a level with prompt-only items and returns their ids by kind.
func (f poolFixture) seed(t *testing.T, level string, counts map[string]int) map[string][]uuid.UUID {
	t.Helper()
	seeded := map[string][]uuid.UUID{}
	for _, slot := range poolSlots {
		for i := 0; i < counts[slot.kind]; i++ {
			versionID := uuid.New()
			body := json.RawMessage(fmt.Sprintf(`{"prompt": "%s %s %d"}`, level, slot.kind, i))
			f.content.versions[versionID] = &contentcontract.Version{ID: versionID, Kind: slot.kind, Body: body}
			seeded[slot.kind] = append(seeded[slot.kind], f.lessons.seed(t, level, slot.title, slot.kind, versionID, body))
		}
	}
	return seeded
}

func passingGraders() *domain.GraderRegistry {
	graders := domain.NewGraderRegistry()
	for _, slot := range poolSlots {
		_ = graders.Register(slot.kind, &testPracticeGrader{shouldPass: true})
	}
	return graders
}

func dailySetIDs(t *testing.T, svc *service.Service, userID uuid.UUID) []uuid.UUID {
	t.Helper()
	set, err := svc.GetDailySet(context.Background(), userID, poolLevel)
	if err != nil {
		t.Fatalf("GetDailySet: %v", err)
	}
	ids := make([]uuid.UUID, 0, len(set.Activities))
	for _, activity := range set.Activities {
		ids = append(ids, activity.ID)
	}
	return ids
}

func testClock() *clock.Fake {
	return clock.NewFake(time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC))
}

// --------------------------------------------------------------------------
// The daily set
// --------------------------------------------------------------------------

func TestGetDailySet_SecondSetSharesNothingWithFirstWhileUnseenRemain(t *testing.T) {
	t.Parallel()
	clk := testClock()
	f := newPoolFixture(t, clk, uuid.New(), nil, nil)
	f.seed(t, poolLevel, map[string]int{poolKindReading: 2, poolKindTense: 10, poolKindTransform: 6})
	userID := uuid.New()

	day1 := dailySetIDs(t, f.svc, userID)
	clk.Advance(24 * time.Hour)
	day2 := dailySetIDs(t, f.svc, userID)

	if len(day1) != 9 || len(day2) != 9 {
		t.Fatalf("expected two sets of 9, got %d and %d", len(day1), len(day2))
	}
	first := map[uuid.UUID]bool{}
	for _, id := range day1 {
		first[id] = true
	}
	for _, id := range day2 {
		if first[id] {
			t.Errorf("activity %s appeared on both days while unseen items remained", id)
		}
	}
}

func TestGetDailySet_SeenEverythingDrawsTheUnseenFirstThenTheOldest(t *testing.T) {
	t.Parallel()
	clk := testClock()
	f := newPoolFixture(t, clk, uuid.New(), nil, nil)
	seeded := f.seed(t, poolLevel, map[string]int{poolKindReading: 1, poolKindTense: 6, poolKindTransform: 3})
	userID := uuid.New()

	day1 := dailySetIDs(t, f.svc, userID)
	shown := map[uuid.UUID]bool{}
	for _, id := range day1 {
		shown[id] = true
	}
	var unseenTense uuid.UUID
	for _, id := range seeded[poolKindTense] {
		if !shown[id] {
			unseenTense = id
		}
	}

	clk.Advance(24 * time.Hour)
	day2 := dailySetIDs(t, f.svc, userID)

	if len(day2) != 9 {
		t.Fatalf("expected a full set of 9 drawn partly from seen items, got %d", len(day2))
	}
	found := false
	for _, id := range day2 {
		if id == unseenTense {
			found = true
		}
	}
	if !found {
		t.Errorf("the one tense item the learner had not seen was not drawn before the seen ones")
	}
}

func TestGetDailySet_SameDayReturnsTheStoredSet(t *testing.T) {
	t.Parallel()
	clk := testClock()
	f := newPoolFixture(t, clk, uuid.New(), nil, nil)
	f.seed(t, poolLevel, map[string]int{poolKindReading: 2, poolKindTense: 10, poolKindTransform: 6})
	userID := uuid.New()

	first := dailySetIDs(t, f.svc, userID)
	clk.Advance(2 * time.Hour)
	second := dailySetIDs(t, f.svc, userID)

	if strings.Join(idStrings(first), ",") != strings.Join(idStrings(second), ",") {
		t.Errorf("expected the same set twice in one day")
	}
}

// TestGetDailySet_LosingTheRaceReturnsTheStoredSetAndRecordsNothing. Two requests
// on a learner's first open of the day: the one that stores second must answer
// with the stored set and record no exposures, because the items it drew for
// itself were never shown to anyone.
func TestGetDailySet_LosingTheRaceReturnsTheStoredSetAndRecordsNothing(t *testing.T) {
	t.Parallel()
	f := newPoolFixture(t, testClock(), uuid.New(), nil, nil)
	seeded := f.seed(t, poolLevel, map[string]int{poolKindReading: 2, poolKindTense: 10, poolKindTransform: 6})

	winner := []uuid.UUID{seeded[poolKindReading][1]}
	winner = append(winner, seeded[poolKindTense][5:]...)
	winner = append(winner, seeded[poolKindTransform][3:]...)
	f.repo.raceWinner = winner

	got := dailySetIDs(t, f.svc, uuid.New())

	if strings.Join(idStrings(got), ",") != strings.Join(idStrings(winner), ",") {
		t.Errorf("expected the stored set\n got: %v\nwant: %v", got, winner)
	}
	if n := f.repo.exposureCount(); n != 0 {
		t.Errorf("the losing request recorded %d exposures for items it never showed", n)
	}
}

func TestDailySet_RedactionCarriesNoAnswers(t *testing.T) {
	t.Parallel()
	f := newPoolFixture(t, testClock(), uuid.New(), nil, nil)

	versionID := uuid.New()
	body := json.RawMessage(`{
		"prompt": "Choose the best answer",
		"correct_option_id": "opt_secret",
		"correct_answer": "secret_text",
		"acceptable": ["secret_alt"],
		"explanation": {"text": "en expl", "text_vi": "vi expl"},
		"options": [{"id": "opt_1", "text": "Option A"}]
	}`)
	f.content.versions[versionID] = &contentcontract.Version{
		ID: versionID, Kind: poolKindTense, Body: body, Status: "published", CEFRLevel: poolLevel,
	}
	f.lessons.seed(t, poolLevel, poolTitleTense, poolKindTense, versionID, body)

	set, err := f.svc.GetDailySet(context.Background(), uuid.New(), poolLevel)
	if err != nil {
		t.Fatalf("GetDailySet: %v", err)
	}
	serialized, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("marshal daily set: %v", err)
	}
	for _, secret := range []string{"opt_secret", "secret_text", "secret_alt"} {
		if strings.Contains(string(serialized), secret) {
			t.Errorf("daily set leaked %q", secret)
		}
	}
}

// TestPoolCourseIsLeftOffProgressAndNextActivity. Opening today's practice enrols
// the learner in the pool's course. The dashboard continues the newest enrolment,
// so without the filter the pool became "continue learning" and a course in the
// learner's progress.
func TestPoolCourseIsLeftOffProgressAndNextActivity(t *testing.T) {
	t.Parallel()
	f := newPoolFixture(t, testClock(), uuid.New(), nil, nil)
	f.seed(t, poolLevel, map[string]int{poolKindReading: 1, poolKindTense: 5, poolKindTransform: 3})
	userID := uuid.New()
	_ = dailySetIDs(t, f.svc, userID)

	progress, err := f.svc.Progress(context.Background(), userID)
	if err != nil {
		t.Fatalf("Progress: %v", err)
	}
	if len(progress.Courses) != 0 {
		t.Errorf("expected no courses in progress, got %d (the practice pool)", len(progress.Courses))
	}

	next, err := f.svc.NextActivity(context.Background(), userID)
	if err != nil {
		t.Fatalf("NextActivity: %v", err)
	}
	if next.State != domain.StateNotStarted {
		t.Errorf("expected not_started for a learner enrolled only in the pool, got %s", next.State)
	}
}

// --------------------------------------------------------------------------
// Top-up
// --------------------------------------------------------------------------

// TestTopUpPracticePool_BlindSolveDisagreementRejectsCandidate grades by answer
// key, so the item's own answer passes check 2 and only the blind solve can stop
// it. The control run, with a blind solve that agrees, proves the check is what
// did.
func TestTopUpPracticePool_BlindSolveDisagreementRejectsCandidate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		solve     string
		wantAdded bool
	}{
		{solve: `{"selected_option_id": "B"}`, wantAdded: false},
		{solve: solveOptionA, wantAdded: true},
	} {
		graders := domain.NewGraderRegistry()
		_ = graders.Register(poolKindTense, keyGrader{option: "A"})
		f := newPoolFixture(t, testClock(), uuid.New(), graders, &cannedPracticeAI{solve: tc.solve})

		if err := f.svc.TopUpPracticePool(context.Background()); err != nil {
			t.Fatalf("TopUpPracticePool: %v", err)
		}
		if added := f.lessons.appendedCount() > 0; added != tc.wantAdded {
			t.Errorf("blind solve %s: added=%v, want %v", tc.solve, added, tc.wantAdded)
		}
	}
}

func TestTopUpPracticePool_ThrottlingLimits(t *testing.T) {
	t.Parallel()
	f := newPoolFixture(t, testClock(), uuid.New(), passingGraders(), &cannedPracticeAI{solve: solveOptionA})

	var a2Tense []uuid.UUID
	for _, level := range []string{"A2", "B1", "B2"} {
		seeded := f.seed(t, level, map[string]int{poolKindReading: 50, poolKindTense: 50, poolKindTransform: 50})
		if level == "A2" {
			a2Tense = seeded[poolKindTense]
		}
	}

	// Every slot at its target and nobody running low: nothing to add.
	if err := f.svc.TopUpPracticePool(context.Background()); err != nil {
		t.Fatalf("TopUpPracticePool: %v", err)
	}
	if n := f.lessons.appendedCount(); n != 0 {
		t.Errorf("expected nothing added at target with nobody running low, got %d", n)
	}

	// A learner running low in one slot: that slot grows by five.
	f.repo.runningLow[a2Tense[0]] = true
	if err := f.svc.TopUpPracticePool(context.Background()); err != nil {
		t.Fatalf("TopUpPracticePool: %v", err)
	}
	if n := f.lessons.appendedCount(); n != 5 {
		t.Errorf("expected 5 added to the slot running low, got %d", n)
	}

	// At the ceiling nothing more is added, whoever is running low.
	f.seed(t, "A2", map[string]int{poolKindTense: 200 - 55})
	if err := f.svc.TopUpPracticePool(context.Background()); err != nil {
		t.Fatalf("TopUpPracticePool: %v", err)
	}
	if n := f.lessons.appendedCount(); n != 5 {
		t.Errorf("expected nothing more added at the ceiling of 200, got %d in total", n)
	}
}

func TestTopUpPracticePool_PublishesUnderTheGeneratorAuthor(t *testing.T) {
	t.Parallel()
	author := uuid.New()
	f := newPoolFixture(t, testClock(), author, passingGraders(), &cannedPracticeAI{solve: solveOptionA})
	for _, level := range []string{"A2", "B1", "B2"} {
		counts := map[string]int{poolKindReading: 50, poolKindTense: 50, poolKindTransform: 50}
		if level == poolLevel {
			counts[poolKindTense] = 49
		}
		f.seed(t, level, counts)
	}

	if err := f.svc.TopUpPracticePool(context.Background()); err != nil {
		t.Fatalf("TopUpPracticePool: %v", err)
	}

	published := f.authors.published()
	if len(published) != 1 || f.lessons.appendedCount() != 1 {
		t.Fatalf("expected one item published and appended, got %d published, %d appended",
			len(published), f.lessons.appendedCount())
	}
	if published[0].AuthorID != author {
		t.Errorf("published under %s, want the generator author %s", published[0].AuthorID, author)
	}
}

func TestTopUpPracticePool_WithoutAnOwnerStandsDown(t *testing.T) {
	t.Parallel()
	aiClient := &cannedPracticeAI{solve: solveOptionA}
	f := newPoolFixture(t, testClock(), uuid.Nil, passingGraders(), aiClient)

	if err := f.svc.TopUpPracticePool(context.Background()); err != nil {
		t.Fatalf("TopUpPracticePool: %v", err)
	}
	if n := aiClient.generatedCount(); n != 0 {
		t.Errorf("expected no model calls without an owner for the content, got %d", n)
	}
	if n := f.lessons.appendedCount(); n != 0 {
		t.Errorf("expected nothing appended without an owner, got %d", n)
	}
}

func idStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
