// Package main provides the offline media CLI tool for generating resource renditions.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	sqlc "github.com/fluentra/fluentra/internal/generated/resource/sqlc"
	resourcejob "github.com/fluentra/fluentra/internal/modules/resource/job"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/media/rendition"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/config"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

type mediaCLIConfig struct {
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
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "media renderer error: %v\n", err)
		os.Exit(1)
	}
}

// mediaOptions are the CLI's flags.
type mediaOptions struct {
	all, dryRun                        bool
	limit                              int32
	kinds                              []string
	ffmpeg, pdftoppm, soffice, pdftext string
}

// mediaRun is one batch: what it talks to and what it has counted.
type mediaRun struct {
	opts      mediaOptions
	out       io.Writer
	pool      *pgxpool.Pool
	queries   *sqlc.Queries
	store     storage.Store
	jobClient *job.Client

	processed, ready, skipped, failed int
}

const usage = "Usage: media -all [-limit 50] [-kinds image,pdf,office,audio,video] [-dry-run]\n" +
	"             [-ffmpeg <bin>] [-pdftoppm <bin>] [-soffice <bin>] [-pdftotext <bin>]"

func parseOptions(args []string) (mediaOptions, error) {
	flags := flag.NewFlagSet("media", flag.ContinueOnError)
	allFlag := flags.Bool("all", false, "Render all pending renditions for validated file resources")
	limitFlag := flags.Int("limit", 50, "Maximum number of renditions to process in one batch (1-1000)")
	kindsFlag := flags.String("kinds", "", "Comma-separated filter of kinds to render (image,pdf,office,audio,video)")
	dryRunFlag := flags.Bool("dry-run", false, "Execute render operations without uploading to S3 or updating DB status")
	ffmpegFlag := flags.String("ffmpeg", "", "Path to the ffmpeg binary (defaults to PATH)")
	pdftoppmFlag := flags.String("pdftoppm", "", "Path to the pdftoppm binary (defaults to PATH)")
	sofficeFlag := flags.String("soffice", "", "Path to the soffice (LibreOffice) binary (defaults to PATH)")
	pdftotextFlag := flags.String("pdftotext", "", "Path to the pdftotext binary (defaults to PATH)")
	if err := flags.Parse(args); err != nil {
		return mediaOptions{}, err
	}
	if *limitFlag < 1 || *limitFlag > maxBatch {
		return mediaOptions{}, fmt.Errorf("-limit must be between 1 and %d", maxBatch)
	}
	return mediaOptions{
		all:      *allFlag,
		dryRun:   *dryRunFlag,
		limit:    int32(*limitFlag), //nolint:gosec // G115: bounded to 1..maxBatch just above
		kinds:    expandKindsFilter(*kindsFlag),
		ffmpeg:   *ffmpegFlag,
		pdftoppm: *pdftoppmFlag,
		soffice:  *sofficeFlag,
		pdftext:  *pdftotextFlag,
	}, nil
}

const maxBatch = 1000

func run(ctx context.Context, args []string, out io.Writer) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	if !opts.all {
		_, _ = fmt.Fprintln(out, usage)
		return nil
	}

	cfg, err := loadMediaConfig(ctx)
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	store, err := newStorage(cfg)
	if err != nil {
		return err
	}
	jobClient, err := job.NewClientFromPool(pool)
	if err != nil {
		slog.WarnContext(ctx, "could not initialize job client; classification jobs will not be queued", "error", err)
	}

	_, _ = fmt.Fprintf(out, "Running media renderer into %s; storage %s (limit: %d, dry-run: %v)\n",
		describeDatabase(cfg.Database.DSN), cfg.Storage.Endpoint, opts.limit, opts.dryRun)

	r := &mediaRun{opts: opts, out: out, pool: pool, queries: sqlc.New(pool), store: store, jobClient: jobClient}
	return r.renderBatch(ctx)
}

