package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/content/service"
)

func TestApproveReviewBatch_RequiresABatchAndAReviewer(t *testing.T) {
	svc := service.New(service.Deps{})
	ctx := context.Background()

	_, err := svc.ApproveReviewBatch(ctx, uuid.New(), "", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CONTENT_BATCH_REQUIRED")

	_, err = svc.ApproveReviewBatch(ctx, uuid.Nil, "foundation:PRESENT_PERFECT:run", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CONTENT_REVIEWER_REQUIRED")
}
