package job_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river/rivertype"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/telemetry"
)

// resultSuccess is the metric label a completed job carries.
const resultSuccess = "success"

func testJobRow(kind string) *rivertype.JobRow {
	return &rivertype.JobRow{
		ID: 42, Kind: kind, Queue: job.QueueDefault, Attempt: 1, MaxAttempts: 3,
	}
}

// newTestInstruments returns instruments backed by a manual reader, so a test
// can assert on what was actually exported rather than on a spy.
func newTestInstruments(t *testing.T) (telemetry.Instruments, *metric.ManualReader) {
	t.Helper()
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	instruments, err := telemetry.NewInstruments(provider.Meter("test"))
	if err != nil {
		t.Fatalf("new instruments: %v", err)
	}
	return instruments, reader
}

func collect(t *testing.T, reader *metric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("collect: %v", err)
	}
	return collected
}

// findMetric returns the data points of one metric by name, or nil.
func findMetric(collected metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, scope := range collected.ScopeMetrics {
		for i := range scope.Metrics {
			if scope.Metrics[i].Name == name {
				return &scope.Metrics[i]
			}
		}
	}
	return nil
}

// TestMiddleware_RecoversPanic is the trap this card exists for: a handler that
// panics must not take the worker down, and no handler should have to remember
// to recover for that to hold.
func TestMiddleware_RecoversPanic(t *testing.T) {
	t.Parallel()
	instruments, _ := newTestInstruments(t)
	middleware := job.NewMiddleware(time.Second, instruments)

	err := middleware.Work(context.Background(), testJobRow("panicking"),
		func(context.Context) error { panic("handler exploded") })

	if err == nil {
		t.Fatal("panic was swallowed; River would mark the job successful")
	}
	if !errors.Is(err, job.ErrPanic) {
		t.Fatalf("error = %v, want it to wrap job.ErrPanic", err)
	}
	if !strings.Contains(err.Error(), "handler exploded") {
		t.Errorf("error = %v, want it to name the recovered value", err)
	}
}

// TestMiddleware_SurvivesPanicAndRunsTheNextJob proves recovery leaves the
// middleware usable, not just that one call returned.
func TestMiddleware_SurvivesPanicAndRunsTheNextJob(t *testing.T) {
	t.Parallel()
	instruments, _ := newTestInstruments(t)
	middleware := job.NewMiddleware(time.Second, instruments)

	_ = middleware.Work(context.Background(), testJobRow("panicking"),
		func(context.Context) error { panic("boom") })

	ran := false
	err := middleware.Work(context.Background(), testJobRow("healthy"),
		func(context.Context) error { ran = true; return nil })
	if err != nil || !ran {
		t.Fatalf("next job did not run: err=%v ran=%v", err, ran)
	}
}

// TestMiddleware_TimesOutSlowHandler asserts the handler's own context is
// cancelled, so a job that ignores ctx cannot hold a worker slot forever.
func TestMiddleware_TimesOutSlowHandler(t *testing.T) {
	t.Parallel()
	instruments, _ := newTestInstruments(t)
	middleware := job.NewMiddleware(50*time.Millisecond, instruments)

	err := middleware.Work(context.Background(), testJobRow("slow"), func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return errors.New("handler was never cancelled")
		}
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestMiddleware_PassesDeadlineToHandler(t *testing.T) {
	t.Parallel()
	instruments, _ := newTestInstruments(t)
	middleware := job.NewMiddleware(time.Minute, instruments)

	var hasDeadline bool
	err := middleware.Work(context.Background(), testJobRow("checks-deadline"),
		func(ctx context.Context) error {
			_, hasDeadline = ctx.Deadline()
			return nil
		})
	if err != nil {
		t.Fatalf("work: %v", err)
	}
	if !hasDeadline {
		t.Error("handler context carried no deadline; BR-JOB-04 is unenforced")
	}
}

// TestMiddleware_RecordsDurationAndAttempts checks the metrics are real exports
// with bounded labels, not just method calls on an interface.
func TestMiddleware_RecordsDurationAndAttempts(t *testing.T) {
	t.Parallel()
	instruments, reader := newTestInstruments(t)
	middleware := job.NewMiddleware(time.Second, instruments)

	if err := middleware.Work(context.Background(), testJobRow("metered"),
		func(context.Context) error { return nil }); err != nil {
		t.Fatalf("work: %v", err)
	}

	collected := collect(t, reader)

	duration := findMetric(collected, "job_duration_seconds")
	if duration == nil {
		t.Fatal("job_duration_seconds was not exported")
	}
	histogram, ok := duration.Data.(metricdata.Histogram[float64])
	if !ok || len(histogram.DataPoints) != 1 || histogram.DataPoints[0].Count != 1 {
		t.Fatalf("job_duration_seconds data = %#v", duration.Data)
	}

	attributes := histogram.DataPoints[0].Attributes
	for key, want := range map[string]string{
		"queue": job.QueueDefault, "kind": "metered", "result": resultSuccess,
	} {
		value, found := attributes.Value(attribute.Key(key))
		if !found || value.AsString() != want {
			t.Errorf("attribute %s = %v, want %q", key, value.AsString(), want)
		}
	}

	if findMetric(collected, "job_attempts_total") == nil {
		t.Error("job_attempts_total was not exported")
	}
}

// TestMiddleware_LabelsFailureResults keeps panic, timeout and plain failure
// distinguishable on the dashboard — they need different responses.
func TestMiddleware_LabelsFailureResults(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		handler func(context.Context) error
		want    string
	}{
		"panic":   {func(context.Context) error { panic("x") }, "panic"},
		"error":   {func(context.Context) error { return errors.New("nope") }, "error"},
		"success": {func(context.Context) error { return nil }, resultSuccess},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			instruments, reader := newTestInstruments(t)
			middleware := job.NewMiddleware(time.Second, instruments)

			_ = middleware.Work(context.Background(), testJobRow("labelled"), test.handler)

			duration := findMetric(collect(t, reader), "job_duration_seconds")
			if duration == nil {
				t.Fatal("job_duration_seconds was not exported")
			}
			histogram, ok := duration.Data.(metricdata.Histogram[float64])
			if !ok || len(histogram.DataPoints) == 0 {
				t.Fatalf("data = %#v", duration.Data)
			}
			got, _ := histogram.DataPoints[0].Attributes.Value(attribute.Key("result"))
			if got.AsString() != test.want {
				t.Errorf("result label = %q, want %q", got.AsString(), test.want)
			}
		})
	}
}