func (r *mediaRun) renderBatch(ctx context.Context) error {
	// 1. Plan missing renditions for validated file resources
	plannedCount, err := planRenditions(ctx, r.queries, r.opts.limit)
	if err != nil {
		return fmt.Errorf("plan renditions: %w", err)
	}
	if plannedCount > 0 {
		_, _ = fmt.Fprintf(r.out, "Planned %d pending renditions\n", plannedCount)
	}

	// 2. Claim pending renditions of the requested kinds
	claimed, err := r.queries.ClaimPendingRenditions(ctx, sqlc.ClaimPendingRenditionsParams{
		LimitCount: r.opts.limit,
		Kinds:      r.opts.kinds,
	})
	if err != nil {
		return fmt.Errorf("claim pending renditions: %w", err)
	}
	if len(claimed) == 0 {
		_, _ = fmt.Fprintln(r.out, "No pending renditions to render.")
		return nil
	}
	_, _ = fmt.Fprintf(r.out, "Claimed %d renditions for processing\n", len(claimed))

	for _, item := range claimed {
		r.renderOne(ctx, item)
		if r.opts.dryRun {
			// A dry run renders nothing durable; give the attempt back.
			_ = r.queries.ReleaseRenditionClaim(ctx, item.ID)
		}
	}

	_, _ = fmt.Fprintf(r.out, "Finished media render: processed %d (ready: %d, skipped: %d, failed: %d)\n",
		r.processed, r.ready, r.skipped, r.failed)
	return nil
}

func (r *mediaRun) fail(ctx context.Context, item sqlc.ResourceRendition, reason string) {
	if !r.opts.dryRun {
		_, _ = r.queries.UpdateRenditionFailed(ctx, sqlc.UpdateRenditionFailedParams{ID: item.ID, FailureReason: reason})
	}
	r.failed++
}

