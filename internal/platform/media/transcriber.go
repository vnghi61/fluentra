package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// HTTPTranscriberConfig holds configuration for the OpenAI-compatible speech-to-text adapter.
type HTTPTranscriberConfig struct {
	BaseURL    string
	Model      string
	APIKey     string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// HTTPTranscriber implements Transcriber using an OpenAI-compatible /audio/transcriptions endpoint.
type HTTPTranscriber struct {
	baseURL    string
	model      string
	apiKey     string
	timeout    time.Duration
	httpClient *http.Client
}

// NewHTTPTranscriber creates a new HTTPTranscriber.
func NewHTTPTranscriber(cfg HTTPTranscriberConfig) *HTTPTranscriber {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "whisper-1"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &HTTPTranscriber{
		baseURL:    baseURL,
		model:      model,
		apiKey:     strings.TrimSpace(cfg.APIKey),
		timeout:    timeout,
		httpClient: client,
	}
}

type openAIWord struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type openAISegment struct {
	Text  string       `json:"text"`
	Start float64      `json:"start"`
	End   float64      `json:"end"`
	Words []openAIWord `json:"words"`
}

type openAITranscribeResponse struct {
	Text     string          `json:"text"`
	Language string          `json:"language"`
	Duration float64         `json:"duration"`
	Words    []openAIWord    `json:"words"`
	Segments []openAISegment `json:"segments"`
	Error    *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Transcribe streams audio data to the transcription service and parses the response.
func (h *HTTPTranscriber) Transcribe(ctx context.Context, audio io.Reader, filename string) (*TranscribeResult, error) {
	if audio == nil {
		return nil, ErrEmptyAudio
	}
	body, contentType, err := buildTranscriptionForm(audio, filename, h.model)
	if err != nil {
		return nil, err
	}

	reqCtx := ctx
	if h.timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, h.timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, h.baseURL+"/audio/transcriptions", body)
	if err != nil {
		return nil, fmt.Errorf("create transcription request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, apperr.Wrap(err, apperr.Unavailable, "ASR_REQUEST_FAILED", "transcription request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxTranscriptionResponseBytes))
	if err != nil {
		return nil, apperr.Wrap(err, apperr.Unavailable, "ASR_READ_FAILED", "read transcription response failed")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, transcriptionError(resp.StatusCode, respBody)
	}
	return parseTranscription(respBody)
}

// maxTranscriptionResponseBytes bounds the reply read back. A verbose transcript
// of a three-minute recording with word timings is tens of kilobytes.
const maxTranscriptionResponseBytes = 4 << 20

// mockTranscriptLanguage is the language the mock transcriber reports.
const mockTranscriptLanguage = "english"

func buildTranscriptionForm(audio io.Reader, filename, model string) (*bytes.Buffer, string, error) {
	if strings.TrimSpace(filename) == "" {
		filename = "recording.webm"
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", fmt.Errorf("create multipart form file: %w", err)
	}
	if _, err := io.Copy(part, audio); err != nil {
		return nil, "", fmt.Errorf("copy audio to form: %w", err)
	}
	fields := [][2]string{
		{"model", model},
		{"response_format", "verbose_json"},
		{"timestamp_granularities[]", "word"},
	}
	for _, field := range fields {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			return nil, "", fmt.Errorf("write %s field: %w", field[0], err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", err)
	}
	return body, writer.FormDataContentType(), nil
}

// transcriptionError names the provider's own message when it sent one. The
// raw body is not echoed: it can hold anything, and the error reaches logs.
func transcriptionError(status int, body []byte) error {
	var errResp openAITranscribeResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil && errResp.Error.Message != "" {
		return apperr.New(apperr.Unavailable, "ASR_PROVIDER_ERROR", "transcription provider error: "+errResp.Error.Message)
	}
	return apperr.New(apperr.Unavailable, "ASR_STATUS_ERROR",
		fmt.Sprintf("transcription provider returned status %d", status))
}

func parseTranscription(body []byte) (*TranscribeResult, error) {
	var parsed openAITranscribeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, apperr.Wrap(err, apperr.Unavailable, "ASR_PARSE_FAILED", "parse transcription response")
	}
	result := &TranscribeResult{
		Text:     strings.TrimSpace(parsed.Text),
		Language: parsed.Language,
		Duration: parsed.Duration,
	}
	words := parsed.Words
	if len(words) == 0 {
		for _, seg := range parsed.Segments {
			words = append(words, seg.Words...)
		}
	}
	for _, w := range words {
		result.Words = append(result.Words, WordTiming(w))
	}
	return result, nil
}

// MockTranscriber is an in-memory mock for tests and local development.
type MockTranscriber struct {
	Result *TranscribeResult
	Err    error
	Fn     func(ctx context.Context, audio io.Reader, filename string) (*TranscribeResult, error)
}

// Transcribe executes the mock logic.
func (m *MockTranscriber) Transcribe(ctx context.Context, audio io.Reader, filename string) (*TranscribeResult, error) {
	if m.Fn != nil {
		return m.Fn(ctx, audio, filename)
	}
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Result != nil {
		return m.Result, nil
	}
	return &TranscribeResult{
		Text:     "The quick brown fox jumps over the lazy dog.",
		Language: mockTranscriptLanguage,
		Duration: 3.5,
		Words: []WordTiming{
			{Word: "The", Start: 0.0, End: 0.3},
			{Word: "quick", Start: 0.3, End: 0.7},
			{Word: "brown", Start: 0.7, End: 1.1},
			{Word: "fox", Start: 1.1, End: 1.5},
			{Word: "jumps", Start: 1.5, End: 1.9},
			{Word: "over", Start: 1.9, End: 2.3},
			{Word: "the", Start: 2.3, End: 2.6},
			{Word: "lazy", Start: 2.6, End: 3.0},
			{Word: "dog", Start: 3.0, End: 3.5},
		},
	}, nil
}
