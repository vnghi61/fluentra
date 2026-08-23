//go:build integration

package lesson_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/lesson/repository"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// P7.5's acceptance asks for a p95 under 100 ms on a course with eight lessons,
// and for the measurement to be stated rather than felt. This is that
// measurement, kept in the suite so the number can be reproduced instead of
// quoted from a pull request.
//
// It does not assert. A latency threshold measured on whatever hardware a
// contributor happens to have is a flake, not a gate — the number belongs in a
// report, and CI has k6 (`make test-load`) for the gate. So it is opt-in:
//
//	LESSON_LATENCY=1 go test -tags=integration -run Latency ./internal/modules/lesson/
//
// What it does hold constant is the shape: eight lessons, a warm cache, and the
// two learner reads a course page actually issues.
const latencySamples = 300

func TestCourseReadLatency_Integration(t *testing.T) {
	if os.Getenv("LESSON_LATENCY") == "" {
		t.Skip("set LESSON_LATENCY=1 to measure the learner read latency")
	}

	f := newCacheFixture(t)
	ctx := context.Background()
	repo := repository.New(pool)

	// The fixture ships two lessons; a course with eight is what the criterion
	// names, so top it up. They go in published: this measures the read path.
	var unitID uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT unit_id FROM learn.lessons WHERE id = $1`, f.lessonID).Scan(&unitID); err != nil {
		t.Fatalf("read unit id: %v", err)
	}
	for i := 3; i <= 8; i++ {
		extra, err := repo.CreateLesson(ctx, repository.CreateLessonParams{
			UnitID:           unitID,
			Position:         i,
			Title:            "Lesson " + string(rune('0'+i)),
			SkillFocus:       skillVocabulary,
			EstimatedMinutes: 15,
			Status:           statusPublished,
		})
		if err != nil {
			t.Fatalf("create lesson %d: %v", i, err)
		}
		f.putActivities(t, extra.ID)
	}

	for _, path := range []string{"/courses/" + f.slug, "/lessons/" + f.lessonID.String()} {
		report(t, path, measure(t, f, path))
	}
}

func measure(t *testing.T, f *cacheFixture, path string) []time.Duration {
	t.Helper()

	// Warm the key first: the criterion is about the steady state, and a cold
	// first request would put the loader's latency in every percentile.
	if rec := f.get(t, path); rec.Code != http.StatusOK {
		t.Fatalf("warm-up %s = %d. Body: %s", path, rec.Code, rec.Body.String())
	}
	time.Sleep(200 * time.Millisecond)

	samples := make([]time.Duration, 0, latencySamples)
	for i := 0; i < latencySamples; i++ {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req = req.WithContext(httpx.WithActor(
			context.Background(), httpx.Actor{UserID: f.learnerID, Role: roleUser}))
		rec := httptest.NewRecorder()

		start := time.Now()
		f.router.ServeHTTP(rec, req)
		elapsed := time.Since(start)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s sample %d = %d. Body: %s", path, i, rec.Code, rec.Body.String())
		}
		samples = append(samples, elapsed)
	}
	return samples
}

func report(t *testing.T, path string, samples []time.Duration) {
	t.Helper()

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	at := func(q float64) time.Duration {
		idx := int(q * float64(len(samples)))
		if idx >= len(samples) {
			idx = len(samples) - 1
		}
		return samples[idx]
	}
	t.Logf("%s over %d samples: p50=%v p95=%v p99=%v max=%v",
		path, len(samples), at(0.50), at(0.95), at(0.99), samples[len(samples)-1])
}
