package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	paymentcontract "github.com/fluentra/fluentra/internal/modules/payment/contract"
	resourcecontract "github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/modules/studio/repository"
	"github.com/fluentra/fluentra/internal/modules/studio/service"
)

type mockRepo struct {
	repository.Repository
	profiles       map[uuid.UUID]*domain.CreatorProfile
	payoutAccounts map[uuid.UUID]*domain.PayoutAccount
	drafts         map[uuid.UUID]*domain.CourseDraft
	submissions    map[uuid.UUID]*domain.Submission
	listings       map[uuid.UUID]*domain.Listing
	purchases      map[uuid.UUID]*domain.Purchase
	ledger         []*domain.CreatorLedgerEntry
	takedowns      []*domain.Takedown
	// submitErr stands in for a lookup or write failure inside the submission
	// transaction, and latestErr for a failed latest-submission read.
	submitErr error
	latestErr error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		profiles:       make(map[uuid.UUID]*domain.CreatorProfile),
		payoutAccounts: make(map[uuid.UUID]*domain.PayoutAccount),
		drafts:         make(map[uuid.UUID]*domain.CourseDraft),
		submissions:    make(map[uuid.UUID]*domain.Submission),
		listings:       make(map[uuid.UUID]*domain.Listing),
		purchases:      make(map[uuid.UUID]*domain.Purchase),
	}
}

func (m *mockRepo) GetCreatorProfile(_ context.Context, userID uuid.UUID) (*domain.CreatorProfile, error) {
	if p, ok := m.profiles[userID]; ok {
		return p, nil
	}
	return nil, domain.ErrProfileNotFound
}

