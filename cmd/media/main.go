// Package main provides the offline media CLI tool for generating resource renditions.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
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

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("media", flag.ContinueOnError)
	allFlag := flags.Bool("all", false, "Render all pending renditions for validated file resources")
	limitFlag := flags.Int("limit", 50, "Maximum number of renditions to process in one batch")
	kindsFlag := flags.String("kinds", "", "Comma-separated filter of kinds to render (image,pdf,office,audio,video)")
	dryRunFlag := flags.Bool("dry-run", false, "Execute render operations without uploading to S3 or updating DB status")
	ffmpegFlag := flags.String("ffmpeg", "", "Path to the ffmpeg binary (defaults to PATH)")
	pdftoppmFlag := flags.String("pdftoppm", "", "Path to the pdftoppm binary (defaults to PATH)")
	sofficeFlag := flags.String("soffice", "", "Path to the soffice (LibreOffice) binary (defaults to PATH)")
	pdftotextFlag := flags.String("pdftotext", "", "Path to the pdftotext binary (defaults to PATH)")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if !*allFlag {
		_, _ = fmt.Fprintln(out, "Usage: media -all [-limit 50] [-kinds image,pdf,office,audio,video] [-dry-run] [-ffmpeg <bin>] [-pdftoppm <bin>] [-soffice <bin>]")
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

	queries := sqlc.New(pool)
	jobClient, err := job.NewClientFromPool(pool)
	if err != nil {
		slog.WarnContext(ctx, "could not initialize job client; classification jobs will not be queued", "error", err)
	}

	_, _ = fmt.Fprintf(out, "Running media renderer into %s; storage %s (limit: %d, dry-run: %v)\n",
		describeDatabase(cfg.Database.DSN), cfg.Storage.Endpoint, *limitFlag, *dryRunFlag)

	// 1. Plan missing renditions for validated file resources
	plannedCount, err := planRenditions(ctx, queries, int32(*limitFlag))
	if err != nil {
		return fmt.Errorf("plan renditions: %w", err)
	}
	if plannedCount > 0 {
		_, _ = fmt.Fprintf(out, "Planned %d pending renditions\n", plannedCount)
	}

	// 2. Claim pending renditions
	claimed, err := queries.ClaimPendingRenditions(ctx, int32(*limitFlag))
	if err != nil {
		return fmt.Errorf("claim pending renditions: %w", err)
	}
	if len(claimed) == 0 {
		_, _ = fmt.Fprintln(out, "No pending renditions to render.")
		return nil
	}
	_, _ = fmt.Fprintf(out, "Claimed %d renditions for processing\n", len(claimed))

	allowedKinds := parseKindsFilter(*kindsFlag)

	processed := 0
	readyCount := 0
	skippedCount := 0
	failedCount := 0

	for _, item := range claimed {
		// Kind filter check
		if !isKindAllowed(item.Kind, allowedKinds) {
			continue
		}

		res, err := queries.GetResourceByID(ctx, item.ResourceID)
		if err != nil {
			slog.ErrorContext(ctx, "could not fetch resource for rendition", "resource_id", item.ResourceID, "error", err)
			_, _ = queries.UpdateRenditionFailed(ctx, sqlc.UpdateRenditionFailedParams{
				ID:            item.ID,
				FailureReason: fmt.Sprintf("fetch resource: %v", err),
			})
			failedCount++
			continue
		}

		if res.ObjectKey == nil || *res.ObjectKey == "" {
			_, _ = queries.UpdateRenditionSkipped(ctx, sqlc.UpdateRenditionSkippedParams{
				ID:            item.ID,
				FailureReason: "resource has no original object key in storage",
			})
			skippedCount++
			continue
		}

		processed++
		result, rerr := processRendition(ctx, store, item, res, *ffmpegFlag, *pdftoppmFlag, *sofficeFlag, *pdftotextFlag)
		if rerr != nil {
			slog.WarnContext(ctx, "rendition failed", "resource_id", item.ResourceID, "kind", item.Kind, "error", rerr)
			_, _ = queries.UpdateRenditionFailed(ctx, sqlc.UpdateRenditionFailedParams{
				ID:            item.ID,
				FailureReason: rerr.Error(),
			})
			failedCount++
			continue
		}

		// Record extracted text if present (Stage B: PDF / Office documents)
		if result != nil && result.ExtractedText != nil && !*dryRunFlag {
			text := *result.ExtractedText
			charCount := int32(len([]rune(text)))
			toolVer := result.TextToolVersion
			if toolVer == "" {
				toolVer = "poppler:pdftotext"
			}
			err = dbx.InTx(ctx, pool, func(txCtx context.Context, tx pgx.Tx) error {
				txQueries := queries.WithTx(tx)
				_, err := txQueries.UpsertExtraction(txCtx, sqlc.UpsertExtractionParams{
					ResourceID:  item.ResourceID,
					Source:      "pdf_text",
					Text:        text,
					CharCount:   charCount,
					Truncated:   result.TextTruncated,
					Language:    "",
					ToolVersion: toolVer,
				})
				if err != nil {
					return err
				}
				if text != "" && jobClient != nil {
					_, err = jobClient.EnqueueTx(txCtx, tx, resourcejob.ClassifyResourceArgs{ResourceID: item.ResourceID}, nil)
					if err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				slog.ErrorContext(ctx, "failed to record extraction and enqueue classify job", "error", err, "resource_id", item.ResourceID)
			}
		}

		if result.Skipped {
			_, _ = fmt.Fprintf(out, "Skipped rendition [%s] for resource %s: %s\n", item.Kind, item.ResourceID, result.SkipReason)
			if !*dryRunFlag {
				_, _ = queries.UpdateRenditionSkipped(ctx, sqlc.UpdateRenditionSkippedParams{
					ID:            item.ID,
					FailureReason: result.SkipReason,
				})
			}
			skippedCount++
			continue
		}

		// Upload rendered file to fluentra-derived
		ext := extensionForMIME(result.MIMEType, item.Kind)
		objectKey := fmt.Sprintf("renditions/%s/%s%s", item.ResourceID.String(), item.Kind, ext)

		if !*dryRunFlag {
			f, err := os.Open(result.OutputPath)
			if err != nil {
				_, _ = queries.UpdateRenditionFailed(ctx, sqlc.UpdateRenditionFailedParams{
					ID:            item.ID,
					FailureReason: fmt.Sprintf("open output rendition: %v", err),
				})
				failedCount++
				continue
			}

			sz := int64(0)
			if result.ByteSize != nil {
				sz = *result.ByteSize
			}

			// Explicitly set Content-Type on derived upload (Trap 8)
			if err := store.Put(ctx, storage.BucketDerived, objectKey, f, sz, result.MIMEType); err != nil {
				_ = f.Close()
				slog.ErrorContext(ctx, "failed to upload rendition to fluentra-derived", "error", err, "key", objectKey)
				_, _ = queries.UpdateRenditionFailed(ctx, sqlc.UpdateRenditionFailedParams{
					ID:            item.ID,
					FailureReason: fmt.Sprintf("upload to derived storage: %v", err),
				})
				failedCount++
				continue
			}
			_ = f.Close()

			var w, h, d *int32
			if result.Width != nil {
				v := int32(*result.Width)
				w = &v
			}
			if result.Height != nil {
				v := int32(*result.Height)
				h = &v
			}
			if result.DurationMS != nil {
				v := int32(*result.DurationMS)
				d = &v
			}

			_, err = queries.UpdateRenditionReady(ctx, sqlc.UpdateRenditionReadyParams{
				ID:          item.ID,
				ObjectKey:   &objectKey,
				MimeType:    result.MIMEType,
				Width:       w,
				Height:      h,
				DurationMs:  d,
				ByteSize:    result.ByteSize,
				ToolVersion: result.ToolVersion,
			})
			if err != nil {
				slog.ErrorContext(ctx, "failed to update rendition ready in db", "error", err)
				failedCount++
				continue
			}
		}

		readyCount++
		_, _ = fmt.Fprintf(out, "Rendered [%s] for resource %s -> %s (%s, size %d)\n",
			item.Kind, item.ResourceID, objectKey, result.MIMEType, derefInt64(result.ByteSize))
	}

	_, _ = fmt.Fprintf(out, "Finished media render: processed %d (ready: %d, skipped: %d, failed: %d)\n",
		processed, readyCount, skippedCount, failedCount)
	return nil
}

func processRendition(
	ctx context.Context,
	store storage.Store,
	item sqlc.ResourceRendition,
	res sqlc.ResourceResource,
	ffmpegBin, pdftoppmBin, sofficeBin, pdftotextBin string,
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
	dstFile, err := os.Create(srcLocalPath)
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
		PDFToTextBin: pdftotextBin,
	}

	mimeLower := strings.ToLower(res.DetectedMime)
	switch {
	case strings.HasPrefix(mimeLower, "image/"):
		return rendition.RenderImage(ctx, req)
	case mimeLower == "application/pdf":
		return rendition.RenderPDF(ctx, pdftoppmBin, req)
	case isOfficeMIME(mimeLower):
		return rendition.RenderOffice(ctx, sofficeBin, pdftoppmBin, req)
	case strings.HasPrefix(mimeLower, "audio/"):
		return rendition.RenderAudio(ctx, ffmpegBin, req)
	case strings.HasPrefix(mimeLower, "video/"):
		return rendition.RenderVideo(ctx, ffmpegBin, req)
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
		return []string{rendition.KindPoster, rendition.KindVideo360p, rendition.KindVideo720p}
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

func extensionForMIME(mime, kind string) string {
	m := strings.ToLower(mime)
	switch {
	case strings.Contains(m, "png"):
		return ".png"
	case strings.Contains(m, "jpeg") || strings.Contains(m, "jpg"):
		return ".jpg"
	case strings.Contains(m, "mp4") && kind == rendition.KindAudioWeb:
		return ".m4a"
	case strings.Contains(m, "mp4"):
		return ".mp4"
	case strings.Contains(m, "aac"):
		return ".m4a"
	default:
		if kind == rendition.KindThumbnail || kind == rendition.KindPreview {
			return ".png"
		}
		if kind == rendition.KindPoster {
			return ".jpg"
		}
		if kind == rendition.KindAudioWeb {
			return ".m4a"
		}
		return ".mp4"
	}
}

func parseKindsFilter(kindsFlag string) map[string]bool {
	if strings.TrimSpace(kindsFlag) == "" {
		return nil
	}
	m := make(map[string]bool)
	parts := strings.Split(kindsFlag, ",")
	for _, p := range parts {
		k := strings.ToLower(strings.TrimSpace(p))
		if k != "" {
			m[k] = true
		}
	}
	return m
}

func isKindAllowed(kind string, filter map[string]bool) bool {
	if len(filter) == 0 {
		return true
	}
	if filter[kind] {
		return true
	}
	// Category aliases
	if filter["image"] && (kind == rendition.KindThumbnail || kind == rendition.KindDisplay) {
		return true
	}
	if (filter["pdf"] || filter["office"]) && (kind == rendition.KindThumbnail || kind == rendition.KindPreview) {
		return true
	}
	if filter["audio"] && kind == rendition.KindAudioWeb {
		return true
	}
	if filter["video"] && (kind == rendition.KindPoster || kind == rendition.KindVideo360p || kind == rendition.KindVideo720p) {
		return true
	}
	return false
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
