package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	paymentcontract "github.com/fluentra/fluentra/internal/modules/payment/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/modules/studio/service"
)

type mockOrderCreator struct {
	order *paymentcontract.Order
	err   error
}

func (m *mockOrderCreator) CreateOrder(ctx context.Context, in paymentcontract.CreateOrderInput) (*paymentcontract.Order, error) {
	if m.err != nil {
		return nil, m.err
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

func (m *mockProgressReader) ProgressOf(ctx context.Context, userID uuid.UUID, scope learningcontract.ProgressScope) ([]learningcontract.Progress, error) {
	return m.progress, nil
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
	mayOpen, err := svc.MayOpen(ctx, nil, officialCourseID)
	if err != nil || !mayOpen {
		t.Fatalf("scenario 1: official course should open for anonymous, got %v, err %v", mayOpen, err)
	}
	mayOpen, err = svc.MayOpen(ctx, &learnerID, officialCourseID)
	if err != nil || !mayOpen {
		t.Fatalf("scenario 1: official course should open for learner, got %v, err %v", mayOpen, err)
	}

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
	mayOpen, err = svc.MayOpen(ctx, nil, freeCourseID)
	if err != nil || !mayOpen {
		t.Fatalf("scenario 2: free community course should open for anonymous, got %v, err %v", mayOpen, err)
	}
	mayOpen, err = svc.MayOpen(ctx, &learnerID, freeCourseID)
	if err != nil || !mayOpen {
		t.Fatalf("scenario 2: free community course should open for learner, got %v, err %v", mayOpen, err)
	}

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
	mayOpen, err = svc.MayOpen(ctx, nil, paidCourseID)
	if err != nil || mayOpen {
		t.Fatalf("scenario 3a: paid unowned course should refuse anonymous caller, got %v, err %v", mayOpen, err)
	}
	// 3b. Anonymous caller (uuid.Nil) -> false
	nilUUID := uuid.Nil
	mayOpen, err = svc.MayOpen(ctx, &nilUUID, paidCourseID)
	if err != nil || mayOpen {
		t.Fatalf("scenario 3b: paid unowned course should refuse uuid.Nil caller, got %v, err %v", mayOpen, err)
	}
	// 3c. Authenticated learner without purchase -> false
	mayOpen, err = svc.MayOpen(ctx, &unownedID, paidCourseID)
	if err != nil || mayOpen {
		t.Fatalf("scenario 3c: paid unowned course should refuse learner without purchase, got %v, err %v", mayOpen, err)
	}

	// 4. Paid owned course: active purchase -> true
	_, _ = repo.CreatePurchase(ctx, &domain.Purchase{
		UserID:       learnerID,
		CourseID:     paidCourseID,
		PricePaidVND: 199000,
	})
	mayOpen, err = svc.MayOpen(ctx, &learnerID, paidCourseID)
	if err != nil || !mayOpen {
		t.Fatalf("scenario 4: paid owned course should open for purchaser, got %v, err %v", mayOpen, err)
	}

	// 5. Paid refunded course: purchase revoked -> false
	refundedLearnerID := uuid.New()
	pRefunded, _ := repo.CreatePurchase(ctx, &domain.Purchase{
		UserID:       refundedLearnerID,
		CourseID:     paidCourseID,
		PricePaidVND: 199000,
	})
	_, _ = repo.RevokePurchase(ctx, pRefunded.ID, "learner_refund")
	mayOpen, err = svc.MayOpen(ctx, &refundedLearnerID, paidCourseID)
	if err != nil || mayOpen {
		t.Fatalf("scenario 5: paid refunded course must refuse learner, got %v, err %v", mayOpen, err)
	}

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
	mayOpen, err = svc.MayOpen(ctx, &learnerID, takenDownCourseID)
	if err != nil || !mayOpen {
		t.Fatalf("scenario 6: existing purchaser must keep access to taken-down course, got %v, err %v", mayOpen, err)
	}
	mayOpen, err = svc.MayOpen(ctx, &unownedID, takenDownCourseID)
	if err != nil || mayOpen {
		t.Fatalf("scenario 6: unowned caller must not open taken-down course, got %v, err %v", mayOpen, err)
	}

	// Extra: Creator viewing their own course -> true
	mayOpen, err = svc.MayOpen(ctx, &creatorID, paidCourseID)
	if err != nil || !mayOpen {
		t.Fatalf("creator should always open their own course, got %v, err %v", mayOpen, err)
	}
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

func TestRefundPurchase_Rules(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.NewService(repo, nil, nil, nil)
	progReader := &mockProgressReader{}
	svc.SetProgressReader(progReader)

	creatorID := uuid.New()
	learnerID := uuid.New()
	courseID := uuid.New()

	_, _ = repo.UpsertListing(ctx, &domain.Listing{
		CourseID:        courseID,
		CreatorID:       creatorID,
		PricingModel:    domain.PricingModelOneTime,
		PriceVND:        200000,
		RevenueShareBPS: 7000,
		Status:          domain.ListingStatusActive,
	})

	// 1. Refund within 7 days and 10% progress (< 20%) -> SUCCESS
	p, _ := repo.CreatePurchase(ctx, &domain.Purchase{
		UserID:       learnerID,
		CourseID:     courseID,
		PricePaidVND: 200000,
	})
	score10 := 10
	progReader.progress = []learningcontract.Progress{
		{ScopeID: courseID, Score: &score10},
	}

	err := svc.RefundPurchase(ctx, learnerID, p.ID)
	if err != nil {
		t.Fatalf("refund within window and under 20%% should succeed, got: %v", err)
	}

	// Verify purchase revoked
	revokedP, _ := repo.GetPurchaseByID(ctx, p.ID)
	if revokedP.RevokedAt == nil {
		t.Fatalf("expected purchase to be revoked")
	}

	// Verify reversal in creator ledger (-140,000 VND creator share)
	entries, _ := repo.ListLedgerEntriesByCreatorID(ctx, creatorID, 10, 0)
	if len(entries) != 1 {
		t.Fatalf("expected 1 reversal ledger entry, got %d", len(entries))
	}
	if entries[0].Kind != domain.LedgerKindRefund || entries[0].AmountVND != -140000 {
		t.Errorf("expected refund reversal amount -140,000 VND, got %+v", entries[0])
	}

	// 2. Duplicate refund fails
	errDup := svc.RefundPurchase(ctx, learnerID, p.ID)
	if !errors.Is(errDup, domain.ErrAlreadyRefunded) {
		t.Fatalf("expected ErrAlreadyRefunded, got: %v", errDup)
	}

	// 3. Refund with >= 20% progress fails
	learner2 := uuid.New()
	p2, _ := repo.CreatePurchase(ctx, &domain.Purchase{
		UserID:       learner2,
		CourseID:     courseID,
		PricePaidVND: 200000,
	})
	score25 := 25
	progReader.progress = []learningcontract.Progress{
		{ScopeID: courseID, Score: &score25},
	}

	errProg := svc.RefundPurchase(ctx, learner2, p2.ID)
	if !errors.Is(errProg, domain.ErrRefundProgressExceeded) {
		t.Fatalf("expected ErrRefundProgressExceeded, got: %v", errProg)
	}

	// 4. Refund after 7-day window fails
	learner3 := uuid.New()
	p3, _ := repo.CreatePurchase(ctx, &domain.Purchase{
		UserID:       learner3,
		CourseID:     courseID,
		PricePaidVND: 200000,
	})
	p3.GrantedAt = time.Now().Add(-8 * 24 * time.Hour) // 8 days ago
	progReader.progress = nil

	errWindow := svc.RefundPurchase(ctx, learner3, p3.ID)
	if !errors.Is(errWindow, domain.ErrRefundWindowExpired) {
		t.Fatalf("expected ErrRefundWindowExpired, got: %v", errWindow)
	}
}
