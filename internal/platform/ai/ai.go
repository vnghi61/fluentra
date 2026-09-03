// Package ai is the single door through which every LLM call passes.
//
// Business code asks for a task by name and gets a typed answer. It never sees
// a provider, a base URL, a model name or a prompt string — which is what makes
// swapping the model a configuration change rather than a code change, and what
// keeps rule L11 (no inline prompt strings) enforceable by inspection.
//
// # What is built, and what is not
//
// The module's AGENT.md specifies far more than this: routing by model tier,
// budget and quota enforcement, exact-hash and semantic caching, PII redaction,
// streaming, a repair pass on invalid structured output, an `ai` schema with a
// usage audit trail, and an eval harness. None of that is here.
//
// What is here is the shape those things would attach to — a Client interface, a
// versioned prompt registry, and two providers — built for the one caller that
// needs it today: the vocabulary verification job, which runs on a schedule,
// processes a bounded batch, and can afford to fail. Anything on a request path,
// or anything a learner is graded by, needs the rest of the spec first.
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Task names a unit of work, and is the key to its prompt template.
//
// A name rather than a prompt: the caller says what it wants done, and which
// template and model serve it is decided here.
type Task string

// The tasks that exist.
const (
	// TaskVerifyVocabulary checks one uploaded word and writes examples for it.
	TaskVerifyVocabulary Task = "vocab_verify"
)

// Request is one unit of work.
type Request struct {
	Task Task
	// Vars are rendered into the template. A variable the template does not use
	// is ignored; one it uses and Vars does not carry renders empty, which the
	// templates are written to tolerate.
	Vars map[string]any
}

// Response is what a provider returned.
type Response struct {
	Text     string
	Model    string
	Provider string
	// Usage is best-effort: not every provider reports it, and a local one
	// generally does not.
	PromptTokens     int
	CompletionTokens int
}

// Client performs AI tasks.
type Client interface {
	// Complete renders the task's template and returns the model's reply.
	Complete(ctx context.Context, req Request) (Response, error)
}

// ErrDisabled is returned when no provider is configured.
//
// A distinct error rather than a nil client, so a caller can tell "AI is turned
// off here" from "the call failed" and skip its work quietly instead of
// retrying something that will never succeed.
var ErrDisabled = errors.New("ai: no provider configured")

// CompleteJSON runs a task and decodes its reply into `out`.
//
// Models wrap JSON in a markdown fence however firmly they are asked not to, and
// some prepend a sentence. Rather than treat that as a failure — which would
// make the whole feature depend on the model's obedience — the fence is stripped
// and the outermost JSON value is taken. Anything beyond that is a real failure
// and is reported as one.
func CompleteJSON(ctx context.Context, client Client, req Request, out any) error {
	if client == nil {
		return ErrDisabled
	}
	response, err := client.Complete(ctx, req)
	if err != nil {
		return err
	}

	payload := extractJSON(response.Text)
	if payload == "" {
		return fmt.Errorf("ai: task %s returned no JSON: %.200q", req.Task, response.Text)
	}
	if err := json.Unmarshal([]byte(payload), out); err != nil {
		return fmt.Errorf("ai: task %s returned invalid JSON: %w", req.Task, err)
	}
	return nil
}

// extractJSON finds the outermost JSON object or array in a model's reply.
func extractJSON(text string) string {
	trimmed := strings.TrimSpace(text)

	// A fenced block, with or without a language tag.
	if start := strings.Index(trimmed, "```"); start >= 0 {
		rest := trimmed[start+3:]
		if newline := strings.IndexByte(rest, '\n'); newline >= 0 {
			rest = rest[newline+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			trimmed = strings.TrimSpace(rest[:end])
		}
	}

	for _, pair := range [][2]byte{{'{', '}'}, {'[', ']'}} {
		start := strings.IndexByte(trimmed, pair[0])
		end := strings.LastIndexByte(trimmed, pair[1])
		if start >= 0 && end > start {
			return trimmed[start : end+1]
		}
	}
	return ""
}
