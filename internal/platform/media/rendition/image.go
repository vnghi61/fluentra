package rendition

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

const (
	MaxImagePixels    = 50_000_000
	ThumbnailLongEdge = 320
	DisplayLongEdge   = 2048
	JPEGQuality       = 85
	ToolGoImage       = "go-image/draw"
)

var (
	ErrDecompressionBomb = errors.New("image exceeds maximum allowed resolution (50 megapixels)")
	ErrUnsupportedKind   = errors.New("unsupported rendition kind for image")
)

// RenderImage processes an image file purely in Go using golang.org/x/image/draw.
func RenderImage(ctx context.Context, req RenderRequest) (*RenderResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	f, err := os.Open(req.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("open source image: %w", err)
	}
	defer func() { _ = f.Close() }()

	// 1. Decompression bomb guard: inspect config BEFORE decoding full pixels
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("decode image config: %w", err)
	}

	pixels := int64(cfg.Width) * int64(cfg.Height)
	if pixels > MaxImagePixels {
		return nil, fmt.Errorf("%w: %dx%d (%d pixels > %d max)", ErrDecompressionBomb, cfg.Width, cfg.Height, pixels, MaxImagePixels)
	}

	origW := cfg.Width
	origH := cfg.Height
	longEdge := max(origW, origH)

	// 2. Check kind & skip conditions
	var targetW, targetH int
	switch req.Kind {
	case KindThumbnail:
		if longEdge <= ThumbnailLongEdge {
			targetW = origW
			targetH = origH
		} else {
			targetW, targetH = scalePreservingAspect(origW, origH, ThumbnailLongEdge)
		}
	case KindDisplay:
		if longEdge <= ThumbnailLongEdge {
			return &RenderResult{
				Skipped:     true,
				SkipReason:  "source is already smaller than thumbnail threshold (no display rendition needed)",
				ToolVersion: ToolGoImage,
			}, nil
		}
		if longEdge <= DisplayLongEdge {
			// Source is already between 320 and 2048: readable as-is without resampling
			targetW = origW
			targetH = origH
		} else {
			targetW, targetH = scalePreservingAspect(origW, origH, DisplayLongEdge)
		}
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedKind, req.Kind)
	}

	// 3. Rewind and decode image
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek source image: %w", err)
	}

	srcImg, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode source image: %w", err)
	}

	// 4. Resize if dimensions changed
	var dstImg image.Image
	if targetW == origW && targetH == origH {
		dstImg = srcImg
	} else {
		rgba := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
		draw.CatmullRom.Scale(rgba, rgba.Bounds(), srcImg, srcImg.Bounds(), draw.Over, nil)
		dstImg = rgba
	}

	// 5. Encode output file
	isPNG := format == "png" || strings.Contains(strings.ToLower(req.SourceMIME), "png")
	var ext, outMIME string
	if isPNG {
		ext = ".png"
		outMIME = "image/png"
	} else {
		ext = ".jpg"
		outMIME = "image/jpeg"
	}

	outFilename := fmt.Sprintf("%s_%s%s", req.ResourceID.String(), req.Kind, ext)
	outPath := filepath.Join(req.TempDir, outFilename)

	outFile, err := os.Create(outPath)
	if err != nil {
		return nil, fmt.Errorf("create output rendition file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	if isPNG {
		if err := png.Encode(outFile, dstImg); err != nil {
			return nil, fmt.Errorf("encode png rendition: %w", err)
		}
	} else {
		if err := jpeg.Encode(outFile, dstImg, &jpeg.Options{Quality: JPEGQuality}); err != nil {
			return nil, fmt.Errorf("encode jpeg rendition: %w", err)
		}
	}

	stat, err := outFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat output rendition: %w", err)
	}
	size := stat.Size()

	return &RenderResult{
		OutputPath:  outPath,
		MIMEType:    outMIME,
		Width:       &targetW,
		Height:      &targetH,
		ByteSize:    &size,
		ToolVersion: ToolGoImage,
	}, nil
}

func scalePreservingAspect(w, h, maxLongEdge int) (int, int) {
	if w <= 0 || h <= 0 || maxLongEdge <= 0 {
		return 1, 1
	}
	if w >= h {
		targetW := maxLongEdge
		targetH := int(math.Round(float64(h) * float64(maxLongEdge) / float64(w)))
		return targetW, max(1, targetH)
	}
	targetH := maxLongEdge
	targetW := int(math.Round(float64(w) * float64(maxLongEdge) / float64(h)))
	return max(1, targetW), targetH
}