func (m *mockRepo) UpsertCreatorProfile(
	_ context.Context, userID uuid.UUID, bio, headline string,
) (*domain.CreatorProfile, error) {
	p := &domain.CreatorProfile{
		UserID:    userID,
		Bio:       bio,
		Headline:  headline,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.profiles[userID] = p
	return p, nil
}

func (m *mockRepo) GetPayoutAccount(_ context.Context, creatorID uuid.UUID) (*domain.PayoutAccount, error) {
	// The read is the default account, as the query and GET endpoint are: an
	// account explicitly saved as non-default is not the payout destination.
	account := m.payoutAccounts[creatorID]
	if account == nil || !account.IsDefault {
		return nil, nil
	}
	return account, nil
}

func (
	m *mockRepo) UpsertPayoutAccount(_ context.Context,
	creatorID uuid.UUID,
	bankCode,
	accountNumber,
	accountHolderName string,
	isDefault bool) (*domain.PayoutAccount,
	error,
) {
	// One account per creator: a repeated save updates the row it already has
	// rather than appending a second one.
	id := uuid.New()
	createdAt := time.Now()
	if existing, ok := m.payoutAccounts[creatorID]; ok {
		id = existing.ID
		createdAt = existing.CreatedAt
	}
	acc := &domain.PayoutAccount{
		ID:                id,
		CreatorID:         creatorID,
		BankCode:          bankCode,
		AccountNumber:     accountNumber,
		AccountHolderName: accountHolderName,
		IsDefault:         isDefault,
		CreatedAt:         createdAt,
		UpdatedAt:         time.Now(),
	}
	m.payoutAccounts[creatorID] = acc
	return acc, nil
}

func (m *mockRepo) CreateCourseDraft(_ context.Context, draft *domain.CourseDraft) (*domain.CourseDraft, error) {
	draft.ID = uuid.New()
	draft.CreatedAt = time.Now()
	draft.UpdatedAt = time.Now()
	m.drafts[draft.ID] = draft
	return draft, nil
}

func (m *mockRepo) GetCourseDraftByID(_ context.Context, id uuid.UUID) (*domain.CourseDraft, error) {
	if d, ok := m.drafts[id]; ok {
		return d, nil
	}
	return nil, domain.ErrDraftNotFound
}

func (
	m *mockRepo,
) ListCourseDraftsByOwner(_ context.Context, ownerID uuid.UUID, _, _ int) ([]*domain.CourseDraft, int64, error) {
	var list []*domain.CourseDraft
	for _, d := range m.drafts {
		if d.OwnerID == ownerID {
			list = append(list, d)
		}
	}
	return list, int64(len(list)), nil
}

func (m *mockRepo) UpdateCourseDraft(_ context.Context, draft *domain.CourseDraft) (*domain.CourseDraft, error) {
	m.drafts[draft.ID] = draft
	return draft, nil
}

func (
	m *mockRepo) UpdateCourseDraftStatus(_ context.Context,
	id uuid.UUID,
	status string) (*domain.CourseDraft,
	error,
) {
	if d, ok := m.drafts[id]; ok {
		d.Status = status
		return d, nil
	}
	return nil, domain.ErrDraftNotFound
}

// SubmitDraft mirrors the repository's transactional submission: it re-checks
// the draft under the caller's owner, allocates the next version and moves the
// draft to submitted as one step.
func (m *mockRepo) SubmitDraft(
	_ context.Context, ownerID uuid.UUID, sub *domain.Submission,
) (*domain.Submission, error) {
	if m.submitErr != nil {
		return nil, m.submitErr
	}
	draft, ok := m.drafts[sub.DraftID]
	if !ok {
		return nil, domain.ErrDraftNotFound
	}
	if draft.OwnerID != ownerID {
		return nil, domain.ErrDraftNotFound
	}
	if draft.Status != domain.DraftStatusDraft && draft.Status != domain.DraftStatusChangesRequested {
		return nil, domain.ErrCannotSubmit
	}
	version := 1
	for _, s := range m.submissions {
		if s.DraftID == sub.DraftID && s.Version >= version {
			version = s.Version + 1
		}
	}
	sub.ID = uuid.New()
	sub.Version = version
	sub.Status = domain.SubmissionStatusSubmitted
	sub.SubmittedAt = time.Now()
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()
	draft.Status = domain.DraftStatusSubmitted
	m.submissions[sub.ID] = sub
	return sub, nil
}

func (m *mockRepo) GetSubmissionByID(_ context.Context, id uuid.UUID) (*domain.Submission, error) {
	if s, ok := m.submissions[id]; ok {
		return s, nil
	}
	return nil, domain.ErrSubmissionNotFound
}

func (m *mockRepo) GetLatestSubmissionByDraftID(_ context.Context, draftID uuid.UUID) (*domain.Submission, error) {
	if m.latestErr != nil {
		return nil, m.latestErr
	}
	var latest *domain.Submission
	for _, s := range m.submissions {
		if s.DraftID == draftID {
			if latest == nil || s.Version > latest.Version {
				latest = s
			}
		}
	}
	return latest, nil
}

func (
	m *mockRepo,
) ListSubmissionsByStatus(_ context.Context, status string, _, _ int) ([]*domain.Submission, int64, error) {
	var list []*domain.Submission
	for _, s := range m.submissions {
		if s.Status == status {
			list = append(list, s)
		}
	}
	return list, int64(len(list)), nil
}

func (
	m *mockRepo) UpdateSubmissionVerification(_ context.Context,
	id uuid.UUID,
	status string,
	report []byte,
	feedback *string) (*domain.Submission,
	error,
) {
	if s, ok := m.submissions[id]; ok {
		s.Status = status
		s.VerificationReport = report
		s.Feedback = feedback
		return s, nil
	}
	return nil, domain.ErrSubmissionNotFound
}

func (
	m *mockRepo) UpdateSubmissionReview(_ context.Context,
	id uuid.UUID,
	status string,
	reviewerID uuid.UUID,
	feedback *string) (*domain.Submission,
	error,
) {
	if s, ok := m.submissions[id]; ok {
		s.Status = status
		s.ReviewerID = &reviewerID
		s.Feedback = feedback
		now := time.Now()
		s.ReviewedAt = &now
		return s, nil
	}
	return nil, domain.ErrSubmissionNotFound
}

func (m *mockRepo) UpsertListing(_ context.Context, l *domain.Listing) (*domain.Listing, error) {
	l.PublishedAt = time.Now()
	m.listings[l.CourseID] = l
	return l, nil
}

func (m *mockRepo) GetListingByCourseID(_ context.Context, courseID uuid.UUID) (*domain.Listing, error) {
	if l, ok := m.listings[courseID]; ok {
		return l, nil
	}
	return nil, domain.ErrListingNotFound
}

func (m *mockRepo) BatchGetListings(_ context.Context, courseIDs []uuid.UUID) (map[uuid.UUID]*domain.Listing, error) {
	out := make(map[uuid.UUID]*domain.Listing)
	for _, id := range courseIDs {
		if l, ok := m.listings[id]; ok {
			out[id] = l
		}
	}
	return out, nil
}

func (m *mockRepo) CreatePurchase(_ context.Context, p *domain.Purchase) (*domain.Purchase, error) {
	p.ID = uuid.New()
	p.GrantedAt = time.Now()
	m.purchases[p.ID] = p
	return p, nil
}

func (m *mockRepo) GetPurchaseByID(_ context.Context, id uuid.UUID) (*domain.Purchase, error) {
	if p, ok := m.purchases[id]; ok {
		return p, nil
	}
	return nil, domain.ErrPurchaseNotFound
}

func (m *mockRepo) GetActivePurchase(_ context.Context, userID, courseID uuid.UUID) (*domain.Purchase, error) {
	for _, p := range m.purchases {
		if p.UserID == userID && p.CourseID == courseID && p.RevokedAt == nil {
			return p, nil
		}
	}
	return nil, nil
}

func (
	m *mockRepo,
) ListPurchasesByUserID(_ context.Context, userID uuid.UUID, _, _ int) ([]*domain.Purchase, int64, error) {
	var list []*domain.Purchase
	for _, p := range m.purchases {
		if p.UserID == userID {
			list = append(list, p)
		}
	}
	return list, int64(len(list)), nil
}

func (m *mockRepo) RevokePurchase(_ context.Context, id uuid.UUID, reason string) (*domain.Purchase, error) {
	if p, ok := m.purchases[id]; ok {
		now := time.Now()
		p.RevokedAt = &now
		p.RevokeReason = &reason
		return p, nil
	}
	return nil, domain.ErrPurchaseNotFound
}

func (
	m *mockRepo) CreateLedgerEntry(_ context.Context,
	e *domain.CreatorLedgerEntry) (*domain.CreatorLedgerEntry,
	error,
) {
	e.ID = uuid.New()
	e.CreatedAt = time.Now()
	m.ledger = append(m.ledger, e)
	return e, nil
}

// ---------------------------------------- creator trust and moderation

func (m *mockRepo) RecordApprovedCourse(
	_ context.Context, creatorID uuid.UUID,
) (*domain.CreatorProfile, error) {
	p := m.profiles[creatorID]
	if p == nil {
		p = &domain.CreatorProfile{UserID: creatorID}
		m.profiles[creatorID] = p
	}
	p.ApprovedCourseCount++
	if p.TrustedAt == nil && p.ApprovedCourseCount >= domain.TrustThreshold && p.UpheldReportCount == 0 {
		now := time.Now()
		p.TrustedAt = &now
	}
	return p, nil
}

func (m *mockRepo) RecordUpheldReport(
	_ context.Context, creatorID uuid.UUID,
) (*domain.CreatorProfile, error) {
	p := m.profiles[creatorID]
	if p == nil {
		p = &domain.CreatorProfile{UserID: creatorID}
		m.profiles[creatorID] = p
	}
	p.UpheldReportCount++
	p.TrustedAt = nil
	return p, nil
}

func (m *mockRepo) SuspendCreator(
	_ context.Context, creatorID uuid.UUID, reason string,
) (*domain.CreatorProfile, error) {
	p := m.profiles[creatorID]
	if p == nil {
		return nil, domain.ErrProfileNotFound
	}
	now := time.Now()
	p.SuspendedAt = &now
	p.SuspendedReason = &reason
	p.TrustedAt = nil
	return p, nil
}

func (m *mockRepo) ReinstateCreator(
	_ context.Context, creatorID uuid.UUID,
) (*domain.CreatorProfile, error) {
	p := m.profiles[creatorID]
	if p == nil {
		return nil, domain.ErrProfileNotFound
	}
	p.SuspendedAt = nil
	p.SuspendedReason = nil
	return p, nil
}

func (m *mockRepo) CreateTakedown(
	_ context.Context, courseID, actorID uuid.UUID, reason string,
) (*domain.Takedown, error) {
	t := &domain.Takedown{
		ID: uuid.New(), CourseID: courseID, ActorID: actorID,
		Reason: reason, CreatedAt: time.Now(),
	}
	m.takedowns = append(m.takedowns, t)
	return t, nil
}

func (m *mockRepo) GetOpenTakedown(_ context.Context, courseID uuid.UUID) (*domain.Takedown, error) {
	for i := len(m.takedowns) - 1; i >= 0; i-- {
		if m.takedowns[i].CourseID == courseID && m.takedowns[i].ReinstatedAt == nil {
			return m.takedowns[i], nil
		}
	}
	return nil, domain.ErrTakedownNotFound
}

func (m *mockRepo) ReinstateTakedown(_ context.Context, id, actorID uuid.UUID) (*domain.Takedown, error) {
	for _, t := range m.takedowns {
		if t.ID == id && t.ReinstatedAt == nil {
			now := time.Now()
			t.ReinstatedAt = &now
			t.ReinstatedBy = &actorID
			return t, nil
		}
	}
	return nil, domain.ErrTakedownNotFound
}

func (m *mockRepo) SetListingStatus(
	_ context.Context, courseID uuid.UUID, status string,
) (*domain.Listing, error) {
	l := m.listings[courseID]
	if l == nil {
		return nil, domain.ErrListingNotFound
	}
	l.Status = status
	return l, nil
}

func (m *mockRepo) GetSaleLedgerEntryByPurchaseID(
	_ context.Context, purchaseID uuid.UUID,
) (*domain.CreatorLedgerEntry, error) {
	for _, e := range m.ledger {
		if e.Kind == domain.LedgerKindSale && e.PurchaseID != nil && *e.PurchaseID == purchaseID {
			return e, nil
		}
	}
	return nil, domain.ErrLedgerEntryNotFound
}

func (
	m *mockRepo,
) ListLedgerEntriesByCreatorID(_ context.Context, creatorID uuid.UUID, _, _ int) ([]*domain.CreatorLedgerEntry, error) {
	var list []*domain.CreatorLedgerEntry
	for _, e := range m.ledger {
		if e.CreatorID == creatorID {
			list = append(list, e)
		}
	}
	return list, nil
}

func (m *mockRepo) GetCreatorBalance(_ context.Context, creatorID uuid.UUID) (int64, error) {
	var bal int64
	for _, e := range m.ledger {
		matchKind := e.Kind == domain.LedgerKindSale ||
			e.Kind == domain.LedgerKindRefund ||
			e.Kind == domain.LedgerKindPayout ||
			e.Kind == domain.LedgerKindAdjustment
		if e.CreatorID == creatorID && matchKind {
			bal += e.AmountVND
		}
	}
	return bal, nil
}

func (m *mockRepo) GetCreatorLifetimeEarnings(_ context.Context, creatorID uuid.UUID) (int64, error) {
	var sum int64
	for _, e := range m.ledger {
		if e.CreatorID == creatorID && e.Kind == domain.LedgerKindSale {
			sum += e.AmountVND
		}
	}
	return sum, nil
}

func (m *mockRepo) GetCreatorTotalPaidOut(_ context.Context, creatorID uuid.UUID) (int64, error) {
	var sum int64
	for _, e := range m.ledger {
		if e.CreatorID == creatorID && e.Kind == domain.LedgerKindPayout {
			if e.AmountVND < 0 {
				sum += -e.AmountVND
			} else {
				sum += e.AmountVND
			}
		}
	}
	return sum, nil
}

type mockVerifier struct {
	verifyFunc func(_ context.Context, req learningcontract.VerifyItemRequest) error
	// calls is how many times VerifyItem ran, which is how a test proves the
	// submit request did not touch a model.
	calls int
}

func (v *mockVerifier) VerifyItem(ctx context.Context, req learningcontract.VerifyItemRequest) error {
	v.calls++
	if v.verifyFunc != nil {
		return v.verifyFunc(ctx, req)
	}
	return nil
}

type mockLessonAuthor struct {
	lessoncontract.Author
	coursesCreated int
	courseID       uuid.UUID
	// visibilities records how the course was ensured on each call, in order.
	visibilities []string
}

func (a *mockLessonAuthor) EnsureCourse(_ context.Context, spec lessoncontract.CourseSpec) (uuid.UUID, error) {
	a.coursesCreated++
	a.visibilities = append(a.visibilities, spec.Visibility)
	if a.courseID == uuid.Nil {
		a.courseID = uuid.New()
	}
	return a.courseID, nil
}

func (a *mockLessonAuthor) EnsureUnit(_ context.Context, _ lessoncontract.UnitSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (a *mockLessonAuthor) EnsureLesson(_ context.Context, _ lessoncontract.LessonSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (
	a *mockLessonAuthor,
) SyncActivities(_ context.Context, _ uuid.UUID, _ []lessoncontract.ActivitySpec) error {
	return nil
}

func (
	a *mockLessonAuthor,
) AppendActivity(_ context.Context, _ uuid.UUID, _ lessoncontract.ActivitySpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

type mockContentAuthor struct {
	contentcontract.Author
}

func (a *mockContentAuthor) EnsurePublished(_ context.Context, _ contentcontract.AuthorSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func createTestCourseStructure(unitsCount, lessonsPerUnit, activitiesPerLesson int) []byte {
	var units []domain.UnitDraft
	for u := 1; u <= unitsCount; u++ {
		var lessons []domain.LessonDraft
		for l := 1; l <= lessonsPerUnit; l++ {
			var activities []domain.ActivityDraft
			for a := 1; a <= activitiesPerLesson; a++ {
				activities = append(activities, domain.ActivityDraft{
					Kind:   "vocab_multiple_choice",
					Weight: 10,
					Body:   json.RawMessage(`{"prompt":"Select word","answer":"test"}`),
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

func TestCreatorProfileAndPayoutAccount(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, nil, nil, nil)
	userID := uuid.New()

	profile, err := svc.UpsertCreatorProfile(ctx, userID, "English Teacher", "IELTS Expert")
	if err != nil {
		t.Fatalf("upsert profile error: %v", err)
	}
	if profile.Bio != "English Teacher" || profile.Headline != "IELTS Expert" {
		t.Errorf("unexpected profile data: %+v", profile)
	}

	payout, err := svc.UpsertPayoutAccount(ctx, userID, "MB", "0123456789", "NGUYEN VAN A", true)
	if err != nil {
		t.Fatalf("upsert payout error: %v", err)
	}
	if payout.BankCode != "MB" || payout.AccountNumber != "0123456789" {
		t.Errorf("unexpected payout data: %+v", payout)
	}
}

func TestDraftCreationAndGate1Verification(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	verifier := &mockVerifier{}
	svc := service.NewService(repo, verifier, nil, nil)
	creatorID := uuid.New()

	// 1. Create Draft
	structRaw := createTestCourseStructure(1, 3, 7) // 21 activities
	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title:       "IELTS Mastery",
		Slug:        "ielts-mastery",
		Description: "Comprehensive course",
		CEFRLevel:   "B2",
		Structure:   structRaw,
	})
	if err != nil {
		t.Fatalf("create draft error: %v", err)
	}
	if draft.Status != domain.DraftStatusDraft {
		t.Fatalf("expected status draft, got %s", draft.Status)
	}

	// 2. Submit records the submission and returns; it must not run a
	// verifier or call a model inside the request.
	sub, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("submit draft error: %v", err)
	}
	if sub.Status != domain.SubmissionStatusSubmitted {
		t.Fatalf("expected submission status submitted, got %s", sub.Status)
	}
	if verifier.calls != 0 {
		t.Fatalf("submit must not call ItemVerifier, calls = %d", verifier.calls)
	}
	if repo.drafts[draft.ID].Status != domain.DraftStatusSubmitted {
		t.Fatalf("draft status = %s, want submitted", repo.drafts[draft.ID].Status)
	}

	// 3. Gate 1 run explicitly passes and moves the submission to review.
	verified, err := svc.RunGate1Verification(ctx, sub.ID)
	if err != nil {
		t.Fatalf("run Gate 1: %v", err)
	}
	if verified.Status != domain.SubmissionStatusInReview {
		t.Fatalf("expected submission status in_review, got %s", verified.Status)
	}
	if verifier.calls == 0 {
		t.Fatal("Gate 1 should have called ItemVerifier")
	}

	// A second submit while the first is live is a conflict, not a second
	// active submission.
	if _, err := svc.SubmitDraft(ctx, creatorID, draft.ID); !errors.Is(err, domain.ErrCannotSubmit) {
		t.Fatalf("expected ErrCannotSubmit on repeated submit, got %v", err)
	}

}

func TestGate1VerificationRejectsInvalidAnswerKey(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	verifier := &mockVerifier{}
	svc := service.NewService(repo, verifier, nil, nil)
	creatorID := uuid.New()
	structRaw := createTestCourseStructure(1, 3, 7)

	// Verification failure when verifier returns error.
	verifier.verifyFunc = func(_ context.Context, _ learningcontract.VerifyItemRequest) error {
		return errors.New("invalid answer key")
	}

	// Create a new draft that will fail verification
	draftFail, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title:       "Failing Course",
		Slug:        "failing-course",
		Description: "Course with bad keys",
		CEFRLevel:   "B1",
		Structure:   structRaw,
	})
	if err != nil {
		t.Fatalf("create draft error: %v", err)
	}

	subFail, err := svc.SubmitDraft(ctx, creatorID, draftFail.ID)
	if err != nil {
		t.Fatalf("submit draft error: %v", err)
	}
	if subFail.Status != domain.SubmissionStatusSubmitted {
		t.Fatalf("expected submitted before Gate 1, got %s", subFail.Status)
	}

	verifiedFail, err := svc.RunGate1Verification(ctx, subFail.ID)
	if err != nil {
		t.Fatalf("run Gate 1: %v", err)
	}
	if verifiedFail.Status != domain.SubmissionStatusChangesRequested {
		t.Fatalf("expected changes_requested on verification failure, got %s", verifiedFail.Status)
	}
}

func TestGate2Moderation_BR_STUDIO_06_SelfReviewForbidden(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	verifier := &mockVerifier{}
	lessonAuthor := &mockLessonAuthor{}
	contentAuthor := &mockContentAuthor{}
	svc := service.NewService(repo, verifier, lessonAuthor, contentAuthor)

	creatorID := uuid.New()
	structRaw := createTestCourseStructure(1, 3, 7)
	draft, _ := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title:       "Business English",
		Slug:        "business-english",
		Description: "For work",
		CEFRLevel:   "B2",
		Structure:   structRaw,
	})
	sub, _ := svc.SubmitDraft(ctx, creatorID, draft.ID)
	// Gate 1 runs out of band now; the moderation decision needs the
	// submission to have passed it.
	if _, err := svc.RunGate1Verification(ctx, sub.ID); err != nil {
		t.Fatalf("run Gate 1: %v", err)
	}

	// Attempt approve with reviewer == creator
	_, errApprove := svc.ApproveSubmission(ctx, creatorID, sub.ID)
	if !errors.Is(errApprove, domain.ErrSelfReviewForbidden) {
		t.Fatalf("expected ErrSelfReviewForbidden on approve, got: %v", errApprove)
	}

	// Attempt reject with reviewer == creator
	_, errReject := svc.RejectSubmission(ctx, creatorID, sub.ID, "rejected", "Some feedback")
	if !errors.Is(errReject, domain.ErrSelfReviewForbidden) {
		t.Fatalf("expected ErrSelfReviewForbidden on reject, got: %v", errReject)
	}

	// Moderator approval with distinct reviewer
	moderatorID := uuid.New()
	approvedSub, err := svc.ApproveSubmission(ctx, moderatorID, sub.ID)
	if err != nil {
		t.Fatalf("approve error: %v", err)
	}
	if approvedSub.Status != domain.SubmissionStatusApproved {
		t.Errorf("expected approved status, got %s", approvedSub.Status)
	}
	// The course is built unlisted and made public only once its listing
	// exists, so a failure to price it cannot leave a paid course free.
	want := []string{"unlisted", "public"}
	if len(lessonAuthor.visibilities) != len(want) {
		t.Fatalf("expected the course to be ensured %d times, got %d (%v)",
			len(want), len(lessonAuthor.visibilities), lessonAuthor.visibilities)
	}
	for i, v := range want {
		if lessonAuthor.visibilities[i] != v {
			t.Errorf("ensure %d had visibility %q, want %q", i+1, lessonAuthor.visibilities[i], v)
		}
	}
}

const testPayoutPending = "pending"

type mockPayoutManager struct {
	paymentcontract.PayoutManager
	pendingTotal int64
	created      []paymentcontract.CreatePayoutInput
}

func (m *mockPayoutManager) GetPendingPayoutTotal(_ context.Context, _ uuid.UUID) (int64, error) {
	return m.pendingTotal, nil
}

func (m *mockPayoutManager) CreatePayout(
	_ context.Context, in paymentcontract.CreatePayoutInput,
) (*paymentcontract.Payout, error) {
	m.created = append(m.created, in)
	return &paymentcontract.Payout{
		ID:        uuid.New(),
		CreatorID: in.CreatorID,
		AmountVND: in.AmountVND,
		Status:    testPayoutPending,
		ActorID:   in.ActorID,
		CreatedAt: time.Now(),
	}, nil
}

func TestEarnings_InitialStateAndSales(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, nil, nil, nil)
	payoutMgr := &mockPayoutManager{}
	svc.SetPayoutManager(payoutMgr)
	creatorID := uuid.New()

	earnings, err := svc.GetEarnings(ctx, creatorID)
	if err != nil {
		t.Fatalf("get earnings error: %v", err)
	}
	if earnings.AvailableBalanceVND != 0 || earnings.CanRequestPayout {
		t.Errorf("expected 0 balance and can_request_payout = false, got %+v", earnings)
	}

	_, _ = repo.CreateLedgerEntry(ctx, &domain.CreatorLedgerEntry{
		CreatorID:      creatorID,
		Kind:           domain.LedgerKindSale,
		AmountVND:      1000000,
		GrossAmountVND: 1428571,
		FeeAmountVND:   428571,
		Note:           "Sale 1",
	})

	earnings, err = svc.GetEarnings(ctx, creatorID)
	if err != nil {
		t.Fatalf("get earnings error: %v", err)
	}
	if earnings.AvailableBalanceVND != 1000000 || earnings.LifetimeEarningsVND != 1000000 {
		t.Errorf("expected 1,000,000 balance and lifetime, got %+v", earnings)
	}
	if earnings.CanRequestPayout || earnings.PayoutAccountConfigured {
		t.Errorf("cannot request payout without bank account configured")
	}

	_, err = svc.RequestPayout(ctx, creatorID, nil)
	if !errors.Is(err, domain.ErrPayoutAccountRequired) {
		t.Fatalf("expected ErrPayoutAccountRequired, got %v", err)
	}
}

func TestEarnings_ConfigureAccountAndValidateAmounts(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, nil, nil, nil)
	payoutMgr := &mockPayoutManager{}
	svc.SetPayoutManager(payoutMgr)
	creatorID := uuid.New()

	_, _ = repo.CreateLedgerEntry(ctx, &domain.CreatorLedgerEntry{
		CreatorID: creatorID,
		Kind:      domain.LedgerKindSale,
		AmountVND: 1000000,
	})

	_, err := svc.UpsertPayoutAccount(ctx, creatorID, "VCB", "1017588888", "NGUYEN VAN A", true)
	if err != nil {
		t.Fatalf("upsert payout account error: %v", err)
	}

	earnings, err := svc.GetEarnings(ctx, creatorID)
	if err != nil {
		t.Fatalf("get earnings error: %v", err)
	}
	if !earnings.CanRequestPayout || !earnings.PayoutAccountConfigured {
		t.Errorf("expected can_request_payout = true after account configured")
	}
	if earnings.PayoutMaskedAccount == nil || *earnings.PayoutMaskedAccount != "******8888" {
		t.Errorf("expected masked account ******8888, got %v", earnings.PayoutMaskedAccount)
	}

	smallAmt := int64(300000)
	_, err = svc.RequestPayout(ctx, creatorID, &smallAmt)
	if !errors.Is(err, domain.ErrPayoutBelowMinimum) {
		t.Fatalf("expected ErrPayoutBelowMinimum, got %v", err)
	}

	largeAmt := int64(1500000)
	_, err = svc.RequestPayout(ctx, creatorID, &largeAmt)
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestEarnings_RequestPayoutAndFulfill(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, nil, nil, nil)
	payoutMgr := &mockPayoutManager{}
	svc.SetPayoutManager(payoutMgr)
	creatorID := uuid.New()

	_, _ = repo.CreateLedgerEntry(ctx, &domain.CreatorLedgerEntry{
		CreatorID: creatorID,
		Kind:      domain.LedgerKindSale,
		AmountVND: 1000000,
	})
	_, _ = svc.UpsertPayoutAccount(ctx, creatorID, "VCB", "1017588888", "NGUYEN VAN A", true)

	validAmt := int64(700000)
	payout, err := svc.RequestPayout(ctx, creatorID, &validAmt)
	if err != nil {
		t.Fatalf("request payout error: %v", err)
	}
	if payout.AmountVND != 700000 || payout.Status != testPayoutPending {
		t.Errorf("unexpected payout created: %+v", payout)
	}
	if len(payoutMgr.created) != 1 {
		t.Fatalf("expected 1 payout created via payout manager")
	}

	payoutMgr.pendingTotal = 700000
	earnings, err := svc.GetEarnings(ctx, creatorID)
	if err != nil {
		t.Fatalf("get earnings error: %v", err)
	}
	if earnings.AvailableBalanceVND != 300000 {
		t.Errorf("expected 300,000 available balance, got %d", earnings.AvailableBalanceVND)
	}
	if earnings.CanRequestPayout {
		t.Errorf("expected can_request_payout = false when available < threshold")
	}

	err = svc.HandlePayoutSent(ctx, paymentcontract.EventPayoutSent{
		PayoutID:      payout.ID,
		CreatorID:     creatorID,
		AmountVND:     700000,
		BankReference: "VCB-REF-123456",
	})
	if err != nil {
		t.Fatalf("handle payout sent error: %v", err)
	}

	payoutMgr.pendingTotal = 0
	earnings, err = svc.GetEarnings(ctx, creatorID)
	if err != nil {
		t.Fatalf("get earnings error: %v", err)
	}
	if earnings.AvailableBalanceVND != 300000 {
		t.Errorf("expected 300,000 available balance after fulfillment, got %d", earnings.AvailableBalanceVND)
	}
	if earnings.TotalPaidOutVND != 700000 {
		t.Errorf("expected total paid out 700,000, got %d", earnings.TotalPaidOutVND)
	}
	if earnings.LifetimeEarningsVND != 1000000 {
		t.Errorf("expected lifetime earnings unchanged at 1,000,000, got %d", earnings.LifetimeEarningsVND)
	}
}

// mockMaterialPublisher stands in for the resource module's read-and-copy
// surface so Gate 1's material_ready check can be exercised without storage.
type mockMaterialPublisher struct {
	material *resourcecontract.Material
	err      error
	copied   []string
	objects  *resourcecontract.PublishedObjects
	// owners records the owner each MaterialForOwner call was scoped to, so a
	// test can prove Gate 1 resolves creator-controlled ids through the draft
	// owner rather than through whoever asked.
	owners []uuid.UUID
}

func (m *mockMaterialPublisher) MaterialForOwner(
	_ context.Context, ownerID, _ uuid.UUID,
) (*resourcecontract.Material, error) {
	m.owners = append(m.owners, ownerID)
	if m.err != nil {
		return nil, m.err
	}
	return m.material, nil
}

func (m *mockMaterialPublisher) CopyForPublication(
	_ context.Context, _ uuid.UUID, prefix string,
) (*resourcecontract.PublishedObjects, error) {
	m.copied = append(m.copied, prefix)
	if m.objects != nil {
		return m.objects, nil
	}
	return &resourcecontract.PublishedObjects{Original: prefix + "original.mp4"}, nil
}

const testSkillVocabulary = "vocabulary"

func materialCourseStructure(resourceID uuid.UUID) []byte {
	return materialCourseStructureOf(resourceID, "video")
}

// materialCourseStructureOf builds a three-lesson course whose first lesson is
// a single material, plus the twenty exercises a course needs. The material
// kind is a hint only; Gate 1 derives the real kind from the resource MIME.
func materialCourseStructureOf(resourceID uuid.UUID, materialKind string) []byte {
	material := domain.ActivityDraft{
		Kind:   domain.KindLessonMaterial,
		Weight: 0,
		Body:   json.RawMessage(`{"resource_id":"` + resourceID.String() + `","title":"Intro"}`),
		Material: &domain.MaterialDraft{
			ResourceID:   &resourceID,
			MaterialKind: materialKind,
			Title:        "Intro",
		},
	}
	exercises := func() []domain.ActivityDraft {
		acts := make([]domain.ActivityDraft, 0, 10)
		for i := 0; i < 10; i++ {
			acts = append(acts, domain.ActivityDraft{
				Kind:   "vocab_multiple_choice",
				Weight: 10,
				Body:   json.RawMessage(`{"prompt":"Select","answer":"test"}`),
			})
		}
		return acts
	}
	structure := domain.CourseStructure{Units: []domain.UnitDraft{{
		Title: "Unit 1",
		Lessons: []domain.LessonDraft{
			{Title: "Watch", SkillFocus: "listening", CEFRLevel: "B1",
				Activities: []domain.ActivityDraft{material}},
			{Title: "A", SkillFocus: testSkillVocabulary, CEFRLevel: "B1", Activities: exercises()},
			{Title: "B", SkillFocus: testSkillVocabulary, CEFRLevel: "B1", Activities: exercises()},
		},
	}}}
	raw, _ := json.Marshal(structure)
	return raw
}

func TestGate1_MaterialReady(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, &mockVerifier{}, &mockLessonAuthor{}, &mockContentAuthor{})
	resourceID := uuid.New()
	svc.SetMaterialPublisher(&mockMaterialPublisher{material: &resourcecontract.Material{
		ID:           resourceID,
		Status:       resourcecontract.MaterialValidated,
		DetectedMIME: testVideoMIME,
		Renditions: []resourcecontract.Rendition{
			{Kind: testVideoRendition, Status: resourcecontract.RenditionReady},
		},
	}})

	creatorID := uuid.New()
	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "With Video", Slug: "with-video", Description: "d", CEFRLevel: "B1",
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
		t.Fatalf("a ready material should pass Gate 1, got %s: %s",
			verified.Status, string(verified.VerificationReport))
	}
}

