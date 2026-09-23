package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	resourcecontract "github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Private practice generated from a learner's own upload (WO 21 Stage B).
//
// The set is one row plus ten real activities in a hidden per-learner course,
// so the existing runner sits it and the existing attempt flow grades it. It
// never enters the shared bank and never reaches another learner
// (BR-RESOURCE-12): the course is `origin = generated`, which the catalogue
// query excludes, and every read here filters on the owner.

const (
	// resourcePracticeCount is D21-2's "one set of 10 practice items".
	resourcePracticeCount = 10
	// resourcePracticeSourceChars is how much extracted text the model sees.
	// The extraction is capped at 400,000 characters and the classifier reads
	// the first 8,000; the generator gets the same window.
	resourcePracticeSourceChars = 8000
	// resourcePracticeSweepLimit bounds one generation sweep. Each set is ten
	// model calls, so the sweep is deliberately small.
	resourcePracticeSweepLimit = 5
	// resourcePracticeCoursePrefix names the hidden per-learner course.
	resourcePracticeCoursePrefix = "resource-practice-"

	resourcePracticeCourseTitle       = "Practice from my files"
	resourcePracticeCourseDescription = "Private practice generated from your own uploads. Only you can see it."
	resourcePracticeUnitTitle         = "My resources"
	resourcePracticeUnitDescription   = "One set per file, regenerated when you ask for it."
	resourcePracticeLessonTitle       = "Practice from my uploads"
)

