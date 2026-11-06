package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fluentra/fluentra/internal/platform/storage"
)

// mimeAudioMPEG is the content type of a rendered clip.
const mimeAudioMPEG = "audio/mpeg"

// SynthesiserEngine produces raw audio bytes for given text and voice.
type SynthesiserEngine interface {
	EngineName() string
	EngineVersion() string
	Render(ctx context.Context, text, voice string) (data []byte, mimeType string, err error)
}

// CachedSynthesiser manages synthesis by checking tts_cache first.
type CachedSynthesiser struct {
	cache       TTSCache
	engine      SynthesiserEngine
	storage     AudioUploader
	bucket      string
	voice       string
	secondVoice string
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

// WithVoice sets the configured voice (SPEECH_TTS_VOICE) every clip is rendered in.
func (s *CachedSynthesiser) WithVoice(voice string) *CachedSynthesiser {
	s.voice = voice
	return s
}

// WithSecondVoice sets the voice a conversation's second speaker is rendered in
// (SPEECH_TTS_VOICE_B, WO 22 D22-23). Without one, both speakers share the
// configured voice and a conversation is intelligible, just less real.
func (s *CachedSynthesiser) WithSecondVoice(voice string) *CachedSynthesiser {
	s.secondVoice = voice
	return s
}

// Synthesise retrieves cached audio or synthesises and caches it.
//
// The voice argument is ignored; the clip is rendered in the configured voice.
// See ConfiguredVoice.
func (s *CachedSynthesiser) Synthesise(ctx context.Context, text, _ string) (string, error) {
	if turns := DialogueTurns(text, nil); len(turns) >= 2 {
		return s.synthesiseDialogue(ctx, text, turns)
	}
	voice := ConfiguredVoice(s.voice)
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

	objectKey := ObjectKey(voice, textHash, mimeType)

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

// ScriptTurn is one speaker's line in a conversation (WO 22 D22-23).
type ScriptTurn struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

// SynthesiseTurns renders a conversation turn by turn, each in its speaker's
// voice, and joins them into one clip. A script with fewer than two turns is
// rendered as a single clip in the configured voice, so a monologue is
// unchanged. It reports the object key of the clip.
func (s *CachedSynthesiser) SynthesiseTurns(ctx context.Context, turns []ScriptTurn) (string, error) {
	if len(turns) < 2 {
		return s.Synthesise(ctx, turnsText(turns), "")
	}
	return s.synthesiseDialogue(ctx, turnsText(turns), turns)
}

// synthesiseDialogue finds or renders a two-voice clip, cached under the script's
// hash and DialogueVoiceKey. Without an engine it only looks up, and a clip
// voiced before conversations had two voices still counts.
func (s *CachedSynthesiser) synthesiseDialogue(ctx context.Context, script string, turns []ScriptTurn) (string, error) {
	voice := DialogueVoiceKey(s.voice, s.secondVoice)
	textHash := HashText(script)
	if s.cache != nil {
		if key, found, err := s.cache.Get(ctx, textHash, voice); err == nil && found && key != "" {
			return key, nil
		}
	}
	if s.engine == nil {
		if s.cache != nil {
			key, found, err := s.cache.Get(ctx, textHash, ConfiguredVoice(s.voice))
			if err == nil && found && key != "" {
				return key, nil
			}
		}
		return "", ErrTTSNotFound
	}

	data, mimeType, err := RenderTurns(ctx, s.engine, turns, s.voice, s.secondVoice)
	if err != nil {
		return "", fmt.Errorf("render tts: %w", err)
	}
	objectKey := ObjectKey(voice, textHash, mimeType)
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

// ParseScriptTurns reads a listening body's conversation turns. A body without
// turns returns nil, and its script renders as one clip in the configured voice
// (WO 22 D22-23).
func ParseScriptTurns(body []byte) []ScriptTurn {
	if len(body) == 0 {
		return nil
	}
	var parsed struct {
		Turns []ScriptTurn `json:"turns"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil
	}
	return parsed.Turns
}

// JoinScriptTurns joins a conversation's turns into the one script the clip is
// rendered from and cached under. cmd/tts uses it so its cache key matches the
// one SynthesiseTurns would write.
func JoinScriptTurns(turns []ScriptTurn) string { return turnsText(turns) }

// turnsText joins a conversation's turns into the one script the clip is
// rendered from and cached under.
func turnsText(turns []ScriptTurn) string {
	lines := make([]string, 0, len(turns))
	for _, turn := range turns {
		text := strings.TrimSpace(turn.Text)
		if text == "" {
			continue
		}
		if speaker := strings.TrimSpace(turn.Speaker); speaker != "" {
			lines = append(lines, speaker+": "+text)
			continue
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}

// DefaultVoice is the voice a script without one is rendered in.
const DefaultVoice = "en_US-lessac-medium"

// ConfiguredVoice is the voice a clip is rendered in, cached under and looked up by.
//
// It comes from configuration (SPEECH_TTS_VOICE), never from the item. Generated
// listening items carried the voice the model was told to write,
// "en-US-Standard-C", which is a cloud provider's voice name and not a voice any
// engine here has: cmd/tts could not render it, and a clip rendered in the
// configured voice was never found under the item's. One voice for rendering,
// caching and lookup is what makes a rendered clip findable, and it lets the
// engine change without republishing a single item.
func ConfiguredVoice(configured string) string {
	if voice := strings.TrimSpace(configured); voice != "" {
		return voice
	}
	return DefaultVoice
}

// EngineMock names the offline engine and synthesiser used in development and tests.
const EngineMock = "mock"

// CacheLocator finds rendered clips in the TTS cache.
//
// cmd/tts renders listening scripts offline and records each clip in the cache,
// not in the published item, which is append-only. Everything that needs a
// clip — drawing a sitting, issuing a play URL — looks here.
type CacheLocator struct {
	cache       TTSCache
	voice       string
	secondVoice string
}

// NewCacheLocator builds a locator over the TTS cache.
func NewCacheLocator(cache TTSCache) *CacheLocator {
	return &CacheLocator{cache: cache}
}

// WithVoice sets the configured voice (SPEECH_TTS_VOICE) clips are looked up by.
func (l *CacheLocator) WithVoice(voice string) *CacheLocator {
	l.voice = voice
	return l
}

// WithSecondVoice sets SPEECH_TTS_VOICE_B, part of the key a conversation clip
// is looked up by. It must match the renderer's.
func (l *CacheLocator) WithSecondVoice(voice string) *CacheLocator {
	l.secondVoice = voice
	return l
}

// AudioKey returns the object key of the clip rendered for script: the
// two-voice conversation clip first, then a clip in the configured voice, which
// is every monologue and any conversation voiced before it had two voices.
//
// The voice argument is the item's and is ignored; see ConfiguredVoice.
func (l *CacheLocator) AudioKey(ctx context.Context, script, _ string) (string, bool, error) {
	if l == nil || l.cache == nil {
		return "", false, nil
	}
	textHash := HashText(script)
	key, found, err := l.cache.Get(ctx, textHash, DialogueVoiceKey(l.voice, l.secondVoice))
	if err != nil || found {
		return key, found, err
	}
	return l.cache.Get(ctx, textHash, ConfiguredVoice(l.voice))
}

// MockSynthesiser provides a deterministic synthesiser for local dev and tests.
type MockSynthesiser struct {
	Prefix string
	// Voice is the configured voice; the one passed to Synthesise is ignored.
	Voice string
}

// Synthesise returns a deterministic mock object key without I/O.
func (m *MockSynthesiser) Synthesise(_ context.Context, text, _ string) (string, error) {
	prefix := m.Prefix
	if prefix == "" {
		prefix = "mock-tts"
	}
	textHash := HashText(text)
	return fmt.Sprintf("%s/%s/%s.mp3", prefix, ConfiguredVoice(m.Voice), textHash), nil
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
	return EngineMock
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
		return m.Data, mimeAudioMPEG, nil
	}
	return []byte("MOCK_AUDIO_MP3_DATA"), mimeAudioMPEG, nil
}
