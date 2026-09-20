package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	paymentcontract "github.com/fluentra/fluentra/internal/modules/payment/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/modules/studio/service"
)

type mockOrderCreator struct {
	order *paymentcontract.Order
	err   error
}

func (
	m *mockOrderCreator) CreateOrder(_ context.Context,
	in paymentcontract.CreateOrderInput) (*paymentcontract.Order,
	error,
) {
	if m.err != nil {
		return nil, m.err
	}
	if m.order != nil {
		return m.order, nil
	}
	return &paymentcontract.Order{
		ID:          uuid.New(),
		UserID:      in.UserID,
		Reference:   "FLU-PAYWALL-1",
		AmountVND:   in.AmountVND,
		SubjectKind: in.SubjectKind,
		SubjectID:   in.SubjectID,
		Status:      "pending",
	}, nil
}

type mockProgressReader struct {
	progress []learningcontract.Progress
}

func (m *mockProgressReader) ProgressOf(
	_ context.Context, _ uuid.UUID, _ learningcontract.ProgressScope,
) ([]learningcontract.Progress, error) {
	return m.progress, nil
}

// assertMayOpen checks one row of the paywall truth table: MayOpen must not
// fail, and must answer `want`.
func assertMayOpen(
	ctx context.Context,
	t *testing.T,
	svc *service.Service,
	scenario string,
	userID *uuid.UUID,
	courseID uuid.UUID,
	want bool,
) {
	t.Helper()
	got, err := svc.MayOpen(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("%s: MayOpen returned an error: %v", scenario, err)
	}
	if got != want {
		t.Fatalf("%s: MayOpen = %v, want %v", scenario, got, want)
	}
}

func TestMayOpen_TruthTable(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, nil, nil, nil)

	creatorID := uuid.New()
	learnerID := uuid.New()
	unownedID := uuid.New()

	// 1. Official course: no listing in studio.listings -> true
	officialCourseID := uuid.New()
	assertMayOpen(ctx, t, svc, "scenario 1: official course should open for anonymous", nil, officialCourseID, true)
	assertMayOpen(ctx, t, svc, "scenario 1: official course should open for learner", &learnerID, officialCourseID, true)

	// 2. Free community course: pricing_model == "free" or price_vnd == 0 -> true
	freeCourseID := uuid.New()
	_, _ = repo.UpsertListing(ctx, &domain.Listing{
		CourseID:        freeCourseID,
		CreatorID:       creatorID,
		PricingModel:    domain.PricingModelFree,
		PriceVND:        0,
		RevenueShareBPS: 7000,
		Status:          domain.ListingStatusActive,
	})
	assertMayOpen(ctx, t, svc, "scenario 2: free community course should open for anonymous", nil, freeCourseID, true)
	assertMayOpen(ctx, t, svc, "scenario 2: free community course should open for learner", &learnerID, freeCourseID, true)

	// 3. Paid community course: unowned -> false
	paidCourseID := uuid.New()
	_, _ = repo.UpsertListing(ctx, &domain.Listing{
		CourseID:        paidCourseID,
		CreatorID:       creatorID,
		PricingModel:    domain.PricingModelOneTime,
		PriceVND:        199000,
		RevenueShareBPS: 7000,
		Status:          domain.ListingStatusActive,
	})
	// 3a. Anonymous caller (nil) -> false
	assertMayOpen(ctx, t, svc, "scenario 3a: paid unowned course should refuse anonymous caller", nil, paidCourseID, false)
	// 3b. Anonymous caller (uuid.Nil) -> false
	nilUUID := uuid.Nil
	assertMayOpen(ctx, t, svc,
		"scenario 3b: paid unowned course should refuse uuid.Nil caller",
		&nilUUID, paidCourseID, false)
	// 3c. Authenticated learner without purchase -> false
	assertMayOpen(ctx, t, svc,
		"scenario 3c: paid unowned course should refuse learner without purchase",
		&unownedID, paidCourseID, false)

	// 4. Paid owned course: active purchase -> true
	_, _ = repo.CreatePurchase(ctx, &domain.Purchase{
		UserID:       learnerID,
		CourseID:     paidCourseID,
		PricePaidVND: 199000,
	})
	assertMayOpen(ctx, t, svc, "scenario 4: paid owned course should open for purchaser", &learnerID, paidCourseID, true)

	// 5. Paid refunded course: purchase revoked -> false
	refundedLearnerID := uuid.New()
	pRefunded, _ := repo.CreatePurchase(ctx, &domain.Purchase{
		UserID:       refundedLearnerID,
		CourseID:     paidCourseID,
		PricePaidVND: 199000,
	})
	_, _ = repo.RevokePurchase(ctx, pRefunded.ID, "learner_refund")
	assertMayOpen(ctx, t, svc,
		"scenario 5: paid refunded course must refuse learner",
		&refundedLearnerID, paidCourseID, false)

	// 6. Paid course taken down (BR-STUDIO-04):
	// Existing purchaser keeps access -> true; unowned caller -> false
	takenDownCourseID := uuid.New()
	_, _ = repo.UpsertListing(ctx, &domain.Listing{
		CourseID:        takenDownCourseID,
		CreatorID:       creatorID,
		PricingModel:    domain.PricingModelOneTime,
		PriceVND:        299000,
		RevenueShareBPS: 7000,
		Status:          domain.ListingStatusTakenDown,
	})
	_, _ = repo.CreatePurchase(ctx, &domain.Purchase{
		UserID:       learnerID,
		CourseID:     takenDownCourseID,
		PricePaidVND: 299000,
	})
	assertMayOpen(ctx, t, svc,
		"scenario 6: existing purchaser must keep access to taken-down course",
		&learnerID, takenDownCourseID, true)
	assertMayOpen(ctx, t, svc,
		"scenario 6: unowned caller must not open taken-down course",
		&unownedID, takenDownCourseID, false)

	// Extra: Creator viewing their own course -> true
	assertMayOpen(ctx, t, svc, "creator should always open their own course: ", &creatorID, paidCourseID, true)
}

