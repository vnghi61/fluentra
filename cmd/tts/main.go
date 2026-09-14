// Package main provides the offline tts CLI tool for pre-generating audio clips.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/fluentra/fluentra/internal/platform/media"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/config"
)

type ttsCLIConfig struct {
	App struct {
		Environment string `koanf:"environment"`
	} `koanf:"app"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"db"`
	Storage struct {
		Endpoint      string `koanf:"endpoint"`
		AccessKey     string `koanf:"access_key"`
		SecretKey     string `koanf:"secret_key"`
		Region        string `koanf:"region"`
		UseSSL        bool   `koanf:"use_ssl"`
		UsePostPolicy bool   `koanf:"use_post_policy"`
	} `koanf:"s3"`
	Speech struct {
		TTSVoice string `koanf:"tts_voice"`
	} `koanf:"speech"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "tts tool error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("tts", flag.ContinueOnError)
	textFlag := flags.String("text", "", "Text to synthesise")
	voiceFlag := flags.String("voice", "", "Voice name, the model file without .onnx (e.g. en_US-lessac-medium)")
	engineFlag := flags.String("engine", media.EnginePiper, "TTS engine: piper, or mock for local development only")
	piperFlag := flags.String("piper", media.EnginePiper, "Path to the piper binary")
	modelsFlag := flags.String("models", "", "Directory holding the piper voice models")
	versionFlag := flags.String("engine-version", "", "Engine version recorded in the TTS cache")
	allFlag := flags.Bool("all", false, "Render every listening_comprehension script that has no clip yet")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if *textFlag == "" && !*allFlag {
		_, _ = fmt.Fprintln(out,
			"Usage: tts [-text <text> -voice <voice>] [-all] [-engine piper|mock] [-piper <bin>] [-models <dir>]")
		return nil
	}

	cfg, err := loadTTSConfig(ctx)
	if err != nil {
		return err
	}
	engine, err := engineFor(*engineFlag, *piperFlag, *modelsFlag, *versionFlag, cfg.App.Environment)
	if err != nil {
		return err
	}
	voice := firstNonEmpty(*voiceFlag, cfg.Speech.TTSVoice, media.DefaultVoice)

	db, err := sql.Open("pgx", cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	uploader, err := newUploader(cfg)
	if err != nil {
		return err
	}

	if *textFlag != "" {
		key, err := processItem(ctx, db, uploader, engine, *textFlag, voice)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "Rendered clip: %s\n", key)
	}
	if *allFlag {
		count, err := processAll(ctx, db, uploader, engine, voice, out)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "Finished: %d clips rendered or already cached\n", count)
	}
	return nil
}

func loadTTSConfig(ctx context.Context) (ttsCLIConfig, error) {
	var cfg ttsCLIConfig
	opts := config.Options{
		Defaults: map[string]any{
			"app.environment":  "development",
			"speech.tts_voice": media.DefaultVoice,
			"s3.endpoint":      "localhost:9000",
			"s3.access_key":    "minioadmin",
			"s3.secret_key":    "minioadmin",
			"s3.region":        "us-east-1",
			"s3.use_ssl":       false,
		},
		EnvSections: []string{"SPEECH"},
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"},
		},
	}
	if err := config.Load(ctx, opts, &cfg); err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

// engineFor builds the engine. The mock renders a few bytes that are not audio,
// so it is refused anywhere but a development database: a learner would get a
// play URL for silence and no one would notice until they pressed play.
func engineFor(name, piperBinary, modelDir, version, environment string) (media.SynthesiserEngine, error) {
	switch name {
	case media.EnginePiper:
		if modelDir == "" {
			return nil, errors.New("-models is required: the directory holding the piper voice models")
		}
		return media.NewPiperEngine(piperBinary, modelDir, version), nil
	case media.EngineMock:
		if environment == "production" || environment == "staging" {
			return nil, fmt.Errorf("the mock engine renders no audio and is refused in %s", environment)
		}
		return &media.MockSynthesiserEngine{Name: media.EngineMock, Version: "1.0.0"}, nil
	default:
		return nil, fmt.Errorf("unknown engine %q: use piper or mock", name)
	}
}

func newUploader(cfg ttsCLIConfig) (storage.Store, error) {
	endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.Storage.Endpoint, "https://"), "http://")
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""),
		Secure: cfg.Storage.UseSSL,
		Region: cfg.Storage.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	if cfg.Storage.UsePostPolicy {
		return storage.NewMinIOStore(minioClient), nil
	}
	return storage.NewMinIOStoreNoPostPolicy(minioClient), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func processItem(
	ctx context.Context,
	db *sql.DB,
	uploader storage.Store,
	engine media.SynthesiserEngine,
	text, voice string,
) (string, error) {
	textHash := media.HashText(text)

	// Check if already in cache
	var existingKey string
	err := db.QueryRowContext(ctx,
		"SELECT object_key FROM content.tts_cache WHERE text_hash = $1 AND voice = $2",
		textHash, voice,
	).Scan(&existingKey)

	if err == nil && existingKey != "" {
		slog.Info("audio already in cache", "textHash", textHash, "voice", voice, "key", existingKey)
		return existingKey, nil
	}

	data, mimeType, err := engine.Render(ctx, text, voice)
	if err != nil {
		return "", fmt.Errorf("render audio: %w", err)
	}
	objectKey := media.ObjectKey(voice, textHash, mimeType)

	if err := uploader.Put(
		ctx, storage.BucketMedia, objectKey, bytes.NewReader(data), int64(len(data)), mimeType,
	); err != nil {
		return "", fmt.Errorf("upload audio to storage: %w", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO content.tts_cache (text_hash, voice, engine, engine_version, object_key, created_at)
		VALUES ($1, $2, $3, $4, $5, clock_timestamp())
		ON CONFLICT (text_hash, voice) DO UPDATE
		SET engine = EXCLUDED.engine,
		    engine_version = EXCLUDED.engine_version,
		    object_key = EXCLUDED.object_key,
		    created_at = clock_timestamp()`,
		textHash, voice, engine.EngineName(), engine.EngineVersion(), objectKey,
	)
	if err != nil {
		return "", fmt.Errorf("save tts cache record: %w", err)
	}

	return objectKey, nil
}

type listeningBody struct {
	Script string `json:"script"`
	Voice  string `json:"voice"`
}

func processAll(
	ctx context.Context,
	db *sql.DB,
	uploader storage.Store,
	engine media.SynthesiserEngine,
	defaultVoice string,
	out io.Writer,
) (int, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, body FROM content.content_versions WHERE kind = 'listening_comprehension'",
	)
	if err != nil {
		return 0, fmt.Errorf("query listening items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	processed := 0
	for rows.Next() {
		var id string
		var rawBody []byte
		if err := rows.Scan(&id, &rawBody); err != nil {
			return processed, fmt.Errorf("scan content version: %w", err)
		}

		var body listeningBody
		if err := json.Unmarshal(rawBody, &body); err != nil {
			continue
		}

		if strings.TrimSpace(body.Script) == "" {
			continue
		}

		voice := body.Voice
		if voice == "" {
			voice = defaultVoice
		}

		key, err := processItem(ctx, db, uploader, engine, body.Script, voice)
		if err != nil {
			_, _ = fmt.Fprintf(out, "Warning: failed to synthesise item %s: %v\n", id, err)
			continue
		}
		processed++
		_, _ = fmt.Fprintf(out, "Processed item %s -> %s\n", id, key)
	}

	return processed, rows.Err()
}
