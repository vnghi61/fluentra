//go:build integration

// Package learning_test drives the attempt lifecycle against a real PostgreSQL
// instance.
//
// The service suite races two goroutines through SubmitAttempt against an
// in-memory repository, which re-implements the `AND status = 'in_progress'`
// guard in Go. That test passes with the guard deleted from learning.sql,
// because the fake never runs the SQL. P8.2 chose the database as the arbiter of
// "one attempt is graded at most once"; this is where that choice is checked.
package learning_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

const attemptDatabase = "fluentra_learning_attempt_test"

var attemptPool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, drop, err := createAttemptDatabase(base, attemptDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", attemptDatabase, err)
		os.Exit(1)
	}
	if err := migrateAttemptDatabase(dsn); err != nil {
		drop()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", attemptDatabase, err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		drop()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", attemptDatabase, err)
		os.Exit(1)
	}
	attemptPool = pool

	code := m.Run()

	attemptPool.Close()
	drop()
	os.Exit(code)
}

func createAttemptDatabase(base, name string) (string, func(), error) {
	maintenance, err := replaceAttemptDatabase(base, "postgres")
	if err != nil {
		return "", nil, err
	}
	admin, err := sql.Open("pgx", maintenance)
	if err != nil {
		return "", nil, fmt.Errorf("open maintenance database: %w", err)
	}
	defer func() { _ = admin.Close() }()

	ctx := context.Background()
	dropStmt := fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", name)
	if _, err := admin.ExecContext(ctx, dropStmt); err != nil {
		return "", nil, fmt.Errorf("drop stale %s: %w", name, err)
	}
	if _, err := admin.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %q", name)); err != nil {
		return "", nil, fmt.Errorf("create %s: %w", name, err)
	}

	dsn, err := replaceAttemptDatabase(base, name)
	if err != nil {
		return "", nil, err
	}
	return dsn, func() {
		cleanup, err := sql.Open("pgx", maintenance)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.ExecContext(context.Background(), dropStmt)
	}, nil
}

func replaceAttemptDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

