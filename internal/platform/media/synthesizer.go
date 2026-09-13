package media

import (
	"bytes"
	"context"
	"fmt"

	"github.com/fluentra/fluentra/internal/platform/storage"
)

// SynthesiserEngine produces raw audio bytes for given text and voice.
type SynthesiserEngine interface {
	EngineName() string
	EngineVersion() string
	Render(ctx context.Context, text, voice string) (data []byte, mimeType string, err error)
}

// CachedSynthesiser manages synthesis by checking tts_cache first.
type CachedSynthesiser struct {
	cache   TTSCache
	engine  SynthesiserEngine
	storage AudioUploader
	bucket  string
}

// NewCachedSynthesiser creates a new CachedSynthesiser.
func NewCachedSynthesiser(
	cache TTSCache,
	engine SynthesiserEngine,
	uploader AudioUploader,
	bucket string,
) *CachedSynthesiser {
	if bucket == "" {
		bucket = storage.BucketMedia
	}
	return &CachedSynthesiser{
		cache:   cache,
		engine:  engine,
		storage: uploader,
		bucket:  bucket,
	}
}

// Synthesise retrieves cached audio or synthesises and caches it.
func (s *CachedSynthesiser) Synthesise(ctx context.Context, text, voice string) (string, error) {
	textHash := HashText(text)

	if s.cache != nil {
		objectKey, found, err := s.cache.Get(ctx, textHash, voice)
		if err == nil && found && objectKey != "" {
			return objectKey, nil
		}
	}

	if s.engine == nil {
		return "", ErrTTSNotFound
	}

	data, mimeType, err := s.engine.Render(ctx, text, voice)
	if err != nil {
		return "", fmt.Errorf("render tts: %w", err)
	}

	objectKey := fmt.Sprintf("tts/%s/%s.mp3", voice, textHash)

	if s.storage != nil {
		if err := s.storage.Put(ctx, s.bucket, objectKey, bytes.NewReader(data), int64(len(data)), mimeType); err != nil {
			return "", fmt.Errorf("upload tts to storage: %w", err)
		}
	}

	if s.cache != nil {
		_ = s.cache.Put(ctx, textHash, voice, s.engine.EngineName(), s.engine.EngineVersion(), objectKey)
	}

	return objectKey, nil
}

// MockSynthesiser provides a deterministic synthesiser for local dev and tests.
type MockSynthesiser struct {
	Prefix string
}

// Synthesise returns a deterministic mock object key without I/O.
func (m *MockSynthesiser) Synthesise(_ context.Context, text, voice string) (string, error) {
	prefix := m.Prefix
	if prefix == "" {
		prefix = "mock-tts"
	}
	textHash := HashText(text)
	return fmt.Sprintf("%s/%s/%s.mp3", prefix, voice, textHash), nil
}

// MockSynthesiserEngine generates dummy audio bytes for testing CachedSynthesiser.
type MockSynthesiserEngine struct {
	Name    string
	Version string
	Data    []byte
}

// EngineName returns the mock engine name.
func (m *MockSynthesiserEngine) EngineName() string {
	if m.Name != "" {
		return m.Name
	}
	return "mock"
}

// EngineVersion returns the mock engine version.
func (m *MockSynthesiserEngine) EngineVersion() string {
	if m.Version != "" {
		return m.Version
	}
	return "1.0.0"
}

// Render returns mock audio bytes.
func (m *MockSynthesiserEngine) Render(_ context.Context, _, _ string) ([]byte, string, error) {
	if len(m.Data) > 0 {
		return m.Data, "audio/mpeg", nil
	}
	return []byte("MOCK_AUDIO_MP3_DATA"), "audio/mpeg", nil
}
