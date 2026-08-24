//go:build integration

// `ListCourseActivityIDs` against a real PostgreSQL instance.
//
// `learning` reads this through `contract.Reader.ListActivitiesByCourseIDs` to
// answer `total_activities` for every enrolled course in one round trip (P8.5
// Trap 1), and it is the only method on that contract whose whole substance is a
// three-table join. A fake repository satisfies the interface and proves nothing
// about the join, the `ANY($1::uuid[])` batch, or the ordering the caller
// inherits, so those are asserted here.
package lesson_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/lesson/repository"
)

// seedCourseWithActivities writes a course of one unit, `lessons` lessons and
// `perLesson` activities in each, and returns the course id with its activity
// ids in unit → lesson → activity position order.
func seedCourseWithActivities(t *testing.T, lessons, perLesson int) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	repo := repository.New(pool)

	course, err := repo.CreateCourse(ctx, repository.CreateCourseParams{
		Slug:           fmt.Sprintf("course-activities-%d", time.Now().UnixNano()),
		Title:          "Course Activities",
		Description:    "Fixture",
		CEFRFrom:       "B1",
		CEFRTo:         "B2",
		Status:         statusPublished,
		EstimatedHours: 10,
	})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM learn.courses WHERE id = $1`, course.ID)
	})

	unit, err := repo.CreateUnit(ctx, repository.CreateUnitParams{
		CourseID: course.ID,
		Position: 1,
		Title:    "Unit 1",
	})
	if err != nil {
		t.Fatalf("create unit: %v", err)
	}

	ordered := make([]uuid.UUID, 0, lessons*perLesson)
	for l := 1; l <= lessons; l++ {
		lsn, lErr := repo.CreateLesson(ctx, repository.CreateLessonParams{
			UnitID:     unit.ID,
			Position:   l,
			Title:      fmt.Sprintf("Lesson %d", l),
			SkillFocus: skillVocabulary,
			Status:     statusPublished,
		})
		if lErr != nil {
			t.Fatalf("create lesson %d: %v", l, lErr)
		}

		inputs := make([]repository.ActivityInputDTO, 0, perLesson)
		for a := 1; a <= perLesson; a++ {
			inputs = append(inputs, repository.ActivityInputDTO{
				Position:         a,
				Kind:             kindMultipleChoice,
				ContentVersionID: uuid.New(),
				Weight:           1,
			})
		}
		created, aErr := repo.ReplaceActivities(ctx, lsn.ID, inputs)
		if aErr != nil {
			t.Fatalf("create activities for lesson %d: %v", l, aErr)
		}
		for _, act := range created {
			ordered = append(ordered, act.ID)
		}
	}

	return course.ID, ordered
}

func TestListCourseActivityIDs_Integration(t *testing.T) {
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	repo := repository.New(pool)

	courseA, wantA := seedCourseWithActivities(t, 3, 2)
	courseB, wantB := seedCourseWithActivities(t, 1, 4)
	empty, _ := seedCourseWithActivities(t, 0, 0)

	// One call answers for every course the learner is enrolled in — the point
	// of the method, and what keeps /me/progress off a per-course read.
	got, err := repo.ListCourseActivityIDs(ctx, []uuid.UUID{courseA, courseB, empty})
	if err != nil {
		t.Fatalf("ListCourseActivityIDs: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("got %d courses in the result, want 3", len(got))
	}
	if len(got[courseA]) != len(wantA) || len(got[courseB]) != len(wantB) {
		t.Fatalf("counts: A=%d (want %d), B=%d (want %d)",
			len(got[courseA]), len(wantA), len(got[courseB]), len(wantB))
	}

	// A course with no activities is present with an empty slice, not missing:
	// `total_activities` for it is 0, and a missing key and a zero count are the
	// same number only if the caller remembers to treat them the same.
	if ids, ok := got[empty]; !ok || len(ids) != 0 {
		t.Errorf("course with no activities: got %v (present=%v), want an empty slice", ids, ok)
	}

	// Ordering is unit → lesson → activity position, which is what makes the
	// first unfinished activity in the slice the next one to do.
	assertSameOrder(t, got[courseA], wantA)

	// Activities never cross courses.
	assertDisjoint(t, got[courseB], wantA)

	// No course ids means no query and no nil map.
	none, err := repo.ListCourseActivityIDs(ctx, nil)
	if err != nil {
		t.Fatalf("ListCourseActivityIDs(nil): %v", err)
	}
	if none == nil || len(none) != 0 {
		t.Errorf("got %v, want an empty map", none)
	}
}

func assertSameOrder(t *testing.T, got, want []uuid.UUID) {
	t.Helper()
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %s, want %s", i, got[i], want[i])
			return
		}
	}
}

func assertDisjoint(t *testing.T, got, other []uuid.UUID) {
	t.Helper()
	seen := make(map[uuid.UUID]bool, len(other))
	for _, id := range other {
		seen[id] = true
	}
	for _, id := range got {
		if seen[id] {
			t.Fatalf("activity %s belongs to the other course", id)
		}
	}
}
