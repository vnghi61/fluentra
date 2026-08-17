package job_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	userjob "github.com/fluentra/fluentra/internal/modules/user/job"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

const testEmail = "learner@example.com"

type fakeDeletionRepo struct {
	mu          sync.Mutex
	users       map[uuid.UUID]domain.User
	profiles    map[uuid.UUID]domain.Profile
	preferences map[uuid.UUID]domain.Preferences
	deletions   map[uuid.UUID]domain.DeletionRequest
}

func newFakeDeletionRepo() *fakeDeletionRepo {
	return &fakeDeletionRepo{
		users:       map[uuid.UUID]domain.User{},
		profiles:    map[uuid.UUID]domain.Profile{},
		preferences: map[uuid.UUID]domain.Preferences{},
		deletions:   map[uuid.UUID]domain.DeletionRequest{},
	}
}

func (f *fakeDeletionRepo) WithTx(_ pgx.Tx) userjob.DeletionRepository {
	return f
}

func (f *fakeDeletionRepo) GetDueDeletions(
	_ context.Context, cutoff time.Time, limit int32,
) ([]domain.DeletionRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.DeletionRequest
	for _, req := range f.deletions {
		if req.Status == domain.DeletionStatusPending && (req.ExecuteAt.Before(cutoff) || req.ExecuteAt.Equal(cutoff)) {
			result = append(result, req)
			if len(result) >= int(limit) {
				break
			}
		}
	}
	return result, nil
}

func (f *fakeDeletionRepo) GetProfile(_ context.Context, userID uuid.UUID) (domain.Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	profile, ok := f.profiles[userID]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	return profile, nil
}

func (f *fakeDeletionRepo) UpdateDeletionStatus(
	_ context.Context, id uuid.UUID, status domain.DeletionStatus,
	startedAt, completedAt *time.Time, errorMessage *string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	req, ok := f.deletions[id]
	if !ok {
		return domain.ErrDeletionNotFound
	}
	req.Status = status
	if startedAt != nil {
		req.StartedAt = startedAt
	}
	if completedAt != nil {
		req.CompletedAt = completedAt
	}
	if errorMessage != nil {
		req.ErrorMessage = errorMessage
	}
	f.deletions[id] = req
	return nil
}

func (f *fakeDeletionRepo) AnonymiseUser(_ context.Context, userID uuid.UUID, anonymisedEmail string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.users[userID]
	if !ok {
		return domain.ErrUserNotFound
	}
	user.Email = anonymisedEmail
	user.Status = domain.StatusDeleted
	f.users[userID] = user
	return nil
}

func (f *fakeDeletionRepo) AnonymiseProfile(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	profile, ok := f.profiles[userID]
	if !ok {
		return nil
	}
	profile.DisplayName = "Deleted User"
	profile.AvatarAssetID = nil
	profile.Country = nil
	profile.Timezone = "UTC"
	profile.DateOfBirth = nil
	f.profiles[userID] = profile
	return nil
}

func (f *fakeDeletionRepo) DeletePreferences(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.preferences, userID)
	return nil
}

func (f *fakeDeletionRepo) DeleteLearningProfile(_ context.Context, _ uuid.UUID) error {
	return nil
}

type fakeEventWriter struct {
	mu     sync.Mutex
	events []writtenEvent
}

type writtenEvent struct {
	Aggregate string
	Topic     string
	Payload   any
}

func (f *fakeEventWriter) Write(
	_ context.Context, _ outbox.DBTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, writtenEvent{
		Aggregate: aggregate,
		Topic:     event,
		Payload:   payload,
	})
	return uuid.New(), nil
}

type fakeBeginner struct{}

func (b *fakeBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return &fakeTx{}, nil
}

type fakeTx struct{}

