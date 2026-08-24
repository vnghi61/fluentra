package learning_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fluentra/fluentra/internal/modules/learning"
	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const (
	testKindQuiz           = "quiz"
	testKindMultipleChoice = "multiple_choice"
	testKindReadingMCQ     = "reading_mcq"
)

type dummyGuard struct{}

func (dummyGuard) Require(_ context.Context, _ string) error {
	return nil
}

func TestModule_New_DefaultGraders(t *testing.T) {
	mod := learning.New(learning.Deps{
		Guard: dummyGuard{},
		Clock: clock.NewFake(time.Now()),
	})
	if mod == nil {
		t.Fatal("expected non-nil learning module")
	}

	cronJobs := mod.CronJobs()
	if len(cronJobs) != 1 {
		t.Fatalf("expected 1 cron job, got %d", len(cronJobs))
	}
	if cronJobs[0].Name != "learning.rotate_partitions" {
		t.Errorf("got job name %s, want learning.rotate_partitions", cronJobs[0].Name)
	}

	r := chi.NewRouter()
	mod.Routes(r)
}

func TestModule_New_CustomGraders(t *testing.T) {
	customGraders := map[string]contract.ExerciseGrader{
		testKindQuiz:           &dummyGrader{},
		testKindMultipleChoice: &dummyGrader{},
		testKindReadingMCQ:     &dummyGrader{},
	}

	mod := learning.New(learning.Deps{
		Guard:         dummyGuard{},
		Graders:       customGraders,
		DeclaredKinds: []string{testKindQuiz, testKindMultipleChoice, testKindReadingMCQ},
	})
	if mod == nil {
		t.Fatal("expected non-nil learning module")
	}
}

// ADR-0015: the registry is validated at startup. A kind the deployment
// declares with nothing registered behind it must stop the process and name the
// kind, rather than surface as a failed request later.
func TestModule_New_DeclaredKindWithoutGraderPanicsAtStartup(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("a declared kind with no grader did not fail startup")
		}
		if msg := fmt.Sprint(r); !strings.Contains(msg, testKindReadingMCQ) {
			t.Errorf("panic does not name the offending kind: %s", msg)
		}
	}()

	_ = learning.New(learning.Deps{
		Guard:         dummyGuard{},
		Graders:       map[string]contract.ExerciseGrader{testKindQuiz: &dummyGrader{}},
		DeclaredKinds: []string{testKindQuiz, testKindReadingMCQ},
	})
}

// The constructor must not invent graders. It did once: with no Graders supplied
// it registered domain.FakeGrader — which returns 100/100 for any response — for
// three activity kinds, and cmd/api supplies none.
func TestModule_New_RegistersNoGraderByDefault(t *testing.T) {
	mod := learning.New(learning.Deps{Guard: dummyGuard{}})
	if mod == nil {
		t.Fatal("expected non-nil learning module")
	}
	for _, kind := range []string{testKindQuiz, testKindMultipleChoice, testKindReadingMCQ, "fake_grader"} {
		if _, ok := mod.Grader(kind); ok {
			t.Errorf("constructor registered a grader for %q without being asked", kind)
		}
	}
}

func TestModule_RotatePartitions_NilPool(t *testing.T) {
	mod := learning.New(learning.Deps{
		Guard: dummyGuard{},
	})
	err := mod.RotatePartitions(context.Background())
	if err == nil {
		t.Fatal("expected error with nil pool")
	}
}

type dummyGrader struct{}

func (dummyGrader) Grade(_ context.Context, _ contract.GradeRequest) (contract.GradeResult, error) {
	return contract.GradeResult{Score: 100, MaxScore: 100, Correct: true}, nil
}
