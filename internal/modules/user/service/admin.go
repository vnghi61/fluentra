package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// SearchUsers implements contract.AdminReader.
func (s *Service) SearchUsers(
	ctx context.Context,
	filter contract.UserFilter,
	cursor string,
	limit int,
) ([]contract.UserSummary, string, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var cursorID *uuid.UUID
	var cursorTime *time.Time
	if cursor != "" {
		cID, cTime, err := parseCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		cursorID = &cID
		cursorTime = &cTime
	}

	// Fetch limit + 1 to check if next_cursor exists
	rows, err := s.repo.SearchUsersAdmin(ctx, filter, cursorID, cursorTime, limit+1)
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(rows) > limit {
		lastItem := rows[limit-1]
		nextCursor = encodeCursor(lastItem.ID, lastItem.CreatedAt)
		rows = rows[:limit]
	}

	summaries := make([]contract.UserSummary, 0, len(rows))
	for _, r := range rows {
		summaries = append(summaries, contract.UserSummary{
			ID:          r.ID,
			Email:       r.Email,
			DisplayName: r.DisplayName,
			Status:      string(r.Status),
			CreatedAt:   r.CreatedAt,
		})
	}

	return summaries, nextCursor, nil
}

// GetUserByID implements contract.AdminReader.
func (s *Service) GetUserByID(ctx context.Context, id uuid.UUID) (*contract.UserDetail, error) {
	user, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.GetProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	pref, err := s.repo.GetPreferences(ctx, id)
	if err != nil {
		// Default to 'en' if preferences not found
		pref = domain.Preferences{Locale: "en"}
	}

	return &contract.UserDetail{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: profile.DisplayName,
		Locale:      pref.Locale,
		Timezone:    profile.Timezone,
		Status:      string(user.Status),
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}, nil
}

// SuspendUser implements contract.AdminManager.
func (s *Service) SuspendUser(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	if id == actorID {
		return fmt.Errorf("cannot suspend self: %w", domain.ErrAccountNotUsable)
	}

	return dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		if err := repo.UpdateUserStatus(ctx, id, domain.StatusSuspended); err != nil {
			return err
		}

		_, err := s.events.Write(ctx, tx, contract.Aggregate, contract.EventSuspended, map[string]any{
			"user_id":     id,
			"actor_id":    actorID,
			"reason":      reason,
			"occurred_at": s.clock.Now(),
		})
		return err
	})
}

// ReinstateUser implements contract.AdminManager.
func (s *Service) ReinstateUser(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	return dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		if err := repo.UpdateUserStatus(ctx, id, domain.StatusActive); err != nil {
			return err
		}

		_, err := s.events.Write(ctx, tx, contract.Aggregate, contract.EventReinstated, map[string]any{
			"user_id":     id,
			"actor_id":    actorID,
			"reason":      reason,
			"occurred_at": s.clock.Now(),
		})
		return err
	})
}

func parseCursor(cursor string) (uuid.UUID, time.Time, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return uuid.Nil, time.Time{}, err
	}
	parts := strings.Split(string(decoded), ",")
	if len(parts) != 2 {
		return uuid.Nil, time.Time{}, fmt.Errorf("invalid cursor split")
	}
	id, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return uuid.Nil, time.Time{}, err
	}
	return id, t, nil
}

func encodeCursor(id uuid.UUID, createdAt time.Time) string {
	raw := id.String() + "," + createdAt.Format(time.RFC3339Nano)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}
