// Package service coordinates creator studio operations, Gate 1 automated checks, and Gate 2 moderation.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	paymentcontract "github.com/fluentra/fluentra/internal/modules/payment/contract"
	resourcecontract "github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/modules/studio/repository"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Service coordinates creator studio operations, Gate 1 automated checks, and Gate 2 moderation.
type Service struct {
	repo               repository.Repository
	itemVerifier       learningcontract.ItemVerifier
	lessonAuthor       lessoncontract.Author
	contentAuthor      contentcontract.Author
	orderCreator       paymentcontract.OrderCreator
	progressReader     learningcontract.ProgressReader
	lessonReader       lessoncontract.Reader
	refundRecorder     paymentcontract.RefundRecorder
	minPriceVND        int64
	maxPriceVND        int64
	revenueShareBPS    int
	payoutManager      paymentcontract.PayoutManager
	payoutThresholdVND int64
	materialPublisher  resourcecontract.MaterialPublisher
}

// NewService constructs the studio Service.
func NewService(
	repo repository.Repository,
	itemVerifier learningcontract.ItemVerifier,
	lessonAuthor lessoncontract.Author,
	contentAuthor contentcontract.Author,
) *Service {
	return &Service{
		repo:               repo,
		itemVerifier:       itemVerifier,
		lessonAuthor:       lessonAuthor,
		contentAuthor:      contentAuthor,
		minPriceVND:        domain.DefaultMinPriceVND,
		maxPriceVND:        domain.DefaultMaxPriceVND,
		revenueShareBPS:    domain.DefaultRevenueShareBPS,
		payoutThresholdVND: domain.DefaultPayoutThresholdVND,
	}
}

// SetOrderCreator configures the order creator for course purchases.
func (s *Service) SetOrderCreator(creator paymentcontract.OrderCreator) {
	s.orderCreator = creator
}

// SetProgressReader configures the learning progress reader for refund evaluations.
func (s *Service) SetProgressReader(reader learningcontract.ProgressReader) {
	s.progressReader = reader
}

// SetLessonReader configures the lesson reader used to size a course when a
// refund asks how much of it the learner has done.
func (s *Service) SetLessonReader(reader lessoncontract.Reader) {
	s.lessonReader = reader
}

// SetRefundRecorder configures the billing side of a refund.
func (s *Service) SetRefundRecorder(recorder paymentcontract.RefundRecorder) {
	s.refundRecorder = recorder
}

// SetPriceBounds overrides default pricing bounds and platform revenue share.
func (s *Service) SetPriceBounds(minVND, maxVND int64, revShareBPS int) {
	if minVND > 0 {
		s.minPriceVND = minVND
	}
	if maxVND > 0 {
		s.maxPriceVND = maxVND
	}
	if revShareBPS > 0 {
		s.revenueShareBPS = revShareBPS
	}
}

// SetPayoutManager configures the billing payout manager adapter.
func (s *Service) SetPayoutManager(manager paymentcontract.PayoutManager) {
	s.payoutManager = manager
}

// SetMaterialPublisher configures the resource read-and-copy surface used to
// publish lesson_material activities (WO 20).
func (s *Service) SetMaterialPublisher(publisher resourcecontract.MaterialPublisher) {
	s.materialPublisher = publisher
}

// SetPayoutThresholdVND sets the minimum creator payout threshold.
func (s *Service) SetPayoutThresholdVND(threshold int64) {
	if threshold > 0 {
		s.payoutThresholdVND = threshold
	}
}

// ---------------------------------------------------------------- Creator Profile

// GetCreatorProfile retrieves the creator's profile by user ID.
func (s *Service) GetCreatorProfile(ctx context.Context, userID uuid.UUID) (*domain.CreatorProfile, error) {
	return s.repo.GetCreatorProfile(ctx, userID)
}

// UpsertCreatorProfile creates or updates the creator's bio and headline.
func (s *Service) UpsertCreatorProfile(
	ctx context.Context, userID uuid.UUID, bio, headline string,
) (*domain.CreatorProfile, error) {
	return s.repo.UpsertCreatorProfile(ctx, userID, bio, headline)
}

// GetPayoutAccount retrieves the creator's registered payout account.
func (s *Service) GetPayoutAccount(ctx context.Context, creatorID uuid.UUID) (*domain.PayoutAccount, error) {
	return s.repo.GetPayoutAccount(ctx, creatorID)
}

// UpsertPayoutAccount registers or updates the creator's payout bank account.
func (s *Service) UpsertPayoutAccount(
	ctx context.Context,
	creatorID uuid.UUID,
	bankCode, accountNumber, accountHolderName string,
	isDefault bool,
) (*domain.PayoutAccount, error) {
	if strings.TrimSpace(bankCode) == "" ||
		strings.TrimSpace(accountNumber) == "" ||
		strings.TrimSpace(accountHolderName) == "" {
		return nil, apperr.New(apperr.Validation, "INVALID_PAYOUT_ACCOUNT", "All bank account details are required")
	}
	return s.repo.UpsertPayoutAccount(ctx, creatorID, bankCode, accountNumber, accountHolderName, isDefault)
}

// ---------------------------------------------------------------- Course Drafts

// CreateDraftRequest holds input parameters for creating a new course draft.
type CreateDraftRequest struct {
	Title           string          `json:"title"`
	Slug            string          `json:"slug"`
	Description     string          `json:"description"`
	CEFRLevel       string          `json:"cefr_level"`
	TopicTaxonomyID *uuid.UUID      `json:"topic_taxonomy_id,omitempty"`
	PriceVND        int64           `json:"price_vnd"`
	Structure       json.RawMessage `json:"structure"`
}

