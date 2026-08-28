package telemetry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Instruments contains the bounded-label metrics shared by platform modules.
type Instruments struct {
	// meter is retained so observable instruments can have their callbacks
	// registered after construction. An observable gauge with no callback never
	// reports anything, which is how job_oldest_pending_seconds stayed silent.
	meter metric.Meter

	HTTPDuration      metric.Float64Histogram
	HTTPActive        metric.Int64UpDownCounter
	DBQueryDuration   metric.Float64Histogram
	DBPoolConnections metric.Int64ObservableGauge
	JobDuration       metric.Float64Histogram
	JobQueueDepth     metric.Int64UpDownCounter
	JobOldestPending  metric.Int64ObservableGauge
	JobAttempts       metric.Int64Counter
	AuthLockout       metric.Int64Counter
	AuthRefreshReuse  metric.Int64Counter

	// LearningFunnel and LearningCohort are what make ROADMAP.md's Phase 2 exit
	// criterion — "D1 retention measurable" — a number a person reads rather than
	// one someone could derive. The events behind the funnel are already in the
	// outbox; a counter beside each write is what puts them on a dashboard.
	LearningFunnel metric.Int64Counter
	LearningCohort metric.Int64ObservableGauge
}

// NewInstruments creates the standard metric instruments from meter.
func NewInstruments(meter metric.Meter) (Instruments, error) {
	httpDuration, err := meter.Float64Histogram("http_server_request_duration_seconds", metric.WithUnit("s"))
	if err != nil {
		return Instruments{}, fmt.Errorf("create HTTP duration histogram: %w", err)
	}
	httpActive, err := meter.Int64UpDownCounter("http_server_active_requests")
	if err != nil {
		return Instruments{}, fmt.Errorf("create HTTP active counter: %w", err)
	}
	dbQueryDuration, err := meter.Float64Histogram("db_query_duration_seconds", metric.WithUnit("s"))
	if err != nil {
		return Instruments{}, fmt.Errorf("create database duration histogram: %w", err)
	}
	dbPoolConnections, err := meter.Int64ObservableGauge("db_pool_connections")
	if err != nil {
		return Instruments{}, fmt.Errorf("create database pool gauge: %w", err)
	}
	jobDuration, err := meter.Float64Histogram("job_duration_seconds", metric.WithUnit("s"))
	if err != nil {
		return Instruments{}, fmt.Errorf("create job duration histogram: %w", err)
	}
	jobQueueDepth, err := meter.Int64UpDownCounter("job_queue_depth")
	if err != nil {
		return Instruments{}, fmt.Errorf("create job queue depth counter: %w", err)
	}
	jobOldestPending, err := meter.Int64ObservableGauge("job_oldest_pending_seconds", metric.WithUnit("s"))
	if err != nil {
		return Instruments{}, fmt.Errorf("create oldest job gauge: %w", err)
	}
	jobAttempts, err := meter.Int64Counter("job_attempts_total")
	if err != nil {
		return Instruments{}, fmt.Errorf("create job attempts counter: %w", err)
	}
	authLockout, err := meter.Int64Counter("auth_lockout_total",
		metric.WithDescription("Total count of account and IP lockouts (BR-AUTH-08)."))
	if err != nil {
		return Instruments{}, fmt.Errorf("create auth lockout counter: %w", err)
	}
	authRefreshReuse, err := meter.Int64Counter("auth_refresh_reuse_total",
		metric.WithDescription("Total count of refresh-token reuse detections (BR-AUTH-04)."))
	if err != nil {
		return Instruments{}, fmt.Errorf("create auth refresh reuse counter: %w", err)
	}
	learningFunnel, err := meter.Int64Counter("learning_funnel_events_total",
		metric.WithDescription("Learner progress events, labelled by funnel step."))
	if err != nil {
		return Instruments{}, fmt.Errorf("create learning funnel counter: %w", err)
	}
	learningCohort, err := meter.Int64ObservableGauge("learning_cohort_learners",
		metric.WithDescription("Distinct learners per retention cohort; d1_returned over d0 is D1 retention."))
	if err != nil {
		return Instruments{}, fmt.Errorf("create learning cohort gauge: %w", err)
	}
	return Instruments{
		meter:             meter,
		HTTPDuration:      httpDuration,
		HTTPActive:        httpActive,
		DBQueryDuration:   dbQueryDuration,
		DBPoolConnections: dbPoolConnections,
		JobDuration:       jobDuration,
		JobQueueDepth:     jobQueueDepth,
		JobOldestPending:  jobOldestPending,
		JobAttempts:       jobAttempts,
		AuthLockout:       authLockout,
		AuthRefreshReuse:  authRefreshReuse,
		LearningFunnel:    learningFunnel,
		LearningCohort:    learningCohort,
	}, nil
}