// ResourcePracticeStore is the persistence this feature needs, kept narrow so
// the service does not grow a second reason to depend on the whole repository.
type ResourcePracticeStore interface {
	UpsertResourcePracticeSet(
		ctx context.Context, resourceID, userID uuid.UUID,
	) (*domain.ResourcePracticeSet, error)
	GetResourcePracticeSet(
		ctx context.Context, resourceID, userID uuid.UUID,
	) (*domain.ResourcePracticeSet, error)
	MarkResourcePracticeSetReady(
		ctx context.Context, resourceID, userID, lessonID uuid.UUID, activityIDs []uuid.UUID,
	) (*domain.ResourcePracticeSet, error)
	MarkResourcePracticeSetFailed(
		ctx context.Context, resourceID, userID uuid.UUID, reason string,
	) (*domain.ResourcePracticeSet, error)
	ClaimGeneratingResourcePracticeSets(
		ctx context.Context, limit int32,
	) ([]domain.ResourcePracticeSet, error)
	DeleteResourcePracticeSetsForUser(ctx context.Context, userID uuid.UUID) error
	// ResourcePracticeCourseAnchor names one activity of this learner's, so the
	// read path can resolve the hidden course their sets live in. uuid.Nil when
	// they have no sets.
	ResourcePracticeCourseAnchor(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
}

// resourcePracticeCourseSlug is one hidden course per learner.
func resourcePracticeCourseSlug(userID uuid.UUID) string {
	return resourcePracticeCoursePrefix + userID.String()
}

// RequestResourcePractice asks for today's set, generating it if needed.
//
// The preconditions are checked here rather than in the job so the learner gets
// an answer now: a resource with no extracted text or no classification cannot
// become practice, and a failed job would only say so a minute later.
func (s *Service) RequestResourcePractice(
	ctx context.Context, userID, resourceID uuid.UUID,
) (*domain.ResourcePracticeSetDTO, error) {
	resource, err := s.ownedResource(ctx, userID, resourceID)
	if err != nil {
		return nil, err
	}
	if s.resourcePractice == nil {
		return nil, apperr.New(apperr.Internal, "RESOURCE_PRACTICE_UNAVAILABLE",
			"Practice from a resource is not configured on this deployment.")
	}

	existing, err := s.resourcePractice.GetResourcePracticeSet(ctx, resourceID, userID)
	if err != nil {
		return nil, fmt.Errorf("read resource practice set: %w", err)
	}
	if existing != nil {
		switch existing.Status {
		case domain.ResourcePracticeReady:
			// Regenerable once a day: today's set is returned as it is.
			if existing.GeneratedOn.Format("2006-01-02") == s.clock.Now().Format("2006-01-02") {
				return s.assembleResourcePracticeDTO(ctx, existing)
			}
		case domain.ResourcePracticeGenerating:
			return s.assembleResourcePracticeDTO(ctx, existing)
		case domain.ResourcePracticeFailed:
			// A failure is retryable the same day; it cost the learner nothing.
		}
	}

	if err := s.resourcePracticeReadiness(resource); err != nil {
		return nil, err
	}

	set, err := s.resourcePractice.UpsertResourcePracticeSet(ctx, resourceID, userID)
	if err != nil {
		return nil, fmt.Errorf("queue resource practice: %w", err)
	}
	return s.assembleResourcePracticeDTO(ctx, set)
}

// GetResourcePractice reads a set without generating anything.
func (s *Service) GetResourcePractice(
	ctx context.Context, userID, resourceID uuid.UUID,
) (*domain.ResourcePracticeSetDTO, error) {
	if _, err := s.ownedResource(ctx, userID, resourceID); err != nil {
		return nil, err
	}
	if s.resourcePractice == nil {
		return nil, apperr.New(apperr.Internal, "RESOURCE_PRACTICE_UNAVAILABLE",
			"Practice from a resource is not configured on this deployment.")
	}
	set, err := s.resourcePractice.GetResourcePracticeSet(ctx, resourceID, userID)
	if err != nil {
		return nil, fmt.Errorf("read resource practice set: %w", err)
	}
	if set == nil {
		return nil, apperr.New(apperr.NotFound, "RESOURCE_PRACTICE_NOT_FOUND",
			"No practice has been generated from this file yet.")
	}
	return s.assembleResourcePracticeDTO(ctx, set)
}

// GeneratePendingResourcePractice is the scheduled sweep.
//
// It runs in the worker rather than the request because ten model calls can
// outlive an HTTP timeout; the request only writes the row and returns 202.
func (s *Service) GeneratePendingResourcePractice(ctx context.Context) error {
	if s.resource == nil || s.resourcePractice == nil || s.ai == nil || s.lessonAuthor == nil {
		return nil
	}
	sets, err := s.resourcePractice.ClaimGeneratingResourcePracticeSets(ctx, resourcePracticeSweepLimit)
	if err != nil {
		return fmt.Errorf("claim resource practice sets: %w", err)
	}
	for _, set := range sets {
		if err := s.generateResourcePractice(ctx, set); err != nil {
			slog.WarnContext(ctx, "resource practice generation failed",
				"resource_id", set.ResourceID, "user_id", set.UserID, "error", err)
			if _, markErr := s.resourcePractice.MarkResourcePracticeSetFailed(
				ctx, set.ResourceID, set.UserID, learnerReason(err),
			); markErr != nil {
				slog.WarnContext(ctx, "could not record resource practice failure",
					"resource_id", set.ResourceID, "error", markErr)
			}
		}
	}
	return nil
}

func (s *Service) generateResourcePractice(
	ctx context.Context, set domain.ResourcePracticeSet,
) error {
	resource, err := s.resource.GetResource(ctx, set.ResourceID, set.UserID)
	if err != nil {
		return fmt.Errorf("read resource: %w", err)
	}
	if resource == nil {
		return errors.New("resource no longer exists")
	}
	if err := s.resourcePracticeReadiness(resource); err != nil {
		return err
	}

	level := resourcePracticeLevel(resource)
	skill := resourcePracticeSkill(resource)
	kind := resourcePracticeKind(skill)
	items, err := s.Generate(ctx, contract.GenerateRequest{
		Kind:       kind,
		CEFRLevel:  level,
		NodeCodes:  resource.Classification.NodeCodes,
		Count:      resourcePracticeCount,
		Purpose:    purposeResource,
		OwnerID:    &set.UserID,
		SourceText: resourcePracticeSource(resource),
		SlugPrefix: fmt.Sprintf("resource-%s-%s", set.ResourceID.String(), kind),
	})
	if err != nil {
		return fmt.Errorf("generate items: %w", err)
	}

	lessonID, activityIDs, err := s.authorResourcePracticeLesson(ctx, set.UserID, kind, skill, level, items)
	if err != nil {
		return fmt.Errorf("author practice lesson: %w", err)
	}
	if _, err := s.resourcePractice.MarkResourcePracticeSetReady(
		ctx, set.ResourceID, set.UserID, lessonID, activityIDs,
	); err != nil {
		return fmt.Errorf("mark resource practice ready: %w", err)
	}
	return nil
}

// authorResourcePracticeLesson appends the generated items to the learner's
// hidden course and returns the lesson and the activity ids that make the set.
func (s *Service) authorResourcePracticeLesson(
	ctx context.Context,
	userID uuid.UUID,
	kind, skill, level string,
	items []contract.GeneratedItem,
) (uuid.UUID, []uuid.UUID, error) {
	courseID, err := s.lessonAuthor.EnsureCourse(ctx, lessoncontract.CourseSpec{
		Slug:           resourcePracticeCourseSlug(userID),
		Title:          resourcePracticeCourseTitle,
		Description:    resourcePracticeCourseDescription,
		CEFRFrom:       level,
		CEFRTo:         level,
		EstimatedHours: 1,
	})
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("ensure course: %w", err)
	}
	unitID, err := s.lessonAuthor.EnsureUnit(ctx, lessoncontract.UnitSpec{
		CourseID:    courseID,
		Position:    1,
		Title:       resourcePracticeUnitTitle,
		Description: resourcePracticeUnitDescription,
	})
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("ensure unit: %w", err)
	}
	levelPtr := level
	lessonID, err := s.lessonAuthor.EnsureLesson(ctx, lessoncontract.LessonSpec{
		UnitID:           unitID,
		Position:         1,
		Title:            resourcePracticeLessonTitle,
		SkillFocus:       skill,
		EstimatedMinutes: 10,
		CEFRLevel:        &levelPtr,
	})
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("ensure lesson: %w", err)
	}

	activityIDs := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		activityID, err := s.lessonAuthor.AppendActivity(ctx, lessonID, lessoncontract.ActivitySpec{
			Kind:             kind,
			ContentVersionID: item.ContentVersionID,
			Config:           item.Body,
			Weight:           1,
		})
		if err != nil {
			return uuid.Nil, nil, fmt.Errorf("append activity: %w", err)
		}
		activityIDs = append(activityIDs, activityID)
	}

	// StartAttempt refuses an activity in a course the learner is not enrolled
	// in, and this course is not one they chose.
	s.ensurePoolEnrollment(ctx, userID, courseID)
	return lessonID, activityIDs, nil
}

