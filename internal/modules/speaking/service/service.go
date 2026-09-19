// Package service implements the business logic for the speaking module.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/speaking/contract"
	"github.com/fluentra/fluentra/internal/modules/speaking/domain"
	"github.com/fluentra/fluentra/internal/modules/speaking/repository"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

var hoChiMinhZone = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)

func startOfLearnerDay(now time.Time) time.Time {
	inZone := now.In(hoChiMinhZone)
	return time.Date(inZone.Year(), inZone.Month(), inZone.Day(), 0, 0, 0, 0, hoChiMinhZone)
}

// AttemptCounter counts daily speaking attempts for rate limiting.
type AttemptCounter interface {
	CountAttemptsTowardLimitSince(ctx context.Context, userID uuid.UUID, grader string, since time.Time) (int, error)
}

// ConsentStore reads and records voice-recording consent. Narrow and injectable
// so the consent gate can be exercised without a database, the way the grader
// narrows its own dependencies.
type ConsentStore interface {
	GetConsent(ctx context.Context, userID uuid.UUID) (*contract.SpeakingConsent, error)
	RecordConsent(ctx context.Context, userID uuid.UUID) (*contract.SpeakingConsent, error)
}

// Deps holds dependencies for the speaking service.
type Deps struct {
	Repo *repository.Repository
	// Defaults to Repo. Supplied separately only by tests.
	Consent    ConsentStore
	Storage    storage.Store
	Counter    AttemptCounter
	Clock      clock.Clock
	DailyLimit int
	Bucket     string
}

// Service manages audio recording intents, GDPR deletions, and retention policies.
type Service struct {
	repo       *repository.Repository
	consent    ConsentStore
	storage    storage.Store
	counter    AttemptCounter
	clock      clock.Clock
	dailyLimit int
	bucket     string
}

// New constructs a new speaking service.
func New(deps Deps) *Service {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}
	limit := deps.DailyLimit
	if limit <= 0 {
		limit = 30
	}
	b := deps.Bucket
	if b == "" {
		b = storage.BucketMedia
	}

	consent := deps.Consent
	if consent == nil && deps.Repo != nil {
		consent = deps.Repo
	}

	return &Service{
		repo:       deps.Repo,
		consent:    consent,
		storage:    deps.Storage,
		counter:    deps.Counter,
		clock:      timekeeper,
		dailyLimit: limit,
		bucket:     b,
	}
}

// UploadIntent generates a presigned PUT URL for browser audio upload to fluentra-media.
func (s *Service) UploadIntent(
	ctx context.Context, userID uuid.UUID, contentType string,
) (*contract.UploadIntentResult, error) {
	if !domain.IsValidAudioFormat(contentType) {
		return nil, domain.ErrUnsupportedAudioFormat
	}

	// BR-SPEAKING-03, enforced where it bites: no presigned URL without consent.
	// Checking it in the browser alone left the rule as a suggestion, since the
	// endpoint answers anyone who asks it directly.
	if s.consent == nil {
		return nil, domain.ErrConsentRequired
	}
	consent, err := s.consent.GetConsent(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read speaking consent: %w", err)
	}
	if !consent.Consented {
		return nil, domain.ErrConsentRequired
	}

	// Daily recordings limit check
	var used int
	if s.counter != nil && s.dailyLimit > 0 {
		startOfDay := startOfLearnerDay(s.clock.Now())
		used, err = s.counter.CountAttemptsTowardLimitSince(ctx, userID, contract.KindSpeakingTask, startOfDay)
		if err != nil {
			return nil, fmt.Errorf("count daily speaking recordings: %w", err)
		}
		if used >= s.dailyLimit {
			return nil, domain.ErrDailyLimitReached
		}
	}

	ext := ".webm"
	lowerCT := strings.ToLower(contentType)
	if strings.Contains(lowerCT, "mp4") {
		ext = ".mp4"
	} else if strings.Contains(lowerCT, "wav") {
		ext = ".wav"
	} else if strings.Contains(lowerCT, "ogg") {
		ext = ".ogg"
	} else if strings.Contains(lowerCT, "mpeg") || strings.Contains(lowerCT, "mp3") {
		ext = ".mp3"
	}

	recordingID := uuid.New().String()
	objectKey := fmt.Sprintf("recordings/%s/%s%s", userID.String(), recordingID, ext)

	intent, err := s.storage.PresignPut(
		ctx,
		s.bucket,
		objectKey,
		contentType,
		15*1024*1024, // 15MB max recording
		storage.DefaultPresignPutExpiry,
	)
	if err != nil {
		return nil, fmt.Errorf("presign speaking recording put: %w", err)
	}

	return &contract.UploadIntentResult{
		UploadURL:            intent.URL,
		ObjectKey:            objectKey,
		ExpiresAt:            intent.ExpiresAt,
		DailyRecordingsUsed:  used,
		DailyRecordingsLimit: s.dailyLimit,
	}, nil
}

