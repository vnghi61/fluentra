package service_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/modules/user/service"
)

// Fixtures shared across this package's tests.
const (
	nameNghi          = "Nghi"
	timezoneHoChiMinh = "Asia/Ho_Chi_Minh"
	timezoneUTC       = "UTC"
)

// fakeRepo is an in-memory stand-in for the repository. It exists so the
// service's rules can be tested without a database — and, more usefully, so
// the number of reads a use case performs can be counted, which is the
// acceptance criterion for the batched contract call.
type fakeRepo struct {
	mu sync.Mutex

	users       map[uuid.UUID]domain.User
	profiles    map[uuid.UUID]domain.Profile
	preferences map[uuid.UUID]domain.Preferences
	summaries   map[uuid.UUID]domain.Summary
	exports     map[uuid.UUID]domain.ExportRequest
	deletions   map[uuid.UUID]domain.DeletionRequest
	avatars     map[string]domain.AvatarAsset

	// calls counts every repository method by name. A test asserting "one
	// query for N ids" asserts on this.
	calls map[string]int

	failOn map[string]error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:       map[uuid.UUID]domain.User{},
		profiles:    map[uuid.UUID]domain.Profile{},
		preferences: map[uuid.UUID]domain.Preferences{},
		summaries:   map[uuid.UUID]domain.Summary{},
		exports:     map[uuid.UUID]domain.ExportRequest{},
		avatars:     map[string]domain.AvatarAsset{},
		deletions:   map[uuid.UUID]domain.DeletionRequest{},
		calls:       map[string]int{},
		failOn:      map[string]error{},
	}
}

func (f *fakeRepo) record(method string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[method]++
	return f.failOn[method]
}

func (f *fakeRepo) callCount(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[method]
}