// A document material passes once its preview rendition is ready; the detected
// MIME, not the draft's material_kind hint, decides which rendition is needed.
func TestGate1_DocumentMaterialReady(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, &mockVerifier{}, &mockLessonAuthor{}, &mockContentAuthor{})
	resourceID := uuid.New()
	svc.SetMaterialPublisher(&mockMaterialPublisher{material: &resourcecontract.Material{
		ID:           resourceID,
		Status:       resourcecontract.MaterialValidated,
		DetectedMIME: "application/pdf",
		Renditions: []resourcecontract.Rendition{
			{Kind: "preview", Status: resourcecontract.RenditionReady},
		},
	}})

	creatorID := uuid.New()
	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "With Document", Slug: "with-document", Description: "d", CEFRLevel: "B1",
		Structure: materialCourseStructureOf(resourceID, "document"),
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
		t.Fatalf("a ready document should pass Gate 1, got %s: %s",
			verified.Status, string(verified.VerificationReport))
	}
}

// A document whose preview is not ready fails, exactly as a processing video
// does: the runner's card has no image to show.
func TestGate1_DocumentPreviewNotReadyFails(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, &mockVerifier{}, &mockLessonAuthor{}, &mockContentAuthor{})
	resourceID := uuid.New()
	svc.SetMaterialPublisher(&mockMaterialPublisher{material: &resourcecontract.Material{
		ID:           resourceID,
		Status:       resourcecontract.MaterialValidated,
		DetectedMIME: "application/pdf",
		Renditions: []resourcecontract.Rendition{
			{Kind: "preview", Status: testPayoutPending},
		},
	}})

	creatorID := uuid.New()
	draft, _ := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Doc Processing", Slug: "doc-processing", Description: "d", CEFRLevel: "B1",
		Structure: materialCourseStructureOf(resourceID, "document"),
	})
	sub, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("submit draft: %v", err)
	}
	verified, err := svc.RunGate1Verification(ctx, sub.ID)
	if err != nil {
		t.Fatalf("run Gate 1: %v", err)
	}
	if verified.Status != domain.SubmissionStatusChangesRequested {
		t.Fatalf("a document still processing should fail Gate 1, got %s", verified.Status)
	}
}

