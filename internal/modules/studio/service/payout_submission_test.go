package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	resourcecontract "github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/modules/studio/service"
)

const (
	testVideoMIME      = "video/mp4"
	testVideoRendition = "video_360p"
)

// TestPayoutAccount_FirstSaveAndRepeatedSaves covers the defect where a first
// payout save inserted a row whose foreign key needed a creator profile the
// Studio UI never created.
func TestPayoutAccount_FirstSaveAndRepeatedSaves(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, nil, nil, nil)
	creatorID := uuid.New()

	if _, err := repo.GetCreatorProfile(ctx, creatorID); !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("fixture should have no creator profile, got %v", err)
	}

	first, err := svc.UpsertPayoutAccount(ctx, creatorID, "MB", "0123456789", "NGUYEN VAN A", true)
	if err != nil {
		t.Fatalf("first payout save should succeed without a separate profile action: %v", err)
	}
	if !first.IsDefault {
		t.Fatal("a saved account is the default destination")
	}

	// A repeated save updates the current account instead of appending one.
	second, err := svc.UpsertPayoutAccount(ctx, creatorID, "VCB", "1017588888", "NGUYEN VAN A", true)
	if err != nil {
		t.Fatalf("repeat payout save: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("repeated save created a second account: %s then %s", first.ID, second.ID)
	}
	if second.BankCode != "VCB" || second.AccountNumber != "1017588888" {
		t.Errorf("repeated save did not update details: %+v", second)
	}
	if len(repo.payoutAccounts) != 1 {
		t.Errorf("expected one account per creator, got %d", len(repo.payoutAccounts))
	}

	// An explicit non-default is stored as such, and is not the account the
	// payout flow reads.
	if _, err := svc.UpsertPayoutAccount(ctx, creatorID, "VCB", "1017588888", "NGUYEN VAN A", false); err != nil {
		t.Fatalf("explicit non-default save: %v", err)
	}
	account, err := svc.GetPayoutAccount(ctx, creatorID)
	if err != nil {
		t.Fatalf("get payout account: %v", err)
	}
	if account != nil {
		t.Errorf("a non-default account is not the payout destination, got %+v", account)
	}
}

// TestSubmitDraft_NoVerifierCall is the contract's POST /studio/courses/{id}/submit
// must return immediately: no ItemVerifier, no external AI in the request.
func TestSubmitDraft_NoVerifierCall(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	verifier := &mockVerifier{}
	svc := service.NewService(repo, verifier, nil, nil)
	creatorID := uuid.New()

	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Async Submit", Slug: "async-submit", Description: "d", CEFRLevel: "B1",
		Structure: createTestCourseStructure(1, 3, 7),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	sub, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("submit draft: %v", err)
	}
	if verifier.calls != 0 {
		t.Fatalf("submit called ItemVerifier %d times", verifier.calls)
	}
	if sub.Status != domain.SubmissionStatusSubmitted {
		t.Fatalf("submission status = %s, want submitted", sub.Status)
	}
	if sub.SubmittedAt.IsZero() {
		t.Error("the persisted submission should carry its submitted_at")
	}
	if repo.drafts[draft.ID].Status != domain.DraftStatusSubmitted {
		t.Errorf("draft status = %s, want submitted", repo.drafts[draft.ID].Status)
	}
}

// TestSubmitDraft_Gate2DecisionIsPersisted proves the reviewer decision made at
// submit time survives the write; recomputing it at review time is what the
// stored columns exist to prevent.
func TestSubmitDraft_Gate2DecisionIsPersisted(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, &mockVerifier{}, nil, nil)
	creatorID := uuid.New()

	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Paid Course", Slug: "paid-course", Description: "d", CEFRLevel: "B1",
		Structure: createTestCourseStructure(1, 3, 7),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	price := int64(199_000)
	if _, err := svc.UpdateDraft(ctx, creatorID, draft.ID, service.UpdateDraftRequest{PriceVND: &price}); err != nil {
		t.Fatalf("price draft: %v", err)
	}

	sub, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("submit draft: %v", err)
	}
	persisted := repo.submissions[sub.ID]
	if persisted == nil {
		t.Fatal("submission was not persisted")
	}
	if !persisted.Gate2Required {
		t.Error("a paid course must need a human, persisted")
	}
	if persisted.Gate2Reason == nil || *persisted.Gate2Reason == "" {
		t.Error("the stored decision should carry its reason")
	}
}

