package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/modules/studio/service"
)

// draftStructure is the smallest tree that passes the structural checks:
// three lessons and twenty activities, which is the floor WO 15 §7.2 sets.
func draftStructure(t *testing.T) json.RawMessage {
	t.Helper()
	lesson := func(title string) map[string]any {
		activities := make([]map[string]any, 0, 7)
		for i := 0; i < 7; i++ {
			activities = append(activities, map[string]any{
				"kind":   "grammar_tense_choice",
				"body":   json.RawMessage(`{"prompt":"x"}`),
				"config": json.RawMessage(`{"prompt":"x"}`),
				"weight": 1,
			})
		}
		return map[string]any{"title": title, "activities": activities}
	}
	raw, err := json.Marshal(map[string]any{
		"units": []map[string]any{{
			"title":   "Unit one",
			"lessons": []map[string]any{lesson("One"), lesson("Two"), lesson("Three")},
		}},
	})
	if err != nil {
		t.Fatalf("marshal structure: %v", err)
	}
	return raw
}

// newModerationFixture builds a service with a creator profile in place.
func newModerationFixture(ctx context.Context, t *testing.T) (*mockRepo, *service.Service, uuid.UUID) {
	t.Helper()
	repo := newMockRepo()
	svc := service.NewService(repo, nil, nil, nil)
	creatorID := uuid.New()
	if _, err := repo.UpsertCreatorProfile(ctx, creatorID, "", ""); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return repo, svc, creatorID
}