func TestGate1_MaterialNotOwnedFails(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, &mockVerifier{}, &mockLessonAuthor{}, &mockContentAuthor{})
	// MaterialForOwner answers not-found for a resource the draft owner does
	// not own, which is what a pasted foreign resource id looks like.
	svc.SetMaterialPublisher(&mockMaterialPublisher{err: errors.New("resource not found")})

	creatorID := uuid.New()
	draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Stolen Video", Slug: "stolen-video", Description: "d", CEFRLevel: "B1",
		Structure: materialCourseStructure(uuid.New()),
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
	if verified.Status != domain.SubmissionStatusChangesRequested {
		t.Fatalf("a foreign material resource should fail Gate 1, got %s", verified.Status)
	}
	found := false
	for _, f := range decodeFailures(t, verified.VerificationReport) {
		if f.Check == "material_ready" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a material_ready failure, report: %s", string(verified.VerificationReport))
	}
}

func TestGate1_MaterialStillProcessingFails(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, &mockVerifier{}, &mockLessonAuthor{}, &mockContentAuthor{})
	resourceID := uuid.New()
	svc.SetMaterialPublisher(&mockMaterialPublisher{material: &resourcecontract.Material{
		ID:           resourceID,
		Status:       resourcecontract.MaterialValidated,
		DetectedMIME: testVideoMIME,
		Renditions: []resourcecontract.Rendition{
			{Kind: testVideoRendition, Status: testPayoutPending},
		},
	}})

	creatorID := uuid.New()
	draft, _ := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Processing", Slug: "processing", Description: "d", CEFRLevel: "B1",
		Structure: materialCourseStructure(resourceID),
	})
	sub, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("submit draft: %v", err)
	}
	verified, err := svc.RunGate1Verification(ctx, sub.ID)
	if err != nil {
		t.Fatalf("run Gate 1: %v", err)
	}
	if verified.Status != domain.SubmissionStatusChangesRequested {
		t.Fatalf("a video still processing should fail Gate 1, got %s", verified.Status)
	}
}