// TestSubmitDraft_FailureLeavesNoPartialState: a lookup or write failure inside
// the submission transaction must propagate and leave neither a submission nor
// a moved draft behind.
func TestSubmitDraft_FailureLeavesNoPartialState(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, &mockVerifier{}, nil, nil)
	creatorID := uuid.New()

	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Rollback", Slug: "rollback", Description: "d", CEFRLevel: "B1",
		Structure: createTestCourseStructure(1, 3, 7),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	repo.submitErr = errors.New("submission version lookup failed")
	if _, err := svc.SubmitDraft(ctx, creatorID, draft.ID); !errors.Is(err, repo.submitErr) {
		t.Fatalf("expected the repository failure to propagate, got %v", err)
	}
	if len(repo.submissions) != 0 {
		t.Errorf("a failed submit left %d submission(s) behind", len(repo.submissions))
	}
	if repo.drafts[draft.ID].Status != domain.DraftStatusDraft {
		t.Errorf("a failed submit moved the draft to %s", repo.drafts[draft.ID].Status)
	}

	// The same path answers a state conflict rather than a second active
	// submission when the draft is already submitted.
	repo.submitErr = nil
	if _, err := svc.SubmitDraft(ctx, creatorID, draft.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.SubmitDraft(ctx, creatorID, draft.ID); !errors.Is(err, domain.ErrCannotSubmit) {
		t.Fatalf("expected ErrCannotSubmit on repeated submit, got %v", err)
	}
}

// TestGetDraftDetail_LatestSubmissionAndOwnership covers the optional
// latest_submission on the owner-scoped draft detail endpoint.
func TestGetDraftDetail_LatestSubmissionAndOwnership(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, &mockVerifier{}, nil, nil)
	creatorID := uuid.New()

	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Detail", Slug: "detail", Description: "d", CEFRLevel: "B1",
		Structure: createTestCourseStructure(1, 3, 7),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	_, latest, err := svc.GetDraftDetail(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("get detail before submit: %v", err)
	}
	if latest != nil {
		t.Errorf("a draft with no submissions must return no latest_submission, got %+v", latest)
	}

	otherCreator := uuid.New()
	if _, _, err := svc.GetDraftDetail(ctx, otherCreator, draft.ID); !errors.Is(err, domain.ErrDraftNotFound) {
		t.Errorf("another creator must receive a 404, got %v", err)
	}

	sub, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("submit draft: %v", err)
	}
	if _, err := svc.RunGate1Verification(ctx, sub.ID); err != nil {
		t.Fatalf("run Gate 1: %v", err)
	}

	_, latest, err = svc.GetDraftDetail(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if latest == nil || latest.ID != sub.ID || latest.Version != 1 {
		t.Fatalf("expected the persisted latest submission, got %+v", latest)
	}

	// A failed lookup propagates rather than silently rendering "no
	// submissions".
	repo.latestErr = errors.New("latest submission read failed")
	if _, _, err := svc.GetDraftDetail(ctx, creatorID, draft.ID); !errors.Is(err, repo.latestErr) {
		t.Fatalf("expected the latest-submission failure to propagate, got %v", err)
	}
}

// TestGate1_MaterialsGoToThePublisherNotTheItemVerifier: a lesson_material is
// something to read or watch, so Gate 1 checks it through the resource
// contract and never asks ItemVerifier to grade a document.
func TestGate1_MaterialsGoToThePublisherNotTheItemVerifier(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	verifier := &mockVerifier{}
	svc := service.NewService(repo, verifier, &mockLessonAuthor{}, &mockContentAuthor{})
	resourceID := uuid.New()
	publisher := &mockMaterialPublisher{material: &resourcecontract.Material{
		ID:           resourceID,
		Status:       resourcecontract.MaterialValidated,
		DetectedMIME: testVideoMIME,
		Renditions: []resourcecontract.Rendition{
			{Kind: testVideoRendition, Status: resourcecontract.RenditionReady},
		},
	}}
	svc.SetMaterialPublisher(publisher)

	creatorID := uuid.New()
	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Watch and Practise", Slug: "watch-and-practise", Description: "d", CEFRLevel: "B1",
		Structure: materialCourseStructure(resourceID),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	sub, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("submit draft: %v", err)
	}
	verified, err := svc.RunGate1Verification(ctx, sub.ID)
	if err != nil {
		t.Fatalf("run Gate 1: %v", err)
	}
	if verified.Status != domain.SubmissionStatusInReview {
		t.Fatalf("ready material should pass, got %s: %s",
			verified.Status, string(verified.VerificationReport))
	}
	// Twenty graded exercises, not the twenty-first material.
	if verifier.calls != 20 {
		t.Errorf("ItemVerifier calls = %d, want 20 graded exercises", verifier.calls)
	}
	if len(publisher.owners) != 1 || publisher.owners[0] != creatorID {
		t.Errorf("materials must be resolved through the draft owner, got owners %v", publisher.owners)
	}
}