func TestClaimCourse(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, nil, nil, nil)

	creatorID := uuid.New()
	learnerID := uuid.New()

	// 1. Claim a free course
	freeCourseID := uuid.New()
	_, _ = repo.UpsertListing(ctx, &domain.Listing{
		CourseID:        freeCourseID,
		CreatorID:       creatorID,
		PricingModel:    domain.PricingModelFree,
		PriceVND:        0,
		RevenueShareBPS: 7000,
		Status:          domain.ListingStatusActive,
	})

	purchase, err := svc.ClaimCourse(ctx, learnerID, freeCourseID)
	if err != nil {
		t.Fatalf("claim free course error: %v", err)
	}
	if purchase.PricePaidVND != 0 {
		t.Errorf("expected price paid 0, got %d", purchase.PricePaidVND)
	}
	if purchase.UserID != learnerID || purchase.CourseID != freeCourseID {
		t.Errorf("unexpected purchase attributes: %+v", purchase)
	}

	// 2. Claiming again fails with conflict
	_, errDuplicate := svc.ClaimCourse(ctx, learnerID, freeCourseID)
	if !errors.Is(errDuplicate, domain.ErrCourseAlreadyPurchased) {
		t.Fatalf("expected ErrCourseAlreadyPurchased on duplicate claim, got: %v", errDuplicate)
	}

	// 3. Claiming paid course fails
	paidCourseID := uuid.New()
	_, _ = repo.UpsertListing(ctx, &domain.Listing{
		CourseID:        paidCourseID,
		CreatorID:       creatorID,
		PricingModel:    domain.PricingModelOneTime,
		PriceVND:        149000,
		RevenueShareBPS: 7000,
		Status:          domain.ListingStatusActive,
	})

	_, errPaid := svc.ClaimCourse(ctx, learnerID, paidCourseID)
	if !errors.Is(errPaid, domain.ErrCourseNotFree) {
		t.Fatalf("expected ErrCourseNotFree on paid claim, got: %v", errPaid)
	}
}

