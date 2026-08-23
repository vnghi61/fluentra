// Package repository implements data access and database operations for the lesson module.
package repository

import (
	"encoding/json"

	"github.com/fluentra/fluentra/internal/generated/lesson/sqlc"
	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
)

// ToContractCourse maps an sqlc LearnCourse row to a contract.Course domain object.
func ToContractCourse(c sqlc.LearnCourse) *contract.Course {
	return &contract.Course{
		ID:             c.ID,
		Slug:           c.Slug,
		Title:          c.Title,
		Description:    c.Description,
		CEFRFrom:       c.CefrFrom,
		CEFRTo:         c.CefrTo,
		Status:         c.Status,
		EstimatedHours: int(c.EstimatedHours),
	}
}

// ToContractCourses maps a slice of sqlc LearnCourse rows.
func ToContractCourses(rows []sqlc.LearnCourse) []*contract.Course {
	courses := make([]*contract.Course, len(rows))
	for i, r := range rows {
		courses[i] = ToContractCourse(r)
	}
	return courses
}

// ToContractUnit maps an sqlc LearnCourseUnit row to a contract.Unit domain object.
func ToContractUnit(u sqlc.LearnCourseUnit) *contract.Unit {
	return &contract.Unit{
		ID:          u.ID,
		CourseID:    u.CourseID,
		Position:    int(u.Position),
		Title:       u.Title,
		Description: u.Description,
	}
}

// ToContractUnits maps a slice of sqlc LearnCourseUnit rows.
func ToContractUnits(rows []sqlc.LearnCourseUnit) []*contract.Unit {
	units := make([]*contract.Unit, len(rows))
	for i, r := range rows {
		units[i] = ToContractUnit(r)
	}
	return units
}

// ToContractActivity maps an sqlc LearnActivity row to a contract.Activity domain object.
func ToContractActivity(a sqlc.LearnActivity) contract.Activity {
	var cfg json.RawMessage
	if len(a.Config) > 0 {
		cfg = json.RawMessage(a.Config)
	} else {
		cfg = json.RawMessage("{}")
	}

	return contract.Activity{
		ID:               a.ID,
		LessonID:         a.LessonID,
		Position:         int(a.Position),
		Kind:             a.Kind,
		ContentVersionID: a.ContentVersionID,
		Config:           cfg,
		Weight:           int(a.Weight),
	}
}

// ToContractActivities maps a slice of sqlc LearnActivity rows.
func ToContractActivities(rows []sqlc.LearnActivity) []contract.Activity {
	activities := make([]contract.Activity, len(rows))
	for i, r := range rows {
		activities[i] = ToContractActivity(r)
	}
	return activities
}

// ToContractLesson maps an sqlc LearnLesson row and its activities to a contract.Lesson domain object.
func ToContractLesson(l sqlc.LearnLesson, activities []contract.Activity) *contract.Lesson {
	return &contract.Lesson{
		ID:               l.ID,
		UnitID:           l.UnitID,
		Position:         int(l.Position),
		Title:            l.Title,
		SkillFocus:       l.SkillFocus,
		EstimatedMinutes: int(l.EstimatedMinutes),
		Status:           l.Status,
		Activities:       activities,
	}
}

// ToPrerequisiteEdges maps sqlc ListAllPrerequisitesInCourseRow slice to domain.PrerequisiteEdge slice.
func ToPrerequisiteEdges(rows []sqlc.ListAllPrerequisitesInCourseRow) []domain.PrerequisiteEdge {
	edges := make([]domain.PrerequisiteEdge, len(rows))
	for i, r := range rows {
		edges[i] = domain.PrerequisiteEdge{
			LessonID:         r.LessonID,
			RequiresLessonID: r.RequiresLessonID,
		}
	}
	return edges
}
