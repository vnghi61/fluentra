package domain

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
)

// Allowed MIME types per WO-17 §6.
const (
	MIMEPDF  = "application/pdf"
	MIMEDOC  = "application/msword"
	MIMEDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	MIMEPPT  = "application/vnd.ms-powerpoint"
	MIMEPPTX = "application/vnd.openxmlformats-officedocument.presentationml.presentation"

	MIMEPNG  = "image/png"
	MIMEJPEG = "image/jpeg"
	MIMEWebP = "image/webp"

	MIMEMP3 = "audio/mpeg"
	MIMEWAV = "audio/wav"
	MIMEM4A = "audio/mp4"

	MIMEMP4  = "video/mp4"
	MIMEWebM = "video/webm"
)

var allowedMIMETypes = map[string]bool{
	MIMEPDF:  true,
	MIMEDOC:  true,
	MIMEDOCX: true,
	MIMEPPT:  true,
	MIMEPPTX: true,

	MIMEPNG:  true,
	MIMEJPEG: true,
	MIMEWebP: true,

	MIMEMP3:       true,
	MIMEWAV:       true,
	"audio/x-wav": true,
	"audio/wave":  true,
	MIMEM4A:       true,
	"audio/x-m4a": true,
	"audio/m4a":   true,

	MIMEMP4:      true,
	MIMEWebM:     true,
	"audio/webm": true,
}

// IsAllowedMIME reports whether the MIME type is in the intake allow-list.
func IsAllowedMIME(mime string) bool {
	base := NormalizeMIME(mime)
	return allowedMIMETypes[base]
}

// NormalizeMIME cleans and lowercases a MIME type, stripping parameters.
func NormalizeMIME(mime string) string {
	base, _, _ := strings.Cut(mime, ";")
	return strings.TrimSpace(strings.ToLower(base))
}

// MIMEKind groups MIME types into high-level categories for kind disagreement checks (D17-4).
type MIMEKind string

// Supported high-level MIME kinds.
const (
	KindDocument MIMEKind = "document"
	KindImage    MIMEKind = "image"
	KindAudio    MIMEKind = "audio"
	KindVideo    MIMEKind = "video"
	KindUnknown  MIMEKind = "unknown"
)

// KindOf classifies a MIME type into its high-level family.
func KindOf(mime string) MIMEKind {
	normalized := NormalizeMIME(mime)
	switch {
	case normalized == MIMEPDF || normalized == MIMEDOC || normalized == MIMEDOCX ||
		normalized == MIMEPPT || normalized == MIMEPPTX:
		return KindDocument
	case strings.HasPrefix(normalized, "image/"):
		return KindImage
	case strings.HasPrefix(normalized, "audio/"):
		return KindAudio
	case strings.HasPrefix(normalized, "video/"):
		return KindVideo
	default:
		return KindUnknown
	}
}

// SniffAndValidateMIME inspects the leading bytes of a file, checks against the declared MIME type,
// and ensures the detected type is in the allow-list (BR-RESOURCE-04).
func SniffAndValidateMIME(declared string, head []byte) (string, error) {
	if len(head) == 0 {
		return "", fmt.Errorf("%w: empty byte stream", ErrResourceTypeNotSupported)
	}

	declared = NormalizeMIME(declared)
	detected := detectMagic(head)
	if detected == "" {
		detected = NormalizeMIME(http.DetectContentType(head))
	}
	if detected == "audio/x-wav" || detected == "audio/wave" {
		detected = MIMEWAV
	}

	detected = resolveContainerMIME(detected, declared, head)
	if !IsAllowedMIME(detected) {
		return "", fmt.Errorf("%w: detected mime %q is not in allow-list", ErrResourceTypeNotSupported, detected)
	}

	if err := checkKindAgreement(declared, detected); err != nil {
		return "", err
	}

	return detected, nil
}

func resolveContainerMIME(detected, declared string, head []byte) string {
	switch detected {
	case "application/zip":
		if declared == MIMEDOCX || declared == MIMEPPTX {
			return declared
		}
	case "application/octet-stream":
		if (declared == MIMEDOC || declared == MIMEPPT) && isOLE2Header(head) {
			return declared
		}
	}
	return detected
}

func isOLE2Header(head []byte) bool {
	return len(head) >= 8 && bytes.Equal(head[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
}

func checkKindAgreement(declared, detected string) error {
	declaredKind := KindOf(declared)
	detectedKind := KindOf(detected)
	if declaredKind != KindUnknown && declaredKind != detectedKind {
		return fmt.Errorf(
			"%w: declared kind %q disagrees with detected kind %q (%s vs %s)",
			ErrResourceTypeNotSupported, declaredKind, detectedKind, declared, detected,
		)
	}
	return nil
}

func detectMagic(b []byte) string {
	if mime := detectDocMagic(b); mime != "" {
		return mime
	}
	if mime := detectImageMagic(b); mime != "" {
		return mime
	}
	return detectAudioVideoMagic(b)
}

func detectDocMagic(b []byte) string {
	switch {
	case len(b) >= 5 && bytes.Equal(b[:5], []byte("%PDF-")):
		return MIMEPDF
	case len(b) >= 4 && bytes.Equal(b[:4], []byte("PK\x03\x04")):
		return "application/zip"
	case isOLE2Header(b):
		return "application/octet-stream"
	default:
		return ""
	}
}

func detectImageMagic(b []byte) string {
	switch {
	case len(b) >= 8 && bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")):
		return MIMEPNG
	case len(b) >= 3 && bytes.Equal(b[:3], []byte{0xFF, 0xD8, 0xFF}):
		return MIMEJPEG
	case len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return MIMEWebP
	default:
		return ""
	}
}

func detectAudioVideoMagic(b []byte) string {
	if mime := detectAudioMagic(b); mime != "" {
		return mime
	}
	return detectVideoMagic(b)
}

func detectAudioMagic(b []byte) string {
	switch {
	case len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WAVE")):
		return MIMEWAV
	case len(b) >= 3 && bytes.Equal(b[:3], []byte("ID3")):
		return MIMEMP3
	case len(b) >= 2 && b[0] == 0xFF && (b[1]&0xE0) == 0xE0:
		return MIMEMP3
	case len(b) >= 12 && bytes.Equal(b[4:8], []byte("ftyp")):
		major := string(b[8:12])
		if major == "M4A " || major == "M4B " {
			return MIMEM4A
		}
		return ""
	default:
		return ""
	}
}

func detectVideoMagic(b []byte) string {
	switch {
	case len(b) >= 8 && bytes.Equal(b[4:8], []byte("ftyp")):
		return MIMEMP4
	case len(b) >= 4 && bytes.Equal(b[:4], []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return MIMEWebM
	default:
		return ""
	}
}
