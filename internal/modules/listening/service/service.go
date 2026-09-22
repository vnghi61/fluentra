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

// SittingPlayPolicy says how many plays a clip has in an exam sitting. cmd/api
// implements it with the exam module, which listening may not import.
type SittingPlayPolicy interface {
	ListeningPlayPolicy(ctx context.Context, userID, sittingID, versionID uuid.UUID) (int, error)
}

// PlacementPlayPolicy answers the play limit for a clip in a placement test:
// the context must be the caller's open placement session serving that clip.
type PlacementPlayPolicy interface {
	PlacementListeningPlays(ctx context.Context, userID, sessionID, versionID uuid.UUID) (int, error)
}

// AudioLocator finds the rendered clip for a script in the TTS cache.
type AudioLocator interface {
	AudioKey(ctx context.Context, script, voice string) (objectKey string, found bool, err error)
}

// StorageSigner generates short-lived presigned URLs for audio playback.
type StorageSigner interface {
	PresignGet(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
}

// Deps holds the dependencies required to construct a listening Service.
type Deps struct {
	Repo      Repository
	Content   ContentReader
	Learning  AttemptReader
	Storage   StorageSigner
	Sittings  SittingPlayPolicy
	Placement PlacementPlayPolicy
	Audio     AudioLocator
	Clock     clock.Clock
}

// Service orchestrates play limits, presigned playback URLs, and transcript access.
type Service struct {
	repo      Repository
	content   ContentReader
	learning  AttemptReader
	storage   StorageSigner
	sittings  SittingPlayPolicy
	placement PlacementPlayPolicy
	audio     AudioLocator
	clock     clock.Clock
}

// New constructs a listening Service.
func New(deps Deps) *Service {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}
	return &Service{
		repo:      deps.Repo,
		content:   deps.Content,
		learning:  deps.Learning,
		storage:   deps.Storage,
		sittings:  deps.Sittings,
		placement: deps.Placement,
		audio:     deps.Audio,
		clock:     timekeeper,
	}
}

type listeningBody struct {
	Script          string                              `json:"script,omitempty"`
	Voice           string                              `json:"voice,omitempty"`
	AudioObjectKey  string                              `json:"audio_object_key,omitempty"`
	DurationSeconds int                                 `json:"duration_seconds,omitempty"`
	Prompt          string                              `json:"prompt,omitempty"`
	ImageURL        string                              `json:"image_url,omitempty"`
	AudioURL        string                              `json:"audio_url,omitempty"`
	CorrectAnswer   string                              `json:"correct_answer,omitempty"`
	CorrectOptionID string                              `json:"correct_option_id,omitempty"`
	Key             string                              `json:"key,omitempty"`
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
	switch contextType {
	case domain.ContextTypeAttempt, domain.ContextTypeExam, domain.ContextTypePlacement:
	default:
		return nil, domain.ErrInvalidContext
	}

	body, err := s.loadBody(ctx, versionID)
	if err != nil {
		return nil, err
	}

	audioKey, err := s.audioKey(ctx, body)
	if err != nil {
		return nil, err
	}

	maxAllowed, err := s.playsAllowed(ctx, userID, versionID, contextType, contextID)
	if err != nil {
		return nil, err
	}
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

// loadBody reads a listening item's full body; anything that is not a listening item is not found.
func (s *Service) loadBody(ctx context.Context, versionID uuid.UUID) (listeningBody, error) {
	var body listeningBody
	version, err := s.content.GetVersion(ctx, versionID)
	if err != nil || version == nil || version.Kind != domain.KindListeningComprehension {
		return body, domain.ErrItemNotFound
	}
	if len(version.Body) > 0 {
		if err := json.Unmarshal(version.Body, &body); err != nil {
			return body, fmt.Errorf("unmarshal listening body: %w", err)
		}
	}
	return body, nil
}

// audioKey is the clip's object: named in the body, or found in the TTS cache,
// where the offline renderer records it without touching the published body.
func (s *Service) audioKey(ctx context.Context, body listeningBody) (string, error) {
	if key := strings.TrimSpace(body.AudioObjectKey); key != "" {
		return key, nil
	}
	if s.audio == nil || strings.TrimSpace(body.Script) == "" {
		return "", domain.ErrAudioNotReady
	}
	key, found, err := s.audio.AudioKey(ctx, body.Script, body.Voice)
	if err != nil {
		return "", fmt.Errorf("look up listening audio: %w", err)
	}
	if !found {
		return "", domain.ErrAudioNotReady
	}
	return key, nil
}

// playsAllowed checks that the context belongs to the caller and holds the clip,
// and returns its play limit.
//
// The context is what the limit counts against, so it cannot be taken on the
// client's word: a new random context_id per request would be a fresh set of
// plays every time, and the limit would be no limit.
func (s *Service) playsAllowed(
	ctx context.Context, userID, versionID uuid.UUID, contextType string, contextID uuid.UUID,
) (int, error) {
	switch contextType {
	case domain.ContextTypeExam:
		if s.sittings == nil {
			return 0, domain.ErrPlayNotAllowed
		}
		return s.sittings.ListeningPlayPolicy(ctx, userID, contextID, versionID)
	case domain.ContextTypePlacement:
		if s.placement == nil {
			return 0, domain.ErrPlayNotAllowed
		}
		plays, err := s.placement.PlacementListeningPlays(ctx, userID, contextID, versionID)
		if err != nil {
			return 0, domain.ErrPlayNotAllowed
		}
		return plays, nil
	}

	if s.learning == nil {
		return 0, domain.ErrPlayNotAllowed
	}
	attempt, err := s.learning.GetAttemptForGrading(ctx, contextID)
	if err != nil || attempt == nil {
		return 0, domain.ErrPlayNotAllowed
	}
	if attempt.UserID != userID || attempt.ContentVersionID != versionID || attempt.Status != attemptInProgress {
		return 0, domain.ErrPlayNotAllowed
	}
	return domain.MaxAttemptPlays, nil
}

const attemptInProgress = "in_progress"

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