// CreateDraft initializes a new course draft authored by a creator.
func (s *Service) CreateDraft(
	ctx context.Context, ownerID uuid.UUID, req CreateDraftRequest,
) (*domain.CourseDraft, error) {
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Slug) == "" || strings.TrimSpace(req.CEFRLevel) == "" {
		return nil, apperr.New(apperr.Validation, "INVALID_DRAFT", "Title, slug, and CEFR level are required")
	}

	minVND := s.minPriceVND
	if minVND <= 0 {
		minVND = domain.DefaultMinPriceVND
	}
	maxVND := s.maxPriceVND
	if maxVND <= 0 {
		maxVND = domain.DefaultMaxPriceVND
	}
	if req.PriceVND < 0 || (req.PriceVND > 0 && (req.PriceVND < minVND || req.PriceVND > maxVND)) {
		return nil, domain.ErrPriceOutOfBounds
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

// UpdateDraftRequest holds optional updates for a course draft.
type UpdateDraftRequest struct {
	Title           *string         `json:"title,omitempty"`
	Slug            *string         `json:"slug,omitempty"`
	Description     *string         `json:"description,omitempty"`
	CEFRLevel       *string         `json:"cefr_level,omitempty"`
	TopicTaxonomyID *uuid.UUID      `json:"topic_taxonomy_id,omitempty"`
	PriceVND        *int64          `json:"price_vnd,omitempty"`
	Structure       json.RawMessage `json:"structure,omitempty"`
}

// UpdateDraft replaces the fields a request set on a draft the caller owns.
//
// Only a draft or one that came back with changes requested may be edited: a
// submission in review is what a moderator is looking at, and a published
// course is edited by submitting a new version.
func (s *Service) UpdateDraft(
	ctx context.Context, ownerID, draftID uuid.UUID, req UpdateDraftRequest,
) (*domain.CourseDraft, error) {
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

	applyDraftFields(existing, req)
	if req.PriceVND != nil {
		if err := s.checkPrice(*req.PriceVND); err != nil {
			return nil, err
		}
		existing.PriceVND = *req.PriceVND
	}

	return s.repo.UpdateCourseDraft(ctx, existing)
}

// applyDraftFields copies the fields a request actually set. A nil pointer
// means "leave it", which is why this is a wall of small ifs rather than a
// struct copy.
func applyDraftFields(draft *domain.CourseDraft, req UpdateDraftRequest) {
	if req.Title != nil {
		draft.Title = *req.Title
	}
	if req.Slug != nil {
		draft.Slug = *req.Slug
	}
	if req.Description != nil {
		draft.Description = *req.Description
	}
	if req.CEFRLevel != nil {
		draft.CEFRLevel = *req.CEFRLevel
	}
	if req.TopicTaxonomyID != nil {
		draft.TopicTaxonomyID = req.TopicTaxonomyID
	}
	if len(req.Structure) > 0 {
		draft.Structure = req.Structure
	}
}

// checkPrice enforces BR-STUDIO-01: free, or a whole number of VND inside the
// configured bounds.
func (s *Service) checkPrice(priceVND int64) error {
	if priceVND == 0 {
		return nil
	}
	minVND := s.minPriceVND
	if minVND <= 0 {
		minVND = domain.DefaultMinPriceVND
	}
	maxVND := s.maxPriceVND
	if maxVND <= 0 {
		maxVND = domain.DefaultMaxPriceVND
	}
	if priceVND < minVND || priceVND > maxVND {
		return domain.ErrPriceOutOfBounds
	}
	return nil
}

// GetDraft fetches an existing course draft owned by the specified user.
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

// ListDrafts returns paginated drafts owned by the given creator.
func (
	s *Service) ListDrafts(ctx context.Context,
	ownerID uuid.UUID,
	limit,
	offset int) ([]*domain.CourseDraft,
	int64,
	error,
) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListCourseDraftsByOwner(ctx, ownerID, limit, offset)
}

// ---------------------------------------------------------------- Submissions & Gate 1

