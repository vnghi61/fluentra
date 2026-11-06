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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/fluentra/fluentra/cmd/internal/dbtarget"
	"github.com/fluentra/fluentra/internal/platform/media"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/config"
)

type ttsCLIConfig struct {
	App struct {
		// Environment is APP_ENVIRONMENT, which tts-render.yml sets; Env is
		// APP_ENV, which .env and every other command use. Either names it.
		Environment string `koanf:"environment"`
		Env         string `koanf:"env"`
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
		// TTSLengthScale is Piper's --length_scale; above 1 speaks slower for learners.
		TTSLengthScale string `koanf:"tts_length_scale"`
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
	budgetFlag := flags.Duration("budget", 0,
		"With -all, stop starting new items after this long (e.g. 25m); 0 means no limit")
	allowRemoteFlag := flags.Bool("allow-remote-db", false,
		"Render into a remote database even though APP_ENV does not say production or staging")
	lengthScaleFlag := flags.Float64("length-scale", 0,
		"Piper --length_scale, above 1 is slower; SPEECH_TTS_LENGTH_SCALE, or the voice's own pace")
	rerenderStaleFlag := flags.Bool("rerender-stale", false,
		"Treat a cached clip whose engine version differs from -engine-version as missing")

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
	environment := firstNonEmpty(cfg.App.Environment, cfg.App.Env, "development")
	if err := dbtarget.Check(environment, cfg.Database.DSN, *allowRemoteFlag); err != nil {
		return err
	}
	piperBinary := firstNonEmpty(*piperFlag, cfg.Speech.PiperBinary, media.EnginePiper)
	modelDir := firstNonEmpty(*modelsFlag, cfg.Speech.PiperModels)
	lengthScale, err := lengthScaleFrom(*lengthScaleFlag, cfg.Speech.TTSLengthScale)
	if err != nil {
		return err
	}
	engine, err := engineFor(*engineFlag, piperBinary, modelDir, *versionFlag, environment, lengthScale)
	if err != nil {
		return err
	}
	voice := firstNonEmpty(*voiceFlag, cfg.Speech.TTSVoice, media.DefaultVoice)

	// Said before anything is written. This reads .env like every other command,
	// and a .env pointed at production renders into production.
	_, _ = fmt.Fprintf(out, "Rendering with %s %s, voices %s, into %s; storage %s\n",
		engine.EngineName(), engine.EngineVersion(), media.DialogueVoiceKey(voice, cfg.Speech.TTSVoiceB),
		describeDatabase(cfg.Database.DSN), cfg.Storage.Endpoint)

	db, err := sql.Open("pgx", cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	uploader, err := newUploader(cfg)
	if err != nil {
		return err
	}

	r := renderer{
		db: db, uploader: uploader, engine: engine,
		voiceA: voice, voiceB: cfg.Speech.TTSVoiceB, rerenderStale: *rerenderStaleFlag,
	}
	if *textFlag != "" {
		key, _, err := r.item(ctx, *textFlag, nil)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "Rendered clip: %s\n", key)
	}
	if *allFlag {
		return r.runAll(ctx, *budgetFlag, out)
	}
	return nil
}

// writeStepSummary appends the run's coverage to the GitHub Actions job summary,
// so the run page answers "how much of the catalogue can be heard" without
// reading the log. A path of "" (outside Actions) writes nothing.
func writeStepSummary(path string, s renderSummary) error {
	if path == "" {
		return nil
	}
	// The path is set by the Actions runner, not by input.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // see above
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = fmt.Fprint(f, s.Markdown())
	return err
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
			"app.environment":         "",
			"app.env":                 "",
			"speech.tts_voice":        media.DefaultVoice,
			"speech.tts_voice_b":      "en_US-ryan-medium",
			"speech.tts_length_scale": "",
			"speech.piper_binary":     "",
			"speech.piper_models":     "",
			"s3.endpoint":             "localhost:9000",
			"s3.access_key":           "minioadmin",
			"s3.secret_key":           "minioadmin",
			"s3.region":               "us-east-1",
			"s3.use_ssl":              false,
		},
		EnvSections: []string{"SPEECH"},
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: ".env.example"},
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
func engineFor(
	name, piperBinary, modelDir, version, environment string, lengthScale float64,
) (media.SynthesiserEngine, error) {
	switch name {
	case media.EnginePiper:
		if modelDir == "" {
			return nil, errors.New("the piper voice models directory is required: -models or SPEECH_PIPER_MODELS")
		}
		engine := media.NewPiperEngine(piperBinary, modelDir, version)
		engine.LengthScale = lengthScale
		engine.SentenceSilence = sentencePause
		return engine, nil
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

// lengthScaleFrom is the -length-scale flag, else SPEECH_TTS_LENGTH_SCALE, else 0 (the voice's own pace).
func lengthScaleFrom(flagValue float64, configured string) (float64, error) {
	if flagValue != 0 || strings.TrimSpace(configured) == "" {
		return flagValue, nil
	}
	scale, err := strconv.ParseFloat(strings.TrimSpace(configured), 64)
	if err != nil || scale <= 0 {
		return 0, fmt.Errorf("SPEECH_TTS_LENGTH_SCALE must be a positive number, got %q", configured)
	}
	return scale, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// sentencePause is Piper's --sentence_silence: a learner hears where a sentence ends.
const sentencePause = 0.35

// renderer renders listening scripts into the TTS cache.
type renderer struct {
	db       *sql.DB
	uploader storage.Store
	engine   media.SynthesiserEngine
	voiceA   string
	voiceB   string
	// rerenderStale treats a clip rendered by another engine version, which
	// names the speed too, as missing, so a change of voice or pace reaches
	// clips already in the cache.
	rerenderStale bool
}

// item finds or renders one script: a conversation in two voices without the
// speakers' names, anything else in voice A. It is cached under the script's
// hash, which is what the API looks a clip up by.
func (r renderer) item(
	ctx context.Context, script string, turns []media.ScriptTurn,
) (key string, cached bool, err error) {
	dialogue := media.DialogueTurns(script, turns)
	source := script
	if strings.TrimSpace(source) == "" {
		source = media.JoinScriptTurns(dialogue)
	}
	textHash := media.HashText(source)

	voice := media.ConfiguredVoice(r.voiceA)
	if len(dialogue) >= 2 {
		voice = media.DialogueVoiceKey(r.voiceA, r.voiceB)
	}
	if existing, ok := r.cached(ctx, textHash, voice); ok {
		return existing, true, nil
	}

	var data []byte
	var mimeType string
	if len(dialogue) >= 2 {
		data, mimeType, err = media.RenderTurns(ctx, r.engine, dialogue, r.voiceA, r.voiceB)
	} else {
		data, mimeType, err = r.engine.Render(ctx, script, voice)
	}
	if err != nil {
		return "", false, fmt.Errorf("render audio: %w", err)
	}
	key, err = r.store(ctx, textHash, voice, data, mimeType)
	return key, false, err
}

func (r renderer) cached(ctx context.Context, textHash, voice string) (string, bool) {
	var objectKey, engineVersion string
	err := r.db.QueryRowContext(ctx,
		"SELECT object_key, engine_version FROM content.tts_cache WHERE text_hash = $1 AND voice = $2",
		textHash, voice,
	).Scan(&objectKey, &engineVersion)
	if err != nil || objectKey == "" {
		return "", false
	}
	if r.rerenderStale && engineVersion != r.engine.EngineVersion() {
		return "", false
	}
	return objectKey, true
}

func (r renderer) store(ctx context.Context, textHash, voice string, data []byte, mimeType string) (string, error) {
	objectKey := media.ObjectKey(voice, textHash, mimeType)
	if err := r.uploader.Put(
		ctx, storage.BucketMedia, objectKey, bytes.NewReader(data), int64(len(data)), mimeType,
	); err != nil {
		return "", fmt.Errorf("upload audio to storage: %w", err)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO content.tts_cache (text_hash, voice, engine, engine_version, object_key, created_at)
		VALUES ($1, $2, $3, $4, $5, clock_timestamp())
		ON CONFLICT (text_hash, voice) DO UPDATE
		SET engine = EXCLUDED.engine,
		    engine_version = EXCLUDED.engine_version,
		    object_key = EXCLUDED.object_key,
		    created_at = clock_timestamp()`,
		textHash, voice, r.engine.EngineName(), r.engine.EngineVersion(), objectKey,
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

// listeningItem is one listening version with something to say.
type listeningItem struct {
	ID   string
	Body listeningBody
}

// loadListeningItems reads every listening version up front and closes the
// cursor before anything is rendered.
//
// The loop used to render while iterating the open result set. A render is a
// Piper run, an upload and two queries, so on a full catalogue the cursor stayed
// open for twenty minutes, and the pooler in front of production closed the
// connection under it: rows.Err() came back as a bare "unexpected EOF" and took
// the whole run down with the rest of the items unrendered.
func loadListeningItems(ctx context.Context, db *sql.DB) ([]listeningItem, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, body FROM content.content_versions WHERE kind = 'listening_comprehension' ORDER BY id",
	)
	if err != nil {
		return nil, fmt.Errorf("query listening items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []listeningItem
	for rows.Next() {
		var id string
		var rawBody []byte
		if err := rows.Scan(&id, &rawBody); err != nil {
			return nil, fmt.Errorf("scan content version: %w", err)
		}
		var body listeningBody
		if err := json.Unmarshal(rawBody, &body); err != nil {
			continue
		}
		if strings.TrimSpace(body.Script) == "" && len(body.Turns) < 2 {
			continue
		}
		items = append(items, listeningItem{ID: id, Body: body})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read listening items: %w", err)
	}
	return items, nil
}

// renderSummary is what one -all run did. Cached + Rendered is how many
// listening items can be heard now; Failed + Remaining is how many cannot.
type renderSummary struct {
	Cached    int
	Rendered  int
	Failed    int
	Remaining int
}

// Total is every listening item with a script.
func (s renderSummary) Total() int { return s.Cached + s.Rendered + s.Failed + s.Remaining }

// Markdown is the summary as a GitHub job summary table.
func (s renderSummary) Markdown() string {
	voiced := s.Cached + s.Rendered
	percent := 100
	if total := s.Total(); total > 0 {
		percent = voiced * 100 / total
	}
	return fmt.Sprintf("### Listening audio coverage\n\n"+
		"**%d of %d** listening items have a clip (%d%%).\n\n"+
		"| Already voiced | Rendered now | Failed | Left for the next run |\n"+
		"|---:|---:|---:|---:|\n"+
		"| %d | %d | %d | %d |\n",
		voiced, s.Total(), percent, s.Cached, s.Rendered, s.Failed, s.Remaining)
}

// renderItems renders items until they run out or the deadline passes. A zero
// deadline means no limit. Stopping at the deadline is not a failure: the cache
// skips what is done, so the next run carries on from here.
func renderItems(
	items []listeningItem,
	render func(listeningItem) (key string, cached bool, err error),
	deadline time.Time,
	now func() time.Time,
	out io.Writer,
) renderSummary {
	var summary renderSummary
	for i, item := range items {
		if !deadline.IsZero() && !now().Before(deadline) {
			summary.Remaining = len(items) - i
			_, _ = fmt.Fprintf(out, "Budget spent: stopping with %d item(s) left for the next run\n", summary.Remaining)
			break
		}
		key, cached, err := render(item)
		if err != nil {
			summary.Failed++
			_, _ = fmt.Fprintf(out, "Warning: failed to synthesise item %s: %v\n", item.ID, err)
			continue
		}
		if cached {
			// Already voiced: counted, not printed, so the log shows the work.
			summary.Cached++
			continue
		}
		summary.Rendered++
		_, _ = fmt.Fprintf(out, "Processed item %s -> %s\n", item.ID, key)
	}
	return summary
}

// runAll renders every listening item within budget and reports the coverage.
func (r renderer) runAll(ctx context.Context, budget time.Duration, out io.Writer) error {
	var deadline time.Time
	if budget > 0 {
		deadline = time.Now().Add(budget)
	}
	summary, err := r.all(ctx, deadline, out)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Finished: %d already voiced, %d rendered now, %d failed, %d left for the next run\n",
		summary.Cached, summary.Rendered, summary.Failed, summary.Remaining)
	if err := writeStepSummary(os.Getenv("GITHUB_STEP_SUMMARY"), summary); err != nil {
		_, _ = fmt.Fprintf(out, "Warning: could not write the job summary: %v\n", err)
	}
	return nil
}

func (r renderer) all(ctx context.Context, deadline time.Time, out io.Writer) (renderSummary, error) {
	items, err := loadListeningItems(ctx, r.db)
	if err != nil {
		return renderSummary{}, err
	}
	_, _ = fmt.Fprintf(out, "%d listening item(s) with a script\n", len(items))
	render := func(item listeningItem) (string, bool, error) {
		return r.item(ctx, item.Body.Script, item.Body.Turns)
	}
	return renderItems(items, render, deadline, time.Now, out), nil
}