// TestMiddleware_WorksWithoutInstruments keeps telemetry optional: a worker
// booted before the meter provider exists must still run jobs.
func TestMiddleware_WorksWithoutInstruments(t *testing.T) {
	t.Parallel()
	middleware := job.NewMiddleware(time.Second, telemetry.Instruments{})
	if err := middleware.Work(context.Background(), testJobRow("bare"),
		func(context.Context) error { return nil }); err != nil {
		t.Fatalf("work: %v", err)
	}
}

func TestNewMiddleware_FallsBackToDefaultTimeout(t *testing.T) {
	t.Parallel()
	middleware := job.NewMiddleware(0, telemetry.Instruments{})

	var deadline time.Time
	_ = middleware.Work(context.Background(), testJobRow("default-timeout"),
		func(ctx context.Context) error {
			deadline, _ = ctx.Deadline()
			return nil
		})

	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > job.DefaultJobTimeout {
		t.Errorf("deadline in %s, want a positive value up to %s", remaining, job.DefaultJobTimeout)
	}
}

// resultError is the `result` label value a failed run carries.
const resultError = "error"

// TestCronScheduler_ReportsAttempts is what makes the ScheduledJobFailing alert
// possible.
//
// Cron jobs used to emit nothing — a failed partition rotation was a log line
// and nothing else, so the job could fail every six hours for a month while the
// alerting stack showed green. They now report the same job_attempts_total the
// River middleware does, under queue="cron", which is what the alert watches.
func TestCronScheduler_ReportsAttempts(t *testing.T) {
	instruments, reader := newTestInstruments(t)
	scheduler := job.NewCronScheduler(nil).WithInstruments(instruments)

	scheduler.RecordForTest(context.Background(), "srs.rotate_partitions", resultError, time.Second)

	collected := collect(t, reader)
	attempts := findMetric(collected, "job_attempts_total")
	if attempts == nil {
		t.Fatal("a cron job run exported no job_attempts_total")
	}

	sum, ok := attempts.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("job_attempts_total is %T, want a Sum[int64]", attempts.Data)
	}
	if len(sum.DataPoints) != 1 {
		t.Fatalf("got %d data points, want 1", len(sum.DataPoints))
	}

	labels := map[string]string{}
	for _, attribute := range sum.DataPoints[0].Attributes.ToSlice() {
		labels[string(attribute.Key)] = attribute.Value.String()
	}
	// The alert selects on queue and result and groups by kind; all three have to
	// be present and spelled the way the rule file spells them.
	if labels["queue"] != "cron" {
		t.Errorf("queue = %q, want cron — the alert selects queue=\"cron\"", labels["queue"])
	}
	if labels["kind"] != "srs.rotate_partitions" {
		t.Errorf("kind = %q, want the job name", labels["kind"])
	}
	if labels["result"] != resultError {
		t.Errorf("result = %q, want %s", labels["result"], resultError)
	}
}

// TestCronScheduler_WithoutInstrumentsIsSilent: cmd/migrate and the tests build a
// scheduler with no meter, and that must not panic.
func TestCronScheduler_WithoutInstrumentsIsSilent(_ *testing.T) {
	scheduler := job.NewCronScheduler(nil)
	scheduler.RecordForTest(context.Background(), "any.job", "success", time.Second)
}