func migrateAttemptDatabase(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	sources, err := migrations.Flattened()
	if err != nil {
		return fmt.Errorf("flatten migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// stubLessonReader answers the hops the engine needs. `lesson`'s real reader is
// exercised in its own suite; what matters here is the database behind
// `learning`, so the hierarchy is fixed and the rest is real.
//
// It describes the smallest course that crosses every boundary at once — one
// activity in one lesson in one unit, and no lesson after it — so a single
// submit has to publish all three completion events.
type stubLessonReader struct {
	hierarchy *lessoncontract.ActivityHierarchy
	lesson    *lessoncontract.Lesson
}

func (s stubLessonReader) GetLesson(context.Context, uuid.UUID) (*lessoncontract.Lesson, error) {
	return s.lesson, nil
}

func (s stubLessonReader) ListLessons(context.Context, uuid.UUID) ([]*lessoncontract.Lesson, error) {
	return []*lessoncontract.Lesson{s.lesson}, nil
}

func (s stubLessonReader) NextLesson(
	context.Context, uuid.UUID, *uuid.UUID,
) (*lessoncontract.Lesson, error) {
	return nil, nil
}

func (s stubLessonReader) ResolveActivity(
	_ context.Context, _ uuid.UUID,
) (*lessoncontract.ActivityHierarchy, error) {
	return s.hierarchy, nil
}

// barrierRepo holds the first two callers at ClaimAttemptForGrading until both
// have arrived, then lets them through together.
//
// Without it the race is not a race. SubmitAttempt reads the attempt and returns
// early when it is already grading, so on a fast local database the first
// caller usually finishes the whole submit before the second one starts, and the
// SQL guard is never the thing that decides. The pre-check is a courtesy; the
// `AND status = 'in_progress'` in ClaimAttemptForGrading is the guarantee, and
// it is only exercised when two callers reach the UPDATE having both seen
// 'in_progress'. This forces exactly that interleaving.
type barrierRepo struct {
	service.Repository
	arrive  chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBarrierRepo(inner service.Repository, callers int) *barrierRepo {
	return &barrierRepo{
		Repository: inner,
		arrive:     make(chan struct{}, callers),
		release:    make(chan struct{}),
	}
}

func (b *barrierRepo) ClaimAttemptForGrading(
	ctx context.Context, params repository.ClaimAttemptParams,
) (*domain.Attempt, error) {
	b.arrive <- struct{}{}
	if len(b.arrive) == cap(b.arrive) {
		b.once.Do(func() { close(b.release) })
	}
	select {
	case <-b.release:
	case <-time.After(5 * time.Second):
		// Never block the suite forever if only one caller arrives.
	}
	return b.Repository.ClaimAttemptForGrading(ctx, params)
}

// repositoryAdapter mirrors module.go's, so this suite runs the same wiring.
type repositoryAdapter struct {
	*repository.Repository
}

func (a repositoryAdapter) WithTx(tx pgx.Tx) service.Repository {
	return repositoryAdapter{Repository: a.Repository.WithTx(tx)}
}

// barrierRepo wraps an already-bound repository, so it has to forward WithTx
// too or the barrier disappears inside the transaction.
func (b *barrierRepo) WithTx(tx pgx.Tx) service.Repository {
	return b.Repository.WithTx(tx)
}

// countingGrader records how many times grading actually ran.
type countingGrader struct {
	calls atomic.Int64
}

func (g *countingGrader) Grade(
	_ context.Context, _ contract.GradeRequest,
) (contract.GradeResult, error) {
	g.calls.Add(1)
	return contract.GradeResult{Score: 90, MaxScore: 100, Correct: true, Feedback: "ok"}, nil
}

type attemptFixture struct {
	svc      *service.Service
	grader   *countingGrader
	graders  *domain.GraderRegistry
	reader   stubLessonReader
	userID   uuid.UUID
	courseID uuid.UUID
	activity uuid.UUID
}

func newAttemptFixture(t *testing.T) *attemptFixture {
	t.Helper()
	return newAttemptFixtureWithBarrier(t, 0)
}

func newAttemptFixtureWithBarrier(t *testing.T, callers int) *attemptFixture {
	t.Helper()

	if attemptPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	userID := uuid.New()
	if _, err := attemptPool.Exec(ctx,
		`INSERT INTO core.users (id, email, status) VALUES ($1, $2, 'active')`,
		userID, fmt.Sprintf("learner-%s@example.com", userID),
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = attemptPool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
	})

	courseID, unitID, lessonID, activityID := seedCourseTree(t)

	registry := domain.NewGraderRegistry()
	grader := &countingGrader{}
	if err := registry.Register("multiple_choice", grader); err != nil {
		t.Fatalf("register grader: %v", err)
	}

	var repo service.Repository = repositoryAdapter{Repository: repository.New(attemptPool)}
	if callers > 0 {
		repo = newBarrierRepo(repo, callers)
	}

	reader := stubLessonReader{
		hierarchy: &lessoncontract.ActivityHierarchy{
			ActivityID:       activityID,
			LessonID:         lessonID,
			UnitID:           unitID,
			CourseID:         courseID,
			Kind:             "multiple_choice",
			ContentVersionID: uuid.New(),
			Weight:           1,
			LessonSkillFocus: "vocabulary",
		},
		lesson: &lessoncontract.Lesson{
			ID:         lessonID,
			UnitID:     unitID,
			SkillFocus: "vocabulary",
			Activities: []lessoncontract.Activity{{ID: activityID, LessonID: lessonID}},
		},
	}

	svc := service.New(service.Deps{
		Pool:    attemptPool,
		Repo:    repo,
		Graders: registry,
		Clock:   clock.Real{},
		// The real outbox writer, not a fake. The events have to be written by
		// the same code cmd/api runs, through the transaction dbx.InTx opened,
		// or "they land in the outbox in the grading transaction" is a claim no
		// test in this repository makes.
		Events: outboxAdapter{Writer: outbox.NewWriter()},
		Lesson: reader,
	})

	return &attemptFixture{
		svc:      svc,
		grader:   grader,
		graders:  registry,
		reader:   reader,
		userID:   userID,
		courseID: courseID,
		activity: activityID,
	}
}

// seedCourseTree writes the lesson-owned rows the attempts foreign key needs.
// It is raw SQL rather than lesson's repository on purpose: this suite is about
// learning, and reaching for another module's repository here would make its
// failures land in this file.
func seedCourseTree(t *testing.T) (course, unit, lesson, activity uuid.UUID) {
	t.Helper()

	// Setup and its cleanup both run outside any request, so the context is
	// made here rather than taken from the caller — the caller's would be
	// cancelled by the time t.Cleanup fires.
	ctx := context.Background()

	course, unit, lesson, activity = uuid.New(), uuid.New(), uuid.New(), uuid.New()
	slug := fmt.Sprintf("attempt-course-%d", time.Now().UnixNano())

	stmts := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO learn.courses (id, slug, title, cefr_from, cefr_to, status)
		  VALUES ($1, $2, 'Attempt Course', 'B1', 'B2', 'published')`, []any{course, slug}},
		{`INSERT INTO learn.course_units (id, course_id, position, title)
		  VALUES ($1, $2, 1, 'Unit 1')`, []any{unit, course}},
		{`INSERT INTO learn.lessons (id, unit_id, position, title, skill_focus, status)
		  VALUES ($1, $2, 1, 'Lesson 1', 'vocabulary', 'published')`, []any{lesson, unit}},
		{`INSERT INTO learn.activities (id, lesson_id, position, kind, content_version_id, weight)
		  VALUES ($1, $2, 1, 'multiple_choice', $3, 1)`, []any{activity, lesson, uuid.New()}},
	}
	for _, stmt := range stmts {
		if _, err := attemptPool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("seed course tree: %v", err)
		}
	}

	t.Cleanup(func() {
		_, _ = attemptPool.Exec(context.Background(), `DELETE FROM learn.courses WHERE id = $1`, course)
	})
	return course, unit, lesson, activity
}

// The assertion P8.3 turns on, and the one the service suite cannot make: two
// submissions racing through real transactions must grade exactly once.
//
// Delete `AND status = 'in_progress'` from ClaimAttemptForGrading and this fails
// with two grader calls; the in-memory version keeps passing.
func TestSubmitAttempt_ConcurrentSubmitGradesOnce_Integration(t *testing.T) {
	const callers = 2

	f := newAttemptFixtureWithBarrier(t, callers)
	ctx := context.Background()

	started, err := f.svc.StartAttempt(ctx, f.userID, f.activity)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	key := uuid.New()
	var wg sync.WaitGroup
	errs := make([]error, callers)

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = f.svc.SubmitAttempt(
				ctx, f.userID, started.AttemptID, key, json.RawMessage(`{"choice":"a"}`),
			)
		}(i)
	}
	wg.Wait()

	// The guarantee: the grader ran once. Both callers passed the pre-check and
	// reached the UPDATE together, so this is the database's answer, not the
	// service's.
	if calls := f.grader.calls.Load(); calls != 1 {
		t.Errorf("grader ran %d times, want exactly 1", calls)
	}

	// Neither caller may see an error: one graded, the other is a retry of the
	// same key and gets the attempt's state back.
	for i, err := range errs {
		if err != nil {
			t.Errorf("caller %d failed: %v", i, err)
		}
	}

	var status string
	var score *int32
	if err := attemptPool.QueryRow(ctx,
		`SELECT status, score FROM learn.attempts WHERE id = $1`, started.AttemptID,
	).Scan(&status, &score); err != nil {
		t.Fatalf("read attempt back: %v", err)
	}
	if status != domain.StatusGraded {
		t.Errorf("attempt status = %q, want graded", status)
	}
	if score == nil || *score != 90 {
		t.Errorf("stored score = %v, want 90", score)
	}
}

// A second client submitting the same attempt under a different key is a
// conflict, not a retry — and it must not grade again.
func TestSubmitAttempt_DifferentKeyIsAConflict_Integration(t *testing.T) {
	f := newAttemptFixture(t)
	ctx := context.Background()

	started, err := f.svc.StartAttempt(ctx, f.userID, f.activity)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	if _, err := f.svc.SubmitAttempt(
		ctx, f.userID, started.AttemptID, uuid.New(), json.RawMessage(`{"choice":"a"}`),
	); err != nil {
		t.Fatalf("first submit: %v", err)
	}

	if _, err := f.svc.SubmitAttempt(
		ctx, f.userID, started.AttemptID, uuid.New(), json.RawMessage(`{"choice":"b"}`),
	); err == nil {
		t.Fatal("a different idempotency key was accepted as a retry")
	}

	if calls := f.grader.calls.Load(); calls != 1 {
		t.Errorf("grader ran %d times, want exactly 1", calls)
	}
}

// outboxAdapter is the same four-line shim module.go installs, so this suite
// exercises the production writer rather than a stand-in for it.
type outboxAdapter struct {
	*outbox.Writer
}

func (a outboxAdapter) Write(
	ctx context.Context, tx service.OutboxTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	return a.Writer.Write(ctx, outboxExec{tx}, aggregate, event, payload)
}

type outboxExec struct{ inner service.OutboxTx }

func (e outboxExec) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return e.inner.Exec(ctx, sql, args...)
}

// The rollup and the events are one transaction. If the outbox write is not in
// it, this passes with the attempt graded and no event recorded.
func TestSubmitAttempt_RollupAndProgressArePersisted_Integration(t *testing.T) {
	f := newAttemptFixture(t)
	ctx := context.Background()

	started, err := f.svc.StartAttempt(ctx, f.userID, f.activity)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}
	if _, err := f.svc.SubmitAttempt(
		ctx, f.userID, started.AttemptID, uuid.New(), json.RawMessage(`{"choice":"a"}`),
	); err != nil {
		t.Fatalf("submit: %v", err)
	}

	// One activity, in one lesson, in one unit, with nothing after it: the
	// rollup crosses every boundary, so all four scopes are written.
	for _, scope := range []string{"activity", "lesson", "unit", "course"} {
		var rows int
		if err := attemptPool.QueryRow(ctx,
			`SELECT count(*) FROM learn.progress WHERE user_id = $1 AND scope = $2 AND status = 'completed'`,
			f.userID, scope,
		).Scan(&rows); err != nil {
			t.Fatalf("count %s progress: %v", scope, err)
		}
		if rows != 1 {
			t.Errorf("%s progress rows = %d, want 1", scope, rows)
		}
	}

	// And the three events are in the outbox, written by the real writer inside
	// the transaction that graded the attempt.
	for _, event := range []string{
		contract.EventActivityCompleted,
		contract.EventLessonCompleted,
		contract.EventCourseCompleted,
	} {
		var rows int
		if err := attemptPool.QueryRow(ctx,
			`SELECT count(*) FROM ops.outbox_events
			 WHERE aggregate = $1 AND event = $2 AND payload->>'user_id' = $3`,
			contract.Aggregate, event, f.userID.String(),
		).Scan(&rows); err != nil {
			t.Fatalf("count %s: %v", event, err)
		}
		if rows != 1 {
			t.Errorf("%s outbox rows = %d, want 1", event, rows)
		}
	}
}

// The other half of "one transaction": when the rollup fails partway, the
// attempt is not graded and the events already written are gone with it.
//
// Publishing after commit passes the test above and fails this one.
func TestSubmitAttempt_FailedRollupLeavesNoEvents_Integration(t *testing.T) {
	f := newAttemptFixture(t)
	ctx := context.Background()

	started, err := f.svc.StartAttempt(ctx, f.userID, f.activity)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	// A second service over the same rows, whose lesson-scope progress write
	// fails after activity.completed has already gone to the outbox.
	failing := service.New(service.Deps{
		Pool:    attemptPool,
		Repo:    failingRollupRepo{Repository: repositoryAdapter{Repository: repository.New(attemptPool)}},
		Graders: f.graders,
		Clock:   clock.Real{},
		Events:  outboxAdapter{Writer: outbox.NewWriter()},
		Lesson:  f.reader,
	})

	if _, err := failing.SubmitAttempt(
		ctx, f.userID, started.AttemptID, uuid.New(), json.RawMessage(`{"choice":"a"}`),
	); err == nil {
		t.Fatal("a failing rollup reported success")
	}

	var status string
	if err := attemptPool.QueryRow(ctx,
		`SELECT status FROM learn.attempts WHERE id = $1`, started.AttemptID,
	).Scan(&status); err != nil {
		t.Fatalf("read attempt back: %v", err)
	}
	if status == domain.StatusGraded {
		t.Error("the attempt was graded although the rollup failed")
	}

	var events int
	if err := attemptPool.QueryRow(ctx,
		`SELECT count(*) FROM ops.outbox_events
		 WHERE aggregate = $1 AND payload->>'user_id' = $2`, contract.Aggregate, f.userID.String(),
	).Scan(&events); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if events != 0 {
		t.Errorf("%d event(s) survived a rolled-back grading transaction, want 0", events)
	}
}

// failingRollupRepo lets the activity scope through and refuses the lesson one,
// so the transaction fails after an event has been written.
type failingRollupRepo struct {
	service.Repository
}

func (r failingRollupRepo) WithTx(tx pgx.Tx) service.Repository {
	return failingRollupRepo{Repository: r.Repository.WithTx(tx)}
}

func (r failingRollupRepo) UpsertProgress(
	ctx context.Context, params repository.UpsertProgressParams,
) (*repository.ProgressDTO, error) {
	if params.Scope == "lesson" {
		return nil, errors.New("rollup failed")
	}
	return r.Repository.UpsertProgress(ctx, params)
}
