package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/modules/studio/repository"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Service coordinates creator studio operations, Gate 1 automated checks, and Gate 2 moderation.
type Service struct {
	repo          repository.Repository
	itemVerifier  learningcontract.ItemVerifier
	lessonAuthor  lessoncontract.Author
	contentAuthor contentcontract.Author
}

// NewService constructs the studio Service.
func NewService(
	repo repository.Repository,
	itemVerifier learningcontract.ItemVerifier,
	lessonAuthor lessoncontract.Author,
	contentAuthor contentcontract.Author,
) *Service {
	return &Service{
		repo:          repo,
		itemVerifier:  itemVerifier,
		lessonAuthor:  lessonAuthor,
		contentAuthor: contentAuthor,
	}
}

// ---------------------------------------------------------------- Creator Profile

func (s *Service) GetCreatorProfile(ctx context.Context, userID uuid.UUID) (*domain.CreatorProfile, error) {
	return s.repo.GetCreatorProfile(ctx, userID)
}

func (s *Service) UpsertCreatorProfile(ctx context.Context, userID uuid.UUID, bio, headline string) (*domain.CreatorProfile, error) {
	return s.repo.UpsertCreatorProfile(ctx, userID, bio, headline)
}

func (s *Service) GetPayoutAccount(ctx context.Context, creatorID uuid.UUID) (*domain.PayoutAccount, error) {
	return s.repo.GetPayoutAccount(ctx, creatorID)
}

func (s *Service) UpsertPayoutAccount(
	ctx context.Context,
	creatorID uuid.UUID,
	bankCode, accountNumber, accountHolderName string,
	isDefault bool,
) (*domain.PayoutAccount, error) {
	if strings.TrimSpace(bankCode) == "" || strings.TrimSpace(accountNumber) == "" || strings.TrimSpace(accountHolderName) == "" {
		return nil, apperr.New(apperr.Validation, "INVALID_PAYOUT_ACCOUNT", "All bank account details are required")
	}
	return s.repo.UpsertPayoutAccount(ctx, creatorID, bankCode, accountNumber, accountHolderName, isDefault)
}

// ---------------------------------------------------------------- Course Drafts

type CreateDraftRequest struct {
	Title           string          `json:"title"`
	Slug            string          `json:"slug"`
	Description     string          `json:"description"`
	CEFRLevel       string          `json:"cefr_level"`
	TopicTaxonomyID *uuid.UUID      `json:"topic_taxonomy_id,omitempty"`
	PriceVND        int64           `json:"price_vnd"`
	Structure       json.RawMessage `json:"structure"`
}

func (s *Service) CreateDraft(ctx context.Context, ownerID uuid.UUID, req CreateDraftRequest) (*domain.CourseDraft, error) {
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Slug) == "" || strings.TrimSpace(req.CEFRLevel) == "" {
		return nil, apperr.New(apperr.Validation, "INVALID_DRAFT", "Title, slug, and CEFR level are required")
	}

	structure := req.Structure
	if len(structure) == 0 {
		structure = []byte(`{"units":[]}`)
	}

	draft := &domain.CourseDraft{
		OwnerID:         ownerID,
		Title:           req.Title,
		Slug:            req.Slug,
		Description:     req.Description,
		CEFRLevel:       req.CEFRLevel,
		TopicTaxonomyID: req.TopicTaxonomyID,
		PriceVND:        req.PriceVND,
		Status:          domain.DraftStatusDraft,
		Structure:       structure,
	}

	return s.repo.CreateCourseDraft(ctx, draft)
}

type UpdateDraftRequest struct {
	Title           *string         `json:"title,omitempty"`
	Slug            *string         `json:"slug,omitempty"`
	Description     *string         `json:"description,omitempty"`
	CEFRLevel       *string         `json:"cefr_level,omitempty"`
	TopicTaxonomyID *uuid.UUID      `json:"topic_taxonomy_id,omitempty"`
	PriceVND        *int64          `json:"price_vnd,omitempty"`
	Structure       json.RawMessage `json:"structure,omitempty"`
}

