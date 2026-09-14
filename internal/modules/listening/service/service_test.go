package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/listening/domain"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type fakeRepo struct {
	plays map[string]int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{plays: make(map[string]int64)}
}

func (r *fakeRepo) playKey(userID, versionID, contextID uuid.UUID) string {
	return userID.String() + ":" + versionID.String() + ":" + contextID.String()
}

func (r *fakeRepo) CountPlays(_ context.Context, userID, contentVersionID, contextID uuid.UUID) (int64, error) {
	key := r.playKey(userID, contentVersionID, contextID)
	return r.plays[key], nil
}

func (r *fakeRepo) RecordPlay(
	_ context.Context,
	id, userID, contentVersionID uuid.UUID,
	contextType string,
	contextID uuid.UUID,
) (domain.PlayRecord, error) {
	key := r.playKey(userID, contentVersionID, contextID)
	r.plays[key]++
	return domain.PlayRecord{
		ID:               id,
		UserID:           userID,
		ContentVersionID: contentVersionID,
		ContextType:      contextType,
		ContextID:        contextID,
		PlayedAt:         time.Now(),
	}, nil
}

type fakeContentReader struct {
	versions map[uuid.UUID]*contentcontract.Version
}

func (f *fakeContentReader) GetVersion(_ context.Context, id uuid.UUID) (*contentcontract.Version, error) {
	v, ok := f.versions[id]
	if !ok {
		return nil, nil
	}
	return v, nil
}

type fakeAttemptReader struct {
	attempts map[uuid.UUID]*learningcontract.AttemptDetail
}

func (f *fakeAttemptReader) GetAttemptForGrading(
	_ context.Context, attemptID uuid.UUID,
) (*learningcontract.AttemptDetail, error) {
	a, ok := f.attempts[attemptID]
	if !ok {
		return nil, nil
	}
	return a, nil
}

type fakeStorageSigner struct {
}

func (f *fakeStorageSigner) PresignGet(
	_ context.Context, bucket, objectKey string, expiry time.Duration,
) (string, error) {
	url := "https://media.fluentra.local/" + bucket + "/" + objectKey + "?exp=" + expiry.String()
	return url, nil
}

type fakeSittings struct {
	sittingID uuid.UUID
	plays     int
}

func (f fakeSittings) ListeningPlayPolicy(_ context.Context, _, sittingID, _ uuid.UUID) (int, error) {
	if sittingID != f.sittingID {
		return 0, domain.ErrPlayNotAllowed
	}
	return f.plays, nil
}

type fakePlacementPlays struct {
	sessionID uuid.UUID
}

func (f fakePlacementPlays) PlacementListeningPlays(_ context.Context, _, sessionID, _ uuid.UUID) (int, error) {
	if sessionID != f.sessionID {
		return 0, errors.New("not the caller's open placement session")
	}
	return 1, nil
}

func TestRecordPlay_PlacementPolicy_OnePlayInTheCallersSession(t *testing.T) {
	userID := uuid.New()
	versionID := uuid.New()
	sessionID := uuid.New()
	bodyJSON, _ := json.Marshal(listeningBody{AudioObjectKey: "tts/v/clip.wav"})
	svc := New(Deps{
		Repo: newFakeRepo(),
		Content: &fakeContentReader{versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Kind: domain.KindListeningComprehension, Body: bodyJSON},
		}},
		Storage:   &fakeStorageSigner{},
		Placement: fakePlacementPlays{sessionID: sessionID},
	})
	ctx := context.Background()

	first, err := svc.RecordPlay(ctx, userID, versionID, domain.ContextTypePlacement, sessionID)
	if err != nil {
		t.Fatalf("first placement play: %v", err)
	}
	if first.PlaysAllowed != 1 {
		t.Fatalf("a placement clip allows %d plays, want 1", first.PlaysAllowed)
	}
	if _, err := svc.RecordPlay(ctx, userID, versionID, domain.ContextTypePlacement, sessionID); !errors.Is(
		err, domain.ErrPlayLimitReached) {
		t.Fatalf("expected ErrPlayLimitReached on the second play, got %v", err)
	}
	if _, err := svc.RecordPlay(ctx, userID, versionID, domain.ContextTypePlacement, uuid.New()); !errors.Is(
		err, domain.ErrPlayNotAllowed) {
		t.Fatalf("expected ErrPlayNotAllowed for a session that is not the caller's, got %v", err)
	}
}

type fakeAudio struct{}

