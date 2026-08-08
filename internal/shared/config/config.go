package config

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Key naming convention
//
// An environment variable maps to exactly one configuration key by lowercasing it
// and replacing the FIRST underscore with a dot. Everything after that first
// underscore is the leaf name and keeps its underscores:
//
//	DB_DSN                      -> db.dsn
//	S3_ACCESS_KEY               -> s3.access_key
//	HTTP_READ_TIMEOUT           -> http.read_timeout
//	OTEL_EXPORTER_OTLP_ENDPOINT -> otel.exporter_otlp_endpoint
//
// The segment before the first underscore is the *section*. Only sections a
// caller has declared are read from the environment — see Options.EnvSections.
// A variable with no underscore has no section and is ignored, which is what
// keeps the host's own environment (PATH, TEMP, HOME…) out of the config tree.

// RequiredKey identifies a mandatory configuration value and its documentation source.
type RequiredKey struct {
	Name       string
	DocSection string
}

// Options controls layered configuration loading.
type Options struct {
	Defaults map[string]any
	File     string
	// EnvSections extends the set of environment sections that may be read.
	// Sections are also derived from Defaults and Required, so a caller that
	// declares every key it reads normally leaves this empty.
	EnvSections []string
	Required    []RequiredKey
}

// Load applies defaults, then a YAML file, then environment variables to target.
func Load(ctx context.Context, options Options, target any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	config := koanf.New(".")
	if err := config.Load(confmap.Provider(options.Defaults, "."), nil); err != nil {
		return fmt.Errorf("load config defaults: %w", err)
	}
	if options.File != "" {
		if _, err := os.Stat(options.File); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("stat config file: %w", err)
			}
		} else if err := config.Load(file.Provider(options.File), yaml.Parser()); err != nil {
			return fmt.Errorf("load config file: %w", err)
		}
	}
	if err := config.Load(env.Provider("", ".", envKey(allowedSections(options))), nil); err != nil {
		return fmt.Errorf("load config environment: %w", err)
	}
	for _, required := range options.Required {
		if !config.Exists(required.Name) || strings.TrimSpace(config.String(required.Name)) == "" {
			return fmt.Errorf("required config key %s is missing; see %s", required.Name, required.DocSection)
		}
	}
	if err := config.Unmarshal("", target); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}
	return nil
}

// EnvKey returns the configuration key an environment variable maps to,
// ignoring the section allowlist. It exists so tests and tooling can check a
// declared key against `.env.example` without duplicating the convention.
func EnvKey(variable string) string {
	lower := strings.ToLower(strings.TrimSpace(variable))
	index := strings.Index(lower, "_")
	if index <= 0 || index == len(lower)-1 {
		return ""
	}
	return lower[:index] + "." + lower[index+1:]
}

// ParseEnvFile reads a `KEY=value` file such as `.env.example`, ignoring blank
// lines, comments and trailing inline comments.
func ParseEnvFile(path string) (map[string]string, error) {
	// The caller supplies this path (a repository file such as `.env.example`);
	// it never comes from a request.
	handle, err := os.Open(path) //nolint:gosec // caller-supplied repository path
	if err != nil {
		return nil, fmt.Errorf("open env file: %w", err)
	}
	defer func() { _ = handle.Close() }()

	values := make(map[string]string)
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		values[strings.TrimSpace(name)] = trimInlineComment(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read env file: %w", err)
	}
	return values, nil
}

// trimInlineComment removes a trailing `# …` comment that is not inside quotes.
func trimInlineComment(value string) string {
	quoted := false
	for index, character := range value {
		switch character {
		case '"':
			quoted = !quoted
		case '#':
			if !quoted {
				return strings.Trim(strings.TrimSpace(value[:index]), `"`)
			}
		}
	}
	return strings.Trim(strings.TrimSpace(value), `"`)
}

// allowedSections is the set of environment sections this caller may read. It
// is derived from the keys the caller has already declared, so an undeclared
// key cannot enter the config tree from the environment.
func allowedSections(options Options) map[string]struct{} {
	sections := make(map[string]struct{}, len(options.Defaults)+len(options.Required)+len(options.EnvSections))
	for _, name := range options.EnvSections {
		sections[strings.ToLower(name)] = struct{}{}
	}
	for key := range options.Defaults {
		sections[sectionOf(key)] = struct{}{}
	}
	for _, required := range options.Required {
		sections[sectionOf(required.Name)] = struct{}{}
	}
	delete(sections, "")
	return sections
}

func sectionOf(key string) string {
	section, _, _ := strings.Cut(strings.ToLower(key), ".")
	return section
}

func envKey(allowed map[string]struct{}) func(string) string {
	return func(variable string) string {
		key := EnvKey(variable)
		if key == "" {
			return ""
		}
		if _, ok := allowed[sectionOf(key)]; !ok {
			return ""
		}
		return key
	}
}
