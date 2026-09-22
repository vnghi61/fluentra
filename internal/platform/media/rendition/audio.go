package rendition

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Audio rendering limits and tool label.
const (
	// AudioTimeout bounds one ffmpeg transcode.
	AudioTimeout = 120 * time.Second
	ToolFFmpeg   = "ffmpeg"
)

var durationRegex = regexp.MustCompile(`Duration:\s*(\d+):(\d+):(\d+\.?\d*)`)

// RenderAudio transcodes audio into web AAC (m4a) with 128k bitrate and capped size.
func RenderAudio(ctx context.Context, ffmpegBin string, req RenderRequest) (*RenderResult, error) {
	if ffmpegBin == "" {
		p, err := exec.LookPath("ffmpeg")
		if err != nil {
			return &RenderResult{
				Skipped:     true,
				SkipReason:  "ffmpeg is not installed or available on PATH",
				ToolVersion: "ffmpeg:missing",
			}, nil
		}
		ffmpegBin = p
	}

	execCtx, cancel := context.WithTimeout(ctx, AudioTimeout)
	defer cancel()

	outPath := filepath.Join(req.TempDir, fmt.Sprintf("%s_audio.m4a", req.ResourceID.String()))

	// ffmpeg -nostdin -protocol_whitelist file -i <src> -vn -c:a aac -b:a 128k -fs 50M <out>
	rawArgs := BuildFFmpegArgs(
		"-i", req.SourcePath,
		"-vn",
		"-c:a", "aac",
		"-b:a", "128k",
		"-fs", "50M",
		"-y",
		outPath,
	)

	// Validate security requirements
	if err := ValidateFFmpegArgs(rawArgs); err != nil {
		return nil, err
	}

	//nolint:gosec // G204: LookPath'd ffmpeg; ValidateFFmpegArgs has just checked the protocol whitelist
	cmd := exec.CommandContext(execCtx, ffmpegBin, rawArgs...)
	outBytes, err := cmd.CombinedOutput()
	outputStr := string(outBytes)

	if err != nil {
		return nil, fmt.Errorf("ffmpeg audio transcode failed: %w (output: %s)", err, strings.TrimSpace(outputStr))
	}

	stat, err := os.Stat(outPath)
	if err != nil {
		return nil, fmt.Errorf("stat rendered audio file: %w", err)
	}
	sz := stat.Size()

	durationMS := parseDurationMS(outputStr)

	return &RenderResult{
		OutputPath:  outPath,
		MIMEType:    "audio/mp4",
		DurationMS:  durationMS,
		ByteSize:    &sz,
		ToolVersion: ToolFFmpeg,
	}, nil
}

func parseDurationMS(logOutput string) *int {
	matches := durationRegex.FindStringSubmatch(logOutput)
	if len(matches) < 4 {
		return nil
	}
	hours, _ := strconv.Atoi(matches[1])
	mins, _ := strconv.Atoi(matches[2])
	secs, _ := strconv.ParseFloat(matches[3], 64)

	totalSecs := float64(hours*3600+mins*60) + secs
	ms := int(totalSecs * 1000)
	return &ms
}
