package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fluentra/fluentra/internal/generated/learning/sqlc"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

// GetActivePlacementSessionByUser returns the currently active placement test session, if any.
func (r *Repository) GetActivePlacementSessionByUser(
	ctx context.Context, userID uuid.UUID,
) (*domain.PlacementSession, error) {
	row, err := r.queries.GetActivePlacementSessionByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toDomainPlacementSession(row), nil
}

// GetPlacementSessionByID finds a placement session by its ID.
func (r *Repository) GetPlacementSessionByID(
	ctx context.Context, id uuid.UUID,
) (*domain.PlacementSession, error) {
	row, err := r.queries.GetPlacementSessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toDomainPlacementSession(row), nil
}

// GetLatestCompletedPlacementSession returns the most recently completed placement test session for a user.
func (r *Repository) GetLatestCompletedPlacementSession(
	ctx context.Context, userID uuid.UUID,
) (*domain.PlacementSession, error) {
	row, err := r.queries.GetLatestCompletedPlacementSession(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toDomainPlacementSession(row), nil
}

// CreatePlacementSession records a newly started adaptive placement test session.
func (r *Repository) CreatePlacementSession(
	ctx context.Context, s *domain.PlacementSession,
) (*domain.PlacementSession, error) {
	var thetaNum pgtype.Numeric
	_ = thetaNum.Scan(strconv.FormatFloat(s.ThetaEstimate, 'f', 2, 64))

	var confNum pgtype.Numeric
	_ = confNum.Scan(strconv.FormatFloat(s.Confidence, 'f', 3, 64))

	respBytes, err := json.Marshal(s.Responses)
	if err != nil {
		return nil, fmt.Errorf("marshal responses: %w", err)
	}
	stateBytes, err := json.Marshal(s.AdaptiveState)
	if err != nil {
		return nil, fmt.Errorf("marshal adaptive state: %w", err)
	}

	row, err := r.queries.CreatePlacementSession(ctx, sqlc.CreatePlacementSessionParams{
		UserID:             s.UserID,
		Status:             s.Status,
		Stage:              s.Stage,
		ThetaEstimate:      thetaNum,
		PlacedLevel:        s.PlacedLevel,
		Confidence:         confNum,
		CurrentActivityID:  s.CurrentActivityID,
		CurrentItemKind:    s.CurrentItemKind,
		CurrentItemLevel:   s.CurrentItemLevel,
		Responses:          respBytes,
		AdaptiveState:      stateBytes,
		StartedAt:          s.StartedAt,
		ExpiresAt:          s.ExpiresAt,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toDomainPlacementSession(row), nil
}

// UpdatePlacementSessionProgress updates intermediate progress of an active placement test.
func (r *Repository) UpdatePlacementSessionProgress(
	ctx context.Context, s *domain.PlacementSession,
) (*domain.PlacementSession, error) {
	var thetaNum pgtype.Numeric
	_ = thetaNum.Scan(strconv.FormatFloat(s.ThetaEstimate, 'f', 2, 64))

	var confNum pgtype.Numeric
	_ = confNum.Scan(strconv.FormatFloat(s.Confidence, 'f', 3, 64))

	respBytes, err := json.Marshal(s.Responses)
	if err != nil {
		return nil, fmt.Errorf("marshal responses: %w", err)
	}
	stateBytes, err := json.Marshal(s.AdaptiveState)
	if err != nil {
		return nil, fmt.Errorf("marshal adaptive state: %w", err)
	}

	row, err := r.queries.UpdatePlacementSessionProgress(ctx, sqlc.UpdatePlacementSessionProgressParams{
		ID:                 s.ID,
		Stage:              s.Stage,
		ThetaEstimate:      thetaNum,
		PlacedLevel:        s.PlacedLevel,
		Confidence:         confNum,
		CurrentActivityID:  s.CurrentActivityID,
		CurrentItemKind:    s.CurrentItemKind,
		CurrentItemLevel:   s.CurrentItemLevel,
		Responses:          respBytes,
		AdaptiveState:      stateBytes,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toDomainPlacementSession(row), nil
}

// CompletePlacementSession marks a placement test session as completed.
func (r *Repository) CompletePlacementSession(
	ctx context.Context, s *domain.PlacementSession,
) (*domain.PlacementSession, error) {
	var thetaNum pgtype.Numeric
	_ = thetaNum.Scan(strconv.FormatFloat(s.ThetaEstimate, 'f', 2, 64))

	var confNum pgtype.Numeric
	_ = confNum.Scan(strconv.FormatFloat(s.Confidence, 'f', 3, 64))

	respBytes, err := json.Marshal(s.Responses)
	if err != nil {
		return nil, fmt.Errorf("marshal responses: %w", err)
	}
	stateBytes, err := json.Marshal(s.AdaptiveState)
	if err != nil {
		return nil, fmt.Errorf("marshal adaptive state: %w", err)
	}

	row, err := r.queries.CompletePlacementSession(ctx, sqlc.CompletePlacementSessionParams{
		ID:            s.ID,
		ThetaEstimate: thetaNum,
		PlacedLevel:   s.PlacedLevel,
		Confidence:    confNum,
		Responses:     respBytes,
		AdaptiveState: stateBytes,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toDomainPlacementSession(row), nil
}

// ExpireStalePlacementSessions sweeps expired in-progress placement sessions.
func (r *Repository) ExpireStalePlacementSessions(ctx context.Context) (int64, error) {
	n, err := r.queries.ExpireStalePlacementSessions(ctx)
	if err != nil {
		return 0, mapPgError(err)
	}
	return n, nil
}

// CreatePlacementResult saves placement test result and links it to the session.
func (r *Repository) CreatePlacementResult(
	ctx context.Context, res *domain.PlacementResult,
) (*domain.PlacementResult, error) {
	perSkillBytes, err := json.Marshal(res.PerSkill)
	if err != nil {
		return nil, fmt.Errorf("marshal per_skill: %w", err)
	}

	row, err := r.queries.CreatePlacementResultWithSession(ctx, sqlc.CreatePlacementResultWithSessionParams{
		UserID:         res.UserID,
		EstimatedLevel: res.EstimatedLevel,
		PerSkill:       perSkillBytes,
		SessionID:      res.SessionID,
		TakenAt:        res.TakenAt,
	})
	if err != nil {
		return nil, mapPgError(err)
	}

	return toDomainPlacementResult(
		row.ID, row.UserID, row.EstimatedLevel, row.PerSkill, row.SessionID, row.TakenAt, row.CreatedAt, row.UpdatedAt,
	), nil
}

// GetWeeklyPlanByUserAndDate retrieves a cached weekly study plan.
func (r *Repository) GetWeeklyPlanByUserAndDate(
	ctx context.Context, userID uuid.UUID, weekStartDate time.Time,
) (*domain.WeeklyPlan, error) {
	var d pgtype.Date
	_ = d.Scan(weekStartDate.Format("2006-01-02"))

	row, err := r.queries.GetWeeklyPlanByUserAndDate(ctx, sqlc.GetWeeklyPlanByUserAndDateParams{
		UserID:        userID,
		WeekStartDate: d,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toDomainWeeklyPlan(row), nil
}

// UpsertWeeklyPlan stores or updates a personalized weekly plan.
func (r *Repository) UpsertWeeklyPlan(
	ctx context.Context, plan *domain.WeeklyPlan,
) (*domain.WeeklyPlan, error) {
	var d pgtype.Date
	_ = d.Scan(plan.WeekStartDate.Format("2006-01-02"))

	distBytes, err := json.Marshal(plan.TimeDistribution)
	if err != nil {
		return nil, fmt.Errorf("marshal time distribution: %w", err)
	}
	targetsBytes, err := json.Marshal(plan.DailyTargets)
	if err != nil {
		return nil, fmt.Errorf("marshal daily targets: %w", err)
	}

	row, err := r.queries.UpsertWeeklyPlan(ctx, sqlc.UpsertWeeklyPlanParams{
		UserID:           plan.UserID,
		WeekStartDate:    d,
		PlacedLevel:      plan.PlacedLevel,
		WeakestSkill:     plan.WeakestSkill,
		TimeDistribution: distBytes,
		DailyTargets:     targetsBytes,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toDomainWeeklyPlan(row), nil
}

func toDomainPlacementSession(row sqlc.LearnPlacementSession) *domain.PlacementSession {
	var theta float64
	if f, err := row.ThetaEstimate.Float64Value(); err == nil && f.Valid {
		theta = f.Float64
	}
	var conf float64
	if f, err := row.Confidence.Float64Value(); err == nil && f.Valid {
		conf = f.Float64
	}
	var responses []domain.PlacementResponseRecord
	if len(row.Responses) > 0 {
		_ = json.Unmarshal(row.Responses, &responses)
	}
	var adaptiveState domain.AdaptiveState
	if len(row.AdaptiveState) > 0 {
		_ = json.Unmarshal(row.AdaptiveState, &adaptiveState)
	}

	return &domain.PlacementSession{
		ID:                row.ID,
		UserID:            row.UserID,
		Status:            row.Status,
		Stage:             row.Stage,
		ThetaEstimate:     theta,
		PlacedLevel:       row.PlacedLevel,
		Confidence:        conf,
		CurrentActivityID: row.CurrentActivityID,
		CurrentItemKind:   row.CurrentItemKind,
		CurrentItemLevel:  row.CurrentItemLevel,
		Responses:         responses,
		AdaptiveState:     adaptiveState,
		StartedAt:         row.StartedAt,
		CompletedAt:       row.CompletedAt,
		ExpiresAt:         row.ExpiresAt,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func toDomainWeeklyPlan(row sqlc.LearnWeeklyPlan) *domain.WeeklyPlan {
	var timeDist map[string]float64
	if len(row.TimeDistribution) > 0 {
		_ = json.Unmarshal(row.TimeDistribution, &timeDist)
	}
	var dailyTargets []domain.DailyPlanTarget
	if len(row.DailyTargets) > 0 {
		_ = json.Unmarshal(row.DailyTargets, &dailyTargets)
	}

	return &domain.WeeklyPlan{
		ID:               row.ID,
		UserID:           row.UserID,
		WeekStartDate:    row.WeekStartDate.Time,
		PlacedLevel:      row.PlacedLevel,
		WeakestSkill:     row.WeakestSkill,
		TimeDistribution: timeDist,
		DailyTargets:     dailyTargets,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func toDomainPlacementResult(
	id, userID uuid.UUID, level string, perSkillRaw []byte, sessionID *uuid.UUID,
	takenAt, createdAt, updatedAt time.Time,
) *domain.PlacementResult {
	var perSkill map[string]float64
	if len(perSkillRaw) > 0 {
		_ = json.Unmarshal(perSkillRaw, &perSkill)
	}
	return &domain.PlacementResult{
		ID:             id,
		UserID:         userID,
		EstimatedLevel: level,
		PerSkill:       perSkill,
		SessionID:      sessionID,
		TakenAt:        takenAt,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}
