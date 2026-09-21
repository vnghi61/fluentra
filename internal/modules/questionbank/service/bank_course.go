// Package service provides question bank orchestration, generation, and publishing logic.
package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
)

const (
	// BankCourseSlug is the slug of the course holding question bank activities.
	BankCourseSlug        = "pool-bank"
	bankCourseTitle       = "Question Bank"
	bankCourseDescription = "Reusable item bank activities pool"
)

type bankCourseManager struct {
	lessonAuthor lessoncontract.Author
}

func newBankCourseManager(author lessoncontract.Author) *bankCourseManager {
	return &bankCourseManager{lessonAuthor: author}
}

// ensureBankLesson ensures the pool-bank course, exam version unit, and part lesson exist.
func (m *bankCourseManager) ensureBankLesson(ctx context.Context, examVersion, partOrKind string) (uuid.UUID, error) {
	if m.lessonAuthor == nil {
		return uuid.Nil, fmt.Errorf("lesson author is required to publish bank items")
	}

	courseID, err := m.lessonAuthor.EnsureCourse(ctx, lessoncontract.CourseSpec{
		Slug:           BankCourseSlug,
		Title:          bankCourseTitle,
		Description:    bankCourseDescription,
		CEFRFrom:       "A1",
		CEFRTo:         "C2",
		EstimatedHours: 200,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("ensure bank course: %w", err)
	}

	unitTitle := strings.ToUpper(strings.TrimSpace(examVersion))
	if unitTitle == "" {
		unitTitle = "GENERAL"
	}

	unitID, err := m.lessonAuthor.EnsureUnit(ctx, lessoncontract.UnitSpec{
		CourseID:    courseID,
		Position:    1,
		Title:       unitTitle,
		Description: fmt.Sprintf("%s assessment items", unitTitle),
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("ensure bank unit %s: %w", unitTitle, err)
	}

	lessonTitle := strings.TrimSpace(partOrKind)
	if lessonTitle == "" {
		lessonTitle = "Default"
	}

	lessonID, err := m.lessonAuthor.EnsureLesson(ctx, lessoncontract.LessonSpec{
		UnitID:           unitID,
		Position:         1,
		Title:            lessonTitle,
		SkillFocus:       lessonTitle,
		EstimatedMinutes: 10,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("ensure bank lesson %s: %w", lessonTitle, err)
	}

	return lessonID, nil
}