// SubmitDraft submits a course draft for Gate 1 and Gate 2 verification.
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

	// A creator with no profile is not trusted, which needsHumanReview already
	// answers for. Refusing the submission instead would make opening the
	// studio a second, separate step before the first draft could be sent.
	profile, err := s.repo.GetCreatorProfile(ctx, ownerID)
	if err != nil && !errors.Is(err, domain.ErrProfileNotFound) {
		return nil, err
	}
	if profile.Suspended() {
		return nil, domain.ErrCreatorSuspended
	}
	gate2Required, gate2Reason := needsHumanReview(profile, draft)

	version := 1
	latest, err := s.repo.GetLatestSubmissionByDraftID(ctx, draftID)
	if err == nil && latest != nil {
		version = latest.Version + 1
	}

	sub := &domain.Submission{
		DraftID:       draftID,
		Version:       version,
		Status:        domain.SubmissionStatusSubmitted,
		SubmittedBy:   ownerID,
		Gate2Required: gate2Required,
		Gate2Reason:   gate2Reason,
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
// gate1Stats counts what Gate 1 managed to check.
type gate1Stats struct {
	checked      int
	blindSolved  int
	blindRejects int
}

// verifyDraftItems runs the item checks over every activity in a draft.
func (s *Service) verifyDraftItems(
	ctx context.Context,
	submissionID uuid.UUID,
	draft *domain.CourseDraft,
	structure *domain.CourseStructure,
) (failures []domain.VerificationFailure, stats gate1Stats) {
	if structure == nil || s.itemVerifier == nil {
		return nil, stats
	}
	// Activity content verification via ItemVerifier.
	//
	// Items are checked even when the structure has complaints, so one missing
	// title does not hide twelve broken answer keys until the next submission.
	// Check 4, the blind solve: a model answers the redacted item and has
	// to agree with the key. It is the only check that catches an answer
	// key that is wrong but self-consistent — every other check asks the
	// item about itself, and a confidently wrong key passes all of them.
	// It was hard-coded off, which left Gate 1 unable to catch the one
	// failure the two-gate design was built around.
	//
	// It costs an AI call per item, so a paid course is checked in full and
	// a free one by sample: money is the line where being wrong is
	// expensive enough to pay for certainty.
	paid := draft.PriceVND > 0
	items := collectDraftItems(structure, draft.CEFRLevel)
	sampled := blindSolveSample(len(items), paid)

	// The bodies already seen, so check 5 has something to deduplicate
	// against. It was never populated, so every item was compared to an
	// empty list and the same passage could be submitted four times.
	seen := make([]json.RawMessage, 0, len(items))

	for i, item := range items {
		stats.checked++
		blind := sampled[i]
		req := learningcontract.VerifyItemRequest{
			Kind:       item.Kind,
			TaskType:   item.TaskType,
			CEFRLevel:  item.CEFRLevel,
			Body:       item.Body,
			Existing:   seen,
			BlindSolve: blind,
		}
		if blind {
			stats.blindSolved++
		}
		if vErr := s.itemVerifier.VerifyItem(ctx, req); vErr != nil {
			outcome := classifyBlindSolve(vErr)
			switch {
			case blind && outcome == blindSolveDisagreed:
				// A single disagreement is as often the model as the item,
				// which is why the pools retry rather than reject. Counted
				// here and judged against the sample below.
				stats.blindRejects++
			case blind && outcome == blindSolveUnavailable:
				// Not evidence about this item. Uncount the sample so the
				// ratio below is over items we actually managed to check.
				stats.blindSolved--
				slog.WarnContext(ctx, "blind solve unavailable for a submitted item",
					"submission_id", submissionID, "kind", item.Kind, "error", vErr)
			default:
				failures = append(failures, domain.VerificationFailure{
					UnitIndex:     item.UnitIndex,
					LessonIndex:   item.LessonIndex,
					ActivityIndex: item.ActivityIndex,
					Kind:          item.Kind,
					Check:         "item_verifier",
					Message:       vErr.Error(),
				})
				continue
			}
		}
		seen = append(seen, item.Body)
	}

	// More than a tenth of the blind-solved sample disagreeing is the item
	// set, not the model.
	if stats.blindSolved > 0 && stats.blindRejects*10 > stats.blindSolved {
		failures = append(failures, domain.VerificationFailure{
			Check: "blind_solve",
			Message: fmt.Sprintf(
				"%d of %d checked answers did not survive an independent solve; the answer keys need review",
				stats.blindRejects, stats.blindSolved),
		})
	}

	return failures, stats
}

// verifyMaterials runs Gate 1's material_ready check: every lesson_material
// activity must point at a validated resource its draft owner owns, with the
// renditions the runner needs already ready.
//
// The resource id is creator-controlled draft jsonb, so it is resolved through
// MaterialForOwner with the *draft owner* — never the caller of the review
// endpoint — and the material kind is derived from the resource's detected MIME
// rather than trusted from the draft (WO 20 Stage B, traps 1 and 3).
func (s *Service) verifyMaterials(
	ctx context.Context, draft *domain.CourseDraft, structure *domain.CourseStructure,
) []domain.VerificationFailure {
	if structure == nil || s.materialPublisher == nil {
		return nil
	}
	var failures []domain.VerificationFailure
	for uIdx, unit := range structure.Units {
		for lIdx, lesson := range unit.Lessons {
			for aIdx, act := range lesson.Activities {
				if act.Kind != domain.KindLessonMaterial {
					continue
				}
				resourceID, ok := materialResourceID(act)
				if !ok {
					failures = append(failures, materialFailure(uIdx, lIdx, aIdx,
						"Material has no resource to publish"))
					continue
				}
				mat, err := s.materialPublisher.MaterialForOwner(ctx, draft.OwnerID, resourceID)
				if err != nil {
					failures = append(failures, materialFailure(uIdx, lIdx, aIdx,
						"Material resource does not exist or is not yours"))
					continue
				}
				if mat.Status != resourcecontract.MaterialValidated {
					failures = append(failures, materialFailure(uIdx, lIdx, aIdx,
						fmt.Sprintf("Material is %s, not validated yet", mat.Status)))
					continue
				}
				if msg, ok := missingMaterialRendition(mat); !ok {
					failures = append(failures, materialFailure(uIdx, lIdx, aIdx, msg))
				}
			}
		}
	}
	return failures
}

// materialResourceID reads the resource id an activity points at, from the
// material object or, as the editor also writes it, from the activity body.
func materialResourceID(act domain.ActivityDraft) (uuid.UUID, bool) {
	if act.Material != nil && act.Material.ResourceID != nil && *act.Material.ResourceID != uuid.Nil {
		return *act.Material.ResourceID, true
	}
	var body struct {
		ResourceID uuid.UUID `json:"resource_id"`
	}
	if len(act.Body) > 0 && json.Unmarshal(act.Body, &body) == nil && body.ResourceID != uuid.Nil {
		return body.ResourceID, true
	}
	return uuid.Nil, false
}

// missingMaterialRendition reports the message to fail with when the material
// lacks a rendition the runner needs, or ok when it is publishable.
func missingMaterialRendition(mat *resourcecontract.Material) (string, bool) {
	ready := map[string]bool{}
	for _, r := range mat.Renditions {
		if r.Status == resourcecontract.RenditionReady {
			ready[r.Kind] = true
		}
	}
	// The resource intake also accepts images and audio, which are neither a
	// document nor a video. Without this they would wait forever for a preview
	// they never get, and the creator would read "still processing".
	if !isMaterialMIME(mat.DetectedMIME) {
		return "Material must be a PDF, Word or PowerPoint document, or an MP4 or WebM video", false
	}
	if strings.HasPrefix(mat.DetectedMIME, "video/") {
		if !ready["video_360p"] {
			return "Video is still processing; its 360p rendition is not ready", false
		}
		return "", true
	}
	if !ready["preview"] {
		return "Document is still processing; its preview is not ready", false
	}
	return "", true
}

// materialMIMEs are the detected types a lesson material may have: the
// documents the media pipeline renders a preview for, and web video.
var materialMIMEs = map[string]bool{
	"application/pdf":    true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
	"application/vnd.ms-powerpoint":                                             true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	"video/mp4":  true,
	"video/webm": true,
}

func isMaterialMIME(detected string) bool {
	base, _, _ := strings.Cut(detected, ";")
	return materialMIMEs[strings.ToLower(strings.TrimSpace(base))]
}

// materialFailure builds a failure attributed to the material_ready check.
func materialFailure(uIdx, lIdx, aIdx int, message string) domain.VerificationFailure {
	return domain.VerificationFailure{
		UnitIndex:     uIdx,
		LessonIndex:   lIdx,
		ActivityIndex: aIdx,
		Kind:          domain.KindLessonMaterial,
		Check:         "material_ready",
		Message:       message,
	}
}

// RunGate1Verification is the automated gate (WO 15 §7).
//
// It checks the shape of the course, the kinds it uses, the safety of its text
// and every activity's answer key, and moves the submission to in_review when
// all of it passes. A submission that fails comes back to the creator with the
// report rather than reaching a moderator (BR-STUDIO-07).
func (s *Service) RunGate1Verification(
	ctx context.Context, submissionID uuid.UUID,
) (*domain.Submission, error) {
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

	itemFailures, stats := s.verifyDraftItems(ctx, sub.ID, draft, structure)
	failures = append(failures, itemFailures...)
	failures = append(failures, s.verifyMaterials(ctx, draft, structure)...)

	passed := len(failures) == 0
	report := domain.VerificationReport{
		Passed:            passed,
		ItemsChecked:      stats.checked,
		BlindSolved:       stats.blindSolved,
		BlindSolveRejects: stats.blindRejects,
		Failures:          failures,
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

	// A free course from a trusted creator that passed every check does not
	// wait for a person. This is what the trust model is for: a moderator's
	// time goes to courses that are sold, creators nobody has vouched for, and
	// anyone who has been wrong before.
	if passed && !updatedSub.Gate2Required {
		if err := s.PublishApprovedByGate1(ctx, updatedSub.ID); err != nil {
			slog.ErrorContext(ctx, "could not auto-publish a verified submission; it waits for review",
				"submission_id", updatedSub.ID, "error", err)
			return updatedSub, nil
		}
		return s.repo.GetSubmissionByID(ctx, updatedSub.ID)
	}

	return updatedSub, nil
}

// needsHumanReview decides whether a submission goes to a moderator (WO 15 §8).
//
// Money is the first line: a course somebody pays for is read by a person, every
// time. After that it is about the creator — their first courses, and anyone a
// report has been upheld against, are reviewed until they have earned otherwise.
//
// Decided once, when the submission is made, and stored. Recomputing it at
// review time would let a creator who became trusted while queued have their
// submission silently skip the human who was about to read it.
func needsHumanReview(profile *domain.CreatorProfile, draft *domain.CourseDraft) (bool, *string) {
	switch {
	case draft.PriceVND > 0:
		reason := "the course is sold, so a person reads it"
		return true, &reason
	case profile == nil || !profile.Trusted():
		reason := "the creator has not published enough reviewed courses yet"
		return true, &reason
	case profile.UpheldReportCount > 0:
		reason := "a report against this creator has been upheld"
		return true, &reason
	default:
		return false, nil
	}
}

// PublishApprovedByGate1 publishes a free submission from a trusted creator
// that passed the automated gate, without waiting for a moderator.
//
// It is the whole point of the trust model: a moderator's time is spent on
// courses that are sold, on creators nobody has vouched for yet, and on anyone
// who has been wrong before — not on the fourth free course from somebody whose
// last three were fine.
func (s *Service) PublishApprovedByGate1(ctx context.Context, submissionID uuid.UUID) error {
	sub, err := s.repo.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return err
	}
	if sub.Gate2Required || sub.Status != domain.SubmissionStatusInReview {
		return nil
	}
	const note = "Published automatically: verified, and from a trusted creator"
	_, err = s.publishSubmission(ctx, sub, sub.SubmittedBy, note)
	return err
}

// TakedownCourse removes a community course from sale.
//
// Existing purchasers keep it (BR-STUDIO-04): taking a course down is not a
// refund, and revoking what somebody bought because somebody else complained is
// a different decision with a different owner.
func (s *Service) TakedownCourse(
	ctx context.Context, actorID, courseID uuid.UUID, reason string,
) (*domain.Takedown, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, domain.ErrReasonRequired
	}
	listing, err := s.repo.GetListingByCourseID(ctx, courseID)
	if err != nil {
		return nil, err
	}

	takedown, err := s.repo.CreateTakedown(ctx, courseID, actorID, reason)
	if err != nil {
		return nil, fmt.Errorf("record takedown: %w", err)
	}
	if _, err := s.repo.SetListingStatus(ctx, courseID, domain.ListingStatusTakenDown); err != nil {
		return nil, fmt.Errorf("take listing down: %w", err)
	}
	if _, err := s.repo.RecordUpheldReport(ctx, listing.CreatorID); err != nil {
		slog.ErrorContext(ctx, "could not count a takedown against its creator",
			"creator_id", listing.CreatorID, "error", err)
	}

	slog.InfoContext(ctx, "community course taken down",
		"course_id", courseID, "actor_id", actorID, "reason", reason)
	return takedown, nil
}

