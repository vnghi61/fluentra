package domain

import (
	"context"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
)

// FakeGrader provides a deterministic synchronous grader for Phase 2 development and testing.
type FakeGrader struct {
	Score       int
	MaxScore    int
	Correct     bool
	Feedback    string
	ReviewItems []contract.ReviewItem
}

// NewFakeGrader returns a FakeGrader that awards 100/100 by default.
func NewFakeGrader() *FakeGrader {
	return &FakeGrader{
		Score:    100,
		MaxScore: 100,
		Correct:  true,
		Feedback: "Correct! Well done.",
	}
}

// Grade implements contract.ExerciseGrader.
func (f *FakeGrader) Grade(_ context.Context, req contract.GradeRequest) (contract.GradeResult, error) {
	reviewItems := f.ReviewItems
	if reviewItems == nil {
		reviewItems = []contract.ReviewItem{
			{
				ContentVersionID: req.ContentVersionID,
				Skill:            "vocabulary",
				InitialGrade:     "good",
			},
		}
	}
	return contract.GradeResult{
		Score:       f.Score,
		MaxScore:    f.MaxScore,
		Correct:     f.Correct,
		Feedback:    f.Feedback,
		Async:       false,
		ReviewItems: reviewItems,
	}, nil
}

// AsyncFakeGrader simulates an asynchronous AI-based grader for Phase 3 readiness.
type AsyncFakeGrader struct{}

// NewAsyncFakeGrader creates an AsyncFakeGrader.
func NewAsyncFakeGrader() *AsyncFakeGrader {
	return &AsyncFakeGrader{}
}

// Grade implements contract.ExerciseGrader, always returning Async: true.
func (a *AsyncFakeGrader) Grade(_ context.Context, _ contract.GradeRequest) (contract.GradeResult, error) {
	return contract.GradeResult{
		Async: true,
	}, nil
}
