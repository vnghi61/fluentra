package rendition_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/platform/media/rendition"
)

// TestFFmpegArgs_ProtocolWhitelistRequired verifies that every ffmpeg command built
// includes -protocol_whitelist file to prevent SSRF (Trap 1, §13).
func TestFFmpegArgs_ProtocolWhitelistRequired(t *testing.T) {
	testCases := [][]string{
		{"-i", "input.mp4", "out.mp4"},
		{"-ss", "1", "-i", "video.mp4", "-frames:v", "1", "poster.jpg"},
		{"-i", "audio.mp3", "-vn", "-c:a", "aac", "out.m4a"},
	}

	for _, tc := range testCases {
		built := rendition.BuildFFmpegArgs(tc...)
		if err := rendition.ValidateFFmpegArgs(built); err != nil {
			t.Errorf("built args %v failed validation: %v", built, err)
		}

		// Direct inspection
		found := false
		for i, a := range built {
			if a == rendition.ProtocolWhitelistArg && i+1 < len(built) && built[i+1] == rendition.ProtocolWhitelistVal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("args %v missing %s %s", built, rendition.ProtocolWhitelistArg, rendition.ProtocolWhitelistVal)
		}
	}
}

// TestImage_DecompressionBombRefused verifies that a PNG claiming 60000x60000
// is refused before it is decoded, preventing memory exhaustion (Trap 2, §13).
func TestImage_DecompressionBombRefused(t *testing.T) {
	tmpDir := t.TempDir()
	bombPath := filepath.Join(tmpDir, "bomb.png")

	// Craft a minimal PNG header with 60000x60000 dimensions (3.6 gigapixels)
	// PNG signature (8 bytes) + IHDR chunk (25 bytes)
	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n") // PNG signature

	// IHDR chunk: 4 bytes length (13), 4 bytes "IHDR", 4 bytes width, 4 bytes height, 5 bytes specs, 4 bytes CRC
	ihdrData := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdrData[0:4], 60000) // Width = 60000
	binary.BigEndian.PutUint32(ihdrData[4:8], 60000) // Height = 60000
	ihdrData[8] = 8                                  // Bit depth
	ihdrData[9] = 2                                  // Color type (truecolor)
	ihdrData[10] = 0                                 // Compression
	ihdrData[11] = 0                                 // Filter
	ihdrData[12] = 0                                 // Interlace

	ihdrChunk := make([]byte, 8+13+4)
	binary.BigEndian.PutUint32(ihdrChunk[0:4], 13)
	copy(ihdrChunk[4:8], "IHDR")
	copy(ihdrChunk[8:21], ihdrData)
	crc := crc32.ChecksumIEEE(ihdrChunk[4:21])
	binary.BigEndian.PutUint32(ihdrChunk[21:25], crc)

	buf.Write(ihdrChunk)

	if err := os.WriteFile(bombPath, buf.Bytes(), 0600); err != nil {
		t.Fatalf("write bomb file: %v", err)
	}

	req := rendition.RenderRequest{
		ResourceID: uuid.New(),
		Kind:       rendition.KindThumbnail,
		SourcePath: bombPath,
		SourceMIME: mimePNG,
		TempDir:    tmpDir,
	}

	_, err := rendition.RenderImage(context.Background(), req)
	if err == nil {
		t.Fatal("expected decompression bomb to be refused, got nil error")
	}
	if !strings.Contains(err.Error(), "50 megapixels") {
		t.Errorf("expected error to mention 50 megapixels, got %v", err)
	}
}