// ReinstateCourse puts a taken-down course back on sale.
func (s *Service) ReinstateCourse(
	ctx context.Context, actorID, courseID uuid.UUID,
) (*domain.Takedown, error) {
	open, err := s.repo.GetOpenTakedown(ctx, courseID)
	if err != nil {
		return nil, err
	}
	reinstated, err := s.repo.ReinstateTakedown(ctx, open.ID, actorID)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.SetListingStatus(ctx, courseID, domain.ListingStatusActive); err != nil {
		return nil, fmt.Errorf("put the listing back: %w", err)
	}
	return reinstated, nil
}

// SuspendCreator stops a creator submitting or selling. Their published
// courses stay readable for the learners who bought them.
func (s *Service) SuspendCreator(
	ctx context.Context, creatorID uuid.UUID, reason string,
) (*domain.CreatorProfile, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, domain.ErrReasonRequired
	}
	profile, err := s.repo.SuspendCreator(ctx, creatorID, reason)
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "creator suspended", "creator_id", creatorID, "reason", reason)
	return profile, nil
}

// ReinstateCreator lifts a suspension. Trust is not restored with it: a
// creator who was suspended goes back through review.
func (s *Service) ReinstateCreator(
	ctx context.Context, creatorID uuid.UUID,
) (*domain.CreatorProfile, error) {
	return s.repo.ReinstateCreator(ctx, creatorID)
}

// draftItem is one activity of a draft, flattened with the position that names
// it in a failure report.
type draftItem struct {
	UnitIndex     int
	LessonIndex   int
	ActivityIndex int
	Kind          string
	TaskType      string
	CEFRLevel     string
	Body          json.RawMessage
}

func collectDraftItems(structure *domain.CourseStructure, courseCEFR string) []draftItem {
	items := make([]draftItem, 0)
	for uIdx, unit := range structure.Units {
		for lIdx, lesson := range unit.Lessons {
			cefr := lesson.CEFRLevel
			if cefr == "" {
				cefr = courseCEFR
			}
			for aIdx, act := range lesson.Activities {
				items = append(items, draftItem{
					UnitIndex:     uIdx,
					LessonIndex:   lIdx,
					ActivityIndex: aIdx,
					Kind:          act.Kind,
					TaskType:      act.TaskType,
					CEFRLevel:     cefr,
					Body:          act.Body,
				})
			}
		}
	}
	return items
}

// blindSolveSampleRate is the share of a free course's items that are blind
// solved, and blindSolveSampleMin the floor under it: a five-item course
// sampled at a fifth would be checked once, which proves nothing.
const (
	blindSolveSampleRate = 5 // one in five
	blindSolveSampleMin  = 5
)

// blindSolveSample marks which items to blind solve. Every item of a paid
// course; an evenly spread sample of a free one, so the choice does not fall
// on one lesson.
func blindSolveSample(count int, paid bool) []bool {
	marks := make([]bool, count)
	if count == 0 {
		return marks
	}
	if paid {
		for i := range marks {
			marks[i] = true
		}
		return marks
	}
	wanted := count / blindSolveSampleRate
	if wanted < blindSolveSampleMin {
		wanted = blindSolveSampleMin
	}
	if wanted > count {
		wanted = count
	}
	for i := 0; i < wanted; i++ {
		marks[i*count/wanted] = true
	}
	return marks
}

// blindSolveOutcome classifies a verifier error on a blind-solved item.
//
// Three things wear the same words. The model disagreeing with the key is the
// creator's problem and is counted against the sample. The provider being out
// of quota, or answering with something unparseable, is ours — those are not
// evidence about the item, and counting them would fail a correct course
// because an AI provider was down.
type blindSolveOutcome int

const (
	blindSolveNotApplicable blindSolveOutcome = iota // some other check failed
	blindSolveDisagreed                              // the model answered, and was right to disagree
	blindSolveUnavailable                            // we could not ask
)

func classifyBlindSolve(err error) blindSolveOutcome {
	if err == nil {
		return blindSolveNotApplicable
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "blind solve") {
		return blindSolveNotApplicable
	}
	if strings.Contains(msg, "call failed") || strings.Contains(msg, "parse blind solve") ||
		strings.Contains(msg, "unavailable") || strings.Contains(msg, "quota") {
		return blindSolveUnavailable
	}
	return blindSolveDisagreed
}

// ---------------------------------------------------------------- Gate 2 Moderation

