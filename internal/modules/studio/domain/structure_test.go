package domain_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/studio/domain"
)

func createValidCourseStructure(unitsCount, lessonsPerUnit, activitiesPerLesson int, kind string) []byte {
	var units []domain.UnitDraft
	for u := 1; u <= unitsCount; u++ {
		var lessons []domain.LessonDraft
		for l := 1; l <= lessonsPerUnit; l++ {
			var activities []domain.ActivityDraft
			for a := 1; a <= activitiesPerLesson; a++ {
				activities = append(activities, domain.ActivityDraft{
					Kind:   kind,
					Weight: 10,
					Body:   json.RawMessage(`{"prompt":"Select the correct word","answer":"test"}`),
				})
			}
			lessons = append(lessons, domain.LessonDraft{
				Title:            fmt.Sprintf("Lesson %d-%d", u, l),
				SkillFocus:       "vocabulary",
				EstimatedMinutes: 10,
				CEFRLevel:        "B1",
				Activities:       activities,
			})
		}
		units = append(units, domain.UnitDraft{
			Title:       fmt.Sprintf("Unit %d", u),
			Description: "Unit description",
			Lessons:     lessons,
		})
	}

	raw, _ := json.Marshal(domain.CourseStructure{Units: units})
	return raw
}

func TestValidateStructureAndSafety_ValidDraft(t *testing.T) {
	// 1 unit, 3 lessons, 7 activities per lesson = 21 activities in total
	raw := createValidCourseStructure(1, 3, 7, "vocab_multiple_choice")
	structure, failures, err := domain.ValidateStructureAndSafety("Valid Course Title", "A great course description", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("expected 0 failures, got %d: %+v", len(failures), failures)
	}
	if structure == nil || len(structure.Units) != 1 {
		t.Fatalf("expected parsed structure with 1 unit")
	}
}

func TestValidateStructureAndSafety_Check1_UnitLimitsAndTitles(t *testing.T) {
	// Empty title
	raw := createValidCourseStructure(1, 3, 7, "vocab_multiple_choice")
	var s domain.CourseStructure
	_ = json.Unmarshal(raw, &s)
	s.Units[0].Title = ""
	sRaw, _ := json.Marshal(s)

	_, failures, err := domain.ValidateStructureAndSafety("Title", "Description", sRaw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, f := range failures {
		if f.Check == "structure" && f.Message == "Unit 1 has empty title" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected empty unit title failure, got: %+v", failures)
	}
}

func TestValidateStructureAndSafety_Check2_MinimumSize(t *testing.T) {
	// 1 unit, 2 lessons, 5 activities = 10 activities (less than 3 lessons and less than 20 activities)
	raw := createValidCourseStructure(1, 2, 5, "vocab_multiple_choice")
	_, failures, err := domain.ValidateStructureAndSafety("Title", "Description", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundLessons := false
	foundActivities := false
	for _, f := range failures {
		if f.Check == "minimum_size" {
			if f.Message == "Course must have at least 3 lessons in total, found 2" {
				foundLessons = true
			}
			if f.Message == "Course must have at least 20 activities in total, found 10" {
				foundActivities = true
			}
		}
	}
	if !foundLessons || !foundActivities {
		t.Fatalf("expected minimum size failures, got: %+v", failures)
	}
}

func TestValidateStructureAndSafety_Check3_KindAllowed(t *testing.T) {
	// Use listening_comprehension which is explicitly excluded in v1 (BR-STUDIO-10)
	raw := createValidCourseStructure(1, 3, 7, "listening_comprehension")
	_, failures, err := domain.ValidateStructureAndSafety("Title", "Description", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundKind := false
	for _, f := range failures {
		if f.Check == "kind_allowed" {
			foundKind = true
			break
		}
	}
	if !foundKind {
		t.Fatalf("expected kind_allowed failure for excluded kind, got: %+v", failures)
	}
}

func TestValidateStructureAndSafety_Check6_Safety(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		desc        string
		bodyContent string
	}{
		{
			name:  "phone in title",
			title: "Call me at 0901234567 for lessons",
			desc:  "Valid description",
		},
		{
			name:  "email in description",
			title: "Valid Title",
			desc:  "Contact teacher@gmail.com for help",
		},
		{
			name:        "url in activity body",
			title:       "Valid Title",
			desc:        "Valid description",
			bodyContent: "Visit https://external-academy.com to buy more courses",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := createValidCourseStructure(1, 3, 7, "vocab_multiple_choice")
			if tc.bodyContent != "" {
				var s domain.CourseStructure
				_ = json.Unmarshal(raw, &s)
				s.Units[0].Lessons[0].Activities[0].Body = json.RawMessage(fmt.Sprintf(`{"text":%q}`, tc.bodyContent))
				raw, _ = json.Marshal(s)
			}

			_, failures, err := domain.ValidateStructureAndSafety(tc.title, tc.desc, raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			foundSafety := false
			for _, f := range failures {
				if f.Check == "safety" {
					foundSafety = true
					break
				}
			}
			if !foundSafety {
				t.Fatalf("expected safety failure for case %s, got: %+v", tc.name, failures)
			}
		})
	}
}
