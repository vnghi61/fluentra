// Package rendition turns an uploaded resource into the web renditions WO 18
// describes — thumbnails, previews, web audio and video — and extracts the text
// Stage B classifies. Every external tool is invoked with a timeout, and ffmpeg
// only ever with -protocol_whitelist file.
package rendition

import (
	"context"

	"github.com/google/uuid"
)

// Supported rendition kinds per WO-18 §2 and §5.
const (
	KindThumbnail = "thumbnail"
	KindDisplay   = "display"
	KindPreview   = "preview"
	KindAudioWeb  = "audio_web"
	KindPoster    = "poster"
	KindVideo360p = "video_360p"
	KindVideo720p = "video_720p"
)

// RenderRequest contains all inputs needed to render one rendition.
type RenderRequest struct {
	ResourceID   uuid.UUID
	Kind         string
	SourcePath   string
	SourceMIME   string
	TempDir      string
	PDFToTextBin string
}

// RenderResult contains the outcome and measurements of a render operation.
type RenderResult struct {
	OutputPath  string
	MIMEType    string
	Width       *int
	Height      *int
	DurationMS  *int
	ByteSize    *int64
	ToolVersion string
	Skipped     bool
	SkipReason  string

	// Text extraction (Stage B)
	ExtractedText   *string
	TextTruncated   bool
	TextToolVersion string
}

// Renderer performs rendering for specific media formats.
type Renderer interface {
	Render(ctx context.Context, req RenderRequest) (*RenderResult, error)
}
