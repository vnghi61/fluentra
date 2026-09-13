// Package service implements the business logic for listening plays and transcripts.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/listening/domain"
	"github.com/fluentra/fluentra/internal/platform/media"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// Repository specifies the database persistence methods required by the service.
type Repository interface {
	CountPlays(ctx context.Context, userID, contentVersionID, contextID uuid.UUID) (int64, error)
	RecordPlay(
		ctx context.Context,
		id, userID, contentVersionID uuid.UUID,
		contextType string,
		contextID uuid.UUID,
	) (domain.PlayRecord, error)
}

// ContentReader narrows contentcontract.Reader to what the listening service needs.
type ContentReader interface {
	GetVersion(ctx context.Context, id uuid.UUID) (*contentcontract.Version, error)
}

// AttemptReader reads learning attempt state to verify eligibility for transcript release.
type AttemptReader interface {
	GetAttemptForGrading(ctx context.Context, attemptID uuid.UUID) (*learningcontract.AttemptDetail, error)
}

// StorageSigner generates short-lived presigned URLs for audio playback.
type StorageSigner interface {
	PresignGet(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
}

// Deps holds the dependencies required to construct a listening Service.
type Deps struct {
	Repo     Repository
	Content  ContentReader
	Learning AttemptReader
	Storage  StorageSigner
	Clock    clock.Clock
}

// Service orchestrates play limits, presigned playback URLs, and transcript access.
type Service struct {
	repo     Repository
	content  ContentReader
	learning AttemptReader
	storage  StorageSigner
	clock    clock.Clock
}

// New constructs a listening Service.
func New(deps Deps) *Service {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}
	return &Service{
		repo:     deps.Repo,
		content:  deps.Content,
		learning: deps.Learning,
		storage:  deps.Storage,
		clock:    timekeeper,
	}
}

type listeningBody struct {
	Script          string                              `json:"script,omitempty"`
	Voice           string                              `json:"voice,omitempty"`
	AudioObjectKey  string                              `json:"audio_object_key,omitempty"`
	DurationSeconds int                                 `json:"duration_seconds,omitempty"`
	Prompt          string                              `json:"prompt,omitempty"`
	CorrectAnswer   string                              `json:"correct_answer,omitempty"`
	CorrectOptionID string                              `json:"correct_option_id,omitempty"`
	Acceptable      []string                            `json:"acceptable,omitempty"`
	Explanation     *learningcontract.AnswerExplanation `json:"explanation,omitempty"`
	Questions       []contentcontract.QuestionItem      `json:"questions,omitempty"`
}

// RecordPlay enforces the play limit and returns a short-lived presigned URL.
func (s *Service) RecordPlay(
	ctx context.Context,
	userID, versionID uuid.UUID,
	contextType string,
	contextID uuid.UUID,
) (*domain.PlayResult, error) {
	if contextType != domain.ContextTypeAttempt && contextType != domain.ContextTypeExam {
		return nil, domain.ErrInvalidContext
	}

	version, err := s.content.GetVersion(ctx, versionID)
	if err != nil || version == nil {
		return nil, domain.ErrItemNotFound
	}
	if version.Kind != domain.KindListeningComprehension {
		return nil, domain.ErrItemNotFound
	}

	var body listeningBody
	if len(version.Body) > 0 {
		if err := json.Unmarshal(version.Body, &body); err != nil {
			return nil, fmt.Errorf("unmarshal listening body: %w", err)
		}
	}

	audioKey := strings.TrimSpace(body.AudioObjectKey)
	if audioKey == "" && strings.TrimSpace(body.Script) != "" {
		voice := body.Voice
		if voice == "" {
			voice = "en_US-lessac-medium"
		}
		audioKey = fmt.Sprintf("tts/%s/%s.mp3", voice, media.HashText(body.Script))
	}
	if audioKey == "" {
		return nil, domain.ErrAudioNotReady
	}

	maxAllowed := domain.MaxPlays(contextType)
	existingCount, err := s.repo.CountPlays(ctx, userID, versionID, contextID)
	if err != nil {
		return nil, err
	}
	if existingCount >= int64(maxAllowed) {
		return nil, domain.ErrPlayLimitReached
	}

	playID := uuid.Must(uuid.NewV7())
	if _, err := s.repo.RecordPlay(ctx, playID, userID, versionID, contextType, contextID); err != nil {
		return nil, err
	}

	clipSeconds := body.DurationSeconds
	if clipSeconds <= 0 {
		clipSeconds = 60
	}
	expiry := time.Duration(clipSeconds+60) * time.Second
	if expiry < 2*time.Minute {
		expiry = 2 * time.Minute
	}

	audioURL, err := s.storage.PresignGet(ctx, storage.BucketMedia, audioKey, expiry)
	if err != nil {
		return nil, fmt.Errorf("presign audio get: %w", err)
	}

	return &domain.PlayResult{
		AudioURL:     audioURL,
		PlaysUsed:    int(existingCount) + 1,
		PlaysAllowed: maxAllowed,
		ExpiresAt:    s.clock.Now().Add(expiry),
	}, nil
}

// GetTranscript returns the script only if the caller owns a graded attempt for this content version.
func (s *Service) GetTranscript(
	ctx context.Context,
	userID, versionID, attemptID uuid.UUID,
) (string, error) {
	if s.learning == nil {
		return "", domain.ErrTranscriptLocked
	}

	attempt, err := s.learning.GetAttemptForGrading(ctx, attemptID)
	if err != nil || attempt == nil {
		return "", domain.ErrTranscriptLocked
	}

	if attempt.UserID != userID || attempt.ContentVersionID != versionID {
		return "", domain.ErrTranscriptLocked
	}

	// Must be graded or completed before transcript is released (ADR-0025)
	if attempt.Status != "graded" && attempt.Status != "completed" {
		return "", domain.ErrTranscriptLocked
	}

	version, err := s.content.GetVersion(ctx, versionID)
	if err != nil || version == nil {
		return "", domain.ErrItemNotFound
	}
	if version.Kind != domain.KindListeningComprehension {
		return "", domain.ErrItemNotFound
	}

	var body listeningBody
	if len(version.Body) > 0 {
		if err := json.Unmarshal(version.Body, &body); err != nil {
			return "", fmt.Errorf("unmarshal listening body: %w", err)
		}
	}

	return body.Script, nil
}
