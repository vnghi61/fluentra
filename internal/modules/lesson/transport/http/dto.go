// Package http implements the HTTP transport handlers and DTOs for the lesson module.
package http

import (
	"encoding/json"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
)

// CourseSummaryResponse matches OpenAPI CourseSummary.
type CourseSummaryResponse struct {
	ID             uuid.UUID `json:"id"`
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Description    string    `json:"description,omitempty"`
	CEFRFrom       string    `json:"cefr_from"`
	CEFRTo         string    `json:"cefr_to"`
	Status         string    `json:"status"`
	EstimatedHours int       `json:"estimated_hours"`
}

// CourseListResponse matches OpenAPI CourseList.
type CourseListResponse struct {
	Courses []CourseSummaryResponse `json:"courses"`
}

// LessonSummaryResponse matches OpenAPI LessonSummary.
type LessonSummaryResponse struct {
	ID               uuid.UUID `json:"id"`
	UnitID           uuid.UUID `json:"unit_id"`
	Position         int       `json:"position"`
	Title            string    `json:"title"`
	SkillFocus       string    `json:"skill_focus"`
	EstimatedMinutes int       `json:"estimated_minutes"`
	Status           string    `json:"status"`
	Locked           bool      `json:"locked"`
	LockReason       *string   `json:"lock_reason"`
}

// CourseUnitResponse matches OpenAPI CourseUnit.
type CourseUnitResponse struct {
	ID          uuid.UUID               `json:"id"`
	CourseID    uuid.UUID               `json:"course_id"`
	Position    int                     `json:"position"`
	Title       string                  `json:"title"`
	Description string                  `json:"description,omitempty"`
	Lessons     []LessonSummaryResponse `json:"lessons"`
}

// CourseDetailResponse matches OpenAPI CourseDetail.
type CourseDetailResponse struct {
	ID             uuid.UUID            `json:"id"`
	Slug           string               `json:"slug"`
	Title          string               `json:"title"`
	Description    string               `json:"description,omitempty"`
	CEFRFrom       string               `json:"cefr_from"`
	CEFRTo         string               `json:"cefr_to"`
	Status         string               `json:"status"`
	EstimatedHours int                  `json:"estimated_hours"`
	Units          []CourseUnitResponse `json:"units"`
}

// LessonActivityResponse matches OpenAPI LessonActivity.
type LessonActivityResponse struct {
	ID               uuid.UUID                `json:"id"`
	LessonID         uuid.UUID                `json:"lesson_id"`
	Position         int                      `json:"position"`
	Kind             string                   `json:"kind"`
	ContentVersionID uuid.UUID                `json:"content_version_id"`
	Config           json.RawMessage          `json:"config,omitempty"`
	Weight           int                      `json:"weight"`
	Content          *contentcontract.Version `json:"content,omitempty"`
}

// LessonDetailResponse matches OpenAPI LessonDetail.
type LessonDetailResponse struct {
	ID               uuid.UUID                `json:"id"`
	UnitID           uuid.UUID                `json:"unit_id"`
	Position         int                      `json:"position"`
	Title            string                   `json:"title"`
	SkillFocus       string                   `json:"skill_focus"`
	EstimatedMinutes int                      `json:"estimated_minutes"`
	Status           string                   `json:"status"`
	Activities       []LessonActivityResponse `json:"activities"`
}

// CreateCourseRequest payload for POST /admin/courses.
type CreateCourseRequest struct {
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	CEFRFrom       string `json:"cefr_from"`
	CEFRTo         string `json:"cefr_to"`
	EstimatedHours int    `json:"estimated_hours"`
}

// ActivityInput for PUT /admin/lessons/{id}/activities.
type ActivityInput struct {
	Position         int             `json:"position"`
	Kind             string          `json:"kind"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	Config           json.RawMessage `json:"config"`
	Weight           int             `json:"weight"`
}

// UpdateActivitiesRequest payload for PUT /admin/lessons/{id}/activities.
type UpdateActivitiesRequest struct {
	Activities []ActivityInput `json:"activities"`
}

// UpdateActivitiesResponse matches the returned activity array for PUT /admin/lessons/{id}/activities.
type UpdateActivitiesResponse struct {
	Activities []LessonActivityResponse `json:"activities"`
}

func toCourseSummaryResponse(dto service.CourseSummaryDTO) CourseSummaryResponse {
	return CourseSummaryResponse{
		ID:             dto.ID,
		Slug:           dto.Slug,
		Title:          dto.Title,
		Description:    dto.Description,
		CEFRFrom:       dto.CEFRFrom,
		CEFRTo:         dto.CEFRTo,
		Status:         dto.Status,
		EstimatedHours: dto.EstimatedHours,
	}
}

func toCourseDetailResponse(dto *service.CourseDetailDTO) CourseDetailResponse {
	units := make([]CourseUnitResponse, len(dto.Units))
	for i, u := range dto.Units {
		lessons := make([]LessonSummaryResponse, len(u.Lessons))
		for j, l := range u.Lessons {
			lessons[j] = LessonSummaryResponse{
				ID:               l.ID,
				UnitID:           l.UnitID,
				Position:         l.Position,
				Title:            l.Title,
				SkillFocus:       l.SkillFocus,
				EstimatedMinutes: l.EstimatedMinutes,
				Status:           l.Status,
				Locked:           l.Locked,
				LockReason:       l.LockReason,
			}
		}
		units[i] = CourseUnitResponse{
			ID:          u.ID,
			CourseID:    u.CourseID,
			Position:    u.Position,
			Title:       u.Title,
			Description: u.Description,
			Lessons:     lessons,
		}
	}

	return CourseDetailResponse{
		ID:             dto.ID,
		Slug:           dto.Slug,
		Title:          dto.Title,
		Description:    dto.Description,
		CEFRFrom:       dto.CEFRFrom,
		CEFRTo:         dto.CEFRTo,
		Status:         dto.Status,
		EstimatedHours: dto.EstimatedHours,
		Units:          units,
	}
}

func toLessonDetailResponse(dto *service.LessonDetailDTO) LessonDetailResponse {
	acts := make([]LessonActivityResponse, len(dto.Activities))
	for i, a := range dto.Activities {
		acts[i] = LessonActivityResponse{
			ID:               a.ID,
			LessonID:         a.LessonID,
			Position:         a.Position,
			Kind:             a.Kind,
			ContentVersionID: a.ContentVersionID,
			Config:           a.Config,
			Weight:           a.Weight,
			Content:          a.Content,
		}
	}

	return LessonDetailResponse{
		ID:               dto.ID,
		UnitID:           dto.UnitID,
		Position:         dto.Position,
		Title:            dto.Title,
		SkillFocus:       dto.SkillFocus,
		EstimatedMinutes: dto.EstimatedMinutes,
		Status:           dto.Status,
		Activities:       acts,
	}
}
