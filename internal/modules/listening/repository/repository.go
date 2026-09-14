// Package repository provides database persistence for the listening module.
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	sqlclistening "github.com/fluentra/fluentra/internal/generated/listening/sqlc"
	"github.com/fluentra/fluentra/internal/modules/listening/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository provides typed access to listening persistence.
type Repository struct {
	queries *sqlclistening.Queries
}

// New constructs a new listening Repository.
func New(db dbx.Querier) *Repository {
	return &Repository{queries: sqlclistening.New(db)}
}

// CountPlays returns how many times a user has played a listening clip in a specific context.
func (r *Repository) CountPlays(ctx context.Context, userID, contentVersionID, contextID uuid.UUID) (int64, error) {
	count, err := r.queries.CountPlays(ctx, sqlclistening.CountPlaysParams{
		UserID:           userID,
		ContentVersionID: contentVersionID,
		ContextID:        contextID,
	})
	if err != nil {
		return 0, fmt.Errorf("count listening plays: %w", err)
	}
	return count, nil
}

// RecordPlay inserts a record of a playback event.
func (r *Repository) RecordPlay(
	ctx context.Context,
	id, userID, contentVersionID uuid.UUID,
	contextType string,
	contextID uuid.UUID,
) (domain.PlayRecord, error) {
	row, err := r.queries.RecordPlay(ctx, sqlclistening.RecordPlayParams{
		ID:               id,
		UserID:           userID,
		ContentVersionID: contentVersionID,
		ContextType:      contextType,
		ContextID:        contextID,
	})
	if err != nil {
		return domain.PlayRecord{}, fmt.Errorf("record listening play: %w", err)
	}
	return domain.PlayRecord{
		ID:               row.ID,
		UserID:           row.UserID,
		ContentVersionID: row.ContentVersionID,
		ContextType:      row.ContextType,
		ContextID:        row.ContextID,
		PlayedAt:         row.PlayedAt,
	}, nil
}
