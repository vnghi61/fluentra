package rendition

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // registers the JPEG decoder that measures the poster frame
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Video limits (WO 18 D18-6) and timeouts.
const (
	// MaxVideoSizeBytes is the largest source that gets web renditions; above
	// it the original and a poster are kept.
	MaxVideoSizeBytes   = 200 * 1024 * 1024 // 200 MB per D18-6
	MaxVideoDurationSec = 10 * 60           // 10 minutes per D18-6
	PosterTimeout       = 30 * time.Second
	VideoTimeout        = 20 * time.Minute
)

// RenderVideo handles poster frames and progressive MP4 encodings for video resources.
func RenderVideo(ctx context.Context, ffmpegBin string, req RenderRequest) (*RenderResult, error) {
	if ffmpegBin == "" {
		p, err := exec.LookPath("ffmpeg")
		if err != nil {
			//nolint:nilerr // a missing tool is a skipped rendition, not a failed one
			return &RenderResult{
				Skipped:     true,
				SkipReason:  "ffmpeg is not installed or available on PATH",
				ToolVersion: "ffmpeg:missing",
			}, nil
		}
		ffmpegBin = p
	}

	srcStat, err := os.Stat(req.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("stat video source: %w", err)
	}

	// 1. Poster rendition
	if req.Kind == KindPoster {
		return renderPoster(ctx, ffmpegBin, req)
	}

	// 2. Video renditions (video_360p, video_720p)
	// Check D18-6 size limit: anything > 200 MB keeps original & poster only
	if srcStat.Size() > MaxVideoSizeBytes {
		return &RenderResult{
			Skipped:     true,
			SkipReason:  fmt.Sprintf("video source size (%d bytes) exceeds 200MB limit (D18-6)", srcStat.Size()),
			ToolVersion: ToolFFmpeg,
		}, nil
	}

	switch req.Kind {
	case KindVideo360p:
		return renderVideoProfile(ctx, ffmpegBin, req, 360)
	case KindVideo720p:
		return renderVideoProfile(ctx, ffmpegBin, req, 720)
	default:
		return nil, fmt.Errorf("%w: %s for video", ErrUnsupportedKind, req.Kind)
	}
}

func renderPoster(ctx context.Context, ffmpegBin string, req RenderRequest) (*RenderResult, error) {
	execCtx, cancel := context.WithTimeout(ctx, PosterTimeout)
	defer cancel()

	outPath := filepath.Join(req.TempDir, fmt.Sprintf("%s_poster.jpg", req.ResourceID.String()))

	// ffmpeg -nostdin -protocol_whitelist file -ss 1 -i <src> -frames:v 1 -vf scale='min(1280,iw)':-2 -y <out>
	rawArgs := BuildFFmpegArgs(
		"-ss", "1",
		"-i", req.SourcePath,
		"-frames:v", "1",
		"-vf", "scale='min(1280,iw)':-2",
		"-y",
		outPath,
	)

	if err := ValidateFFmpegArgs(rawArgs); err != nil {
		return nil, err
	}

	//nolint:gosec // G204: LookPath'd ffmpeg; ValidateFFmpegArgs has just checked the protocol whitelist
	cmd := exec.CommandContext(execCtx, ffmpegBin, rawArgs...)
	if _, err := cmd.CombinedOutput(); err != nil {
		// If seeking 1s failed (video might be shorter than 1s), retry at 0s
		retryArgs := BuildFFmpegArgs(
			"-ss", "0",
			"-i", req.SourcePath,
			"-frames:v", "1",
			"-vf", "scale='min(1280,iw)':-2",
			"-y",
			outPath,
		)
		if err := ValidateFFmpegArgs(retryArgs); err != nil {
			return nil, err
		}
		//nolint:gosec // G204: as above, with the retry's validated arguments
		cmdRetry := exec.CommandContext(execCtx, ffmpegBin, retryArgs...)
		outBytesRetry, errRetry := cmdRetry.CombinedOutput()
		if errRetry != nil {
			return nil, fmt.Errorf("ffmpeg poster extraction failed: %w (output: %s)",
				errRetry, strings.TrimSpace(string(outBytesRetry)))
		}
	}

	stat, err := os.Stat(outPath)
	if err != nil {
		return nil, fmt.Errorf("stat poster file: %w", err)
	}
	sz := stat.Size()

	f, err := os.Open(outPath) //nolint:gosec // G304: the poster we just wrote into our temp dir
	var w, h *int
	if err == nil {
		if cfg, _, err := image.DecodeConfig(f); err == nil {
			wVal := cfg.Width
			hVal := cfg.Height
			w = &wVal
			h = &hVal
		}
		_ = f.Close()
	}

	return &RenderResult{
		OutputPath:  outPath,
		MIMEType:    mimeJPEG,
		Width:       w,
		Height:      h,
		ByteSize:    &sz,
		ToolVersion: ToolFFmpeg,
	}, nil
}

func renderVideoProfile(ctx context.Context, ffmpegBin string, req RenderRequest, maxH int) (*RenderResult, error) {
	execCtx, cancel := context.WithTimeout(ctx, VideoTimeout)
	defer cancel()

	outFilename := fmt.Sprintf("%s_%dp.mp4", req.ResourceID.String(), maxH)
	outPath := filepath.Join(req.TempDir, outFilename)

	// ffmpeg -nostdin -protocol_whitelist file -i <src> -vf scale=-2:'min(maxH,ih)'
	//   -c:v libx264 -preset veryfast -crf 23 -c:a aac -b:a 128k -movflags +faststart -y <out>
	scaleFilter := fmt.Sprintf("scale=-2:'min(%d,ih)'", maxH)
	rawArgs := BuildFFmpegArgs(
		"-i", req.SourcePath,
		"-vf", scaleFilter,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-y",
		outPath,
	)

	if err := ValidateFFmpegArgs(rawArgs); err != nil {
		return nil, err
	}

	//nolint:gosec // G204: LookPath'd ffmpeg; ValidateFFmpegArgs has just checked the protocol whitelist
	cmd := exec.CommandContext(execCtx, ffmpegBin, rawArgs...)
	outBytes, err := cmd.CombinedOutput()
	outputStr := string(outBytes)

	if err != nil {
		return nil, fmt.Errorf("ffmpeg video transcode (%dp) failed: %w (output: %s)",
			maxH, err, strings.TrimSpace(outputStr))
	}

	// Check duration limit from ffmpeg output
	durationMS := parseDurationMS(outputStr)
	if durationMS != nil && *durationMS > MaxVideoDurationSec*1000 {
		// Clean up output file
		_ = os.Remove(outPath)
		return &RenderResult{
			Skipped:     true,
			SkipReason:  fmt.Sprintf("video duration (%d ms) exceeds 10 minutes limit (D18-6)", *durationMS),
			ToolVersion: ToolFFmpeg,
		}, nil
	}

	stat, err := os.Stat(outPath)
	if err != nil {
		return nil, fmt.Errorf("stat encoded video file: %w", err)
	}
	sz := stat.Size()

	return &RenderResult{
		OutputPath:  outPath,
		MIMEType:    "video/mp4",
		DurationMS:  durationMS,
		ByteSize:    &sz,
		ToolVersion: ToolFFmpeg,
	}, nil
}
