package http

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
)

// StartAttemptResponse matches the OpenAPI schema for POST /activities/{id}/attempts.
type StartAttemptResponse struct {
	AttemptID  uuid.UUID `json:"attempt_id"`
	ActivityID uuid.UUID `json:"activity_id"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
}

// SubmitAttemptRequest matches the OpenAPI schema for POST /attempts/{id}/submit request body.
type SubmitAttemptRequest struct {
	Response json.RawMessage `json:"response"`
}

// SubmitAttemptResponse matches the OpenAPI schema for POST /attempts/{id}/submit (200 & 202).
type SubmitAttemptResponse struct {
	AttemptID uuid.UUID `json:"attempt_id"`
	Status    string    `json:"status"`
	Score     *int      `json:"score"`
	MaxScore  *int      `json:"max_score"`
	Correct   *bool     `json:"correct"`
	Feedback  *string   `json:"feedback"`
}

// AttemptDetailResponse matches the OpenAPI schema for GET /attempts/{id}.
type AttemptDetailResponse struct {
	ID          uuid.UUID       `json:"id"`
	ActivityID  uuid.UUID       `json:"activity_id"`
	UserID      uuid.UUID       `json:"user_id"`
	Status      string          `json:"status"`
	Response    json.RawMessage `json:"response,omitempty"`
	Score       *int            `json:"score,omitempty"`
	MaxScore    *int            `json:"max_score,omitempty"`
	Feedback    *string         `json:"feedback,omitempty"`
	DurationMs  *int64          `json:"duration_ms,omitempty"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

func toStartAttemptResponse(dto *service.StartAttemptDTO) StartAttemptResponse {
	return StartAttemptResponse{
		AttemptID:  dto.AttemptID,
		ActivityID: dto.ActivityID,
		Status:     dto.Status,
		StartedAt:  dto.StartedAt,
	}
}

func toSubmitAttemptResponse(dto *service.SubmitAttemptResultDTO) SubmitAttemptResponse {
	return SubmitAttemptResponse{
		AttemptID: dto.AttemptID,
		Status:    dto.Status,
		Score:     dto.Score,
		MaxScore:  dto.MaxScore,
		Correct:   dto.Correct,
		Feedback:  dto.Feedback,
	}
}

func toAttemptDetailResponse(dto *service.AttemptDetailDTO) AttemptDetailResponse {
	return AttemptDetailResponse{
		ID:          dto.ID,
		ActivityID:  dto.ActivityID,
		UserID:      dto.UserID,
		Status:      dto.Status,
		Response:    dto.Response,
		Score:       dto.Score,
		MaxScore:    dto.MaxScore,
		Feedback:    dto.Feedback,
		DurationMs:  dto.DurationMs,
		StartedAt:   dto.StartedAt,
		CompletedAt: dto.CompletedAt,
	}
}

// EnrollmentResponse matches OpenAPI Enrollment schema.
type EnrollmentResponse struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	CourseID    uuid.UUID  `json:"course_id"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

func toEnrollmentResponse(e *domain.Enrollment) EnrollmentResponse {
	return EnrollmentResponse{
		ID:          e.ID,
		UserID:      e.UserID,
		CourseID:    e.CourseID,
		Status:      e.Status,
		StartedAt:   e.StartedAt,
		CompletedAt: e.CompletedAt,
	}
}

// StartSessionRequest matches OpenAPI StartSessionRequest schema.
type StartSessionRequest struct {
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// CompleteSessionRequest matches OpenAPI CompleteSessionRequest schema.
type CompleteSessionRequest struct {
	ActivitiesCompleted *int `json:"activities_completed,omitempty"`
}

// LearningSessionResponse matches OpenAPI LearningSession schema.
type LearningSessionResponse struct {
	ID                  uuid.UUID  `json:"id"`
	UserID              uuid.UUID  `json:"user_id"`
	StartedAt           time.Time  `json:"started_at"`
	EndedAt             *time.Time `json:"ended_at,omitempty"`
	ActivitiesCompleted int        `json:"activities_completed"`
	Minutes             int        `json:"minutes"`
}

func toLearningSessionResponse(s *domain.LearningSession) LearningSessionResponse {
	return LearningSessionResponse{
		ID:                  s.ID,
		UserID:              s.UserID,
		StartedAt:           s.StartedAt,
		EndedAt:             s.EndedAt,
		ActivitiesCompleted: s.ActivitiesCompleted,
		Minutes:             s.Minutes,
	}
}

// SkillMasteryResponse matches OpenAPI SkillMastery schema.
type SkillMasteryResponse struct {
	Skill      string    `json:"skill"`
	Level      string    `json:"level"`
	Confidence float64   `json:"confidence"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NextActivityResponse matches OpenAPI NextActivity schema.
type NextActivityResponse struct {
	ActivityID       uuid.UUID `json:"activity_id"`
	LessonID         uuid.UUID `json:"lesson_id"`
	UnitID           uuid.UUID `json:"unit_id"`
	CourseID         uuid.UUID `json:"course_id"`
	Title            string    `json:"title"`
	Kind             string    `json:"kind"`
	Skill            string    `json:"skill"`
	EstimatedMinutes *int      `json:"estimated_minutes,omitempty"`
}

// DashboardResponse matches OpenAPI DashboardResponse schema.
type DashboardResponse struct {
	State           string                 `json:"state"`
	NextActivity    *NextActivityResponse  `json:"next_activity,omitempty"`
	DueReviewsCount int                    `json:"due_reviews_count"`
	SkillMastery    []SkillMasteryResponse `json:"skill_mastery"`
}

// CourseProgressResponse matches OpenAPI CourseProgress schema.
type CourseProgressResponse struct {
	CourseID            uuid.UUID  `json:"course_id"`
	Status              string     `json:"status"`
	CompletedActivities int        `json:"completed_activities"`
	TotalActivities     int        `json:"total_activities"`
	Percentage          int        `json:"percentage"`
	Score               *int       `json:"score,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

// ProgressResponse matches OpenAPI ProgressResponse schema.
type ProgressResponse struct {
	Courses []CourseProgressResponse `json:"courses"`
	Skills  []SkillMasteryResponse   `json:"skills"`
}

func toSkillMasteryResponse(m domain.SkillMastery) SkillMasteryResponse {
	return SkillMasteryResponse{
		Skill:      m.Skill,
		Level:      m.Level,
		Confidence: m.Confidence,
		UpdatedAt:  m.UpdatedAt,
	}
}

func toNextActivityResponse(a *domain.NextActivity) *NextActivityResponse {
	if a == nil {
		return nil
	}
	return &NextActivityResponse{
		ActivityID:       a.ActivityID,
		LessonID:         a.LessonID,
		UnitID:           a.UnitID,
		CourseID:         a.CourseID,
		Title:            a.Title,
		Kind:             a.Kind,
		Skill:            a.Skill,
		EstimatedMinutes: a.EstimatedMinutes,
	}
}

func toDashboardResponse(d *domain.DashboardData) DashboardResponse {
	masteries := make([]SkillMasteryResponse, len(d.SkillMastery))
	for i, m := range d.SkillMastery {
		masteries[i] = toSkillMasteryResponse(m)
	}
	return DashboardResponse{
		State:           d.State,
		NextActivity:    toNextActivityResponse(d.NextActivity),
		DueReviewsCount: d.DueReviewsCount,
		SkillMastery:    masteries,
	}
}

func toProgressResponse(p *domain.ProgressData) ProgressResponse {
	courses := make([]CourseProgressResponse, len(p.Courses))
	for i, c := range p.Courses {
		courses[i] = CourseProgressResponse{
			CourseID:            c.CourseID,
			Status:              c.Status,
			CompletedActivities: c.CompletedActivities,
			TotalActivities:     c.TotalActivities,
			Percentage:          c.Percentage,
			Score:               c.Score,
			CompletedAt:         c.CompletedAt,
		}
	}
	skills := make([]SkillMasteryResponse, len(p.Skills))
	for i, s := range p.Skills {
		skills[i] = toSkillMasteryResponse(s)
	}
	return ProgressResponse{
		Courses: courses,
		Skills:  skills,
	}
}
