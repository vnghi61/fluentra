package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

// PartConstraints reads an exam part's published format as the learning
// generator's structure check expects it (WO 22 Stage I).
//
// The exam module owns assess.exam_parts, so it is the module that answers
// this. Questionbank declares the interface it needs, so questionbank does not
// have to import exam — exam depends on questionbank, and an import back would
// be a cycle.
func (s *Service) PartConstraints(
	ctx context.Context, partID uuid.UUID,
) (*learningcontract.ExamPartConstraints, error) {
	part, err := s.repo.GetExamPartByID(ctx, partID)
	if err != nil {
		return nil, err
	}
	if part == nil {
		return nil, nil
	}

	constraints := &learningcontract.ExamPartConstraints{}
	if len(part.Constraints) > 0 {
		_ = json.Unmarshal(part.Constraints, constraints)
	}
	// A listening item without a script cannot be sat, and the spec's section
	// is the statement that there is a recording.
	if part.Section == skillListening {
		constraints.AudioRequired = true
	}
	return constraints, nil
}
