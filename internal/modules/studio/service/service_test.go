package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	paymentcontract "github.com/fluentra/fluentra/internal/modules/payment/contract"
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

func (m *mockRepo) GetPayoutAccount(ctx context.Context, creatorID uuid.UUID) (*domain.PayoutAccount, error) {
	return m.payoutAccounts[creatorID], nil
}

func (m *mockRepo) UpsertPayoutAccount(ctx context.Context, creatorID uuid.UUID, bankCode, accountNumber, accountHolderName string, isDefault bool) (*domain.PayoutAccount, error) {
	acc := &domain.PayoutAccount{
		ID:                uuid.New(),
		CreatorID:         creatorID,
		BankCode:          bankCode,
		AccountNumber:     accountNumber,
		AccountHolderName: accountHolderName,
		IsDefault:         isDefault,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	m.payoutAccounts[creatorID] = acc
	return acc, nil
}

func (m *mockRepo) CreateCourseDraft(ctx context.Context, draft *domain.CourseDraft) (*domain.CourseDraft, error) {
	draft.ID = uuid.New()
	draft.CreatedAt = time.Now()
	draft.UpdatedAt = time.Now()
	m.drafts[draft.ID] = draft
	return draft, nil
}

func (m *mockRepo) GetCourseDraftByID(ctx context.Context, id uuid.UUID) (*domain.CourseDraft, error) {
	if d, ok := m.drafts[id]; ok {
		return d, nil
	}
	return nil, domain.ErrDraftNotFound
}

func (m *mockRepo) ListCourseDraftsByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]*domain.CourseDraft, int64, error) {
	var list []*domain.CourseDraft
	for _, d := range m.drafts {
		if d.OwnerID == ownerID {
			list = append(list, d)
		}
	}
	return list, int64(len(list)), nil
}

func (m *mockRepo) UpdateCourseDraft(ctx context.Context, draft *domain.CourseDraft) (*domain.CourseDraft, error) {
	m.drafts[draft.ID] = draft
	return draft, nil
}

func (m *mockRepo) UpdateCourseDraftStatus(ctx context.Context, id uuid.UUID, status string) (*domain.CourseDraft, error) {
	if d, ok := m.drafts[id]; ok {
		d.Status = status
		return d, nil
	}
	return nil, domain.ErrDraftNotFound
}

func (m *mockRepo) CreateSubmission(ctx context.Context, sub *domain.Submission) (*domain.Submission, error) {
	sub.ID = uuid.New()
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()
	m.submissions[sub.ID] = sub
	return sub, nil
}

func (m *mockRepo) GetSubmissionByID(ctx context.Context, id uuid.UUID) (*domain.Submission, error) {
	if s, ok := m.submissions[id]; ok {
		return s, nil
	}
	return nil, domain.ErrSubmissionNotFound
}