func (fakeAudio) AudioKey(_ context.Context, script, voice string) (string, bool, error) {
	return "tts/" + voice + "/" + script + ".wav", true, nil
}

func TestRecordPlay_AContextThatIsNotTheCallersIsRefused(t *testing.T) {
	userID := uuid.New()
	versionID := uuid.New()
	bodyJSON, _ := json.Marshal(listeningBody{AudioObjectKey: "tts/v/clip.wav"})
	contentReader := &fakeContentReader{versions: map[uuid.UUID]*contentcontract.Version{
		versionID: {ID: versionID, Kind: domain.KindListeningComprehension, Body: bodyJSON},
	}}
	othersAttempt := uuid.New()
	svc := New(Deps{
		Repo:     newFakeRepo(),
		Content:  contentReader,
		Storage:  &fakeStorageSigner{},
		Sittings: fakeSittings{sittingID: uuid.New(), plays: 1},
		Learning: &fakeAttemptReader{attempts: map[uuid.UUID]*learningcontract.AttemptDetail{
			othersAttempt: {ID: othersAttempt, UserID: uuid.New(), ContentVersionID: versionID, Status: "in_progress"},
		}},
	})

	// A made-up context ID would otherwise be a fresh set of plays on every request.
	_, err := svc.RecordPlay(context.Background(), userID, versionID, domain.ContextTypeAttempt, uuid.New())
	if !errors.Is(err, domain.ErrPlayNotAllowed) {
		t.Fatalf("expected ErrPlayNotAllowed for an unknown attempt, got %v", err)
	}
	_, err = svc.RecordPlay(context.Background(), userID, versionID, domain.ContextTypeAttempt, othersAttempt)
	if !errors.Is(err, domain.ErrPlayNotAllowed) {
		t.Fatalf("expected ErrPlayNotAllowed for another learner's attempt, got %v", err)
	}
	_, err = svc.RecordPlay(context.Background(), userID, versionID, domain.ContextTypeExam, uuid.New())
	if !errors.Is(err, domain.ErrPlayNotAllowed) {
		t.Fatalf("expected ErrPlayNotAllowed for a sitting that is not the caller's, got %v", err)
	}
}

func TestRecordPlay_ContextValidation(t *testing.T) {
	svc := New(Deps{
		Repo:    newFakeRepo(),
		Content: &fakeContentReader{versions: make(map[uuid.UUID]*contentcontract.Version)},
	})

	_, err := svc.RecordPlay(context.Background(), uuid.New(), uuid.New(), "invalid_context", uuid.New())
	if !errors.Is(err, domain.ErrInvalidContext) {
		t.Fatalf("expected ErrInvalidContext, got %v", err)
	}
}

func TestRecordPlay_ItemNotFoundOrWrongKind(t *testing.T) {
	contentReader := &fakeContentReader{versions: make(map[uuid.UUID]*contentcontract.Version)}
	svc := New(Deps{
		Repo:    newFakeRepo(),
		Content: contentReader,
	})

	// Missing item
	missingID := uuid.New()
	_, err := svc.RecordPlay(context.Background(), uuid.New(), missingID, domain.ContextTypeAttempt, uuid.New())
	if !errors.Is(err, domain.ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}

	// Wrong kind
	readingID := uuid.New()
	contentReader.versions[readingID] = &contentcontract.Version{
		ID:   readingID,
		Kind: "reading_comprehension",
	}
	_, err = svc.RecordPlay(context.Background(), uuid.New(), readingID, domain.ContextTypeAttempt, uuid.New())
	if !errors.Is(err, domain.ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for non-listening item, got %v", err)
	}
}

func TestRecordPlay_ExamPolicy_AllowsOnePlayOnly(t *testing.T) {
	userID := uuid.New()
	versionID := uuid.New()
	examID := uuid.New()

	bodyJSON, _ := json.Marshal(listeningBody{
		Script:          "Hello and welcome to the exam.",
		Voice:           "en_US-lessac-medium",
		DurationSeconds: 45,
	})
	contentReader := &fakeContentReader{versions: map[uuid.UUID]*contentcontract.Version{
		versionID: {
			ID:   versionID,
			Kind: domain.KindListeningComprehension,
			Body: bodyJSON,
		},
	}}

	repo := newFakeRepo()
	svc := New(Deps{
		Repo:     repo,
		Content:  contentReader,
		Storage:  &fakeStorageSigner{},
		Sittings: fakeSittings{sittingID: examID, plays: 1},
		Audio:    fakeAudio{},
		Clock:    clock.Real{},
	})

	// First play in exam context: allowed
	res1, err := svc.RecordPlay(context.Background(), userID, versionID, domain.ContextTypeExam, examID)
	if err != nil {
		t.Fatalf("first play failed: %v", err)
	}
	if res1.PlaysUsed != 1 || res1.PlaysAllowed != 1 {
		t.Fatalf("expected 1/1 plays, got %d/%d", res1.PlaysUsed, res1.PlaysAllowed)
	}
	if res1.AudioURL == "" {
		t.Fatal("audio URL should not be empty")
	}

	// Second play in exam context: refused
	_, err = svc.RecordPlay(context.Background(), userID, versionID, domain.ContextTypeExam, examID)
	if !errors.Is(err, domain.ErrPlayLimitReached) {
		t.Fatalf("expected ErrPlayLimitReached on 2nd exam play, got %v", err)
	}
}