func (s *Service) UpdateDraft(ctx context.Context, ownerID, draftID uuid.UUID, req UpdateDraftRequest) (*domain.CourseDraft, error) {
	existing, err := s.repo.GetCourseDraftByID(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if existing.OwnerID != ownerID {
		return nil, domain.ErrDraftNotFound
	}
	if existing.Status != domain.DraftStatusDraft && existing.Status != domain.DraftStatusChangesRequested {
		return nil, apperr.New(apperr.Conflict, "CANNOT_EDIT_DRAFT", "Draft cannot be edited while in review or published")
	}

	if req.Title != nil {
		existing.Title = *req.Title
	}
	if req.Slug != nil {
		existing.Slug = *req.Slug
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.CEFRLevel != nil {
		existing.CEFRLevel = *req.CEFRLevel
	}
	if req.TopicTaxonomyID != nil {
		existing.TopicTaxonomyID = req.TopicTaxonomyID
	}
	if req.PriceVND != nil {
		existing.PriceVND = *req.PriceVND
	}
	if len(req.Structure) > 0 {
		existing.Structure = req.Structure
	}

	return s.repo.UpdateCourseDraft(ctx, existing)
}

func (s *Service) GetDraft(ctx context.Context, ownerID, draftID uuid.UUID) (*domain.CourseDraft, error) {
	draft, err := s.repo.GetCourseDraftByID(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if draft.OwnerID != ownerID {
		return nil, domain.ErrDraftNotFound
	}
	return draft, nil
}

func (s *Service) ListDrafts(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]*domain.CourseDraft, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListCourseDraftsByOwner(ctx, ownerID, limit, offset)
}

// ---------------------------------------------------------------- Submissions & Gate 1

func (s *Service) SubmitDraft(ctx context.Context, ownerID, draftID uuid.UUID) (*domain.Submission, error) {
	draft, err := s.repo.GetCourseDraftByID(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if draft.OwnerID != ownerID {
		return nil, domain.ErrDraftNotFound
	}
	if draft.Status != domain.DraftStatusDraft && draft.Status != domain.DraftStatusChangesRequested {
		return nil, domain.ErrCannotSubmit
	}

	version := 1
	latest, err := s.repo.GetLatestSubmissionByDraftID(ctx, draftID)
	if err == nil && latest != nil {
		version = latest.Version + 1
	}

	sub := &domain.Submission{
		DraftID:     draftID,
		Version:     version,
		Status:      domain.SubmissionStatusSubmitted,
		SubmittedBy: ownerID,
	}

	created, err := s.repo.CreateSubmission(ctx, sub)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.UpdateCourseDraftStatus(ctx, draftID, domain.DraftStatusSubmitted); err != nil {
		return nil, err
	}

	// Trigger Gate 1 verification immediately
	return s.RunGate1Verification(ctx, created.ID)
}

// RunGate1Verification executes automated Gate 1 checks against the submitted draft.
func (s *Service) RunGate1Verification(ctx context.Context, submissionID uuid.UUID) (*domain.Submission, error) {
	sub, err := s.repo.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	draft, err := s.repo.GetCourseDraftByID(ctx, sub.DraftID)
	if err != nil {
		return nil, err
	}

	// Mark verifying
	if _, err := s.repo.UpdateSubmissionVerification(ctx, sub.ID, domain.SubmissionStatusVerifying, nil, nil); err != nil {
		return nil, err
	}
	if _, err := s.repo.UpdateCourseDraftStatus(ctx, draft.ID, domain.DraftStatusVerifying); err != nil {
		return nil, err
	}

	// 1. Structure, Minimum size, Kind allowed, Safety checks
	structure, failures, err := domain.ValidateStructureAndSafety(draft.Title, draft.Description, draft.Structure)
	if err != nil {
		failures = append(failures, domain.VerificationFailure{
			Check:   "structure",
			Message: fmt.Sprintf("Failed to parse course structure: %v", err),
		})
	}

	totalItemsChecked := 0

	// 2. Activity content verification via ItemVerifier
	if structure != nil && len(failures) == 0 && s.itemVerifier != nil {
		for uIdx, unit := range structure.Units {
			for lIdx, lesson := range unit.Lessons {
				for aIdx, act := range lesson.Activities {
					totalItemsChecked++
					cefr := lesson.CEFRLevel
					if cefr == "" {
						cefr = draft.CEFRLevel
					}
					req := learningcontract.VerifyItemRequest{
						Kind:       act.Kind,
						TaskType:   act.TaskType,
						CEFRLevel:  cefr,
						Body:       act.Body,
						BlindSolve: false,
					}
					if vErr := s.itemVerifier.VerifyItem(ctx, req); vErr != nil {
						failures = append(failures, domain.VerificationFailure{
							UnitIndex:     uIdx,
							LessonIndex:   lIdx,
							ActivityIndex: aIdx,
							Kind:          act.Kind,
							Check:         "item_verifier",
							Message:       vErr.Error(),
						})
					}
				}
			}
		}
	}

	passed := len(failures) == 0
	report := domain.VerificationReport{
		Passed:       passed,
		ItemsChecked: totalItemsChecked,
		Failures:     failures,
	}
	reportRaw, _ := json.Marshal(report)

	var targetSubStatus string
	var targetDraftStatus string
	var feedback *string

	if passed {
		targetSubStatus = domain.SubmissionStatusInReview
		targetDraftStatus = domain.DraftStatusInReview
	} else {
		targetSubStatus = domain.SubmissionStatusChangesRequested
		targetDraftStatus = domain.DraftStatusChangesRequested
		msg := fmt.Sprintf("Automated Gate 1 verification failed with %d issues.", len(failures))
		feedback = &msg
	}

	updatedSub, err := s.repo.UpdateSubmissionVerification(ctx, sub.ID, targetSubStatus, reportRaw, feedback)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.UpdateCourseDraftStatus(ctx, draft.ID, targetDraftStatus); err != nil {
		return nil, err
	}

	return updatedSub, nil
}

// ---------------------------------------------------------------- Gate 2 Moderation

func (s *Service) ListModerationQueue(ctx context.Context, limit, offset int) ([]contract.ModerationQueueItem, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// List submissions waiting for review
	submissions, total, err := s.repo.ListSubmissionsByStatus(ctx, domain.SubmissionStatusInReview, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	items := make([]contract.ModerationQueueItem, len(submissions))
	for i, sub := range submissions {
		draft, err := s.repo.GetCourseDraftByID(ctx, sub.DraftID)
		if err != nil {
			return nil, 0, err
		}
		items[i] = contract.ModerationQueueItem{
			Submission: contract.Submission{
				ID:                 sub.ID,
				DraftID:            sub.DraftID,
				Version:            sub.Version,
				Status:             sub.Status,
				SubmittedBy:        sub.SubmittedBy,
				ReviewerID:         sub.ReviewerID,
				Feedback:           sub.Feedback,
				VerificationReport: sub.VerificationReport,
				SubmittedAt:        sub.SubmittedAt,
				ReviewedAt:         sub.ReviewedAt,
				CreatedAt:          sub.CreatedAt,
				UpdatedAt:          sub.UpdatedAt,
			},
			Draft: contract.CourseDraft{
				ID:              draft.ID,
				OwnerID:         draft.OwnerID,
				Title:           draft.Title,
				Slug:            draft.Slug,
				Description:     draft.Description,
				CEFRLevel:       draft.CEFRLevel,
				TopicTaxonomyID: draft.TopicTaxonomyID,
				PriceVND:        draft.PriceVND,
				Status:          draft.Status,
				Structure:       draft.Structure,
				CreatedAt:       draft.CreatedAt,
				UpdatedAt:       draft.UpdatedAt,
			},
		}
	}

	return items, total, nil
}

func (s *Service) ApproveSubmission(ctx context.Context, reviewerID, submissionID uuid.UUID) (*domain.Submission, error) {
	sub, err := s.repo.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	// BR-STUDIO-06 / BR-CONTENT-03: reviewer cannot review their own submission
	if sub.SubmittedBy == reviewerID {
		return nil, domain.ErrSelfReviewForbidden
	}

	if sub.Status != domain.SubmissionStatusInReview && sub.Status != domain.SubmissionStatusSubmitted {
		return nil, apperr.New(apperr.Conflict, "INVALID_STATE", "Submission is not in reviewable state")
	}

	draft, err := s.repo.GetCourseDraftByID(ctx, sub.DraftID)
	if err != nil {
		return nil, err
	}

	// Publish course hierarchy and content versions
	if s.lessonAuthor != nil && s.contentAuthor != nil {
		var structure domain.CourseStructure
		if err := json.Unmarshal(draft.Structure, &structure); err != nil {
			return nil, fmt.Errorf("unmarshal draft structure for publish: %w", err)
		}

		courseSpec := lessoncontract.CourseSpec{
			Slug:            draft.Slug,
			Title:           draft.Title,
			Description:     draft.Description,
			CEFRFrom:        draft.CEFRLevel,
			CEFRTo:          draft.CEFRLevel,
			EstimatedHours:  len(structure.Units) * 5,
			Origin:          "community",
			OwnerID:         &draft.OwnerID,
			Visibility:      "public",
			TopicTaxonomyID: draft.TopicTaxonomyID,
		}
		courseID, err := s.lessonAuthor.EnsureCourse(ctx, courseSpec)
		if err != nil {
			return nil, fmt.Errorf("publish course in lesson module: %w", err)
		}

		for uIdx, unit := range structure.Units {
			unitSpec := lessoncontract.UnitSpec{
				CourseID:    courseID,
				Position:    uIdx + 1,
				Title:       unit.Title,
				Description: unit.Description,
			}
			unitID, err := s.lessonAuthor.EnsureUnit(ctx, unitSpec)
			if err != nil {
				return nil, fmt.Errorf("publish unit: %w", err)
			}

			for lIdx, lesson := range unit.Lessons {
				level := lesson.CEFRLevel
				if level == "" {
					level = draft.CEFRLevel
				}
				lessonSpec := lessoncontract.LessonSpec{
					UnitID:           unitID,
					Position:         lIdx + 1,
					Title:            lesson.Title,
					SkillFocus:       lesson.SkillFocus,
					EstimatedMinutes: lesson.EstimatedMinutes,
					CEFRLevel:        &level,
				}
				lessonID, err := s.lessonAuthor.EnsureLesson(ctx, lessonSpec)
				if err != nil {
					return nil, fmt.Errorf("publish lesson: %w", err)
				}

				activitySpecs := make([]lessoncontract.ActivitySpec, len(lesson.Activities))
				for aIdx, act := range lesson.Activities {
					contentSlug := fmt.Sprintf("%s-u%d-l%d-a%d", draft.Slug, uIdx+1, lIdx+1, aIdx+1)
					versionID, err := s.contentAuthor.EnsurePublished(ctx, contentcontract.AuthorSpec{
						Slug:      contentSlug,
						Kind:      act.Kind,
						CEFRLevel: level,
						Body:      act.Body,
						AuthorID:  draft.OwnerID,
					})
					if err != nil {
						return nil, fmt.Errorf("publish content version: %w", err)
					}
					activitySpecs[aIdx] = lessoncontract.ActivitySpec{
						Position:         aIdx + 1,
						Kind:             act.Kind,
						ContentVersionID: versionID,
						Config:           act.Config,
						Weight:           act.Weight,
					}
				}

				if err := s.lessonAuthor.SyncActivities(ctx, lessonID, activitySpecs); err != nil {
					return nil, fmt.Errorf("sync lesson activities: %w", err)
				}
			}
		}
	}

	feedback := "Approved by moderator"
	approvedSub, err := s.repo.UpdateSubmissionReview(ctx, sub.ID, domain.SubmissionStatusApproved, reviewerID, &feedback)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.UpdateCourseDraftStatus(ctx, draft.ID, domain.DraftStatusPublished); err != nil {
		return nil, err
	}

	return approvedSub, nil
}

func (s *Service) RejectSubmission(
	ctx context.Context,
	reviewerID, submissionID uuid.UUID,
	targetStatus, feedback string,
) (*domain.Submission, error) {
	sub, err := s.repo.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	// BR-STUDIO-06 / BR-CONTENT-03: reviewer cannot review their own submission
	if sub.SubmittedBy == reviewerID {
		return nil, domain.ErrSelfReviewForbidden
	}

	if strings.TrimSpace(feedback) == "" {
		return nil, domain.ErrFeedbackRequired
	}

	if targetStatus != domain.SubmissionStatusRejected && targetStatus != domain.SubmissionStatusChangesRequested {
		targetStatus = domain.SubmissionStatusChangesRequested
	}

	draftStatus := domain.DraftStatusChangesRequested
	if targetStatus == domain.SubmissionStatusRejected {
		draftStatus = domain.DraftStatusRejected
	}

	updatedSub, err := s.repo.UpdateSubmissionReview(ctx, sub.ID, targetStatus, reviewerID, &feedback)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.UpdateCourseDraftStatus(ctx, sub.DraftID, draftStatus); err != nil {
		return nil, err
	}

	return updatedSub, nil
}