func TestPurchaseCourse_And_PaymentMatch(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, nil, nil, nil)
	orderCreator := &mockOrderCreator{}
	svc.SetOrderCreator(orderCreator)

	creatorID := uuid.New()
	learnerID := uuid.New()
	courseID := uuid.New()

	_, _ = repo.UpsertListing(ctx, &domain.Listing{
		CourseID:        courseID,
		CreatorID:       creatorID,
		PricingModel:    domain.PricingModelOneTime,
		PriceVND:        100000,
		RevenueShareBPS: 7000, // 70% creator / 30% platform
		Status:          domain.ListingStatusActive,
	})

	// 1. Purchase course creates pending order
	order, err := svc.PurchaseCourse(ctx, learnerID, courseID)
	if err != nil {
		t.Fatalf("purchase course error: %v", err)
	}
	if order.AmountVND != 100000 {
		t.Errorf("expected order amount 100,000, got %d", order.AmountVND)
	}

	// 2. Simulate SePay webhook match: payment.succeeded event arrives
	err = svc.HandlePaymentSucceeded(ctx, paymentcontract.EventPaymentSucceeded{
		UserID:      learnerID,
		OrderID:     order.ID,
		SubjectKind: "course",
		SubjectID:   courseID,
		AmountVND:   100000,
	})
	if err != nil {
		t.Fatalf("handle payment succeeded error: %v", err)
	}

	// Verify purchase row created
	activePurchase, err := repo.GetActivePurchase(ctx, learnerID, courseID)
	if err != nil || activePurchase == nil {
		t.Fatalf("expected active purchase created for learner, got nil or error: %v", err)
	}
	if activePurchase.PricePaidVND != 100000 {
		t.Errorf("expected price paid 100000, got %d", activePurchase.PricePaidVND)
	}

	// Verify 70/30 split in creator ledger
	entries, err := repo.ListLedgerEntriesByCreatorID(ctx, creatorID, 10, 0)
	if err != nil {
		t.Fatalf("list ledger entries error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 ledger entries (creator share + platform share), got %d", len(entries))
	}

	// Entry 1: Creator share (70% = 70,000 VND)
	if entries[0].Kind != domain.LedgerKindSale || entries[0].AmountVND != 70000 {
		t.Errorf("expected sale entry amount 70,000, got %+v", entries[0])
	}
	// Entry 2: Platform fee (30% = 30,000 VND)
	if entries[1].Kind != domain.LedgerKindPlatformShare || entries[1].AmountVND != 30000 {
		t.Errorf("expected platform share entry amount 30,000, got %+v", entries[1])
	}

	// Purchasing again now fails with ErrCourseAlreadyPurchased
	_, errAlreadyBought := svc.PurchaseCourse(ctx, learnerID, courseID)
	if !errors.Is(errAlreadyBought, domain.ErrCourseAlreadyPurchased) {
		t.Fatalf("expected ErrCourseAlreadyPurchased, got: %v", errAlreadyBought)
	}
}

// refundFixture builds a course of `lessons` lessons with one paid purchase,
// its sale ledger credit, and a recorded order.
type refundFixture struct {
	repo      *mockRepo
	svc       *service.Service
	refunds   *mockRefundRecorder
	progress  *mockProgressReader
	creatorID uuid.UUID
	courseID  uuid.UUID
	lessonIDs []uuid.UUID
}

func newRefundFixture(ctx context.Context, t *testing.T, lessons int) *refundFixture {
	t.Helper()
	repo := newMockRepo()
	svc := service.NewService(repo, nil, nil, nil)
	progress := &mockProgressReader{}
	refunds := &mockRefundRecorder{}
	lessonIDs := make([]uuid.UUID, lessons)
	for i := range lessonIDs {
		lessonIDs[i] = uuid.New()
	}
	courseID := uuid.New()
	svc.SetProgressReader(progress)
	svc.SetLessonReader(&mockLessonReader{courseID: courseID, lessonIDs: lessonIDs})
	svc.SetRefundRecorder(refunds)

	creatorID := uuid.New()
	_, _ = repo.UpsertListing(ctx, &domain.Listing{
		CourseID:        courseID,
		CreatorID:       creatorID,
		PricingModel:    domain.PricingModelOneTime,
		PriceVND:        200000,
		RevenueShareBPS: 7000,
		Status:          domain.ListingStatusActive,
	})
	return &refundFixture{
		repo: repo, svc: svc, refunds: refunds, progress: progress,
		creatorID: creatorID, courseID: courseID, lessonIDs: lessonIDs,
	}
}

// buy records a purchase and the sale credit that HandlePaymentSucceeded
// would have written for it.
func (f *refundFixture) buy(ctx context.Context, t *testing.T, learnerID uuid.UUID) *domain.Purchase {
	t.Helper()
	orderID := uuid.New()
	p, err := f.repo.CreatePurchase(ctx, &domain.Purchase{
		UserID:       learnerID,
		CourseID:     f.courseID,
		OrderID:      &orderID,
		PricePaidVND: 200000,
	})
	if err != nil {
		t.Fatalf("create purchase: %v", err)
	}
	if _, err := f.repo.CreateLedgerEntry(ctx, &domain.CreatorLedgerEntry{
		CreatorID:      f.creatorID,
		Kind:           domain.LedgerKindSale,
		AmountVND:      140000,
		GrossAmountVND: 200000,
		FeeAmountVND:   60000,
		PurchaseID:     &p.ID,
	}); err != nil {
		t.Fatalf("create sale ledger entry: %v", err)
	}
	return p
}

