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

	"github.com/jackc/pgx/v5"
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
		// TTSVoiceB is the second voice a conversation's other speaker is
		// rendered in (WO 22 D22-23). Empty means one voice.
		TTSVoiceB string `koanf:"tts_voice_b"`
		// Where piper and its voice models are, so `make tts` needs no flags.
		PiperBinary string `koanf:"piper_binary"`
		PiperModels string `koanf:"piper_models"`
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
	piperFlag := flags.String("piper", "", "Path to the piper binary; SPEECH_PIPER_BINARY, or piper on PATH")
	modelsFlag := flags.String("models", "", "Directory holding the piper voice models; SPEECH_PIPER_MODELS")
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
	piperBinary := firstNonEmpty(*piperFlag, cfg.Speech.PiperBinary, media.EnginePiper)
	modelDir := firstNonEmpty(*modelsFlag, cfg.Speech.PiperModels)
	engine, err := engineFor(*engineFlag, piperBinary, modelDir, *versionFlag, cfg.App.Environment)
	if err != nil {
		return err
	}
	voice := firstNonEmpty(*voiceFlag, cfg.Speech.TTSVoice, media.DefaultVoice)

	// Said before anything is written. This reads .env like every other command,
	// and a .env pointed at production renders into production.
	_, _ = fmt.Fprintf(out, "Rendering with %s, voice %s, into %s; storage %s\n",
		engine.EngineName(), media.ConfiguredVoice(voice), describeDatabase(cfg.Database.DSN), cfg.Storage.Endpoint)

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
		count, err := processAll(ctx, db, uploader, engine, voice, cfg.Speech.TTSVoiceB, out)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "Finished: %d clips rendered or already cached\n", count)
	}
	return nil
}

// describeDatabase names the database a DSN points at, without its password.
func describeDatabase(dsn string) string {
	parsed, err := pgx.ParseConfig(dsn)
	if err != nil {
		return "an unparseable database DSN"
	}
	return fmt.Sprintf("database %s on %s", parsed.Database, parsed.Host)
}

func loadTTSConfig(ctx context.Context) (ttsCLIConfig, error) {
	var cfg ttsCLIConfig
	opts := config.Options{
		Defaults: map[string]any{
			"app.environment":     "development",
			"speech.tts_voice":    media.DefaultVoice,
			"speech.tts_voice_b":  "",
			"speech.piper_binary": "",
			"speech.piper_models": "",
			"s3.endpoint":         "localhost:9000",
			"s3.access_key":       "minioadmin",
			"s3.secret_key":       "minioadmin",
			"s3.region":           "us-east-1",
			"s3.use_ssl":          false,
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
			return nil, errors.New("the piper voice models directory is required: -models or SPEECH_PIPER_MODELS")
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
	Script string             `json:"script"`
	Turns  []media.ScriptTurn `json:"turns"`
}

// processItemTurns renders a conversation turn by turn, each in its speaker's
// voice, and stores the concatenated clip under one cache row (WO 22 D22-23).
// The first speaker heard is voiceA; anyone else is voiceB.
func processItemTurns(
	ctx context.Context,
	db *sql.DB,
	uploader storage.Store,
	engine media.SynthesiserEngine,
	turns []media.ScriptTurn,
	voiceA, voiceB string,
) (string, error) {
	textHash := media.HashText(media.JoinScriptTurns(turns))
	combinedVoice := voiceA + "+" + voiceB

	var existingKey string
	err := db.QueryRowContext(ctx,
		"SELECT object_key FROM content.tts_cache WHERE text_hash = $1 AND voice = $2",
		textHash, combinedVoice,
	).Scan(&existingKey)
	if err == nil && existingKey != "" {
		return existingKey, nil
	}

	firstSpeaker := strings.TrimSpace(turns[0].Speaker)
	var audio bytes.Buffer
	mimeType := "audio/mpeg"
	for _, turn := range turns {
		voice := voiceB
		if strings.EqualFold(strings.TrimSpace(turn.Speaker), firstSpeaker) {
			voice = voiceA
		}
		data, mime, renderErr := engine.Render(ctx, turn.Text, voice)
		if renderErr != nil {
			return "", fmt.Errorf("render audio turn: %w", renderErr)
		}
		if mime != "" {
			mimeType = mime
		}
		audio.Write(data)
	}

	objectKey := media.ObjectKey(combinedVoice, textHash, mimeType)
	if err := uploader.Put(
		ctx, storage.BucketMedia, objectKey, bytes.NewReader(audio.Bytes()), int64(audio.Len()), mimeType,
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
		textHash, combinedVoice, engine.EngineName(), engine.EngineVersion(), objectKey,
	)
	if err != nil {
		return "", fmt.Errorf("save tts cache record: %w", err)
	}
	return objectKey, nil
}

func processAll(
	ctx context.Context,
	db *sql.DB,
	uploader storage.Store,
	engine media.SynthesiserEngine,
	defaultVoice, secondVoice string,
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

		if strings.TrimSpace(body.Script) == "" && len(body.Turns) < 2 {
			continue
		}

		// A conversation renders turn by turn, each in its speaker's voice; a
		// monologue in the one configured voice. The voice is configuration,
		// never the item's: see media.ConfiguredVoice.
		var key string
		var err error
		if len(body.Turns) >= 2 {
			key, err = processItemTurns(ctx, db, uploader, engine,
				body.Turns, media.ConfiguredVoice(defaultVoice), media.ConfiguredVoice(secondVoice))
		} else {
			key, err = processItem(ctx, db, uploader, engine, body.Script, media.ConfiguredVoice(defaultVoice))
		}
		if err != nil {
			_, _ = fmt.Fprintf(out, "Warning: failed to synthesise item %s: %v\n", id, err)
			continue
		}
		processed++
		_, _ = fmt.Fprintf(out, "Processed item %s -> %s\n", id, key)
	}

	return processed, rows.Err()
}
