package domain_test

import (
	"testing"

	"github.com/fluentra/fluentra/internal/modules/resource/domain"
)

func TestSniffAndValidateMIME_PDF(t *testing.T) {
	pdfBytes := []byte("%PDF-1.4\n...")
	detected, err := domain.SniffAndValidateMIME("application/pdf", pdfBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detected != domain.MIMEPDF {
		t.Errorf("got %q, want %q", detected, domain.MIMEPDF)
	}
}

func TestSniffAndValidateMIME_PNG(t *testing.T) {
	pngBytes := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR...")
	detected, err := domain.SniffAndValidateMIME("image/png", pngBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detected != domain.MIMEPNG {
		t.Errorf("got %q, want %q", detected, domain.MIMEPNG)
	}
}

func TestSniffAndValidateMIME_KindDisagreement(t *testing.T) {
	// PDF file claiming to be PNG (document claiming to be image)
	pdfBytes := []byte("%PDF-1.4\n...")
	_, err := domain.SniffAndValidateMIME("image/png", pdfBytes)
	if err == nil {
		t.Fatalf("expected error for kind disagreement (PDF claiming to be PNG), got nil")
	}
}

func TestSniffAndValidateMIME_ExecutableRejection(t *testing.T) {
	// Windows PE executable header MZ
	exeBytes := []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff\x00\x00")
	_, err := domain.SniffAndValidateMIME("application/pdf", exeBytes)
	if err == nil {
		t.Fatalf("expected error for executable bytes claiming to be PDF, got nil")
	}
}

func TestSniffAndValidateMIME_Empty(t *testing.T) {
	_, err := domain.SniffAndValidateMIME("application/pdf", nil)
	if err == nil {
		t.Fatalf("expected error for empty bytes, got nil")
	}
}