func (r *mediaRun) skip(ctx context.Context, item sqlc.ResourceRendition, reason string) {
	r.skipped++
	if r.opts.dryRun {
		return
	}
	err := dbx.InTx(ctx, r.pool, func(txCtx context.Context, tx pgx.Tx) error {
		if _, err := r.queries.WithTx(tx).UpdateRenditionSkipped(txCtx, sqlc.UpdateRenditionSkippedParams{
			ID: item.ID, FailureReason: reason,
		}); err != nil {
			return err
		}
		return r.queueTranscription(txCtx, tx, item)
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to record skipped rendition", "error", err, "rendition_id", item.ID)
	}
}

// queueTranscription queues the transcript once audio_web settles, ready or
// skipped: the worker then transcribes the rendition, or the upload when there
// is none (WO 19 B.1). The renderer is the only thing that knows audio_web has
// settled, so it is the one that queues it.
func (r *mediaRun) queueTranscription(ctx context.Context, tx pgx.Tx, item sqlc.ResourceRendition) error {
	if item.Kind != rendition.KindAudioWeb || r.jobClient == nil {
		return nil
	}
	_, err := r.jobClient.EnqueueTx(ctx, tx, resourcejob.TranscribeResourceArgs{ResourceID: item.ResourceID}, nil)
	return err
}

func (r *mediaRun) renderOne(ctx context.Context, item sqlc.ResourceRendition) {
	res, err := r.queries.GetResourceByID(ctx, item.ResourceID)
	if err != nil {
		slog.ErrorContext(ctx, "could not fetch resource for rendition", "resource_id", item.ResourceID, "error", err)
		r.fail(ctx, item, fmt.Sprintf("fetch resource: %v", err))
		return
	}
	if res.ObjectKey == nil || *res.ObjectKey == "" {
		r.skip(ctx, item, "resource has no original object key in storage")
		return
	}

	r.processed++
	result, err := processRendition(ctx, r.store, item, res, r.opts)
	if err != nil {
		slog.WarnContext(ctx, "rendition failed", "resource_id", item.ResourceID, "kind", item.Kind, "error", err)
		r.fail(ctx, item, err.Error())
		return
	}

	// Record extracted text if present (Stage B: PDF / Office documents)
	if result.ExtractedText != nil && !r.opts.dryRun {
		r.recordExtraction(ctx, item, result)
	}

	if result.Skipped {
		_, _ = fmt.Fprintf(r.out, "Skipped rendition [%s] for resource %s: %s\n",
			item.Kind, item.ResourceID, result.SkipReason)
		r.skip(ctx, item, result.SkipReason)
		return
	}

	objectKey := fmt.Sprintf("renditions/%s/%s%s",
		item.ResourceID.String(), item.Kind, extensionForMIME(result.MIMEType, item.Kind))
	if !r.opts.dryRun {
		if reason := r.uploadAndMarkReady(ctx, item, result, objectKey); reason != "" {
			r.fail(ctx, item, reason)
			return
		}
	}

	r.ready++
	_, _ = fmt.Fprintf(r.out, "Rendered [%s] for resource %s -> %s (%s, size %d)\n",
		item.Kind, item.ResourceID, objectKey, result.MIMEType, derefInt64(result.ByteSize))
}

// recordExtraction stores the text a render extracted and queues its
// classification in one transaction.
func (r *mediaRun) recordExtraction(ctx context.Context, item sqlc.ResourceRendition, result *rendition.RenderResult) {
	text := *result.ExtractedText
	toolVer := result.TextToolVersion
	if toolVer == "" {
		toolVer = rendition.ToolPopplerText
	}
	err := dbx.InTx(ctx, r.pool, func(txCtx context.Context, tx pgx.Tx) error {
		_, err := r.queries.WithTx(tx).UpsertExtraction(txCtx, sqlc.UpsertExtractionParams{
			ResourceID:  item.ResourceID,
			Source:      "pdf_text",
			Text:        text,
			CharCount:   clampInt32(len([]rune(text))),
			Truncated:   result.TextTruncated,
			Language:    "",
			ToolVersion: toolVer,
		})
		if err != nil {
			return err
		}
		if text == "" || r.jobClient == nil {
			return nil
		}
		_, err = r.jobClient.EnqueueTx(txCtx, tx, resourcejob.ClassifyResourceArgs{ResourceID: item.ResourceID}, nil)
		return err
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to record extraction and enqueue classify job",
			"error", err, "resource_id", item.ResourceID)
	}
}

// uploadAndMarkReady puts the rendered file in fluentra-derived and marks the
// rendition ready. It returns a failure reason, or "" on success.
func (r *mediaRun) uploadAndMarkReady(
	ctx context.Context, item sqlc.ResourceRendition, result *rendition.RenderResult, objectKey string,
) string {
	f, err := os.Open(result.OutputPath) //nolint:gosec // G304: the renderer's output inside our temp dir
	if err != nil {
		return fmt.Sprintf("open output rendition: %v", err)
	}
	defer func() { _ = f.Close() }()

	// Explicitly set Content-Type on derived upload (Trap 8)
	size := derefInt64(result.ByteSize)
	if err := r.store.Put(ctx, storage.BucketDerived, objectKey, f, size, result.MIMEType); err != nil {
		slog.ErrorContext(ctx, "failed to upload rendition to fluentra-derived", "error", err, "key", objectKey)
		return fmt.Sprintf("upload to derived storage: %v", err)
	}

	err = dbx.InTx(ctx, r.pool, func(txCtx context.Context, tx pgx.Tx) error {
		if _, err := r.queries.WithTx(tx).UpdateRenditionReady(txCtx, sqlc.UpdateRenditionReadyParams{
			ID:          item.ID,
			ObjectKey:   &objectKey,
			MimeType:    result.MIMEType,
			Width:       int32Ptr(result.Width),
			Height:      int32Ptr(result.Height),
			DurationMs:  int32Ptr(result.DurationMS),
			ByteSize:    result.ByteSize,
			ToolVersion: result.ToolVersion,
		}); err != nil {
			return err
		}
		return r.queueTranscription(txCtx, tx, item)
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to update rendition ready in db", "error", err)
		return fmt.Sprintf("mark rendition ready: %v", err)
	}
	return ""
}

func clampInt32(v int) int32 {
	switch {
	case v > math.MaxInt32:
		return math.MaxInt32
	case v < math.MinInt32:
		return math.MinInt32
	default:
		return int32(v)
	}
}

func int32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	c := clampInt32(*v)
	return &c
}

func processRendition(
	ctx context.Context,
	store storage.Store,
	item sqlc.ResourceRendition,
	res sqlc.ResourceResource,
	opts mediaOptions,
) (*rendition.RenderResult, error) {
	// Create temp directory for downloading and processing
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("rendition-%s-%s-*", item.ResourceID, item.Kind))
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Download original from fluentra-uploads
	rc, err := store.Get(ctx, storage.BucketUploads, *res.ObjectKey)
	if err != nil {
		return nil, fmt.Errorf("download source from fluentra-uploads: %w", err)
	}
	defer func() { _ = rc.Close() }()

	srcExt := filepath.Ext(*res.ObjectKey)
	if srcExt == "" {
		srcExt = ".bin"
	}
	srcLocalPath := filepath.Join(tmpDir, "source"+srcExt)
	dstFile, err := os.Create(srcLocalPath) //nolint:gosec // G304: a fixed name inside our own temp dir
	if err != nil {
		return nil, fmt.Errorf("create local source file: %w", err)
	}
	if _, err := io.Copy(dstFile, rc); err != nil {
		_ = dstFile.Close()
		return nil, fmt.Errorf("save source to temp file: %w", err)
	}
	_ = dstFile.Close()

	req := rendition.RenderRequest{
		ResourceID:   item.ResourceID,
		Kind:         item.Kind,
		SourcePath:   srcLocalPath,
		SourceMIME:   res.DetectedMime,
		TempDir:      tmpDir,
		PDFToTextBin: opts.pdftext,
	}

	mimeLower := strings.ToLower(res.DetectedMime)
	switch {
	case item.Kind == rendition.KindAudioWeb:
		// Audio or a video's soundtrack alike: ffmpeg -vn takes the audio.
		return rendition.RenderAudio(ctx, opts.ffmpeg, req)
	case strings.HasPrefix(mimeLower, "image/"):
		return rendition.RenderImage(ctx, req)
	case mimeLower == "application/pdf":
		return rendition.RenderPDF(ctx, opts.pdftoppm, req)
	case isOfficeMIME(mimeLower):
		return rendition.RenderOffice(ctx, opts.soffice, opts.pdftoppm, req)
	case strings.HasPrefix(mimeLower, "audio/"):
		return rendition.RenderAudio(ctx, opts.ffmpeg, req)
	case strings.HasPrefix(mimeLower, "video/"):
		return rendition.RenderVideo(ctx, opts.ffmpeg, req)
	default:
		return &rendition.RenderResult{
			Skipped:    true,
			SkipReason: fmt.Sprintf("unsupported detected MIME type: %s", res.DetectedMime),
		}, nil
	}
}