// Funnel steps. The label set is closed on purpose: a dashboard panel per step
// only works if the steps are the same ones every time.
const (
	FunnelEnrolled          = "enrolled"
	FunnelLessonStarted     = "lesson_started"
	FunnelActivityCompleted = "activity_completed"
	FunnelLessonCompleted   = "lesson_completed"
	FunnelCourseCompleted   = "course_completed"
	FunnelReviewAnswered    = "review_answered"
)

// RecordFunnelStep increments learning_funnel_events_total for one step.
func (i Instruments) RecordFunnelStep(ctx context.Context, step string) {
	if i.LearningFunnel != nil {
		i.LearningFunnel.Add(ctx, 1, metric.WithAttributes(attribute.String("step", step)))
	}
}

// Retention cohorts.
const (
	// CohortD0 is the learners who did something yesterday.
	CohortD0 = "d0"
	// CohortD1Returned is the subset of those who came back today.
	CohortD1Returned = "d1_returned"
)

// RetentionSnapshot is one measurement of the two cohorts D1 retention is a
// ratio of.
type RetentionSnapshot struct {
	D0         int64
	D1Returned int64
}

// ObserveRetention registers a callback reporting the latest retention snapshot.
//
// The gauge is observable rather than a plain counter because retention is a
// query over a window, not an event: it is recomputed on a schedule and read
// here, so a scrape that lands between recomputations reports the last real
// answer instead of zero.
func (i Instruments) ObserveRetention(read func() RetentionSnapshot) (metric.Registration, error) {
	if i.meter == nil {
		return nil, errors.New("telemetry: instruments were not built from a meter")
	}
	registration, err := i.meter.RegisterCallback(
		func(_ context.Context, observer metric.Observer) error {
			snapshot := read()
			observer.ObserveInt64(i.LearningCohort, snapshot.D0,
				metric.WithAttributes(attribute.String("cohort", CohortD0)))
			observer.ObserveInt64(i.LearningCohort, snapshot.D1Returned,
				metric.WithAttributes(attribute.String("cohort", CohortD1Returned)))
			return nil
		},
		i.LearningCohort,
	)
	if err != nil {
		return nil, fmt.Errorf("register retention callback: %w", err)
	}
	return registration, nil
}

// RecordDBQuery emits a query duration histogram. queryName is the leading SQL
// keyword (SELECT, INSERT, …) — a bounded label. The guideline's `module` label
// is deliberately absent: a pgx tracer sees raw SQL and has no module context.
func (i Instruments) RecordDBQuery(ctx context.Context, queryName string, duration float64) {
	if i.DBQueryDuration != nil {
		i.DBQueryDuration.Record(ctx, duration, metric.WithAttributes(
			attribute.String("query_name", queryName)))
	}
}

// RecordAuthLockout increments the auth_lockout_total counter.
func (i Instruments) RecordAuthLockout(ctx context.Context) {
	if i.AuthLockout != nil {
		i.AuthLockout.Add(ctx, 1)
	}
}

// RecordAuthRefreshReuse increments the auth_refresh_reuse_total counter.
func (i Instruments) RecordAuthRefreshReuse(ctx context.Context) {
	if i.AuthRefreshReuse != nil {
		i.AuthRefreshReuse.Add(ctx, 1)
	}
}