func (f *fakeRepo) GetUser(_ context.Context, id uuid.UUID) (domain.User, error) {
	if err := f.record("GetUser"); err != nil {
		return domain.User{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.users[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return user, nil
}

func (f *fakeRepo) Exists(_ context.Context, id uuid.UUID) (bool, error) {
	if err := f.record("Exists"); err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.users[id]
	return ok, nil
}

func (f *fakeRepo) CreateUser(_ context.Context, id uuid.UUID, email string, status domain.Status) (
	domain.User, error,
) {
	if err := f.record("CreateUser"); err != nil {
		return domain.User{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.users {
		if equalFold(existing.Email, email) {
			return domain.User{}, domain.ErrEmailAlreadyRegistered
		}
	}
	user := domain.User{ID: id, Email: email, Status: status}
	f.users[id] = user
	return user, nil
}

func (f *fakeRepo) GetProfile(_ context.Context, userID uuid.UUID) (domain.Profile, error) {
	if err := f.record("GetProfile"); err != nil {
		return domain.Profile{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	profile, ok := f.profiles[userID]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	return profile, nil
}

func (f *fakeRepo) CreateProfile(_ context.Context, id, userID uuid.UUID, profile domain.Profile) (
	domain.Profile, error,
) {
	if err := f.record("CreateProfile"); err != nil {
		return domain.Profile{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	profile.ID = id
	profile.UserID = userID
	f.profiles[userID] = profile
	return profile, nil
}

func (f *fakeRepo) UpdateProfile(_ context.Context, userID uuid.UUID, change domain.ProfileChange) (
	domain.Profile, error,
) {
	if err := f.record("UpdateProfile"); err != nil {
		return domain.Profile{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	profile, ok := f.profiles[userID]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	// The same COALESCE semantics as the SQL: a nil field is left alone.
	if change.DisplayName != nil {
		profile.DisplayName = *change.DisplayName
	}
	if change.Country != nil {
		profile.Country = change.Country
	}
	if change.Timezone != nil {
		profile.Timezone = *change.Timezone
	}
	if change.DateOfBirth != nil {
		profile.DateOfBirth = change.DateOfBirth
	}
	f.profiles[userID] = profile
	return profile, nil
}

func (f *fakeRepo) UpdateProfileAvatar(_ context.Context, userID uuid.UUID, avatarAssetID *uuid.UUID) (
	domain.Profile, error,
) {
	if err := f.record("UpdateProfileAvatar"); err != nil {
		return domain.Profile{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	profile, ok := f.profiles[userID]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	profile.AvatarAssetID = avatarAssetID
	profile.UpdatedAt = testNow
	f.profiles[userID] = profile
	return profile, nil
}

func (f *fakeRepo) GetPreferences(_ context.Context, userID uuid.UUID) (domain.Preferences, error) {
	if err := f.record("GetPreferences"); err != nil {
		return domain.Preferences{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	preferences, ok := f.preferences[userID]
	if !ok {
		return domain.Preferences{}, domain.ErrPreferencesNotFound
	}
	return preferences, nil
}

func (f *fakeRepo) CreatePreferences(_ context.Context, id, userID uuid.UUID) (domain.Preferences, error) {
	if err := f.record("CreatePreferences"); err != nil {
		return domain.Preferences{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	preferences := domain.Preferences{
		ID:                   id,
		UserID:               userID,
		Locale:               domain.DefaultLocale,
		Theme:                domain.ThemeSystem,
		DailyGoalMinutes:     15,
		NotificationChannels: []domain.Channel{domain.ChannelInApp, domain.ChannelEmail},
	}
	f.preferences[userID] = preferences
	return preferences, nil
}

func (f *fakeRepo) ReplacePreferences(_ context.Context, preferences domain.Preferences) (
	domain.Preferences, error,
) {
	if err := f.record("ReplacePreferences"); err != nil {
		return domain.Preferences{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.preferences[preferences.UserID]
	if !ok {
		return domain.Preferences{}, domain.ErrPreferencesNotFound
	}
	preferences.ID = existing.ID
	f.preferences[preferences.UserID] = preferences
	return preferences, nil
}

func (f *fakeRepo) GetSummary(_ context.Context, id uuid.UUID) (domain.Summary, error) {
	if err := f.record("GetSummary"); err != nil {
		return domain.Summary{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	summary, ok := f.summaries[id]
	if !ok {
		return domain.Summary{}, domain.ErrUserNotFound
	}
	return summary, nil
}

func (f *fakeRepo) ListSummaries(_ context.Context, ids []uuid.UUID) ([]domain.Summary, error) {
	if err := f.record("ListSummaries"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	found := make([]domain.Summary, 0, len(ids))
	for _, id := range ids {
		if summary, ok := f.summaries[id]; ok {
			found = append(found, summary)
		}
	}
	return found, nil
}

// WithTx returns the same fake. There is no transaction to model: what the
// tests here check is which methods a use case calls and in what order, and a
// fake that pretended to roll back would be testing itself.
func (f *fakeRepo) WithTx(pgx.Tx) service.Repository { return f }

// GetUserByEmail matches case-insensitively, because the real column is citext
// and a fake that matched exactly would let a case bug through.
func (f *fakeRepo) GetUserByEmail(_ context.Context, email string) (domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls["GetUserByEmail"]++
	if err := f.failOn["GetUserByEmail"]; err != nil {
		return domain.User{}, err
	}
	for _, user := range f.users {
		if strings.EqualFold(user.Email, email) {
			return user, nil
		}
	}
	return domain.User{}, domain.ErrUserNotFound
}

func (f *fakeRepo) MarkEmailVerified(_ context.Context, userID uuid.UUID) (domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls["MarkEmailVerified"]++
	if err := f.failOn["MarkEmailVerified"]; err != nil {
		return domain.User{}, err
	}
	user, ok := f.users[userID]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	// Idempotent, like the query's COALESCE: the first timestamp is the one the
	// audit trail already recorded.
	if user.EmailVerifiedAt == nil {
		verifiedAt := time.Now().UTC()
		user.EmailVerifiedAt = &verifiedAt
		f.users[userID] = user
	}
	return user, nil
}

func (f *fakeRepo) PurgeUnverifiedBefore(_ context.Context, cutoff time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls["PurgeUnverifiedBefore"]++
	if err := f.failOn["PurgeUnverifiedBefore"]; err != nil {
		return 0, err
	}
	removed := 0
	for id, user := range f.users {
		if user.EmailVerifiedAt == nil && user.Status == domain.StatusActive && user.CreatedAt.Before(cutoff) {
			delete(f.users, id)
			removed++
		}
	}
	return removed, nil
}

// fakeBeginner satisfies dbx.Beginner with a transaction that records nothing
// and always commits.
type fakeBeginner struct {
	beginErr  error
	commitErr error
	begins    int
	rollbacks int
}

func (b *fakeBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	if b.beginErr != nil {
		return nil, b.beginErr
	}
	b.begins++
	return &fakeTx{owner: b}, nil
}

// fakeTx implements just enough of pgx.Tx to be handed to the service. The
// methods the service never calls panic rather than returning zero values, so
// a future change that starts using one fails loudly here instead of silently
// doing nothing.
type fakeTx struct {
	owner *fakeBeginner
}

func (t *fakeTx) Begin(context.Context) (pgx.Tx, error) {
	panic("nested transactions are not modelled")
}
func (t *fakeTx) Commit(context.Context) error   { return t.owner.commitErr }
func (t *fakeTx) Rollback(context.Context) error { t.owner.rollbacks++; return nil }
func (t *fakeTx) Conn() *pgx.Conn                { return nil }

func (t *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("CopyFrom is not modelled")
}
func (t *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	panic("SendBatch is not modelled")
}
func (t *fakeTx) LargeObjects() pgx.LargeObjects { panic("LargeObjects is not modelled") }

func (t *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("Prepare is not modelled")
}

func (t *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("Query is not modelled")
}
func (t *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row { panic("QueryRow is not modelled") }

// recordingEvents captures what the service published, so a test can assert
// that a write produced exactly one event with the right payload — the thing
// the WP1 gate depends on.
type recordingEvents struct {
	mu       sync.Mutex
	written  []writtenEvent
	writeErr error
}

type writtenEvent struct {
	Aggregate string
	Event     string
	Payload   any
}

func (r *recordingEvents) Write(
	_ context.Context, tx service.OutboxTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	if tx == nil {
		return uuid.Nil, errors.New("the outbox was handed a nil transaction")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writeErr != nil {
		return uuid.Nil, r.writeErr
	}
	r.written = append(r.written, writtenEvent{Aggregate: aggregate, Event: event, Payload: payload})
	return uuid.New(), nil
}

func (r *recordingEvents) events() []writtenEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]writtenEvent(nil), r.written...)
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if lower(a[index]) != lower(b[index]) {
			return false
		}
	}
	return true
}

func lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

func (f *fakeRepo) CreateExportRequest(_ context.Context, id, userID uuid.UUID) (domain.ExportRequest, error) {
	if err := f.record("CreateExportRequest"); err != nil {
		return domain.ExportRequest{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	req := domain.ExportRequest{
		ID:        id,
		UserID:    userID,
		Status:    domain.ExportStatusPending,
		CreatedAt: time.Now().UTC(),
	}
	f.exports[id] = req
	return req, nil
}

func (f *fakeRepo) GetPendingExportForUser(_ context.Context, userID uuid.UUID) (domain.ExportRequest, bool, error) {
	if err := f.record("GetPendingExportForUser"); err != nil {
		return domain.ExportRequest{}, false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, req := range f.exports {
		if req.UserID == userID && (req.Status == domain.ExportStatusPending || req.Status == domain.ExportStatusProcessing) {
			return req, true, nil
		}
	}
	return domain.ExportRequest{}, false, nil
}

func (f *fakeRepo) GetExportByID(_ context.Context, id uuid.UUID) (domain.ExportRequest, error) {
	if err := f.record("GetExportByID"); err != nil {
		return domain.ExportRequest{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	req, ok := f.exports[id]
	if !ok {
		return domain.ExportRequest{}, domain.ErrUserNotFound
	}
	return req, nil
}

func (f *fakeRepo) UpdateExportStatus(
	_ context.Context,
	id uuid.UUID,
	status domain.ExportStatus,
	startedAt, completedAt, expiresAt *time.Time,
	objectKey, errorMessage *string,
) error {
	if err := f.record("UpdateExportStatus"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	req, ok := f.exports[id]
	if !ok {
		return domain.ErrUserNotFound
	}
	req.Status = status
	if startedAt != nil {
		req.StartedAt = startedAt
	}
	if completedAt != nil {
		req.CompletedAt = completedAt
	}
	if expiresAt != nil {
		req.ExpiresAt = expiresAt
	}
	if objectKey != nil {
		req.ObjectKey = objectKey
	}
	if errorMessage != nil {
		req.ErrorMessage = errorMessage
	}
	f.exports[id] = req
	return nil
}

func (f *fakeRepo) GetExpiredExports(_ context.Context, limit int32) ([]domain.ExportRequest, error) {
	if err := f.record("GetExpiredExports"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.ExportRequest
	now := time.Now().UTC()
	for _, req := range f.exports {
		if req.Status == domain.ExportStatusCompleted && req.ExpiresAt != nil && req.ExpiresAt.Before(now) {
			result = append(result, req)
			if int64(len(result)) >= int64(limit) {
				break
			}
		}
	}
	return result, nil
}

func (f *fakeRepo) DeleteExport(_ context.Context, id uuid.UUID) error {
	if err := f.record("DeleteExport"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.exports, id)
	return nil
}

func (f *fakeRepo) CreateDeletionRequest(_ context.Context, id, userID uuid.UUID, executeAt time.Time) (
	domain.DeletionRequest, error,
) {
	if err := f.record("CreateDeletionRequest"); err != nil {
		return domain.DeletionRequest{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	req := domain.DeletionRequest{
		ID:          id,
		UserID:      userID,
		Status:      domain.DeletionStatusPending,
		RequestedAt: time.Now().UTC(),
		ExecuteAt:   executeAt,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	f.deletions[id] = req
	return req, nil
}

func (f *fakeRepo) GetPendingDeletionForUser(_ context.Context, userID uuid.UUID) (
	domain.DeletionRequest, bool, error,
) {
	if err := f.record("GetPendingDeletionForUser"); err != nil {
		return domain.DeletionRequest{}, false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, req := range f.deletions {
		if req.UserID == userID && (req.Status == domain.DeletionStatusPending ||
			req.Status == domain.DeletionStatusProcessing) {
			return req, true, nil
		}
	}
	return domain.DeletionRequest{}, false, nil
}

func (f *fakeRepo) GetDeletionByID(_ context.Context, id uuid.UUID) (domain.DeletionRequest, error) {
	if err := f.record("GetDeletionByID"); err != nil {
		return domain.DeletionRequest{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	req, ok := f.deletions[id]
	if !ok {
		return domain.DeletionRequest{}, domain.ErrDeletionNotFound
	}
	return req, nil
}

func (f *fakeRepo) CancelDeletion(_ context.Context, id uuid.UUID, cancelledAt time.Time) error {
	if err := f.record("CancelDeletion"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	req, ok := f.deletions[id]
	if !ok || req.Status != domain.DeletionStatusPending {
		return domain.ErrDeletionNotCancellable
	}
	req.Status = domain.DeletionStatusCancelled
	req.CancelledAt = &cancelledAt
	req.UpdatedAt = time.Now().UTC()
	f.deletions[id] = req
	return nil
}

func (f *fakeRepo) UpdateDeletionStatus(
	_ context.Context, id uuid.UUID, status domain.DeletionStatus,
	startedAt, completedAt *time.Time, errorMessage *string,
) error {
	if err := f.record("UpdateDeletionStatus"); err != nil {
		return err
	}
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
	req.UpdatedAt = time.Now().UTC()
	f.deletions[id] = req
	return nil
}

func (f *fakeRepo) GetDueDeletions(_ context.Context, cutoff time.Time, limit int32) ([]domain.DeletionRequest, error) {
	if err := f.record("GetDueDeletions"); err != nil {
		return nil, err
	}
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

func (f *fakeRepo) AnonymiseUser(_ context.Context, userID uuid.UUID, anonymisedEmail string) error {
	if err := f.record("AnonymiseUser"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.users[userID]
	if !ok {
		return domain.ErrUserNotFound
	}
	user.Email = anonymisedEmail
	user.Status = domain.StatusDeleted
	user.UpdatedAt = time.Now().UTC()
	f.users[userID] = user
	return nil
}

func (f *fakeRepo) AnonymiseProfile(_ context.Context, userID uuid.UUID) error {
	if err := f.record("AnonymiseProfile"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	profile, ok := f.profiles[userID]
	if !ok {
		return nil
	}
	profile.DisplayName = "Deleted User"
	profile.AvatarAssetID = nil
	profile.Country = nil
	profile.Timezone = timezoneUTC
	profile.DateOfBirth = nil
	profile.UpdatedAt = time.Now().UTC()
	f.profiles[userID] = profile
	return nil
}

func (f *fakeRepo) DeletePreferences(_ context.Context, userID uuid.UUID) error {
	if err := f.record("DeletePreferences"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.preferences, userID)
	return nil
}

func (f *fakeRepo) DeleteLearningProfile(_ context.Context, _ uuid.UUID) error {
	if err := f.record("DeleteLearningProfile"); err != nil {
		return err
	}
	return nil
}

func (f *fakeRepo) UpdateUserStatus(_ context.Context, userID uuid.UUID, status domain.Status) error {
	if err := f.record("UpdateUserStatus"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.users[userID]
	if !ok {
		return domain.ErrUserNotFound
	}
	user.Status = status
	user.UpdatedAt = time.Now().UTC()
	f.users[userID] = user
	return nil
}

func (f *fakeRepo) SearchUsersAdmin(
	_ context.Context,
	_ contract.UserFilter,
	_ *uuid.UUID,
	_ *time.Time,
	_ int,
) ([]service.SearchUserRow, error) {
	if err := f.record("SearchUsersAdmin"); err != nil {
		return nil, err
	}
	return nil, nil
}

type fakeEnqueuer struct {
	mu       sync.Mutex
	enqueued []uuid.UUID
	err      error
}

func (f *fakeEnqueuer) EnqueueExportTx(_ context.Context, _ pgx.Tx, exportID, _ uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.enqueued = append(f.enqueued, exportID)
	return nil
}

// newUserFixture is a registration payload that passes validation, so a test
// that wants to break one field can start from something that works.
func newUserFixture() contract.NewUser {
	return contract.NewUser{
		Email:       "new@example.com",
		DisplayName: "New Learner",
		Locale:      "en",
		Timezone:    timezoneHoChiMinh,
	}
}

// avatarKey identifies one stored variant the way the primary key does.
func avatarKey(assetID uuid.UUID, variant domain.AvatarVariant) string {
	return assetID.String() + "/" + string(variant)
}

func (f *fakeRepo) InsertAvatarAsset(_ context.Context, asset domain.AvatarAsset) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["InsertAvatarAsset"]++
	if err := f.failOn["InsertAvatarAsset"]; err != nil {
		return err
	}
	f.avatars[avatarKey(asset.AssetID, asset.Variant)] = asset
	return nil
}

func (f *fakeRepo) GetAvatarAsset(
	_ context.Context, assetID uuid.UUID, variant domain.AvatarVariant,
) (domain.AvatarAsset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["GetAvatarAsset"]++
	if err := f.failOn["GetAvatarAsset"]; err != nil {
		return domain.AvatarAsset{}, err
	}
	asset, ok := f.avatars[avatarKey(assetID, variant)]
	if !ok {
		return domain.AvatarAsset{}, domain.ErrAvatarAssetNotFound
	}
	return asset, nil
}

func (f *fakeRepo) DeleteAvatarAssetsByAssetID(_ context.Context, assetID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["DeleteAvatarAssetsByAssetID"]++
	if err := f.failOn["DeleteAvatarAssetsByAssetID"]; err != nil {
		return err
	}
	for _, variant := range []domain.AvatarVariant{
		domain.AvatarVariantSmall, domain.AvatarVariantMedium, domain.AvatarVariantLarge,
	} {
		delete(f.avatars, avatarKey(assetID, variant))
	}
	return nil
}