func planRenditions(ctx context.Context, queries *sqlc.Queries, limit int32) (int, error) {
	resources, err := queries.ListValidatedFileResourcesForRenditions(ctx, limit)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, res := range resources {
		kinds := plannedKindsForMIME(res.DetectedMime)
		for _, k := range kinds {
			row, err := queries.InsertRenditionPending(ctx, sqlc.InsertRenditionPendingParams{
				ResourceID: res.ID,
				Kind:       k,
			})
			if err == nil && row.ID != uuid.Nil {
				count++
			}
		}
	}
	return count, nil
}

func plannedKindsForMIME(mime string) []string {
	m := strings.ToLower(mime)
	switch {
	case strings.HasPrefix(m, "image/"):
		return []string{rendition.KindThumbnail, rendition.KindDisplay}
	case m == "application/pdf" || isOfficeMIME(m):
		return []string{rendition.KindThumbnail, rendition.KindPreview}
	case strings.HasPrefix(m, "audio/"):
		return []string{rendition.KindAudioWeb}
	case strings.HasPrefix(m, "video/"):
		return []string{rendition.KindPoster, rendition.KindAudioWeb, rendition.KindVideo360p, rendition.KindVideo720p}
	default:
		return nil
	}
}

func isOfficeMIME(m string) bool {
	return strings.Contains(m, "word") ||
		strings.Contains(m, "document") ||
		strings.Contains(m, "presentation") ||
		strings.Contains(m, "powerpoint") ||
		strings.Contains(m, "sheet") ||
		strings.Contains(m, "excel") ||
		strings.Contains(m, "officedocument")
}