// ObserveJobOldestPending attaches a callback to the job_oldest_pending_seconds
// gauge. Until something registers one, the gauge is declared but never emits a
// value — the metric exists in code and is missing from every dashboard.
//
// observe returns the age in seconds of the oldest job still waiting to run.
// The returned Registration must be unregistered when the observer's data
// source goes away, or the callback will run against a closed pool.
func (i Instruments) ObserveJobOldestPending(
	observe func(context.Context) (int64, error),
) (metric.Registration, error) {
	if i.meter == nil {
		return nil, errors.New("telemetry: instruments were not built from a meter")
	}
	registration, err := i.meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			seconds, err := observe(ctx)
			if err != nil {
				return err
			}
			observer.ObserveInt64(i.JobOldestPending, seconds)
			return nil
		},
		i.JobOldestPending,
	)
	if err != nil {
		return nil, fmt.Errorf("register oldest pending job callback: %w", err)
	}
	return registration, nil
}

// ObserveDBPoolConnections attaches a callback to the db_pool_connections gauge,
// sampling the pool on each collection.
//
// OBSERVABILITY_GUIDELINE §4.2 names the states `acquired`, `idle` and
// `waiting`. The first two are reported as written. The third is not: pgxpool
// exposes no gauge of callers currently blocked in Acquire — the nearest thing,
// ConstructingConns, counts connections being dialled, which rises during
// ordinary warm-up and not only under contention. Publishing that as `waiting`
// would put a number an operator reads as "requests are stuck" on a panel that
// lights up on a healthy cold start, so it is reported as `constructing` under
// its own name instead.
//
// `max` is published alongside them because a connection count without its
// ceiling cannot answer the question the panel exists for, which is how close
// to saturated the pool is.
func (i Instruments) ObserveDBPoolConnections(pool *pgxpool.Pool) (metric.Registration, error) {
	if i.meter == nil {
		return nil, errors.New("telemetry: instruments were not built from a meter")
	}
	registration, err := i.meter.RegisterCallback(
		func(_ context.Context, observer metric.Observer) error {
			stat := pool.Stat()
			observer.ObserveInt64(i.DBPoolConnections, int64(stat.AcquiredConns()),
				metric.WithAttributes(attribute.String("state", "acquired")))
			observer.ObserveInt64(i.DBPoolConnections, int64(stat.IdleConns()),
				metric.WithAttributes(attribute.String("state", "idle")))
			observer.ObserveInt64(i.DBPoolConnections, int64(stat.ConstructingConns()),
				metric.WithAttributes(attribute.String("state", "constructing")))
			observer.ObserveInt64(i.DBPoolConnections, int64(stat.MaxConns()),
				metric.WithAttributes(attribute.String("state", "max")))
			return nil
		},
		i.DBPoolConnections,
	)
	if err != nil {
		return nil, fmt.Errorf("register database pool callback: %w", err)
	}
	return registration, nil
}

// NewDBQueryTracer returns a pgx.QueryTracer that records db_query_duration_seconds
// around every query. `inner` — usually otelpgx — is chained so tracing spans are
// preserved; pgx allows a single tracer, so this wraps it rather than replacing it.
func NewDBQueryTracer(inner pgx.QueryTracer, instruments Instruments) pgx.QueryTracer {
	return dbQueryTracer{inner: inner, instruments: instruments}
}

type dbQueryTracer struct {
	inner       pgx.QueryTracer
	instruments Instruments
}

type dbQueryStartKey struct{}

type dbQueryTiming struct {
	start time.Time
	name  string
}

func (t dbQueryTracer) TraceQueryStart(
	ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData,
) context.Context {
	if t.inner != nil {
		ctx = t.inner.TraceQueryStart(ctx, conn, data)
	}
	return context.WithValue(ctx, dbQueryStartKey{}, dbQueryTiming{
		start: time.Now(),
		name:  queryNameFromSQL(data.SQL),
	})
}

func (t dbQueryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if t.inner != nil {
		t.inner.TraceQueryEnd(ctx, conn, data)
	}
	timing, ok := ctx.Value(dbQueryStartKey{}).(dbQueryTiming)
	if !ok {
		return
	}
	t.instruments.RecordDBQuery(ctx, timing.name, time.Since(timing.start).Seconds())
}

// queryNameFromSQL reduces a raw statement to a bounded label: the leading
// keyword, upper-cased, with anything unrecognised collapsed to "other".
func queryNameFromSQL(sql string) string {
	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return "other"
	}
	switch name := strings.ToUpper(fields[0]); name {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "TRUNCATE", "CREATE", "ALTER", "DROP":
		return name
	default:
		return "other"
	}
}