// The resource intake accepts audio, which is not a lesson material: it must
// fail with a message saying so, not wait for a preview forever.
func TestGate1_MaterialOfUnsupportedTypeFails(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, &mockVerifier{}, &mockLessonAuthor{}, &mockContentAuthor{})
	resourceID := uuid.New()
	svc.SetMaterialPublisher(&mockMaterialPublisher{material: &resourcecontract.Material{
		ID:           resourceID,
		Status:       resourcecontract.MaterialValidated,
		DetectedMIME: "audio/mpeg",
		Renditions: []resourcecontract.Rendition{
			{Kind: "audio_web", Status: resourcecontract.RenditionReady},
		},
	}})

	creatorID := uuid.New()
	draft, _ := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Audio", Slug: "audio", Description: "d", CEFRLevel: "B1",
		Structure: materialCourseStructure(resourceID),
	})
	sub, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("submit draft: %v", err)
	}
	verified, err := svc.RunGate1Verification(ctx, sub.ID)
	if err != nil {
		t.Fatalf("run Gate 1: %v", err)
	}
	found := false
	for _, f := range decodeFailures(t, verified.VerificationReport) {
		if f.Check == "material_ready" && strings.Contains(f.Message, "must be a PDF") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an unsupported-type failure, report: %s", string(verified.VerificationReport))
	}
}

