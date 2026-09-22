package domain

import (
	"context"
	"encoding/json"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
)

// MaterialGrader completes a lesson_material activity. Opening the document or
// watching the video is the whole task, so an explicit `{"done": true}` scores
// full marks (WO 20, D20-8). Video progress is not tracked.
type MaterialGrader struct{}

// NewMaterialGrader returns a MaterialGrader.
func NewMaterialGrader() *MaterialGrader { return &MaterialGrader{} }

// Grade implements contract.ExerciseGrader.
func (g *MaterialGrader) Grade(
	_ context.Context, req contract.GradeRequest,
) (contract.GradeResult, error) {
	var response struct {
		Done bool `json:"done"`
	}
	if len(req.Response) > 0 {
		_ = json.Unmarshal(req.Response, &response)
	}
	if !response.Done {
		return contract.GradeResult{
			Feedback: "Mark the material as done to complete it.",
		}, nil
	}
	return contract.GradeResult{
		Score:    100,
		MaxScore: 100,
		Correct:  true,
		Feedback: "Done.",
	}, nil
}