func TestRecordPlay_AttemptPolicy_AllowsThreePlays(t *testing.T) {
	userID := uuid.New()
	versionID := uuid.New()
	attemptID := uuid.New()

	bodyJSON, _ := json.Marshal(listeningBody{
		AudioObjectKey:  "tts/voices/clip.mp3",
		DurationSeconds: 30,
	})
	contentReader := &fakeContentReader{versions: map[uuid.UUID]*contentcontract.Version{
		versionID: {
			ID:   versionID,
			Kind: domain.KindListeningComprehension,
			Body: bodyJSON,
		},
	}}

	repo := newFakeRepo()
	svc := New(Deps{
		Repo:    repo,
		Content: contentReader,
		Storage: &fakeStorageSigner{},
		Learning: &fakeAttemptReader{attempts: map[uuid.UUID]*learningcontract.AttemptDetail{
			attemptID: {ID: attemptID, UserID: userID, ContentVersionID: versionID, Status: "in_progress"},
		}},
		Clock: clock.Real{},
	})

	for i := 1; i <= 3; i++ {
		res, err := svc.RecordPlay(context.Background(), userID, versionID, domain.ContextTypeAttempt, attemptID)
		if err != nil {
			t.Fatalf("play %d failed: %v", i, err)
		}
		if res.PlaysUsed != i || res.PlaysAllowed != 3 {
			t.Fatalf("play %d: expected %d/3, got %d/%d", i, i, res.PlaysUsed, res.PlaysAllowed)
		}
	}

	// 4th play: refused
	_, err := svc.RecordPlay(context.Background(), userID, versionID, domain.ContextTypeAttempt, attemptID)
	if !errors.Is(err, domain.ErrPlayLimitReached) {
		t.Fatalf("expected ErrPlayLimitReached on 4th play, got %v", err)
	}
}

func TestGetTranscript_LockedUntilGraded(t *testing.T) {
	userID := uuid.New()
	otherUser := uuid.New()
	versionID := uuid.New()
	attemptID := uuid.New()

	bodyJSON, _ := json.Marshal(listeningBody{
		Script: "This is the secret audio script.",
	})
	contentReader := &fakeContentReader{versions: map[uuid.UUID]*contentcontract.Version{
		versionID: {
			ID:   versionID,
			Kind: domain.KindListeningComprehension,
			Body: bodyJSON,
		},
	}}

	attemptReader := &fakeAttemptReader{attempts: map[uuid.UUID]*learningcontract.AttemptDetail{
		attemptID: {
			ID:               attemptID,
			UserID:           userID,
			ContentVersionID: versionID,
			Status:           "grading",
		},
	}}

	svc := New(Deps{
		Repo:     newFakeRepo(),
		Content:  contentReader,
		Learning: attemptReader,
	})

	// In-progress attempt: transcript locked
	_, err := svc.GetTranscript(context.Background(), userID, versionID, attemptID)
	if !errors.Is(err, domain.ErrTranscriptLocked) {
		t.Fatalf("expected ErrTranscriptLocked while grading, got %v", err)
	}

	// Other user's attempt: locked
	_, err = svc.GetTranscript(context.Background(), otherUser, versionID, attemptID)
	if !errors.Is(err, domain.ErrTranscriptLocked) {
		t.Fatalf("expected ErrTranscriptLocked for other user, got %v", err)
	}

	// Graded attempt: reveals script
	attemptReader.attempts[attemptID].Status = "graded"
	script, err := svc.GetTranscript(context.Background(), userID, versionID, attemptID)
	if err != nil {
		t.Fatalf("unexpected error after graded: %v", err)
	}
	if script != "This is the secret audio script." {
		t.Fatalf("expected secret script, got %q", script)
	}
}
