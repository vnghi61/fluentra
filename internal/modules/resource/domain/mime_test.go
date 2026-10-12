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

func TestSniffAndValidateMIME_Spreadsheets(t *testing.T) {
	zipHead := []byte("PK\x03\x04\x14\x00\x06\x00\x08\x00\x00\x00!\x00")
	detected, err := domain.SniffAndValidateMIME(domain.MIMEXLSX, zipHead)
	if err != nil {
		t.Fatalf("xlsx: unexpected error: %v", err)
	}
	if detected != domain.MIMEXLSX {
		t.Errorf("xlsx: got %q, want %q", detected, domain.MIMEXLSX)
	}

	ole2Head := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0x00, 0x00}
	detected, err = domain.SniffAndValidateMIME(domain.MIMEXLS, ole2Head)
	if err != nil {
		t.Fatalf("xls: unexpected error: %v", err)
	}
	if detected != domain.MIMEXLS {
		t.Errorf("xls: got %q, want %q", detected, domain.MIMEXLS)
	}
	if domain.KindOf(domain.MIMEXLSX) != domain.KindDocument {
		t.Errorf("xlsx should be a document")
	}
}

func TestSniffAndValidateMIME_SpreadsheetDeclaredForExecutable(t *testing.T) {
	exeBytes := []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff\x00\x00")
	if _, err := domain.SniffAndValidateMIME(domain.MIMEXLSX, exeBytes); err == nil {
		t.Fatalf("expected executable bytes declared as xlsx to be rejected")
	}
	// A macro-enabled workbook is not on the allow-list.
	zipHead := []byte("PK\x03\x04\x14\x00\x06\x00")
	if _, err := domain.SniffAndValidateMIME("application/vnd.ms-excel.sheet.macroenabled.12", zipHead); err == nil {
		t.Fatalf("expected a macro-enabled workbook to be rejected")
	}
}
