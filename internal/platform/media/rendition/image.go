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

// Image rendition limits and outputs (WO 18 §5).
const (
	// MaxImagePixels is the decompression-bomb ceiling, checked from the header
	// before any pixel is decoded.
	MaxImagePixels    = 50_000_000
	ThumbnailLongEdge = 320
	DisplayLongEdge   = 2048
	JPEGQuality       = 85
	ToolGoImage       = "go-image/draw"

	mimePNG  = "image/png"
	mimeJPEG = "image/jpeg"
)

var (
	// ErrDecompressionBomb refuses an image whose header declares more than
	// MaxImagePixels.
	ErrDecompressionBomb = errors.New("image exceeds maximum allowed resolution (50 megapixels)")
	// ErrUnsupportedKind refuses a rendition kind images do not produce.
	ErrUnsupportedKind = errors.New("unsupported rendition kind for image")
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
		return nil, fmt.Errorf("%w: %dx%d (%d pixels > %d max)",
			ErrDecompressionBomb, cfg.Width, cfg.Height, pixels, MaxImagePixels)
	}

	// 2. Check kind & skip conditions
	targetW, targetH, skip, err := imageTargetSize(req.Kind, cfg.Width, cfg.Height)
	if err != nil || skip != nil {
		return skip, err
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
	dstImg := srcImg
	if targetW != cfg.Width || targetH != cfg.Height {
		rgba := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
		draw.CatmullRom.Scale(rgba, rgba.Bounds(), srcImg, srcImg.Bounds(), draw.Over, nil)
		dstImg = rgba
	}

	// 5. Encode output file
	isPNG := format == "png" || strings.Contains(strings.ToLower(req.SourceMIME), "png")
	outPath, outMIME, size, err := encodeImage(req, dstImg, isPNG)
	if err != nil {
		return nil, err
	}

	return &RenderResult{
		OutputPath:  outPath,
		MIMEType:    outMIME,
		Width:       &targetW,
		Height:      &targetH,
		ByteSize:    &size,
		ToolVersion: ToolGoImage,
	}, nil
}

// imageTargetSize decides the output size for a kind, or a skipped result when
// the source is already small enough that the rendition adds nothing.
func imageTargetSize(kind string, w, h int) (int, int, *RenderResult, error) {
	longEdge := max(w, h)
	switch kind {
	case KindThumbnail:
		if longEdge <= ThumbnailLongEdge {
			return w, h, nil, nil
		}
		tw, th := scalePreservingAspect(w, h, ThumbnailLongEdge)
		return tw, th, nil, nil
	case KindDisplay:
		if longEdge <= ThumbnailLongEdge {
			return 0, 0, &RenderResult{
				Skipped:     true,
				SkipReason:  "source is already smaller than thumbnail threshold (no display rendition needed)",
				ToolVersion: ToolGoImage,
			}, nil
		}
		if longEdge <= DisplayLongEdge {
			// Source is already between 320 and 2048: readable as-is without resampling
			return w, h, nil, nil
		}
		tw, th := scalePreservingAspect(w, h, DisplayLongEdge)
		return tw, th, nil, nil
	default:
		return 0, 0, nil, fmt.Errorf("%w: %s", ErrUnsupportedKind, kind)
	}
}

// encodeImage writes img into the request's temp dir as PNG or JPEG.
func encodeImage(req RenderRequest, img image.Image, isPNG bool) (string, string, int64, error) {
	ext, outMIME := ".jpg", mimeJPEG
	if isPNG {
		ext, outMIME = ".png", mimePNG
	}
	outPath := filepath.Join(req.TempDir, fmt.Sprintf("%s_%s%s", req.ResourceID.String(), req.Kind, ext))

	outFile, err := os.Create(outPath) //nolint:gosec // G304: a file we name, inside the render's own temp dir
	if err != nil {
		return "", "", 0, fmt.Errorf("create output rendition file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	if isPNG {
		err = png.Encode(outFile, img)
	} else {
		err = jpeg.Encode(outFile, img, &jpeg.Options{Quality: JPEGQuality})
	}
	if err != nil {
		return "", "", 0, fmt.Errorf("encode %s rendition: %w", strings.TrimPrefix(ext, "."), err)
	}

	stat, err := outFile.Stat()
	if err != nil {
		return "", "", 0, fmt.Errorf("stat output rendition: %w", err)
	}
	return outPath, outMIME, stat.Size(), nil
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