// DeleteAttemptRecording deletes the recording audio object on demand while keeping the scores.
func (s *Service) DeleteAttemptRecording(ctx context.Context, attemptID, userID uuid.UUID) error {
	key, err := s.repo.MarkRecordingDeleted(ctx, attemptID, userID)
	if err != nil {
		return err
	}
	if key != "" && s.storage != nil {
		_ = s.storage.Delete(ctx, s.bucket, key)
	}
	return nil
}

// PurgeRecordings sweeps recordings older than the retention duration (90 days).
func (s *Service) PurgeRecordings(ctx context.Context, retention time.Duration) (int, error) {
	if retention <= 0 {
		retention = 90 * 24 * time.Hour
	}
	cutoff := s.clock.Now().Add(-retention)

	rows, err := s.repo.ListRecordingsOlderThan(ctx, cutoff, 100)
	if err != nil {
		return 0, fmt.Errorf("list expired recordings: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		if s.storage != nil && r.RecordingKey != "" {
			_ = s.storage.Delete(ctx, s.bucket, r.RecordingKey)
		}
		ids = append(ids, r.ID)
	}

	if err := s.repo.MarkRecordingsDeletedBatch(ctx, ids); err != nil {
		return 0, fmt.Errorf("mark purged batch: %w", err)
	}

	return len(ids), nil
}

// DeleteUserRecordings purges all audio files for a user upon GDPR account deletion.
func (s *Service) DeleteUserRecordings(ctx context.Context, userID uuid.UUID) error {
	keys, err := s.repo.ListUserRecordingKeys(ctx, userID)
	if err != nil {
		return fmt.Errorf("list user recording keys: %w", err)
	}
	for _, key := range keys {
		if s.storage != nil && key != "" {
			_ = s.storage.Delete(ctx, s.bucket, key)
		}
	}
	return nil
}

// GetSpeakingFeedback retrieves feedback for a user attempt, generating an audio playback URL if still active.
func (s *Service) GetSpeakingFeedback(
	ctx context.Context, attemptID, userID uuid.UUID,
) (*contract.SpeakingFeedback, error) {
	fb, err := s.repo.GetFeedbackForUser(ctx, attemptID, userID)
	if err != nil {
		return nil, err
	}
	if fb.RecordingDeletedAt == nil && fb.RecordingKey != "" && s.storage != nil {
		if presignedURL, err := s.storage.PresignGet(ctx, s.bucket, fb.RecordingKey, 15*time.Minute); err == nil {
			fb.AudioURL = presignedURL
		}
	}
	return fb, nil
}

// ListSpeakingSubmissions retrieves a paginated history of speaking submissions
// for a learner.
//
// It reports the grade the attempt was completed with, read back from the row.
// It used to rebuild one here — averaging the criteria bands, or taking the raw
// read-aloud accuracy — and that number did not match the one the learner was
// shown in the runner, because the grader blends the model score with the
// accuracy before awarding it.
func (s *Service) ListSpeakingSubmissions(
	ctx context.Context, userID uuid.UUID, page, pageSize int,
) (*contract.SpeakingSubmissionList, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	total, err := s.repo.CountSubmissionsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count speaking submissions: %w", err)
	}

	items, err := s.repo.ListSubmissionsByUser(ctx, userID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list speaking submissions: %w", err)
	}

	return &contract.SpeakingSubmissionList{
		Items:    items,
		Total:    int(total),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetConsent reports whether the learner has agreed to be recorded.
func (s *Service) GetConsent(
	ctx context.Context, userID uuid.UUID,
) (*contract.SpeakingConsent, error) {
	if s.consent == nil {
		return &contract.SpeakingConsent{Consented: false}, nil
	}
	return s.consent.GetConsent(ctx, userID)
}

// RecordConsent stores the learner's agreement, with the timestamp
// BR-SPEAKING-03 requires.
func (s *Service) RecordConsent(
	ctx context.Context, userID uuid.UUID,
) (*contract.SpeakingConsent, error) {
	if s.consent == nil {
		return nil, fmt.Errorf("speaking: no consent store configured")
	}
	return s.consent.RecordConsent(ctx, userID)
}