// ListModerationQueue retrieves submissions pending staff review.
func (s *Service) ListModerationQueue(
	ctx context.Context, limit, offset int,
) ([]contract.ModerationQueueItem, int64, error) {
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

// publishDraft writes an approved draft into `lesson` and `content`: the
// course, its units and lessons, and a real content version per activity.
//
// The course is built unlisted; ApproveSubmission makes it public once the
// listing exists. It returns the course id and the hours the course is
// estimated at, so the caller can ensure it again with the same numbers.
//
// Not transactional, and it cannot be: the writes cross into two other modules
// through their contracts, which take no transaction. A failure part way
// through leaves an unlisted course nobody can find, which is the safe
// direction for that to fail in.
func (s *Service) publishDraft(
	ctx context.Context, draft *domain.CourseDraft,
) (courseID uuid.UUID, estimatedHours int, err error) {
	if s.lessonAuthor == nil || s.contentAuthor == nil {
		return uuid.Nil, 0, nil
	}

	var structure domain.CourseStructure
	if err := json.Unmarshal(draft.Structure, &structure); err != nil {
		return uuid.Nil, 0, fmt.Errorf("unmarshal draft structure for publish: %w", err)
	}
	estimatedHours = len(structure.Units) * 5

	courseSpec := lessoncontract.CourseSpec{
		Slug:           draft.Slug,
		Title:          draft.Title,
		Description:    draft.Description,
		CEFRFrom:       draft.CEFRLevel,
		CEFRTo:         draft.CEFRLevel,
		EstimatedHours: estimatedHours,
		Origin:         "community",
		OwnerID:        &draft.OwnerID,
		// Built unlisted, made public at the end.
		//
		// A paid course whose listing failed to write is a free course: the
		// paywall reads the listing, and a course with none is treated as
		// ours. Publishing the course before its price exists left that gap
		// open behind a discarded error. Nothing is reachable until the
		// listing is in place.
		Visibility:      "unlisted",
		TopicTaxonomyID: draft.TopicTaxonomyID,
	}
	publishedCourseID, err := s.lessonAuthor.EnsureCourse(ctx, courseSpec)
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("publish course in lesson module: %w", err)
	}

	for uIdx, unit := range structure.Units {
		unitSpec := lessoncontract.UnitSpec{
			CourseID:    publishedCourseID,
			Position:    uIdx + 1,
			Title:       unit.Title,
			Description: unit.Description,
		}
		unitID, err := s.lessonAuthor.EnsureUnit(ctx, unitSpec)
		if err != nil {
			return uuid.Nil, 0, fmt.Errorf("publish unit: %w", err)
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
				return uuid.Nil, 0, fmt.Errorf("publish lesson: %w", err)
			}

			activitySpecs := make([]lessoncontract.ActivitySpec, len(lesson.Activities))
			for aIdx, act := range lesson.Activities {
				contentSlug := fmt.Sprintf("%s-u%d-l%d-a%d", draft.Slug, uIdx+1, lIdx+1, aIdx+1)
				body := act.Body
				config := act.Config
				// A material points at the creator's private resource until
				// publish. The copy makes the course own its bytes, and the
				// content body and activity config carry the copied object keys
				// — never a URL, and never a reference to the creator's account
				// (D20-2, D20-3, BR-STUDIO-08).
				if act.Kind == domain.KindLessonMaterial {
					materialBody, err := s.publishMaterial(ctx, draft, publishedCourseID, contentSlug, act)
					if err != nil {
						return uuid.Nil, 0, err
					}
					body = materialBody
					config = materialBody
				}
				versionID, err := s.contentAuthor.EnsurePublished(ctx, contentcontract.AuthorSpec{
					Slug:      contentSlug,
					Kind:      act.Kind,
					CEFRLevel: level,
					Body:      body,
					AuthorID:  draft.OwnerID,
				})
				if err != nil {
					return uuid.Nil, 0, fmt.Errorf("publish content version: %w", err)
				}
				activitySpecs[aIdx] = lessoncontract.ActivitySpec{
					Position:         aIdx + 1,
					Kind:             act.Kind,
					ContentVersionID: versionID,
					Config:           config,
					Weight:           act.Weight,
				}
			}

			if err := s.lessonAuthor.SyncActivities(ctx, lessonID, activitySpecs); err != nil {
				return uuid.Nil, 0, fmt.Errorf("sync lesson activities: %w", err)
			}
		}
	}

	return publishedCourseID, estimatedHours, nil
}

// publishMaterial copies a lesson_material's resource into the course's own
// storage and returns the content body — object keys only, as Gate 1 requires.
//
// The copy happens before the listing is made public, inside publishDraft's
// existing "built unlisted, made public at the end" sequence: a failed copy
// fails the publish, and nothing half-published is reachable.
func (s *Service) publishMaterial(
	ctx context.Context, draft *domain.CourseDraft, courseID uuid.UUID,
	contentSlug string, act domain.ActivityDraft,
) (json.RawMessage, error) {
	if s.materialPublisher == nil {
		return nil, fmt.Errorf("publish material %s: material publishing is not configured", contentSlug)
	}
	resourceID, ok := materialResourceID(act)
	if !ok {
		return nil, fmt.Errorf("publish material %s: activity has no resource", contentSlug)
	}
	// Ownership is proven again here, not only at Gate 1: the draft could have
	// been edited between the two.
	mat, err := s.materialPublisher.MaterialForOwner(ctx, draft.OwnerID, resourceID)
	if err != nil {
		return nil, fmt.Errorf("publish material %s: %w", contentSlug, err)
	}
	prefix := fmt.Sprintf("course-materials/%s/%s/", courseID, contentSlug)
	objects, err := s.materialPublisher.CopyForPublication(ctx, resourceID, prefix)
	if err != nil {
		return nil, fmt.Errorf("publish material %s: %w", contentSlug, err)
	}

	title, description := "", ""
	if act.Material != nil {
		title = act.Material.Title
		description = act.Material.Description
	}

	objectMap := map[string]any{"original": objects.Original}
	if objects.Poster != nil {
		objectMap["poster"] = *objects.Poster
	}
	if objects.Video360p != nil {
		objectMap["video_360p"] = *objects.Video360p
	}
	if objects.Video720p != nil {
		objectMap["video_720p"] = *objects.Video720p
	}
	if objects.Preview != nil {
		objectMap["preview"] = *objects.Preview
	}

	body := map[string]any{
		"material_kind": materialKindFromMIME(mat.DetectedMIME),
		"title":         title,
		"objects":       objectMap,
	}
	if description != "" {
		body["description"] = description
	}
	if secs, ok := videoDurationSeconds(mat); ok {
		body["duration_seconds"] = secs
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode material %s body: %w", contentSlug, err)
	}
	return raw, nil
}

// materialKindFromMIME derives the material kind from the resource's bytes,
// never from the creator-controlled draft (Stage B trap 3).
func materialKindFromMIME(detected string) string {
	if strings.HasPrefix(detected, "video/") {
		return "video"
	}
	return "document"
}

// videoDurationSeconds reads a video's duration off its renditions' metadata.
func videoDurationSeconds(mat *resourcecontract.Material) (int, bool) {
	for _, r := range mat.Renditions {
		if r.DurationMS != nil && *r.DurationMS > 0 {
			return *r.DurationMS / 1000, true
		}
	}
	return 0, false
}

// ApproveSubmission is the human gate: it publishes the course.
//
// The reviewer may not be the creator (BR-STUDIO-06, and a CHECK constraint
// says so too), and the submission must have passed Gate 1.
func (s *Service) ApproveSubmission(
	ctx context.Context, reviewerID, submissionID uuid.UUID,
) (*domain.Submission, error) {
	sub, err := s.repo.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	// BR-STUDIO-06 / BR-CONTENT-03: reviewer cannot review their own submission
	if sub.SubmittedBy == reviewerID {
		return nil, domain.ErrSelfReviewForbidden
	}

	// BR-STUDIO-07: a submission that has not passed Gate 1 never reaches a
	// human decision. `submitted` and `verifying` are states the automated gate
	// still owns; only `in_review` means it passed and handed over.
	if sub.Status != domain.SubmissionStatusInReview {
		return nil, apperr.New(apperr.Conflict, "INVALID_STATE",
			"Submission has not passed automated verification yet")
	}

	feedback := "Approved by moderator"
	return s.publishSubmission(ctx, sub, reviewerID, feedback)
}