func (m *mockRepo) GetLatestSubmissionByDraftID(ctx context.Context, draftID uuid.UUID) (*domain.Submission, error) {
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

func (m *mockRepo) ListSubmissionsByStatus(ctx context.Context, status string, limit, offset int) ([]*domain.Submission, int64, error) {
	var list []*domain.Submission
	for _, s := range m.submissions {
		if s.Status == status {
			list = append(list, s)
		}
	}
	return list, int64(len(list)), nil
}

func (m *mockRepo) UpdateSubmissionVerification(ctx context.Context, id uuid.UUID, status string, report []byte, feedback *string) (*domain.Submission, error) {
	if s, ok := m.submissions[id]; ok {
		s.Status = status
		s.VerificationReport = report
		s.Feedback = feedback
		return s, nil
	}
	return nil, domain.ErrSubmissionNotFound
}

func (m *mockRepo) UpdateSubmissionReview(ctx context.Context, id uuid.UUID, status string, reviewerID uuid.UUID, feedback *string) (*domain.Submission, error) {
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

func (m *mockRepo) UpsertListing(ctx context.Context, l *domain.Listing) (*domain.Listing, error) {
	l.PublishedAt = time.Now()
	m.listings[l.CourseID] = l
	return l, nil
}

func (m *mockRepo) GetListingByCourseID(ctx context.Context, courseID uuid.UUID) (*domain.Listing, error) {
	if l, ok := m.listings[courseID]; ok {
		return l, nil
	}
	return nil, domain.ErrListingNotFound
}

func (m *mockRepo) BatchGetListings(ctx context.Context, courseIDs []uuid.UUID) (map[uuid.UUID]*domain.Listing, error) {
	out := make(map[uuid.UUID]*domain.Listing)
	for _, id := range courseIDs {
		if l, ok := m.listings[id]; ok {
			out[id] = l
		}
	}
	return out, nil
}

func (m *mockRepo) CreatePurchase(ctx context.Context, p *domain.Purchase) (*domain.Purchase, error) {
	p.ID = uuid.New()
	p.GrantedAt = time.Now()
	m.purchases[p.ID] = p
	return p, nil
}

func (m *mockRepo) GetPurchaseByID(ctx context.Context, id uuid.UUID) (*domain.Purchase, error) {
	if p, ok := m.purchases[id]; ok {
		return p, nil
	}
	return nil, domain.ErrPurchaseNotFound
}

func (m *mockRepo) GetActivePurchase(ctx context.Context, userID, courseID uuid.UUID) (*domain.Purchase, error) {
	for _, p := range m.purchases {
		if p.UserID == userID && p.CourseID == courseID && p.RevokedAt == nil {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) ListPurchasesByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Purchase, int64, error) {
	var list []*domain.Purchase
	for _, p := range m.purchases {
		if p.UserID == userID {
			list = append(list, p)
		}
	}
	return list, int64(len(list)), nil
}

func (m *mockRepo) RevokePurchase(ctx context.Context, id uuid.UUID, reason string) (*domain.Purchase, error) {
	if p, ok := m.purchases[id]; ok {
		now := time.Now()
		p.RevokedAt = &now
		p.RevokeReason = &reason
		return p, nil
	}
	return nil, domain.ErrPurchaseNotFound
}

func (m *mockRepo) CreateLedgerEntry(ctx context.Context, e *domain.CreatorLedgerEntry) (*domain.CreatorLedgerEntry, error) {
	e.ID = uuid.New()
	e.CreatedAt = time.Now()
	m.ledger = append(m.ledger, e)
	return e, nil
}

func (m *mockRepo) ListLedgerEntriesByCreatorID(ctx context.Context, creatorID uuid.UUID, limit, offset int) ([]*domain.CreatorLedgerEntry, error) {
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
	verifyFunc func(ctx context.Context, req learningcontract.VerifyItemRequest) error
}

func (v *mockVerifier) VerifyItem(ctx context.Context, req learningcontract.VerifyItemRequest) error {
	if v.verifyFunc != nil {
		return v.verifyFunc(ctx, req)
	}
	return nil
}

type mockLessonAuthor struct {
	lessoncontract.Author
	coursesCreated int
}

func (a *mockLessonAuthor) EnsureCourse(ctx context.Context, spec lessoncontract.CourseSpec) (uuid.UUID, error) {
	a.coursesCreated++
	return uuid.New(), nil
}

func (a *mockLessonAuthor) EnsureUnit(ctx context.Context, spec lessoncontract.UnitSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (a *mockLessonAuthor) EnsureLesson(ctx context.Context, spec lessoncontract.LessonSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (a *mockLessonAuthor) SyncActivities(ctx context.Context, lessonID uuid.UUID, activities []lessoncontract.ActivitySpec) error {
	return nil
}

func (a *mockLessonAuthor) AppendActivity(ctx context.Context, lessonID uuid.UUID, activity lessoncontract.ActivitySpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

type mockContentAuthor struct {
	contentcontract.Author
}

func (a *mockContentAuthor) EnsurePublished(ctx context.Context, spec contentcontract.AuthorSpec) (uuid.UUID, error) {
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

	// 2. Submit Draft - Gate 1 passes
	sub, err := svc.SubmitDraft(ctx, creatorID, draft.ID)
	if err != nil {
		t.Fatalf("submit draft error: %v", err)
	}
	if sub.Status != domain.SubmissionStatusInReview {
		t.Fatalf("expected submission status in_review, got %s", sub.Status)
	}

	// 3. Test verification failure when verifier returns error
	verifier.verifyFunc = func(ctx context.Context, req learningcontract.VerifyItemRequest) error {
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
	if subFail.Status != domain.SubmissionStatusChangesRequested {
		t.Fatalf("expected changes_requested on verification failure, got %s", subFail.Status)
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
	if lessonAuthor.coursesCreated != 1 {
		t.Errorf("expected 1 course published into lesson, got %d", lessonAuthor.coursesCreated)
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