func (t *fakeTx) Begin(context.Context) (pgx.Tx, error) { panic("nested transactions not supported") }
func (t *fakeTx) Commit(context.Context) error          { return nil }
func (t *fakeTx) Rollback(context.Context) error        { return nil }
func (t *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("not supported")
}
func (t *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("not supported") }
func (t *fakeTx) LargeObjects() pgx.LargeObjects                         { panic("not supported") }
func (t *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("not supported")
}
func (t *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (t *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("not supported")
}
func (t *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("not supported")
}
func (t *fakeTx) Conn() *pgx.Conn { return nil }

func TestDeletionExecutor_DueExecution(t *testing.T) {
	t.Parallel()
	repo := newFakeDeletionRepo()
	events := &fakeEventWriter{}
	userID := uuid.New()
	deletionID := uuid.New()

	now := time.Now().UTC()
	past := now.Add(-1 * time.Hour)

	repo.users[userID] = domain.User{
		ID:     userID,
		Email:  testEmail,
		Status: domain.StatusPendingDeletion,
	}
	repo.profiles[userID] = domain.Profile{
		UserID:      userID,
		DisplayName: "Original Learner",
	}
	repo.preferences[userID] = domain.Preferences{
		UserID: userID,
		Locale: "vi",
	}
	repo.deletions[deletionID] = domain.DeletionRequest{
		ID:        deletionID,
		UserID:    userID,
		Status:    domain.DeletionStatusPending,
		ExecuteAt: past,
	}

	executor := userjob.NewDeletionExecutorWithRepo(&fakeBeginner{}, repo, nil, events, "test-bucket")

	if err := executor.Run(context.Background()); err != nil {
		t.Fatalf("executor.Run: %v", err)
	}

	// 1. Assert user anonymised
	user := repo.users[userID]
	expectedEmail := fmt.Sprintf("deleted-%s@anonymised.invalid", userID)
	if user.Email != expectedEmail {
		t.Errorf("got email %q, want %q", user.Email, expectedEmail)
	}
	if user.Status != domain.StatusDeleted {
		t.Errorf("got status %s, want deleted", user.Status)
	}

	// 2. Assert profile anonymised
	profile := repo.profiles[userID]
	if profile.DisplayName != "Deleted User" {
		t.Errorf("got display name %q, want 'Deleted User'", profile.DisplayName)
	}

	// 3. Assert preferences purged
	if _, exists := repo.preferences[userID]; exists {
		t.Errorf("expected preferences to be purged")
	}

	// 4. Assert deletion status set to processing (awaiting completeness check)
	deletion := repo.deletions[deletionID]
	if deletion.Status != domain.DeletionStatusProcessing {
		t.Errorf("got deletion status %s, want processing", deletion.Status)
	}

	// 5. Assert user.deleted event published
	events.mu.Lock()
	defer events.mu.Unlock()
	if len(events.events) != 1 {
		t.Fatalf("got %d events, want 1", len(events.events))
	}
	if events.events[0].Topic != contract.EventDeleted {
		t.Errorf("got topic %q, want %q", events.events[0].Topic, contract.EventDeleted)
	}
}

func TestDeletionExecutor_IgnoresNotDue(t *testing.T) {
	t.Parallel()
	repo := newFakeDeletionRepo()
	events := &fakeEventWriter{}
	userID := uuid.New()
	deletionID := uuid.New()

	future := time.Now().UTC().Add(10 * 24 * time.Hour)

	repo.users[userID] = domain.User{
		ID:     userID,
		Email:  testEmail,
		Status: domain.StatusPendingDeletion,
	}
	repo.deletions[deletionID] = domain.DeletionRequest{
		ID:        deletionID,
		UserID:    userID,
		Status:    domain.DeletionStatusPending,
		ExecuteAt: future,
	}

	executor := userjob.NewDeletionExecutorWithRepo(&fakeBeginner{}, repo, nil, events, "test-bucket")

	if err := executor.Run(context.Background()); err != nil {
		t.Fatalf("executor.Run: %v", err)
	}

	// Deletion remains pending
	deletion := repo.deletions[deletionID]
	if deletion.Status != domain.DeletionStatusPending {
		t.Errorf("got status %s, want pending", deletion.Status)
	}
	// User email remains untouched
	user := repo.users[userID]
	if user.Email != testEmail {
		t.Errorf("got email %s, want %s", user.Email, testEmail)
	}
}