// Spreadsheets and images are materials too. An image is ready on its display
// copy, or on its thumbnail when it was too small to need one; it is never
// ready on its original alone, which learners are not served.
const (
	testPreviewRendition = "preview"
	testImageKind        = "image"
)

func TestGate1_SpreadsheetAndImageMaterials(t *testing.T) {
	cases := []struct {
		name       string
		mime       string
		kind       string
		renditions []resourcecontract.Rendition
		wantPass   bool
	}{
		{"xlsx with preview", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "document",
			[]resourcecontract.Rendition{{Kind: testPreviewRendition, Status: resourcecontract.RenditionReady}}, true},
		{"xls without preview", "application/vnd.ms-excel", "document",
			[]resourcecontract.Rendition{{Kind: testPreviewRendition, Status: testPayoutPending}}, false},
		{"png with display", "image/png", testImageKind,
			[]resourcecontract.Rendition{{Kind: "display", Status: resourcecontract.RenditionReady}}, true},
		{"small webp with thumbnail only", "image/webp", testImageKind,
			[]resourcecontract.Rendition{{Kind: "thumbnail", Status: resourcecontract.RenditionReady}}, true},
		{"jpeg still processing", "image/jpeg", testImageKind,
			[]resourcecontract.Rendition{{Kind: "display", Status: testPayoutPending}}, false},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc := service.NewService(newMockRepo(), &mockVerifier{}, &mockLessonAuthor{}, &mockContentAuthor{})
			resourceID := uuid.New()
			svc.SetMaterialPublisher(&mockMaterialPublisher{material: &resourcecontract.Material{
				ID:           resourceID,
				Status:       resourcecontract.MaterialValidated,
				DetectedMIME: tc.mime,
				Renditions:   tc.renditions,
			}})
			creatorID := uuid.New()
			draft, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
				Title: tc.name, Slug: fmt.Sprintf("material-%d", i), Description: "d", CEFRLevel: "B1",
				Structure: materialCourseStructureOf(resourceID, tc.kind),
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
			passed := verified.Status == domain.SubmissionStatusInReview
			if passed != tc.wantPass {
				t.Fatalf("pass = %v, want %v; report: %s", passed, tc.wantPass, string(verified.VerificationReport))
			}
		})
	}
}

func TestCreateDraft_MinimumPriceIs5000(t *testing.T) {
	ctx := context.Background()
	svc := service.NewService(newMockRepo(), &mockVerifier{}, &mockLessonAuthor{}, &mockContentAuthor{})
	creatorID := uuid.New()
	if _, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Cheap", Slug: "cheap", CEFRLevel: "A1", PriceVND: 5_000,
	}); err != nil {
		t.Fatalf("5,000 VND should be accepted: %v", err)
	}
	if _, err := svc.CreateDraft(ctx, creatorID, service.CreateDraftRequest{
		Title: "Too cheap", Slug: "too-cheap", CEFRLevel: "A1", PriceVND: 4_999,
	}); !errors.Is(err, domain.ErrPriceOutOfBounds) {
		t.Fatalf("4,999 VND should be out of bounds, got %v", err)
	}
}

func decodeFailures(t *testing.T, report json.RawMessage) []domain.VerificationFailure {
	t.Helper()
	var decoded struct {
		Failures []domain.VerificationFailure `json:"failures"`
	}
	if err := json.Unmarshal(report, &decoded); err != nil {
		t.Fatalf("decode verification report: %v", err)
	}
	return decoded.Failures
}
