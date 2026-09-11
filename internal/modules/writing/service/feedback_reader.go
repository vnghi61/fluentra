package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/writing/contract"
)

// FeedbackQueries is the storage a FeedbackReader reads from.
type FeedbackQueries interface {
	GetWritingFeedback(ctx context.Context, attemptID, userID uuid.UUID) (*contract.WritingFeedback, error)
	ListWritingSubmissions(
		ctx context.Context, userID uuid.UUID, page, pageSize int,
	) (*contract.WritingSubmissionList, error)
}

// FeedbackReaderDeps are the dependencies of a FeedbackReader.
type FeedbackReaderDeps struct {
	Queries FeedbackQueries
}

// FeedbackReader serves a learner's own writing feedback and submissions.
type FeedbackReader struct {
	queries FeedbackQueries
}

// NewFeedbackReader constructs a FeedbackReader.
func NewFeedbackReader(deps FeedbackReaderDeps) *FeedbackReader {
	return &FeedbackReader{queries: deps.Queries}
}

// GetWritingFeedback returns the feedback on one of the learner's attempts.
func (r *FeedbackReader) GetWritingFeedback(
	ctx context.Context, attemptID, userID uuid.UUID,
) (*contract.WritingFeedback, error) {
	return r.queries.GetWritingFeedback(ctx, attemptID, userID)
}

// ListWritingSubmissions returns one page of the learner's submissions.
func (r *FeedbackReader) ListWritingSubmissions(
	ctx context.Context, userID uuid.UUID, page, pageSize int,
) (*contract.WritingSubmissionList, error) {
	return r.queries.ListWritingSubmissions(ctx, userID, page, pageSize)
}

var _ contract.FeedbackReader = (*FeedbackReader)(nil)
