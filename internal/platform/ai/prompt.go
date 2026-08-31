package ai

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/fluentra/fluentra/internal/platform/ai/prompts"
)

// Template is one versioned runtime prompt.
type Template struct {
	Task    Task
	Version int
	// Body is the prompt text below the front matter, as a parsed template.
	Body *template.Template
	// MaxTokens and Temperature come from the front matter, so tuning a task
	// means editing its template rather than the Go that calls it.
	MaxTokens   int
	Temperature float64
	// JSONOutput marks a task whose reply is parsed rather than shown.
	JSONOutput bool
}

// Registry holds the runtime prompt templates, newest version of each.
//
// Loaded once at construction and never reloaded: a prompt that could change
// under a running process is a prompt whose output cannot be attributed to a
// version.
type Registry struct {
	byTask map[Task]Template
}

// NewRegistry loads every embedded runtime template.
//
// A malformed template is an error rather than a skip. A registry that quietly
// dropped one would leave the task it serves failing at call time, in a job, on
// a schedule — which is the hardest place to notice anything.
func NewRegistry() (*Registry, error) {
	return newRegistryFrom(prompts.Files)
}

func newRegistryFrom(source fs.FS) (*Registry, error) {
	entries, err := fs.Glob(source, "*.md")
	if err != nil {
		return nil, fmt.Errorf("ai: list prompt templates: %w", err)
	}
	// Sorted so the highest version of a task wins deterministically rather
	// than by whatever order the file system offered.
	sort.Strings(entries)

	registry := &Registry{byTask: map[Task]Template{}}
	for _, name := range entries {
		raw, err := fs.ReadFile(source, name)
		if err != nil {
			return nil, fmt.Errorf("ai: read %s: %w", name, err)
		}
		parsed, err := parseTemplate(name, raw)
		if err != nil {
			return nil, err
		}
		if existing, ok := registry.byTask[parsed.Task]; ok && existing.Version > parsed.Version {
			continue
		}
		registry.byTask[parsed.Task] = parsed
	}
	return registry, nil
}

// Get returns the template serving a task.
func (r *Registry) Get(task Task) (Template, error) {
	found, ok := r.byTask[task]
	if !ok {
		return Template{}, fmt.Errorf("ai: no prompt template for task %q", task)
	}
	return found, nil
}

// Render fills a template's variables.
func (t Template) Render(vars map[string]any) (string, error) {
	var out bytes.Buffer
	if err := t.Body.Execute(&out, vars); err != nil {
		return "", fmt.Errorf("ai: render %s: %w", t.Task, err)
	}
	return out.String(), nil
}

// parseTemplate reads `<task>.v<N>.md` and its front matter.
func parseTemplate(name string, raw []byte) (Template, error) {
	base := strings.TrimSuffix(path.Base(name), ".md")
	task, version, err := splitVersioned(base)
	if err != nil {
		return Template{}, fmt.Errorf("ai: %s: %w", name, err)
	}

	front, body := splitFrontMatter(string(raw))
	parsed, err := template.New(base).Option("missingkey=zero").Parse(body)
	if err != nil {
		return Template{}, fmt.Errorf("ai: parse %s: %w", name, err)
	}

	out := Template{
		Task:        Task(task),
		Version:     version,
		Body:        parsed,
		MaxTokens:   1024,
		Temperature: 0,
	}
	if value, ok := front["max_tokens"]; ok {
		if parsedValue, convErr := strconv.Atoi(value); convErr == nil {
			out.MaxTokens = parsedValue
		}
	}
	if value, ok := front["temperature"]; ok {
		if parsedValue, convErr := strconv.ParseFloat(value, 64); convErr == nil {
			out.Temperature = parsedValue
		}
	}
	out.JSONOutput = front["output"] == "json"
	return out, nil
}

// splitVersioned reads "vocab_verify.v1" into its task and version.
func splitVersioned(base string) (string, int, error) {
	marker := strings.LastIndex(base, ".v")
	if marker < 0 {
		return "", 0, fmt.Errorf("template name must be <task>.v<N>.md")
	}
	version, err := strconv.Atoi(base[marker+2:])
	if err != nil || version < 1 {
		return "", 0, fmt.Errorf("template name must end in .v<N> with N >= 1")
	}
	return base[:marker], version, nil
}

// splitFrontMatter reads the leading `---` block as flat key/value pairs.
//
// Flat and deliberately shallow: the front matter carries a handful of scalars
// the caller needs, and nothing that would justify a YAML dependency in a
// platform package. Nested keys and lists — `inputs:` in the templates — are
// documentation for the humans who edit them and are skipped here.
func splitFrontMatter(raw string) (map[string]string, string) {
	front := map[string]string{}
	if !strings.HasPrefix(raw, "---\n") {
		return front, raw
	}
	end := strings.Index(raw[4:], "\n---")
	if end < 0 {
		return front, raw
	}
	block, body := raw[4:4+end], raw[4+end+4:]

	for _, line := range strings.Split(block, "\n") {
		// Only top-level scalars: an indented line belongs to a nested key.
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "-") {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" || value == ">-" || value == "|" {
			continue
		}
		front[strings.TrimSpace(key)] = strings.Trim(value, `"'`)
	}
	return front, strings.TrimPrefix(body, "\n")
}