func submitDraft(
	ctx context.Context,
	t *testing.T,
	svc *service.Service,
	ownerID uuid.UUID,
	priceVND int64,
) *domain.Submission {
	t.Helper()
	draft, err := svc.CreateDraft(ctx, ownerID, service.CreateDraftRequest{
		Title:       "Course " + uuid.NewString()[:8],
		Slug:        "course-" + uuid.NewString()[:8],
		Description: "A course.",
		CEFRLevel:   "B1",
		Structure:   draftStructure(t),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if priceVND > 0 {
		if _, err := svc.UpdateDraft(ctx, ownerID, draft.ID, service.UpdateDraftRequest{
			PriceVND: &priceVND,
		}); err != nil {
			t.Fatalf("price draft: %v", err)
		}
	}
	sub, err := svc.SubmitDraft(ctx, ownerID, draft.ID)
	if err != nil {
		t.Fatalf("submit draft: %v", err)
	}
	return sub
}

// TestSubmission_WhoNeedsAHuman is the scaling argument the two-gate design
// rests on, which studio/AGENT.md described and the schema did not carry.
func TestSubmission_WhoNeedsAHuman(t *testing.T) {
	ctx := context.Background()

	t.Run("a new creator is always reviewed", func(t *testing.T) {
		_, svc, creatorID := newModerationFixture(ctx, t)
		sub := submitDraft(ctx, t, svc, creatorID, 0)
		if !sub.Gate2Required {
			t.Fatal("a creator with no approved courses should be reviewed")
		}
	})

	t.Run("a paid course is always reviewed, however trusted the creator", func(t *testing.T) {
		repo, svc, creatorID := newModerationFixture(ctx, t)
		now := time.Now()
		repo.profiles[creatorID].TrustedAt = &now
		repo.profiles[creatorID].ApprovedCourseCount = 10

		sub := submitDraft(ctx, t, svc, creatorID, 199_000)
		if !sub.Gate2Required {
			t.Fatal("money means a person reads it")
		}
		if sub.Gate2Reason == nil {
			t.Fatal("the queue should say why this needs a human")
		}
	})

	t.Run("a trusted creator's free course does not wait for a human", func(t *testing.T) {
		repo, svc, creatorID := newModerationFixture(ctx, t)
		now := time.Now()
		repo.profiles[creatorID].TrustedAt = &now
		repo.profiles[creatorID].ApprovedCourseCount = 3

		sub := submitDraft(ctx, t, svc, creatorID, 0)
		if sub.Gate2Required {
			t.Fatal("a trusted creator's free course should publish on Gate 1 alone")
		}
	})

	t.Run("an upheld report puts a creator back in front of a human", func(t *testing.T) {
		repo, svc, creatorID := newModerationFixture(ctx, t)
		now := time.Now()
		repo.profiles[creatorID].TrustedAt = &now
		repo.profiles[creatorID].ApprovedCourseCount = 5

		if _, err := repo.RecordUpheldReport(ctx, creatorID); err != nil {
			t.Fatalf("record report: %v", err)
		}
		sub := submitDraft(ctx, t, svc, creatorID, 0)
		if !sub.Gate2Required {
			t.Fatal("a creator with an upheld report should be reviewed again")
		}
	})
}

// TestTrust_IsEarnedAtTheThirdApproval.
func TestTrust_IsEarnedAtTheThirdApproval(t *testing.T) {
	ctx := context.Background()
	repo, _, creatorID := newModerationFixture(ctx, t)

	for i := 1; i <= domain.TrustThreshold; i++ {
		profile, err := repo.RecordApprovedCourse(ctx, creatorID)
		if err != nil {
			t.Fatalf("record approval %d: %v", i, err)
		}
		want := i >= domain.TrustThreshold
		if profile.Trusted() != want {
			t.Errorf("after %d approvals trusted = %v, want %v", i, profile.Trusted(), want)
		}
	}
}

// TestSuspendedCreator_CannotSubmit. A suspension that still lets somebody
// submit is a suspension in name only.
func TestSuspendedCreator_CannotSubmit(t *testing.T) {
	ctx := context.Background()
	repo, svc, creatorID := newModerationFixture(ctx, t)

	if _, err := svc.SuspendCreator(ctx, creatorID, "repeated plagiarism"); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Another", Slug: "another-course", CEFRLevel: "B1", Structure: draftStructure(t),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	_, err = svc.SubmitDraft(ctx, creatorID, draft.ID)
	if !errors.Is(err, domain.ErrCreatorSuspended) {
		t.Fatalf("expected ErrCreatorSuspended, got %v", err)
	}

	if _, err := svc.ReinstateCreator(ctx, creatorID); err != nil {
		t.Fatalf("reinstate: %v", err)
	}
	if repo.profiles[creatorID].Trusted() {
		t.Error("reinstating a creator should not restore trust; they go back through review")
	}
}

// TestTakedown_KeepsWhatLearnersBought is BR-STUDIO-04: a takedown removes a
// course from sale. It is not a refund, and revoking what somebody paid for
// because somebody else complained is a different decision.
func TestTakedown_KeepsWhatLearnersBought(t *testing.T) {
	ctx := context.Background()
	repo, svc, creatorID := newModerationFixture(ctx, t)
	courseID := uuid.New()
	learnerID := uuid.New()

	if _, err := repo.UpsertListing(ctx, &domain.Listing{
		CourseID: courseID, CreatorID: creatorID,
		PricingModel: domain.PricingModelOneTime, PriceVND: 199_000,
		RevenueShareBPS: 7000, Status: domain.ListingStatusActive,
	}); err != nil {
		t.Fatalf("listing: %v", err)
	}
	if _, err := repo.CreatePurchase(ctx, &domain.Purchase{
		UserID: learnerID, CourseID: courseID, PricePaidVND: 199_000,
	}); err != nil {
		t.Fatalf("purchase: %v", err)
	}

	if _, err := svc.TakedownCourse(ctx, uuid.New(), courseID, "plagiarised from a textbook"); err != nil {
		t.Fatalf("takedown: %v", err)
	}

	if got := repo.listings[courseID].Status; got != domain.ListingStatusTakenDown {
		t.Errorf("listing status = %q, want taken_down", got)
	}
	assertMayOpen(ctx, t, svc, "a learner who bought it keeps it", &learnerID, courseID, true)

	stranger := uuid.New()
	assertMayOpen(ctx, t, svc, "nobody else can open it", &stranger, courseID, false)

	// And it counts against the creator, who loses trust.
	if repo.profiles[creatorID].UpheldReportCount != 1 {
		t.Errorf("a takedown should count against its creator, got %d",
			repo.profiles[creatorID].UpheldReportCount)
	}
}

// TestTakedown_RequiresAReason. A takedown nobody explained is one nobody can
// review or undo fairly.
func TestTakedown_RequiresAReason(t *testing.T) {
	ctx := context.Background()
	repo, svc, creatorID := newModerationFixture(ctx, t)
	courseID := uuid.New()
	if _, err := repo.UpsertListing(ctx, &domain.Listing{
		CourseID: courseID, CreatorID: creatorID,
		PricingModel: domain.PricingModelFree, Status: domain.ListingStatusActive,
	}); err != nil {
		t.Fatalf("listing: %v", err)
	}

	_, err := svc.TakedownCourse(ctx, uuid.New(), courseID, "   ")
	if !errors.Is(err, domain.ErrReasonRequired) {
		t.Fatalf("expected ErrReasonRequired, got %v", err)
	}

	_, err = svc.SuspendCreator(ctx, creatorID, "")
	if !errors.Is(err, domain.ErrReasonRequired) {
		t.Fatalf("expected ErrReasonRequired on suspend, got %v", err)
	}
}

// TestReinstate_PutsACourseBackOnSale.
func TestReinstate_PutsACourseBackOnSale(t *testing.T) {
	ctx := context.Background()
	repo, svc, creatorID := newModerationFixture(ctx, t)
	courseID := uuid.New()
	if _, err := repo.UpsertListing(ctx, &domain.Listing{
		CourseID: courseID, CreatorID: creatorID,
		PricingModel: domain.PricingModelFree, Status: domain.ListingStatusActive,
	}); err != nil {
		t.Fatalf("listing: %v", err)
	}

	moderator := uuid.New()
	if _, err := svc.TakedownCourse(ctx, moderator, courseID, "reported, and the report was upheld"); err != nil {
		t.Fatalf("takedown: %v", err)
	}
	reinstated, err := svc.ReinstateCourse(ctx, moderator, courseID)
	if err != nil {
		t.Fatalf("reinstate: %v", err)
	}
	if reinstated.ReinstatedAt == nil {
		t.Error("the takedown should record when it was lifted")
	}
	if got := repo.listings[courseID].Status; got != domain.ListingStatusActive {
		t.Errorf("listing status = %q, want active", got)
	}

	if _, err := svc.ReinstateCourse(ctx, moderator, courseID); !errors.Is(err, domain.ErrTakedownNotFound) {
		t.Errorf("reinstating twice should report no open takedown, got %v", err)
	}
}
