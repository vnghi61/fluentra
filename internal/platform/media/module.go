package media

import (
	"time"
)

// Config holds configuration and dependencies for the media platform module.
type Config struct {
	TTSEngine            string
	TTSVoice             string
	ASRBaseURL           string
	ASRModel             string
	ASRAPIKey            string
	ASRTimeout           time.Duration
	DailyRecordingsLimit int
	Storage              AudioUploader
	TTSCache             TTSCache
}

// Module provides audio synthesiser and speech transcriber capabilities.
type Module struct {
	synthesiser          Synthesiser
	transcriber          Transcriber
	dailyRecordingsLimit int
}

// New creates and initializes the media platform module.
func New(cfg Config) *Module {
	var synth Synthesiser
	switch cfg.TTSEngine {
	case EngineMock:
		synth = &MockSynthesiser{}
	default:
		// Default to CachedSynthesiser (offline: cache hit returns key; cache miss returns ErrTTSNotFound)
		synth = NewCachedSynthesiser(cfg.TTSCache, nil, cfg.Storage, "")
	}

	var transcriber Transcriber
	if cfg.ASRBaseURL == "" || cfg.ASRBaseURL == "mock" {
		transcriber = &MockTranscriber{}
	} else {
		transcriber = NewHTTPTranscriber(HTTPTranscriberConfig{
			BaseURL: cfg.ASRBaseURL,
			Model:   cfg.ASRModel,
			APIKey:  cfg.ASRAPIKey,
			Timeout: cfg.ASRTimeout,
		})
	}

	dailyLimit := cfg.DailyRecordingsLimit
	if dailyLimit <= 0 {
		dailyLimit = 30
	}

	return &Module{
		synthesiser:          synth,
		transcriber:          transcriber,
		dailyRecordingsLimit: dailyLimit,
	}
}

// Synthesiser returns the configured audio synthesiser.
func (m *Module) Synthesiser() Synthesiser {
	return m.synthesiser
}

// Transcriber returns the configured speech transcriber.
func (m *Module) Transcriber() Transcriber {
	return m.transcriber
}

// DailyRecordingsLimit returns the maximum number of voice recordings allowed per learner per day.
func (m *Module) DailyRecordingsLimit() int {
	return m.dailyRecordingsLimit
}