// completeLessons marks the first n of the course's lessons done.
func (f *refundFixture) completeLessons(_ uuid.UUID, n int) {
	progress := make([]learningcontract.Progress, 0, n)
	for i := 0; i < n && i < len(f.lessonIDs); i++ {
		progress = append(progress, learningcontract.Progress{
			ScopeID: f.lessonIDs[i],
			Status:  "completed",
		})
	}
	f.progress.progress = progress
}

// TestRefundPurchase_RecordsWhatIsOwed is the regression for a refund that
// took the course away and returned nothing: it revoked access, wrote a
// negative ledger row and never recorded that money was owed, so no refund
// existed for anybody to pay.
func TestRefundPurchase_RecordsWhatIsOwed(t *testing.T) {
	ctx := context.Background()
	f := newRefundFixture(ctx, t, 10)
	learnerID := uuid.New()
	p := f.buy(ctx, t, learnerID)
	f.completeLessons(learnerID, 1) // 10%

	if err := f.svc.RefundPurchase(ctx, learnerID, p.ID); err != nil {
		t.Fatalf("refund within window and under 20%%: %v", err)
	}

	if len(f.refunds.recorded) != 1 {
		t.Fatalf("expected the refund to be recorded against the order, got %d records",
			len(f.refunds.recorded))
	}
	if got := f.refunds.recorded[0].amount; got != 200000 {
		t.Errorf("recorded %d VND owed, want the 200,000 the learner paid", got)
	}
	if f.refunds.recorded[0].orderID != *p.OrderID {
		t.Errorf("refund recorded against the wrong order")
	}

	revoked, _ := f.repo.GetPurchaseByID(ctx, p.ID)
	if revoked.RevokedAt == nil {
		t.Fatalf("expected the purchase to be revoked")
	}
}

// TestRefundPurchase_ReversesTheSplitTheSaleUsed. BR-STUDIO-03: the share is
// recorded per sale, so changing the listing's rate afterwards must not make
// the reversal disagree with the credit it reverses.
func TestRefundPurchase_ReversesTheSplitTheSaleUsed(t *testing.T) {
	ctx := context.Background()
	f := newRefundFixture(ctx, t, 10)
	learnerID := uuid.New()
	p := f.buy(ctx, t, learnerID)

	// The platform changes its cut after the sale.
	_, _ = f.repo.UpsertListing(ctx, &domain.Listing{
		CourseID:        f.courseID,
		CreatorID:       f.creatorID,
		PricingModel:    domain.PricingModelOneTime,
		PriceVND:        200000,
		RevenueShareBPS: 3000,
		Status:          domain.ListingStatusActive,
	})

	if err := f.svc.RefundPurchase(ctx, learnerID, p.ID); err != nil {
		t.Fatalf("refund: %v", err)
	}

	entries, _ := f.repo.ListLedgerEntriesByCreatorID(ctx, f.creatorID, 10, 0)
	var reversal *domain.CreatorLedgerEntry
	for _, e := range entries {
		if e.Kind == domain.LedgerKindRefund {
			reversal = e
		}
	}
	if reversal == nil {
		t.Fatal("expected a refund reversal in the creator ledger")
	}
	if reversal.AmountVND != -140000 {
		t.Errorf("reversed %d, want -140,000 — the split the sale was recorded with",
			reversal.AmountVND)
	}
}

// TestRefundPurchase_MeasuresCompletionNotGrade is the regression for the rule
// that read the course progress row's average grade. That row is only written
// once the course is finished, so the rule became "refund unless you completed
// the course scoring 20 or more" — answer everything wrong and a finished
// course was refundable.
func TestRefundPurchase_MeasuresCompletionNotGrade(t *testing.T) {
	ctx := context.Background()

	t.Run("a finished course is refused however badly it was scored", func(t *testing.T) {
		f := newRefundFixture(ctx, t, 10)
		learnerID := uuid.New()
		p := f.buy(ctx, t, learnerID)
		f.completeLessons(learnerID, 10) // 100% done, no grade anywhere

		err := f.svc.RefundPurchase(ctx, learnerID, p.ID)
		if !errors.Is(err, domain.ErrRefundProgressExceeded) {
			t.Fatalf("expected ErrRefundProgressExceeded, got %v", err)
		}
		if len(f.refunds.recorded) != 0 {
			t.Errorf("a refused refund recorded money owed")
		}
	})

	t.Run("a fifth of the way through is the line", func(t *testing.T) {
		f := newRefundFixture(ctx, t, 10)
		learnerID := uuid.New()
		p := f.buy(ctx, t, learnerID)
		f.completeLessons(learnerID, 2) // exactly 20%

		err := f.svc.RefundPurchase(ctx, learnerID, p.ID)
		if !errors.Is(err, domain.ErrRefundProgressExceeded) {
			t.Fatalf("20%% completed should be refused, got %v", err)
		}
	})
}

