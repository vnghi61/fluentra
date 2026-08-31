package ai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/platform/ai"
)

// verifyResult mirrors what the vocab_verify template asks the model for.
type verifyResult struct {
	Valid          bool     `json:"valid"`
	Lemma          string   `json:"lemma"`
	Definition     string   `json:"definition"`
	MeaningMatches bool     `json:"meaning_matches"`
	Examples       []string `json:"examples"`
}

func verifyRequest() ai.Request {
	return ai.Request{
		Task: ai.TaskVerifyVocabulary,
		Vars: map[string]any{
			"Term":                 "leisure",
			"ProvidedMeaning":      "thời gian rảnh",
			"DictionaryDefinition": "Time when one is not working or occupied; free time.",
			"PartOfSpeech":         "noun",
			"ExampleCount":         5,
		},
	}
}

// ------------------------------------------------------------- the registry

func TestRegistry_LoadsTheVersionedTemplate(t *testing.T) {
	registry, err := ai.NewRegistry()
	require.NoError(t, err)

	tmpl, err := registry.Get(ai.TaskVerifyVocabulary)
	require.NoError(t, err)

	assert.Equal(t, 1, tmpl.Version)
	assert.True(t, tmpl.JSONOutput, "the task is parsed, not displayed, so the front matter must say so")
	assert.Equal(t, 2048, tmpl.MaxTokens, "read from the template's front matter, not hard-coded in Go")
	assert.Zero(t, tmpl.Temperature, "verification must not be creative")
}

func TestRegistry_RendersTheCallersVariables(t *testing.T) {
	registry, err := ai.NewRegistry()
	require.NoError(t, err)
	tmpl, err := registry.Get(ai.TaskVerifyVocabulary)
	require.NoError(t, err)

	rendered, err := tmpl.Render(verifyRequest().Vars)
	require.NoError(t, err)

	assert.Contains(t, rendered, "leisure")
	assert.Contains(t, rendered, "thời gian rảnh")
	assert.Contains(t, rendered, "exactly 5 example sentences")
	assert.NotContains(t, rendered, "{{", "an unrendered placeholder means the model is asked a broken question")
	assert.NotContains(t, rendered, "task: vocab_verify", "front matter is configuration, not prompt text")
}

func TestRegistry_RefusesAnUnknownTask(t *testing.T) {
	registry, err := ai.NewRegistry()
	require.NoError(t, err)

	_, err = registry.Get("no_such_task")
	require.Error(t, err)
}

// ------------------------------------------------------------ the mock provider

func TestMockProvider_AnswersOffline(t *testing.T) {
	client, err := ai.New(ai.Config{})
	require.NoError(t, err, "the default configuration must work with no key and no network")

	var out verifyResult
	require.NoError(t, ai.CompleteJSON(context.Background(), client, verifyRequest(), &out))

	assert.True(t, out.Valid)
	assert.Equal(t, "leisure", out.Lemma)
	assert.Len(t, out.Examples, 5, "the mock must honour the example count the caller asked for")
}

func TestMockProvider_ReportsItselfAsMock(t *testing.T) {
	// A stored verification has to be distinguishable from a real one later.
	registry, err := ai.NewRegistry()
	require.NoError(t, err)

	response, err := ai.NewMockProvider(registry).Complete(context.Background(), verifyRequest())
	require.NoError(t, err)
	assert.Equal(t, ai.MockModelName, response.Model)
}

// -------------------------------------------------- the OpenAI-compatible provider