// ownedResource reads one resource for its owner, or the same not-found a
// foreign id answers (BR-RESOURCE-01).
func (s *Service) ownedResource(
	ctx context.Context, userID, resourceID uuid.UUID,
) (*resourcecontract.Resource, error) {
	if s.resource == nil {
		return nil, apperr.New(apperr.Internal, "RESOURCE_READER_UNAVAILABLE",
			"Reading learner resources is not configured on this deployment.")
	}
	resource, err := s.resource.GetResource(ctx, resourceID, userID)
	if err != nil || resource == nil {
		return nil, apperr.New(apperr.NotFound, "RESOURCE_NOT_FOUND",
			"That file does not exist.")
	}
	return resource, nil
}

// resourcePracticeReadiness is what generation needs from a resource: text to
// work from and at least one spine node to tag the items with.
func (s *Service) resourcePracticeReadiness(resource *resourcecontract.Resource) error {
	if resource.Status != resourcecontract.MaterialValidated ||
		resource.Extraction == nil ||
		strings.TrimSpace(resource.Extraction.Text) == "" {
		return apperr.New(apperr.Validation, "RESOURCE_PRACTICE_NO_TEXT",
			"There is no text to build practice from yet. Wait until the file has been read.")
	}
	if resource.Classification == nil || len(resource.Classification.NodeCodes) == 0 {
		return apperr.New(apperr.Validation, "RESOURCE_PRACTICE_UNCLASSIFIED",
			"This file has not been classified yet. Try again once it has.")
	}
	return nil
}

// resourcePracticeSource is the extraction, truncated to the model's window.
func resourcePracticeSource(resource *resourcecontract.Resource) string {
	text := strings.TrimSpace(resource.Extraction.Text)
	runes := []rune(text)
	if len(runes) > resourcePracticeSourceChars {
		return string(runes[:resourcePracticeSourceChars])
	}
	return text
}

func resourcePracticeLevel(resource *resourcecontract.Resource) string {
	if resource.Classification != nil && resource.Classification.CEFREstimate != nil {
		switch strings.ToUpper(strings.TrimSpace(*resource.Classification.CEFREstimate)) {
		case "A1", "A2", "B1", "B2", "C1", "C2":
			return strings.ToUpper(strings.TrimSpace(*resource.Classification.CEFREstimate))
		}
	}
	return defaultPracticeLevel
}

func resourcePracticeSkill(resource *resourcecontract.Resource) string {
	if resource.Classification != nil && resource.Classification.Skill != nil {
		skill := strings.ToLower(strings.TrimSpace(*resource.Classification.Skill))
		if skill != "" {
			return skill
		}
	}
	return domain.SkillVocabulary
}

// resourcePracticeKind picks among the kinds the practice pool already grades.
// A grammar-tagged file gets a tense choice; anything else a vocabulary choice.
func resourcePracticeKind(skill string) string {
	if skill == domain.SkillGrammar {
		return kindGrammarTenseChoice
	}
	return kindVocabMultipleChoice
}

// learnerReason keeps a job failure readable. The chain goes to the log; the
// learner sees one fixed sentence per kind of failure.
func learnerReason(err error) string {
	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr.Message != "" {
		return appErr.Message
	}
	return "We could not build practice from this file. Try again later."
}

// assembleResourcePracticeDTO renders a set exactly as the daily set is
// rendered, so the runner needs no second code path.
func (s *Service) assembleResourcePracticeDTO(
	ctx context.Context, set *domain.ResourcePracticeSet,
) (*domain.ResourcePracticeSetDTO, error) {
	activities, _, err := s.resolveActivityDTOs(ctx, set.ActivityIDs)
	if err != nil {
		return nil, err
	}
	return &domain.ResourcePracticeSetDTO{
		ResourceID:    set.ResourceID,
		Status:        set.Status,
		FailureReason: set.FailureReason,
		GeneratedOn:   set.GeneratedOn.Format("2006-01-02"),
		Activities:    activities,
	}, nil
}
