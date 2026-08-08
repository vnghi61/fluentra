package config

import (
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

// RequiredKey identifies a mandatory configuration value and its documentation source.
type RequiredKey struct {
	Name       string
	DocSection string
}

// Options controls layered configuration loading.
type Options struct {
	Defaults  map[string]any
	File      string
	EnvPrefix string
	Required  []RequiredKey
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
	if err := config.Load(env.Provider(options.EnvPrefix, ".", envKey(options.EnvPrefix)), nil); err != nil {
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

func envKey(prefix string) func(string) string {
	return func(key string) string {
		key = strings.TrimPrefix(key, prefix)
		return strings.ToLower(strings.ReplaceAll(key, "_", "."))
	}
}