func TestOpenAICompatibleProvider_SendsTheRenderedPromptAndReadsTheReply(t *testing.T) {
	var seen chatBody

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &seen))

		// A fenced reply with a sentence in front of it. Models do this however
		// firmly the template asks them not to, so the parser has to cope.
		// Marshalled rather than hand-escaped: the escaping is what the test is
		// about, and getting it wrong by hand tests the wrong thing.
		reply, err := json.Marshal("Here you go:\n\n```json\n" +
			`{"valid":true,"lemma":"leisure","examples":["He reads at leisure."]}` +
			"\n```")
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"llama3.1:8b",` +
			`"usage":{"prompt_tokens":120,"completion_tokens":40},` +
			`"choices":[{"message":{"role":"assistant","content":` + string(reply) + `}}]}`))
	}))
	defer server.Close()

	client, err := ai.New(ai.Config{
		Provider: ai.ProviderOpenAICompatible,
		BaseURL:  server.URL + "/v1",
		Model:    "llama3.1:8b",
		APIKey:   "test-key",
	})
	require.NoError(t, err)

	var out verifyResult
	require.NoError(t, ai.CompleteJSON(context.Background(), client, verifyRequest(), &out))

	// The prompt that went out is the rendered template, not a string built here.
	require.Len(t, seen.Messages, 1)
	assert.Contains(t, seen.Messages[0].Content, "leisure")
	assert.Equal(t, "llama3.1:8b", seen.Model)
	assert.False(t, seen.Stream, "Ollama streams unless told not to, and a streamed body will not decode")
	assert.Equal(t, 2048, seen.MaxTokens)

	assert.True(t, out.Valid)
	assert.Equal(t, "leisure", out.Lemma)
}

func TestOpenAICompatibleProvider_OmitsTheAuthHeaderWithoutAKey(t *testing.T) {
	// A local server does not want one, and sending an empty bearer token is
	// how a working Ollama setup starts returning 401.
	var hadAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadAuth = r.Header["Authorization"]
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"valid\":true}"}}]}`))
	}))
	defer server.Close()

	client, err := ai.New(ai.Config{
		Provider: ai.ProviderOpenAICompatible,
		BaseURL:  server.URL,
		Model:    "qwen2.5",
	})
	require.NoError(t, err)

	var out verifyResult
	require.NoError(t, ai.CompleteJSON(context.Background(), client, verifyRequest(), &out))
	assert.False(t, hadAuth)
}

func TestOpenAICompatibleProvider_SurfacesTheProvidersOwnExplanation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"free tier daily limit reached"}}`))
	}))
	defer server.Close()

	client, err := ai.New(ai.Config{
		Provider: ai.ProviderOpenAICompatible,
		BaseURL:  server.URL,
		Model:    "free-model",
	})
	require.NoError(t, err)

	var out verifyResult
	err = ai.CompleteJSON(context.Background(), client, verifyRequest(), &out)
	require.Error(t, err)
	// Every provider explains a rejection differently, and the explanation is
	// the only part worth reading in a job log.
	assert.Contains(t, err.Error(), "free tier daily limit reached")
}

func TestOpenAICompatibleProvider_ReportsAReplyWithNoJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"I am unable to help with that."}}]}`))
	}))
	defer server.Close()

	client, err := ai.New(ai.Config{
		Provider: ai.ProviderOpenAICompatible,
		BaseURL:  server.URL,
		Model:    "free-model",
	})
	require.NoError(t, err)

	var out verifyResult
	err = ai.CompleteJSON(context.Background(), client, verifyRequest(), &out)
	require.Error(t, err, "a refusal must fail the item, not be read as a verdict")
}

// -------------------------------------------------------------- configuration

func TestNew_RefusesAnUnusableProviderRatherThanFallingBack(t *testing.T) {
	// Falling back to the mock would mean a deployment that believed it was
	// verifying against a real model while accepting every word it was given.
	_, err := ai.New(ai.Config{Provider: ai.ProviderOpenAICompatible, Model: "x"})
	require.Error(t, err, "no base URL")

	_, err = ai.New(ai.Config{Provider: ai.ProviderOpenAICompatible, BaseURL: "http://localhost:11434/v1"})
	require.Error(t, err, "no model")

	_, err = ai.New(ai.Config{Provider: "anthropic"})
	require.Error(t, err, "an unknown provider name is a typo, not a default")
}

func TestNew_TreatsAnEmptyProviderAsMock(t *testing.T) {
	client, err := ai.New(ai.Config{})
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestCompleteJSON_WithoutAClient(t *testing.T) {
	var out verifyResult
	assert.ErrorIs(t, ai.CompleteJSON(context.Background(), nil, verifyRequest(), &out), ai.ErrDisabled)
}

func TestNewOpenAICompatibleProvider_TrimsATrailingSlashOnTheBaseURL(t *testing.T) {
	// "http://localhost:11434/v1/" is what people paste, and the naive join
	// produces "//chat/completions", which 404s on some servers and not others.
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer server.Close()

	client, err := ai.New(ai.Config{
		Provider: ai.ProviderOpenAICompatible,
		BaseURL:  server.URL + "/v1/",
		Model:    "m",
	})
	require.NoError(t, err)

	var out verifyResult
	require.NoError(t, ai.CompleteJSON(context.Background(), client, verifyRequest(), &out))
	assert.Equal(t, "/v1/chat/completions", path)
	assert.False(t, strings.Contains(path, "//"))
}

// chatBody is the request shape the test server inspects.
type chatBody struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	MaxTokens int  `json:"max_tokens"`
	Stream    bool `json:"stream"`
}