func TestRefundPurchase_Rules(t *testing.T) {
	ctx := context.Background()
	f := newRefundFixture(ctx, t, 10)
	learnerID := uuid.New()

	// 1. Refund within 7 days and 10% progress (< 20%) -> SUCCESS
	p := f.buy(ctx, t, learnerID)
	f.completeLessons(learnerID, 1)

	if err := f.svc.RefundPurchase(ctx, learnerID, p.ID); err != nil {
		t.Fatalf("refund within window and under 20%% should succeed, got: %v", err)
	}

	revokedP, _ := f.repo.GetPurchaseByID(ctx, p.ID)
	if revokedP.RevokedAt == nil {
		t.Fatalf("expected purchase to be revoked")
	}

	// 2. Duplicate refund fails
	errDup := f.svc.RefundPurchase(ctx, learnerID, p.ID)
	if !errors.Is(errDup, domain.ErrAlreadyRefunded) {
		t.Fatalf("expected ErrAlreadyRefunded, got: %v", errDup)
	}

	// 3. Refund after the 7-day window fails
	learner3 := uuid.New()
	p3 := f.buy(ctx, t, learner3)
	p3.GrantedAt = time.Now().Add(-8 * 24 * time.Hour)
	f.progress.progress = nil

	errWindow := f.svc.RefundPurchase(ctx, learner3, p3.ID)
	if !errors.Is(errWindow, domain.ErrRefundWindowExpired) {
		t.Fatalf("expected ErrRefundWindowExpired, got: %v", errWindow)
	}
}

// mockRefundRecorder records the obligations a refund creates.
type mockRefundRecorder struct {
	recorded []recordedRefund
	err      error
}

type recordedRefund struct {
	orderID uuid.UUID
	amount  int64
	reason  string
}

func (m *mockRefundRecorder) RecordRefund(
	_ context.Context, orderID uuid.UUID, amountVND int64, reason string, _ uuid.UUID,
) (*paymentcontract.Refund, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.recorded = append(m.recorded, recordedRefund{orderID: orderID, amount: amountVND, reason: reason})
	return &paymentcontract.Refund{
		ID: uuid.New(), OrderID: orderID, AmountVND: amountVND, Reason: reason, Status: "requested",
	}, nil
}

// mockLessonReader answers with one unit holding every lesson of the course,
// which is all courseCompletion needs to size it.
type mockLessonReader struct {
	courseID  uuid.UUID
	lessonIDs []uuid.UUID
	unitID    uuid.UUID
}

func (m *mockLessonReader) ListUnitsByCourseID(
	_ context.Context, courseID uuid.UUID,
) ([]*lessoncontract.Unit, error) {
	if courseID != m.courseID {
		return nil, nil
	}
	if m.unitID == uuid.Nil {
		m.unitID = uuid.New()
	}
	return []*lessoncontract.Unit{{ID: m.unitID, CourseID: courseID}}, nil
}

func (m *mockLessonReader) ListLessons(_ context.Context, _ uuid.UUID) ([]*lessoncontract.Lesson, error) {
	out := make([]*lessoncontract.Lesson, 0, len(m.lessonIDs))
	for _, id := range m.lessonIDs {
		out = append(out, &lessoncontract.Lesson{ID: id})
	}
	return out, nil
}

func (m *mockLessonReader) GetLesson(context.Context, uuid.UUID) (*lessoncontract.Lesson, error) {
	return nil, nil
}

func (m *mockLessonReader) ListPrerequisitesForLessons(
	context.Context, []uuid.UUID,
) ([]lessoncontract.PrerequisiteItem, error) {
	return nil, nil
}

func (m *mockLessonReader) ListActivitiesByCourseIDs(
	context.Context, []uuid.UUID,
) (map[uuid.UUID][]uuid.UUID, error) {
	return nil, nil
}

func (m *mockLessonReader) NextLesson(
	context.Context, uuid.UUID, *uuid.UUID,
) (*lessoncontract.Lesson, error) {
	return nil, nil
}

func (m *mockLessonReader) ResolveActivity(
	context.Context, uuid.UUID,
) (*lessoncontract.ActivityHierarchy, error) {
	return nil, nil
}