// publishSubmission turns an approved submission into a published course.
//
// Shared by the human gate and by the automatic publish a trusted creator's
// free course gets, so there is one path from "verified" to "in the
// catalogue" rather than two that can drift.
func (s *Service) publishSubmission(
	ctx context.Context, sub *domain.Submission, reviewerID uuid.UUID, feedback string,
) (*domain.Submission, error) {
	draft, err := s.repo.GetCourseDraftByID(ctx, sub.DraftID)
	if err != nil {
		return nil, err
	}

	publishedCourseID, estimatedHours, err := s.publishDraft(ctx, draft)
	if err != nil {
		return nil, err
	}

	// The listing, before the course can be reached and before anything is
	// marked approved. Its failure fails the approval: a course with no listing
	// is a course with no price, and the paywall reads it as ours to give away.
	if publishedCourseID != uuid.Nil {
		pricingModel := domain.PricingModelFree
		if draft.PriceVND > 0 {
			pricingModel = domain.PricingModelOneTime
		}
		bps := s.revenueShareBPS
		if bps <= 0 {
			bps = domain.DefaultRevenueShareBPS
		}
		if _, err := s.repo.UpsertListing(ctx, &domain.Listing{
			CourseID:        publishedCourseID,
			CreatorID:       draft.OwnerID,
			PricingModel:    pricingModel,
			PriceVND:        draft.PriceVND,
			RevenueShareBPS: bps,
			Status:          domain.ListingStatusActive,
		}); err != nil {
			return nil, fmt.Errorf("write listing for course %s: %w", publishedCourseID, err)
		}

		// Now it may be found.
		if _, err := s.lessonAuthor.EnsureCourse(ctx, lessoncontract.CourseSpec{
			Slug:            draft.Slug,
			Title:           draft.Title,
			Description:     draft.Description,
			CEFRFrom:        draft.CEFRLevel,
			CEFRTo:          draft.CEFRLevel,
			EstimatedHours:  estimatedHours,
			Origin:          "community",
			OwnerID:         &draft.OwnerID,
			Visibility:      "public",
			TopicTaxonomyID: draft.TopicTaxonomyID,
		}); err != nil {
			return nil, fmt.Errorf("publish course %s: %w", publishedCourseID, err)
		}
	}

	approvedSub, err := s.repo.UpdateSubmissionReview(ctx, sub.ID, domain.SubmissionStatusApproved, reviewerID, &feedback)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.UpdateCourseDraftStatus(ctx, draft.ID, domain.DraftStatusPublished); err != nil {
		return nil, err
	}

	// Counted towards the trust that lets this creator's next free course
	// publish on the automated gate alone. A failure here costs them a step
	// towards trust, not their published course, so it is logged rather than
	// unwinding an approval.
	if _, err := s.repo.RecordApprovedCourse(ctx, draft.OwnerID); err != nil {
		slog.ErrorContext(ctx, "could not count an approved course towards creator trust",
			"creator_id", draft.OwnerID, "error", err)
	}

	return approvedSub, nil
}

// RejectSubmission records moderator rejection or changes-requested decision.
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

// ---------------------------------------------------------------- Paywall & Access (BR-STUDIO-05)

// MayOpen evaluates course opening permission.
// Truth table:
// 1. official -> true
// 2. free community -> true
// 3. paid unowned -> false
// 4. paid owned -> true
// 5. paid refunded -> false
// 6. paid taken down -> true if owned, false if unowned
func (s *Service) MayOpen(ctx context.Context, userID *uuid.UUID, courseID uuid.UUID) (bool, error) {
	listing, err := s.repo.GetListingByCourseID(ctx, courseID)
	if err != nil {
		if errors.Is(err, domain.ErrListingNotFound) {
			// Official curriculum courses have no studio listing -> open
			return true, nil
		}
		return false, err
	}

	// Free community courses always answer yes
	if listing.PricingModel == domain.PricingModelFree || listing.PriceVND == 0 {
		return true, nil
	}

	// Paid community course:
	// If anonymous caller (nil or uuid.Nil), cannot open
	if userID == nil || *userID == uuid.Nil {
		return false, nil
	}

	// Creator always has access to their own course
	if *userID == listing.CreatorID {
		return true, nil
	}

	// Check if learner has an active purchase (revoked_at IS NULL)
	// BR-STUDIO-04: Taking a course down never revokes a purchase. Every learner who already bought it keeps access.
	purchase, err := s.repo.GetActivePurchase(ctx, *userID, courseID)
	if err != nil {
		return false, err
	}
	if purchase != nil {
		return true, nil
	}

	return false, nil
}

// ---------------------------------------------------------------- Listing & Purchases