// TestImage_Resizing_NoUpscale verifies that images are never upscaled, and thumbnail/display
// dimensions respect the requirements (BR-RESOURCE-12, D18-3).
func TestImage_Resizing_NoUpscale(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create a large 2560x1920 PNG image (4:3, > 2048px)
	largePNGPath := filepath.Join(tmpDir, "large.png")
	createSolidPNG(t, largePNGPath, 2560, 1920, color.RGBA{R: 200, G: 50, B: 50, A: 255})

	// 1a. Thumbnail rendition of large PNG -> 320x240 PNG
	thumbReq := rendition.RenderRequest{
		ResourceID: uuid.New(),
		Kind:       rendition.KindThumbnail,
		SourcePath: largePNGPath,
		SourceMIME: mimePNG,
		TempDir:    tmpDir,
	}
	thumbRes, err := rendition.RenderImage(context.Background(), thumbReq)
	if err != nil {
		t.Fatalf("render thumbnail of large png: %v", err)
	}
	if *thumbRes.Width != 320 || *thumbRes.Height != 240 {
		t.Errorf("thumbnail dimensions = %dx%d, want 320x240", *thumbRes.Width, *thumbRes.Height)
	}
	if thumbRes.MIMEType != mimePNG {
		t.Errorf("mime = %s, want image/png", thumbRes.MIMEType)
	}

	// 1b. Display rendition of large PNG -> 2048x1536 PNG
	dispReq := rendition.RenderRequest{
		ResourceID: uuid.New(),
		Kind:       rendition.KindDisplay,
		SourcePath: largePNGPath,
		SourceMIME: mimePNG,
		TempDir:    tmpDir,
	}
	dispRes, err := rendition.RenderImage(context.Background(), dispReq)
	if err != nil {
		t.Fatalf("render display of large png: %v", err)
	}
	if *dispRes.Width != 2048 || *dispRes.Height != 1536 {
		t.Errorf("display dimensions = %dx%d, want 2048x1536", *dispRes.Width, *dispRes.Height)
	}
	if dispRes.MIMEType != mimePNG {
		t.Errorf("mime = %s, want image/png", dispRes.MIMEType)
	}

	// 2. Small 200x150 JPEG image (below thumbnail size)
	smallJPEGPath := filepath.Join(tmpDir, "small.jpg")
	createSolidJPEG(t, smallJPEGPath, 200, 150)

	// 2a. Thumbnail does not upscale: stays 200x150
	smallThumbReq := rendition.RenderRequest{
		ResourceID: uuid.New(),
		Kind:       rendition.KindThumbnail,
		SourcePath: smallJPEGPath,
		SourceMIME: "image/jpeg",
		TempDir:    tmpDir,
	}
	smallThumbRes, err := rendition.RenderImage(context.Background(), smallThumbReq)
	if err != nil {
		t.Fatalf("render thumbnail of small jpeg: %v", err)
	}
	if *smallThumbRes.Width != 200 || *smallThumbRes.Height != 150 {
		t.Errorf("small thumb dimensions = %dx%d, want 200x150 (never upscale)", *smallThumbRes.Width, *smallThumbRes.Height)
	}

	// 2b. Display rendition is skipped for small image (source already smaller than thumbnail)
	smallDispReq := rendition.RenderRequest{
		ResourceID: uuid.New(),
		Kind:       rendition.KindDisplay,
		SourcePath: smallJPEGPath,
		SourceMIME: "image/jpeg",
		TempDir:    tmpDir,
	}
	smallDispRes, err := rendition.RenderImage(context.Background(), smallDispReq)
	if err != nil {
		t.Fatalf("render display of small jpeg: %v", err)
	}
	if !smallDispRes.Skipped {
		t.Errorf("expected display of small image to be skipped, got result: %+v", smallDispRes)
	}
}

const mimePNG = "image/png"

func createSolidPNG(t *testing.T, path string, w, h int, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path) //nolint:gosec // G304: the test's own temp file
	if err != nil {
		t.Fatalf("create solid png: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode solid png: %v", err)
	}
}

func createSolidJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	f, err := os.Create(path) //nolint:gosec // G304: the test's own temp file
	if err != nil {
		t.Fatalf("create solid jpeg: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode solid jpeg: %v", err)
	}
}
