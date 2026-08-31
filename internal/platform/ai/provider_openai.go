package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatibleConfig configures the one provider adapter this module has.
//
// One adapter rather than one per vendor, because the vendors converged: Ollama,
// OpenRouter, Groq, LM Studio, vLLM and llama.cpp all serve
// `POST {base}/chat/completions` with the same body. Choosing between them is a
// base URL and a model name, which means a learner-facing model change is a
// deployment variable rather than a release — and means a project can run
// entirely on free or local models without a line of code changing.
//
// Written against net/http rather than a vendor SDK. DEPENDENCIES.md §1.12
// sanctions plain HTTP for the OpenAI-compatible providers, and doing it this
// way adds no Go dependency at all.
type OpenAICompatibleConfig struct {
	// BaseURL is the API root, without a trailing slash. For example:
	//   Ollama      http://localhost:11434/v1
	//   OpenRouter  https://openrouter.ai/api/v1
	//   Groq        https://api.groq.com/openai/v1
	BaseURL string
	// Model is the provider's own identifier, e.g. "llama3.1:8b" or
	// "meta-llama/llama-3.1-8b-instruct:free".
	Model string
	// APIKey may be empty. A local server does not want one, and sending an
	// empty Authorization header to one that does not expect it is how a
	// working local setup starts returning 401.
	APIKey string
	// Timeout bounds one call. Generous by default: this runs in a job, and a
	// local model on modest hardware is slow rather than broken.
	Timeout time.Duration
}

// OpenAICompatibleProvider calls any OpenAI-compatible chat completions API.
type OpenAICompatibleProvider struct {
	config   OpenAICompatibleConfig
	registry *Registry
	client   *http.Client
}

// defaultAITimeout is the per-call bound when the configuration gives none.
const defaultAITimeout = 120 * time.Second

// NewOpenAICompatibleProvider builds the provider.
func NewOpenAICompatibleProvider(
	config OpenAICompatibleConfig, registry *Registry,
) (*OpenAICompatibleProvider, error) {
	if strings.TrimSpace(config.BaseURL) == "" {
		return nil, fmt.Errorf("ai: base URL is required")
	}
	if strings.TrimSpace(config.Model) == "" {
		return nil, fmt.Errorf("ai: model is required")
	}
	if registry == nil {
		return nil, fmt.Errorf("ai: prompt registry is required")
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultAITimeout
	}
	config.BaseURL = strings.TrimRight(config.BaseURL, "/")

	return &OpenAICompatibleProvider{
		config:   config,
		registry: registry,
		client:   &http.Client{Timeout: config.Timeout},
	}, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature"`
	// Stream is sent explicitly as false. Ollama streams by default, and a
	// streamed body parsed as one JSON object yields a confusing decode error
	// rather than an obvious one.
	Stream bool `json:"stream"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Complete renders the task's template and asks the model.
func (p *OpenAICompatibleProvider) Complete(ctx context.Context, req Request) (Response, error) {
	tmpl, err := p.registry.Get(req.Task)
	if err != nil {
		return Response{}, err
	}
	rendered, err := tmpl.Render(req.Vars)
	if err != nil {
		return Response{}, err
	}

	body, err := json.Marshal(chatRequest{
		Model:       p.config.Model,
		Messages:    []chatMessage{{Role: "user", Content: rendered}},
		MaxTokens:   tmpl.MaxTokens,
		Temperature: tmpl.Temperature,
		Stream:      false,
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode request: %w", err)
	}

	endpoint := p.config.BaseURL + "/chat/completions"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("ai: build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if p.config.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	response, err := p.client.Do(request)
	if err != nil {
		return Response{}, fmt.Errorf("ai: call %s: %w", endpoint, err)
	}
	defer func() { _ = response.Body.Close() }()

	// Bounded: a misconfigured base URL can point at something that streams
	// indefinitely, and an unbounded read there is an out-of-memory rather than
	// an error message.
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return Response{}, fmt.Errorf("ai: read response: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// The body, truncated, because every provider explains a 400
		// differently and the explanation is the only useful part.
		return Response{}, fmt.Errorf("ai: %s returned %d: %.300s",
			endpoint, response.StatusCode, payload)
	}

	var decoded chatResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return Response{}, fmt.Errorf("ai: decode response: %w", err)
	}
	if decoded.Error != nil {
		return Response{}, fmt.Errorf("ai: provider error: %s", decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return Response{}, fmt.Errorf("ai: provider returned no choices")
	}

	model := decoded.Model
	if model == "" {
		model = p.config.Model
	}
	return Response{
		Text:             decoded.Choices[0].Message.Content,
		Model:            model,
		PromptTokens:     decoded.Usage.PromptTokens,
		CompletionTokens: decoded.Usage.CompletionTokens,
	}, nil
}

var _ Client = (*OpenAICompatibleProvider)(nil)