const (
	extPNG = ".png"
	extJPG = ".jpg"
	extM4A = ".m4a"
	extMP4 = ".mp4"
)

func extensionForMIME(mime, kind string) string {
	m := strings.ToLower(mime)
	switch {
	case strings.Contains(m, "png"):
		return extPNG
	case strings.Contains(m, "jpeg") || strings.Contains(m, "jpg"):
		return extJPG
	case strings.Contains(m, "mp4") && kind == rendition.KindAudioWeb:
		return extM4A
	case strings.Contains(m, "mp4"):
		return extMP4
	case strings.Contains(m, "aac"):
		return extM4A
	}
	switch kind {
	case rendition.KindThumbnail, rendition.KindPreview:
		return extPNG
	case rendition.KindPoster:
		return extJPG
	case rendition.KindAudioWeb:
		return extM4A
	default:
		return extMP4
	}
}

// kindAliases maps a -kinds category to the rendition kinds it covers.
var kindAliases = map[string][]string{
	"image":  {rendition.KindThumbnail, rendition.KindDisplay},
	"pdf":    {rendition.KindThumbnail, rendition.KindPreview},
	"office": {rendition.KindThumbnail, rendition.KindPreview},
	"audio":  {rendition.KindAudioWeb},
	"video":  {rendition.KindPoster, rendition.KindAudioWeb, rendition.KindVideo360p, rendition.KindVideo720p},
}

// expandKindsFilter turns -kinds into the concrete rendition kinds to claim, or
// nil for all of them. A category expands to its kinds; a kind stands for itself.
func expandKindsFilter(kindsFlag string) []string {
	seen := map[string]bool{}
	var kinds []string
	for _, part := range strings.Split(kindsFlag, ",") {
		k := strings.ToLower(strings.TrimSpace(part))
		if k == "" {
			continue
		}
		expanded, isAlias := kindAliases[k]
		if !isAlias {
			expanded = []string{k}
		}
		for _, e := range expanded {
			if !seen[e] {
				seen[e] = true
				kinds = append(kinds, e)
			}
		}
	}
	return kinds
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func describeDatabase(dsn string) string {
	parsed, err := pgx.ParseConfig(dsn)
	if err != nil {
		return "database DSN"
	}
	return fmt.Sprintf("database %s on %s", parsed.Database, parsed.Host)
}

func loadMediaConfig(ctx context.Context) (mediaCLIConfig, error) {
	var cfg mediaCLIConfig
	opts := config.Options{
		Defaults: map[string]any{
			"app.environment": "development",
			"s3.endpoint":     "localhost:9000",
			"s3.access_key":   "minioadmin",
			"s3.secret_key":   "minioadmin",
			"s3.region":       "us-east-1",
			"s3.use_ssl":      false,
		},
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"},
		},
	}
	if err := config.Load(ctx, opts, &cfg); err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func newStorage(cfg mediaCLIConfig) (storage.Store, error) {
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
