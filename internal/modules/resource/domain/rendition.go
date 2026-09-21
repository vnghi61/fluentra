package domain

// Rendition kinds per WO-18 §2 and §5.
const (
	RenditionKindThumbnail = "thumbnail"
	RenditionKindDisplay   = "display"
	RenditionKindPreview   = "preview"
	RenditionKindAudioWeb  = "audio_web"
	RenditionKindPoster    = "poster"
	RenditionKindVideo360p = "video_360p"
	RenditionKindVideo720p = "video_720p"
)

// Rendition statuses per WO-18 §5.
const (
	RenditionStatusPending = "pending"
	RenditionStatusReady   = "ready"
	RenditionStatusFailed  = "failed"
	RenditionStatusSkipped = "skipped"
)

// PlannedRenditionsForMIME returns the rendition kinds expected for a detected MIME type.
func PlannedRenditionsForMIME(mime string) []string {
	switch KindOf(mime) {
	case KindImage:
		return []string{RenditionKindThumbnail, RenditionKindDisplay}
	case KindDocument:
		return []string{RenditionKindThumbnail, RenditionKindPreview}
	case KindAudio:
		return []string{RenditionKindAudioWeb}
	case KindVideo:
		return []string{RenditionKindPoster, RenditionKindVideo360p, RenditionKindVideo720p}
	default:
		return nil
	}
}