// GetListing returns the course listing if it exists.
func (s *Service) GetListing(ctx context.Context, courseID uuid.UUID) (*contract.CourseListing, error) {
	listing, err := s.repo.GetListingByCourseID(ctx, courseID)
	if err != nil {
		if errors.Is(err, domain.ErrListingNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toContractListing(listing), nil
}

// BatchGetListings retrieves listings for multiple course IDs.
func (
	s *Service) BatchGetListings(ctx context.Context,
	courseIDs []uuid.UUID) (map[uuid.UUID]*contract.CourseListing,
	error,
) {
	if len(courseIDs) == 0 {
		return map[uuid.UUID]*contract.CourseListing{}, nil
	}
	listings, err := s.repo.ListListingsByCourseIDs(ctx, courseIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[uuid.UUID]*contract.CourseListing, len(listings))
	for _, l := range listings {
		result[l.CourseID] = toContractListing(l)
	}
	return result, nil
}

// HasPurchased returns whether the given user has an active purchase of the course.
func (s *Service) HasPurchased(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	if userID == uuid.Nil {
		return false, nil
	}
	p, err := s.repo.GetActivePurchase(ctx, userID, courseID)
	if err != nil {
		return false, err
	}
	return p != nil, nil
}

// BatchHasPurchased returns purchase status for a set of courses.
func (
	s *Service) BatchHasPurchased(ctx context.Context,
	userID uuid.UUID,
	courseIDs []uuid.UUID) (map[uuid.UUID]bool,
	error,
) {
	result := make(map[uuid.UUID]bool, len(courseIDs))
	if userID == uuid.Nil || len(courseIDs) == 0 {
		return result, nil
	}
	purchases, err := s.repo.ListActivePurchasesByUserAndCourseIDs(ctx, userID, courseIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range purchases {
		result[p.CourseID] = true
	}
	return result, nil
}

// ClaimCourse grants access to a free community course without payment.
func (s *Service) ClaimCourse(ctx context.Context, userID, courseID uuid.UUID) (*domain.Purchase, error) {
	if userID == uuid.Nil {
		return nil, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required to claim course")
	}

	listing, err := s.repo.GetListingByCourseID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if listing.Status != domain.ListingStatusActive {
		return nil, domain.ErrListingNotFound
	}
	if listing.PricingModel != domain.PricingModelFree && listing.PriceVND > 0 {
		return nil, domain.ErrCourseNotFree
	}

	existing, err := s.repo.GetActivePurchase(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrCourseAlreadyPurchased
	}

	purchase := &domain.Purchase{
		UserID:       userID,
		CourseID:     courseID,
		OrderID:      nil,
		PricePaidVND: 0,
	}
	return s.repo.CreatePurchase(ctx, purchase)
}

// PurchaseCourse creates a billing order to purchase a paid community course.
func (s *Service) PurchaseCourse(ctx context.Context, userID, courseID uuid.UUID) (*paymentcontract.Order, error) {
	if userID == uuid.Nil {
		return nil, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required to purchase course")
	}
	if s.orderCreator == nil {
		return nil, apperr.New(apperr.Internal, "BILLING_UNAVAILABLE", "Billing service is not configured")
	}

	listing, err := s.repo.GetListingByCourseID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if listing.Status != domain.ListingStatusActive {
		return nil, domain.ErrListingNotFound
	}
	if listing.PricingModel != domain.PricingModelOneTime || listing.PriceVND <= 0 {
		return nil, domain.ErrCourseNotPaid
	}

	existing, err := s.repo.GetActivePurchase(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrCourseAlreadyPurchased
	}

	return s.orderCreator.CreateOrder(ctx, paymentcontract.CreateOrderInput{
		UserID:      userID,
		SubjectKind: "course",
		SubjectID:   courseID,
		AmountVND:   listing.PriceVND,
	})
}

// ListUserPurchases returns all course purchases for a user.
func (s *Service) ListUserPurchases(
	ctx context.Context, userID uuid.UUID, limit, offset int,
) ([]*domain.Purchase, int64, error) {
	return s.repo.ListPurchasesByUserID(ctx, userID, limit, offset)
}

// RefundPurchase revokes a purchase and reverses creator ledger credits within policy window.
func (s *Service) RefundPurchase(ctx context.Context, userID, purchaseID uuid.UUID) error {
	purchase, err := s.repo.GetPurchaseByID(ctx, purchaseID)
	if err != nil {
		return err
	}
	if purchase.UserID != userID {
		return domain.ErrPurchaseNotFound
	}
	if purchase.RevokedAt != nil {
		return domain.ErrAlreadyRefunded
	}
	if purchase.PricePaidVND <= 0 {
		return apperr.New(apperr.Validation, "CANNOT_REFUND_FREE", "Free or claimed courses cannot be refunded")
	}

	// 7 days window (WO 15 §3 / §9)
	if time.Since(purchase.GrantedAt) > 7*24*time.Hour {
		return domain.ErrRefundWindowExpired
	}

	completion, err := s.courseCompletion(ctx, userID, purchase.CourseID)
	if err != nil {
		return err
	}
	if completion >= refundMaxCompletionPercent {
		return domain.ErrRefundProgressExceeded
	}

	// The split this sale was actually recorded with, not today's rate.
	// BR-STUDIO-03: changing the revenue share later must not make an old
	// refund fail to balance against the sale it reverses.
	sale, err := s.repo.GetSaleLedgerEntryByPurchaseID(ctx, purchase.ID)
	if err != nil {
		return fmt.Errorf("read the sale this refund reverses: %w", err)
	}

	// The obligation is recorded before anything is taken away. A learner who
	// loses access and is owed nothing on paper is the failure this ordering
	// exists to prevent — it is how the first version of this lost money.
	if purchase.OrderID != nil {
		if s.refundRecorder == nil {
			return apperr.New(apperr.Internal, "BILLING_UNAVAILABLE",
				"Refunds cannot be recorded because billing is not configured")
		}
		if _, err := s.refundRecorder.RecordRefund(
			ctx, *purchase.OrderID, purchase.PricePaidVND, "learner_refund", userID,
		); err != nil {
			return fmt.Errorf("record refund against order %s: %w", *purchase.OrderID, err)
		}
	}

	// Reverse exactly what the sale credited.
	if _, err := s.repo.CreateLedgerEntry(ctx, &domain.CreatorLedgerEntry{
		CreatorID:      sale.CreatorID,
		Kind:           domain.LedgerKindRefund,
		AmountVND:      -sale.AmountVND,
		GrossAmountVND: -sale.GrossAmountVND,
		FeeAmountVND:   -sale.FeeAmountVND,
		PurchaseID:     &purchase.ID,
		Note:           "Self-service learner refund within the 7-day window",
	}); err != nil {
		return fmt.Errorf("reverse creator ledger credit: %w", err)
	}

	if _, err := s.repo.RevokePurchase(ctx, purchase.ID, "learner_refund"); err != nil {
		return fmt.Errorf("revoke purchase: %w", err)
	}

	return nil
}

// HandlePaymentSucceeded creates a purchase and writes ledger credit rows when an order is paid.
func (s *Service) HandlePaymentSucceeded(ctx context.Context, event paymentcontract.EventPaymentSucceeded) error {
	if event.SubjectKind != "course" {
		return nil
	}

	listing, err := s.repo.GetListingByCourseID(ctx, event.SubjectID)
	if err != nil {
		return fmt.Errorf("listing for course %s not found: %w", event.SubjectID, err)
	}

	// Create purchase
	purchase, err := s.repo.CreatePurchase(ctx, &domain.Purchase{
		UserID:       event.UserID,
		CourseID:     event.SubjectID,
		OrderID:      &event.OrderID,
		PricePaidVND: event.AmountVND,
	})
	if err != nil {
		return fmt.Errorf("create purchase on payment match: %w", err)
	}

	// Calculate splits
	bps := int64(listing.RevenueShareBPS)
	if bps <= 0 {
		bps = int64(domain.DefaultRevenueShareBPS)
	}
	creatorShare := (event.AmountVND * bps) / 10000
	platformFee := event.AmountVND - creatorShare

	// Row 1: Creator's share
	_, err = s.repo.CreateLedgerEntry(ctx, &domain.CreatorLedgerEntry{
		CreatorID:      listing.CreatorID,
		Kind:           domain.LedgerKindSale,
		AmountVND:      creatorShare,
		GrossAmountVND: event.AmountVND,
		FeeAmountVND:   platformFee,
		PurchaseID:     &purchase.ID,
		Note:           fmt.Sprintf("Course sale (%d%% creator share)", bps/100),
	})
	if err != nil {
		return fmt.Errorf("create creator ledger entry: %w", err)
	}

	// Row 2: Platform fee
	_, err = s.repo.CreateLedgerEntry(ctx, &domain.CreatorLedgerEntry{
		CreatorID:      listing.CreatorID,
		Kind:           domain.LedgerKindPlatformShare,
		AmountVND:      platformFee,
		GrossAmountVND: event.AmountVND,
		FeeAmountVND:   platformFee,
		PurchaseID:     &purchase.ID,
		Note:           fmt.Sprintf("Platform share (%d%%)", (10000-bps)/100),
	})
	if err != nil {
		return fmt.Errorf("create platform share ledger entry: %w", err)
	}

	return nil
}

func toContractListing(l *domain.Listing) *contract.CourseListing {
	if l == nil {
		return nil
	}
	return &contract.CourseListing{
		CourseID:        l.CourseID,
		CreatorID:       l.CreatorID,
		PricingModel:    l.PricingModel,
		PriceVND:        l.PriceVND,
		RevenueShareBPS: l.RevenueShareBPS,
		Status:          l.Status,
		PublishedAt:     l.PublishedAt,
	}
}

// refundMaxCompletionPercent is how much of a course a learner may have done
// and still refund it (WO 15 §3).
const refundMaxCompletionPercent = 20

// courseCompletion is the percentage of a course's lessons the learner has
// completed.
//
// It used to read the course-scope progress row's `Score` and compare it to 20.
// That column is the learner's average *grade*, not how far through they are,
// and the row is only written once the whole course is finished — so the rule
// became "refund allowed unless you completed the course scoring 20 or more",
// which let anyone answer every question wrong and refund a finished course.
//
// Lesson progress is what measures distance travelled, so that is what this
// counts. A course whose lessons cannot be listed is treated as not started
// rather than as fully done: refusing a refund because of our own read failure
// would take the learner's money over our bug.
func (s *Service) courseCompletion(ctx context.Context, userID, courseID uuid.UUID) (int, error) {
	if s.progressReader == nil || s.lessonReader == nil {
		return 0, nil
	}

	units, err := s.lessonReader.ListUnitsByCourseID(ctx, courseID)
	if err != nil {
		slog.WarnContext(ctx, "could not size course for a refund; treating it as not started",
			"course_id", courseID, "error", err)
		return 0, nil
	}
	lessonIDs := make(map[uuid.UUID]bool)
	for _, unit := range units {
		lessons, err := s.lessonReader.ListLessons(ctx, unit.ID)
		if err != nil {
			slog.WarnContext(ctx, "could not list unit lessons for a refund",
				"unit_id", unit.ID, "error", err)
			return 0, nil
		}
		for _, lesson := range lessons {
			lessonIDs[lesson.ID] = true
		}
	}
	if len(lessonIDs) == 0 {
		return 0, nil
	}

	progress, err := s.progressReader.ProgressOf(ctx, userID, learningcontract.ScopeLesson)
	if err != nil {
		slog.WarnContext(ctx, "could not read lesson progress for a refund",
			"user_id", userID, "error", err)
		return 0, nil
	}
	completed := 0
	for _, p := range progress {
		if lessonIDs[p.ScopeID] && p.Status == progressStatusCompleted {
			completed++
		}
	}
	return completed * 100 / len(lessonIDs), nil
}

// progressStatusCompleted is learn.progress's completed status, as
// learning/domain writes it.
const progressStatusCompleted = "completed"

// ---------------------------------------------------------------- Creator Earnings & Payouts (Step 7)

// GetEarnings returns the creator earnings summary and recent ledger activity.
func (s *Service) GetEarnings(ctx context.Context, creatorID uuid.UUID) (*domain.EarningsSummary, error) {
	balance, err := s.repo.GetCreatorBalance(ctx, creatorID)
	if err != nil {
		return nil, fmt.Errorf("get creator balance: %w", err)
	}

	lifetime, err := s.repo.GetCreatorLifetimeEarnings(ctx, creatorID)
	if err != nil {
		return nil, fmt.Errorf("get lifetime earnings: %w", err)
	}

	totalPaidOut, err := s.repo.GetCreatorTotalPaidOut(ctx, creatorID)
	if err != nil {
		return nil, fmt.Errorf("get total paid out: %w", err)
	}

	var pendingPayout int64
	if s.payoutManager != nil {
		pending, err := s.payoutManager.GetPendingPayoutTotal(ctx, creatorID)
		if err != nil {
			return nil, fmt.Errorf("get pending payout total: %w", err)
		}
		pendingPayout = pending
	}

	available := balance - pendingPayout
	if available < 0 {
		available = 0
	}

	threshold := s.payoutThresholdVND
	if threshold <= 0 {
		threshold = domain.DefaultPayoutThresholdVND
	}

	account, err := s.repo.GetPayoutAccount(ctx, creatorID)
	hasAccount := (err == nil && account != nil)

	var bankCode, holder, maskedAccount *string
	if hasAccount {
		bankCode = &account.BankCode
		holder = &account.AccountHolderName
		masked := maskAccountNumber(account.AccountNumber)
		maskedAccount = &masked
	}

	canRequest := hasAccount && (available >= threshold)

	recentEntries, err := s.repo.ListLedgerEntriesByCreatorID(ctx, creatorID, 20, 0)
	if err != nil {
		return nil, fmt.Errorf("list ledger entries: %w", err)
	}

	return &domain.EarningsSummary{
		AvailableBalanceVND:     available,
		LifetimeEarningsVND:     lifetime,
		PendingPayoutVND:        pendingPayout,
		TotalPaidOutVND:         totalPaidOut,
		PayoutThresholdVND:      threshold,
		CanRequestPayout:        canRequest,
		PayoutAccountConfigured: hasAccount,
		PayoutBankCode:          bankCode,
		PayoutAccountHolder:     holder,
		PayoutMaskedAccount:     maskedAccount,
		RecentLedger:            recentEntries,
	}, nil
}

// RequestPayout initiates a payout request against available creator balance.
func (s *Service) RequestPayout(
	ctx context.Context, creatorID uuid.UUID, requestedAmount *int64,
) (*paymentcontract.Payout, error) {
	if s.payoutManager == nil {
		return nil, apperr.New(apperr.Internal, "PAYOUT_SERVICE_UNAVAILABLE", "Payout service is not available")
	}

	account, err := s.repo.GetPayoutAccount(ctx, creatorID)
	if err != nil || account == nil {
		return nil, domain.ErrPayoutAccountRequired
	}

	balance, err := s.repo.GetCreatorBalance(ctx, creatorID)
	if err != nil {
		return nil, fmt.Errorf("get creator balance: %w", err)
	}

	pendingPayout, err := s.payoutManager.GetPendingPayoutTotal(ctx, creatorID)
	if err != nil {
		return nil, fmt.Errorf("get pending payout total: %w", err)
	}

	available := balance - pendingPayout
	if available < 0 {
		available = 0
	}

	threshold := s.payoutThresholdVND
	if threshold <= 0 {
		threshold = domain.DefaultPayoutThresholdVND
	}

	payoutAmount := available
	if requestedAmount != nil && *requestedAmount > 0 {
		payoutAmount = *requestedAmount
	}

	if payoutAmount < threshold {
		return nil, domain.ErrPayoutBelowMinimum
	}

	if payoutAmount > available {
		return nil, domain.ErrInsufficientBalance
	}

	payout, err := s.payoutManager.CreatePayout(ctx, paymentcontract.CreatePayoutInput{
		CreatorID: creatorID,
		AmountVND: payoutAmount,
		ActorID:   creatorID,
	})
	if err != nil {
		return nil, fmt.Errorf("create payout: %w", err)
	}

	return payout, nil
}

// HandlePayoutSent records a payout debit in the creator ledger upon bank fulfillment.
func (s *Service) HandlePayoutSent(ctx context.Context, event paymentcontract.EventPayoutSent) error {
	note := fmt.Sprintf("Payout fulfilled (ref: %s)", event.BankReference)
	if event.BankReference == "" {
		note = "Payout fulfilled"
	}

	_, err := s.repo.CreateLedgerEntry(ctx, &domain.CreatorLedgerEntry{
		CreatorID:      event.CreatorID,
		Kind:           domain.LedgerKindPayout,
		AmountVND:      -event.AmountVND,
		GrossAmountVND: event.AmountVND,
		FeeAmountVND:   0,
		PayoutID:       &event.PayoutID,
		Note:           note,
	})
	if err != nil {
		return fmt.Errorf("create payout debit ledger entry: %w", err)
	}
	return nil
}

func maskAccountNumber(num string) string {
	cleaned := strings.TrimSpace(num)
	if len(cleaned) <= 4 {
		return cleaned
	}
	return strings.Repeat("*", len(cleaned)-4) + cleaned[len(cleaned)-4:]
}
