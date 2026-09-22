package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/generated/learning/sqlc"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

// The resource practice set's persistence. Kept beside the daily set's for the
// same reason: it is learner-owned state that the runner reads, not content.

func toResourcePracticeSet(row sqlc.LearnResourcePracticeSet) *domain.ResourcePracticeSet {
	return &domain.ResourcePracticeSet{
		ResourceID:    row.ResourceID,
		UserID:        row.UserID,
		LessonID:      row.LessonID,
		ActivityIDs:   row.ActivityIds,
		Status:        row.Status,
		FailureReason: row.FailureReason,
		GeneratedOn:   row.GeneratedOn.Time,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

// UpsertResourcePracticeSet moves a resource's set to generating for today.
func (r *Repository) UpsertResourcePracticeSet(
	ctx context.Context, resourceID, userID uuid.UUID,
) (*domain.ResourcePracticeSet, error) {
	row, err := r.queries.UpsertResourcePracticeSet(ctx, sqlc.UpsertResourcePracticeSetParams{
		ResourceID: resourceID,
		UserID:     userID,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toResourcePracticeSet(row), nil
}

// GetResourcePracticeSet returns the caller's set for a resource, or nil.
func (r *Repository) GetResourcePracticeSet(
	ctx context.Context, resourceID, userID uuid.UUID,
) (*domain.ResourcePracticeSet, error) {
	row, err := r.queries.GetResourcePracticeSet(ctx, sqlc.GetResourcePracticeSetParams{
		ResourceID: resourceID,
		UserID:     userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toResourcePracticeSet(row), nil
}

// MarkResourcePracticeSetReady records the lesson and activities the set holds.
func (r *Repository) MarkResourcePracticeSetReady(
	ctx context.Context, resourceID, userID, lessonID uuid.UUID, activityIDs []uuid.UUID,
) (*domain.ResourcePracticeSet, error) {
	row, err := r.queries.MarkResourcePracticeSetReady(ctx, sqlc.MarkResourcePracticeSetReadyParams{
		ResourceID:  resourceID,
		UserID:      userID,
		LessonID:    &lessonID,
		ActivityIds: activityIDs,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toResourcePracticeSet(row), nil
}

// MarkResourcePracticeSetFailed records why generation stopped.
func (r *Repository) MarkResourcePracticeSetFailed(
	ctx context.Context, resourceID, userID uuid.UUID, reason string,
) (*domain.ResourcePracticeSet, error) {
	row, err := r.queries.MarkResourcePracticeSetFailed(ctx, sqlc.MarkResourcePracticeSetFailedParams{
		ResourceID:    resourceID,
		UserID:        userID,
		FailureReason: reason,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toResourcePracticeSet(row), nil
}

// ClaimGeneratingResourcePracticeSets lists the sets a generation sweep should
// work on, oldest first.
func (r *Repository) ClaimGeneratingResourcePracticeSets(
	ctx context.Context, limit int32,
) ([]domain.ResourcePracticeSet, error) {
	rows, err := r.queries.ClaimGeneratingResourcePracticeSets(ctx, limit)
	if err != nil {
		return nil, mapPgError(err)
	}
	sets := make([]domain.ResourcePracticeSet, 0, len(rows))
	for _, row := range rows {
		sets = append(sets, *toResourcePracticeSet(row))
	}
	return sets, nil
}

// DeleteResourcePracticeSetsForUser erases a learner's sets (user.deleted).
func (r *Repository) DeleteResourcePracticeSetsForUser(ctx context.Context, userID uuid.UUID) error {
	if err := r.queries.DeleteResourcePracticeSetsForUser(ctx, userID); err != nil {
		return fmt.Errorf("delete resource practice sets: %w", mapPgError(err))
	}
	return nil
}
