package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

type fakeTTSCache struct {
	items map[string]string
}

func newFakeTTSCache() *fakeTTSCache {
	return &fakeTTSCache{items: make(map[string]string)}
}

func (c *fakeTTSCache) Get(_ context.Context, textHash, voice string) (string, bool, error) {
	key := textHash + ":" + voice
	val, ok := c.items[key]
	return val, ok, nil
}

func (c *fakeTTSCache) Put(_ context.Context, textHash, voice, _, _, objectKey string) error {
	key := textHash + ":" + voice
	c.items[key] = objectKey
	return nil
}

type fakeStorage struct {
	uploads map[string][]byte
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{uploads: make(map[string][]byte)}
}

func (s *fakeStorage) Put(_ context.Context, bucket, objectKey string, reader io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.uploads[bucket+"/"+objectKey] = data
	return nil
}

func TestHashText(t *testing.T) {
	h1 := HashText("  Hello world!  ")
	h2 := HashText("Hello world!")
	if h1 != h2 {
		t.Fatalf("expected HashText to trim whitespace: %q != %q", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("expected SHA-256 hex length 64, got %d", len(h1))
	}
}

func TestCachedSynthesiser_CacheHit(t *testing.T) {
	cache := newFakeTTSCache()
	storage := newFakeStorage()
	synth := NewCachedSynthesiser(cache, nil, storage, "fluentra-media")

	text := "Listen carefully to the conversation."
	voice := "en_US-lessac-medium"
	hash := HashText(text)
	expectedKey := "tts/" + voice + "/" + hash + ".mp3"

	_ = cache.Put(context.Background(), hash, voice, "piper", "1.0", expectedKey)

	key, err := synth.Synthesise(context.Background(), text, voice)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != expectedKey {
		t.Fatalf("expected key %q, got %q", expectedKey, key)
	}
}

func TestCachedSynthesiser_CacheMiss_NoEngine(t *testing.T) {
	cache := newFakeTTSCache()
	synth := NewCachedSynthesiser(cache, nil, nil, "fluentra-media")

	_, err := synth.Synthesise(context.Background(), "Uncached text", "en_US-lessac-medium")
	if !errors.Is(err, ErrTTSNotFound) {
		t.Fatalf("expected ErrTTSNotFound, got %v", err)
	}
}

func TestCachedSynthesiser_CacheMiss_WithEngine(t *testing.T) {
	cache := newFakeTTSCache()
	storage := newFakeStorage()
	engine := &MockSynthesiserEngine{Data: []byte("TEST_AUDIO_BYTES")}
	synth := NewCachedSynthesiser(cache, engine, storage, "fluentra-media")

	text := "Fresh text to synthesize"
	voice := "en_US-lessac-medium"
	key, err := synth.Synthesise(context.Background(), text, voice)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hash := HashText(text)
	expectedKey := "tts/" + voice + "/" + hash + ".mp3"
	if key != expectedKey {
		t.Fatalf("expected key %q, got %q", expectedKey, key)
	}

	// Verify stored in cache
	cachedKey, found, err := cache.Get(context.Background(), hash, voice)
	if err != nil || !found || cachedKey != expectedKey {
		t.Fatalf("expected cached entry %q, got %q (found: %v, err: %v)", expectedKey, cachedKey, found, err)
	}

	// Verify uploaded to storage
	data, ok := storage.uploads["fluentra-media/"+expectedKey]
	if !ok || string(data) != "TEST_AUDIO_BYTES" {
		t.Fatalf("storage missing expected upload or content mismatch: %q", string(data))
	}
}

func TestMockSynthesiser(t *testing.T) {
	synth := &MockSynthesiser{}
	key, err := synth.Synthesise(context.Background(), "Hello", "voice-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "mock-tts/voice-1/" + HashText("Hello") + ".mp3"
	if key != expected {
		t.Fatalf("expected %q, got %q", expected, key)
	}
}

func TestHTTPTranscriber_NilAudio(t *testing.T) {
	transcriber := NewHTTPTranscriber(HTTPTranscriberConfig{
		BaseURL: "http://localhost:8080",
	})
	_, err := transcriber.Transcribe(context.Background(), nil, "test.webm")
	if !errors.Is(err, ErrEmptyAudio) {
		t.Fatalf("expected ErrEmptyAudio, got %v", err)
	}
}

func TestHTTPTranscriber_Success_WithWords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		resp := openAITranscribeResponse{
			Text:     "Hello world",
			Language: mockTranscriptLanguage,
			Duration: 2.5,
			Words: []openAIWord{
				{Word: "Hello", Start: 0.0, End: 0.8},
				{Word: "world", Start: 0.9, End: 1.5},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	transcriber := NewHTTPTranscriber(HTTPTranscriberConfig{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
	})

	result, err := transcriber.Transcribe(
		context.Background(), bytes.NewReader([]byte("FAKE_WEBM_DATA")), "recording.webm",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Text != "Hello world" {
		t.Fatalf("expected 'Hello world', got %q", result.Text)
	}
	if len(result.Words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(result.Words))
	}
	if result.Words[0].Word != "Hello" || result.Words[1].Word != "world" {
		t.Fatalf("unexpected words: %+v", result.Words)
	}
}

func TestHTTPTranscriber_Success_WithSegmentsFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := openAITranscribeResponse{
			Text:     "Good morning",
			Language: mockTranscriptLanguage,
			Duration: 1.8,
			Segments: []openAISegment{
				{
					Text:  "Good morning",
					Start: 0.0,
					End:   1.8,
					Words: []openAIWord{
						{Word: "Good", Start: 0.0, End: 0.5},
						{Word: "morning", Start: 0.6, End: 1.2},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	transcriber := NewHTTPTranscriber(HTTPTranscriberConfig{
		BaseURL: server.URL,
	})

	result, err := transcriber.Transcribe(context.Background(), bytes.NewReader([]byte("FAKE_WEBM")), "audio.webm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Words) != 2 {
		t.Fatalf("expected 2 words from segments fallback, got %d", len(result.Words))
	}
}

func TestHTTPTranscriber_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid audio format","type":"invalid_request_error"}}`))
	}))
	defer server.Close()

	transcriber := NewHTTPTranscriber(HTTPTranscriberConfig{
		BaseURL: server.URL,
	})

	_, err := transcriber.Transcribe(context.Background(), bytes.NewReader([]byte("BAD_AUDIO")), "bad.txt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperr.Error, got %T: %v", err, err)
	}
}

func TestMockTranscriber(t *testing.T) {
	mock := &MockTranscriber{}
	res, err := mock.Transcribe(context.Background(), bytes.NewReader([]byte("test")), "test.webm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Text == "" || len(res.Words) == 0 {
		t.Fatalf("expected non-empty mock result, got %+v", res)
	}

	customErr := errors.New("custom error")
	mockWithErr := &MockTranscriber{Err: customErr}
	_, err = mockWithErr.Transcribe(context.Background(), bytes.NewReader([]byte("test")), "test.webm")
	if !errors.Is(err, customErr) {
		t.Fatalf("expected %v, got %v", customErr, err)
	}
}

func TestModule_New(t *testing.T) {
	mod := New(Config{
		TTSEngine:            EngineMock,
		ASRBaseURL:           "",
		DailyRecordingsLimit: 25,
	})
	if mod.Synthesiser() == nil {
		t.Fatal("synthesiser should not be nil")
	}
	if mod.Transcriber() == nil {
		t.Fatal("transcriber should not be nil")
	}
	if mod.DailyRecordingsLimit() != 25 {
		t.Fatalf("expected limit 25, got %d", mod.DailyRecordingsLimit())
	}
}
