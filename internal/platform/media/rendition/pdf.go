package rendition

import (
	"context"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	PDFTimeout     = 60 * time.Second
	ToolPopplerPPM = "poppler:pdftoppm"
)

// RenderPDF converts the first page of a PDF into a preview or thumbnail rendition.
func RenderPDF(ctx context.Context, pdftoppmBin string, req RenderRequest) (*RenderResult, error) {
	if pdftoppmBin == "" {
		p, err := exec.LookPath("pdftoppm")
		if err != nil {
			return &RenderResult{
				Skipped:     true,
				SkipReason:  "pdftoppm is not installed or available on PATH",
				ToolVersion: "pdftoppm:missing",
			}, nil
		}
		pdftoppmBin = p
	}

	execCtx, cancel := context.WithTimeout(ctx, PDFTimeout)
	defer cancel()

	outPrefix := filepath.Join(req.TempDir, fmt.Sprintf("%s_page", req.ResourceID.String()))
	// pdftoppm -f 1 -l 1 -png -scale-to 1600 in.pdf outPrefix
	cmd := exec.CommandContext(execCtx, pdftoppmBin, "-f", "1", "-l", "1", "-png", "-scale-to", "1600", req.SourcePath, outPrefix)
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pdftoppm execution failed: %w (output: %s)", err, strings.TrimSpace(string(outBytes)))
	}

	// Locate generated PNG: pdftoppm typically outputs <outPrefix>-1.png or <outPrefix>-01.png or <outPrefix>.png
	matches, err := filepath.Glob(outPrefix + "*.png")
	if err != nil || len(matches) == 0 {
		return nil, fmt.Errorf("pdftoppm did not produce an output image matching prefix %s", outPrefix)
	}
	firstPagePNG := matches[0]

	// If kind is preview, the 1600px rendered page is our preview image
	if req.Kind == KindPreview {
		f, err := os.Open(firstPagePNG)
		if err != nil {
			return nil, fmt.Errorf("open rendered pdf page image: %w", err)
		}
		cfg, _, err := image.DecodeConfig(f)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("decode pdf page config: %w", err)
		}

		stat, err := os.Stat(firstPagePNG)
		if err != nil {
			return nil, fmt.Errorf("stat rendered pdf page image: %w", err)
		}
		sz := stat.Size()
		w := cfg.Width
		h := cfg.Height

		finalOut := filepath.Join(req.TempDir, fmt.Sprintf("%s_preview.png", req.ResourceID.String()))
		if err := os.Rename(firstPagePNG, finalOut); err != nil {
			// If rename fails across volumes, fallback to using firstPagePNG
			finalOut = firstPagePNG
		}

		return &RenderResult{
			OutputPath:  finalOut,
			MIMEType:    "image/png",
			Width:       &w,
			Height:      &h,
			ByteSize:    &sz,
			ToolVersion: ToolPopplerPPM,
		}, nil
	}

	// If kind is thumbnail, scale down the first page PNG to thumbnail
	if req.Kind == KindThumbnail {
		thumbReq := RenderRequest{
			ResourceID: req.ResourceID,
			Kind:       KindThumbnail,
			SourcePath: firstPagePNG,
			SourceMIME: "image/png",
			TempDir:    req.TempDir,
		}
		return RenderImage(ctx, thumbReq)
	}

	return nil, fmt.Errorf("%w: %s for pdf", ErrUnsupportedKind, req.Kind)
}