// TestGate1_PaidCourseWaitsForAHuman guards the bypass the unpersisted
// gate2_required column used to open: a paid submission passed Gate 1 and the
// auto-publish path read the freshly-written submission's zero-valued flag, so
// it never reached a moderator.
func TestGate1_PaidCourseWaitsForAHuman(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	lessonAuthor := &mockLessonAuthor{}
	svc := service.NewService(repo, &mockVerifier{}, lessonAuthor, &mockContentAuthor{})
	creatorID := uuid.New()

	if _, err := repo.UpsertCreatorProfile(ctx, creatorID, "Trusted", "Author"); err != nil {
		t.Fatalf("profile: %v", err)
	}
	now := time.Now()
	repo.profiles[creatorID].TrustedAt = &now
	repo.profiles[creatorID].ApprovedCourseCount = 10

	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Paid But Trusted", Slug: "paid-but-trusted", Description: "d", CEFRLevel: "B1",
		Structure: createTestCourseStructure(1, 3, 7),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	price := int64(199_000)
	if _, err := svc.UpdateDraft(ctx, creatorID, draft.ID, service.UpdateDraftRequest{PriceVND: &price}); err != nil {
		t.Fatalf("price draft: %v", err)
	}

	sub, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	verified, err := svc.RunGate1Verification(ctx, sub.ID)
	if err != nil {
		t.Fatalf("run Gate 1: %v", err)
	}
	if !verified.Gate2Required {
		t.Fatal("Gate 1 dropped the persisted Gate 2 decision")
	}
	if verified.Status != domain.SubmissionStatusInReview {
		t.Fatalf("a paid submission must wait in review, got %s", verified.Status)
	}
	if lessonAuthor.coursesCreated != 0 {
		t.Error("a paid course must not be published by Gate 1 alone")
	}
}

// TestGate1_TrustedFreeCourseAutoPublishes is the other side of the trust
// model: the free course of a trusted creator still publishes without a human.
func TestGate1_TrustedFreeCourseAutoPublishes(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	lessonAuthor := &mockLessonAuthor{}
	svc := service.NewService(repo, &mockVerifier{}, lessonAuthor, &mockContentAuthor{})
	creatorID := uuid.New()

	if _, err := repo.UpsertCreatorProfile(ctx, creatorID, "Trusted", "Author"); err != nil {
		t.Fatalf("profile: %v", err)
	}
	now := time.Now()
	repo.profiles[creatorID].TrustedAt = &now
	repo.profiles[creatorID].ApprovedCourseCount = 10

	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Free And Trusted", Slug: "free-and-trusted", Description: "d", CEFRLevel: "B1",
		Structure: createTestCourseStructure(1, 3, 7),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	sub, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	verified, err := svc.RunGate1Verification(ctx, sub.ID)
	if err != nil {
		t.Fatalf("run Gate 1: %v", err)
	}
	if verified.Status != domain.SubmissionStatusApproved {
		t.Fatalf("a trusted creator's free course should publish on Gate 1, got %s", verified.Status)
	}
	if repo.drafts[draft.ID].Status != domain.DraftStatusPublished {
		t.Errorf("draft status = %s, want published", repo.drafts[draft.ID].Status)
	}
}

// TestLatestSubmission_SurvivesResubmission: a second submission after
// changes_requested is version 2, and the detail endpoint returns it.
func TestLatestSubmission_SurvivesResubmission(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	verifier := &mockVerifier{}
	svc := service.NewService(repo, verifier, nil, nil)
	creatorID := uuid.New()

	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Resubmit", Slug: "resubmit", Description: "d", CEFRLevel: "B1",
		Structure: createTestCourseStructure(1, 3, 7),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	first, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}

	// The first Gate 1 fails the draft back to changes_requested, then a
	// second submit is version 2.
	verifier.verifyFunc = func(_ context.Context, _ learningcontract.VerifyItemRequest) error {
		return errors.New("invalid answer key")
	}
	if _, err := svc.RunGate1Verification(ctx, first.ID); err != nil {
		t.Fatalf("first Gate 1: %v", err)
	}
	if repo.drafts[draft.ID].Status != domain.DraftStatusChangesRequested {
		t.Fatalf("draft status = %s, want changes_requested", repo.drafts[draft.ID].Status)
	}

	second, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("second submission version = %d, want 2", second.Version)
	}
	_, latest, err := svc.GetDraftDetail(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if latest == nil || latest.ID != second.ID || latest.Version != 2 {
		t.Fatalf("expected the version 2 submission, got %+v", latest)
	}
}
