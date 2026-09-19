package domain_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/payment/domain"
)

func TestGenerateReference(t *testing.T) {
	id := uuid.MustParse("0191fa12-3456-789a-bcde-f0123456789a")
	ref := domain.GenerateReference(id)

	if !strings.HasPrefix(ref, "FLU") {
		t.Fatalf("expected reference to start with FLU, got %s", ref)
	}
	if len(ref) != 13 {
		t.Fatalf("expected reference length 13 (FLU + 10), got %d (%s)", len(ref), ref)
	}
	for _, ch := range ref[3:] {
		if (ch < 'A' || ch > 'Z') && (ch < '2' || ch > '7') {
			t.Errorf("expected standard base32 char, got %c in %s", ch, ref)
		}
	}
}

func TestBuildVietQRURL(t *testing.T) {
	url := domain.BuildVietQRURL("VCB", "1017588888", 490000, "FLUABCDE12345")
	expectedParts := []string{
		"https://vietqr.app/img?",
		"acc=1017588888",
		"bank=VCB",
		"amount=490000",
		"des=FLUABCDE12345",
		"template=compact",
	}
	for _, part := range expectedParts {
		if !strings.Contains(url, part) {
			t.Errorf("expected QR url to contain %q, got %s", part, url)
		}
	}
}

func TestStripNonAlphanumeric(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello, World! 123", "HelloWorld123"},
		{"FLU12345-CHUYEN-TIEN", "FLU12345CHUYENTIEN"},
		{"@#$%^&*()_+", ""},
		{"ABC 123 xyz", "ABC123xyz"},
	}

	for _, c := range cases {
		got := domain.StripNonAlphanumeric(c.input)
		if got != c.expected {
			t.Errorf("StripNonAlphanumeric(%q) = %q; want %q", c.input, got, c.expected)
		}
	}
}

func TestContentMatchesReference(t *testing.T) {
	cases := []struct {
		content   string
		reference string
		expected  bool
	}{
		{"NGUYEN VAN A chuyen tien FLUABCDE12345 them", "FLUABCDE12345", true},
		{"SEVN63DC8E5C fluabcde12345 chuyen khoan", "FLUABCDE12345", true},
		{"FLU-ABCDE-12345 transfer", "FLUABCDE12345", true},
		{"Chuyen tien hoc phi", "FLUABCDE12345", false},
		{"", "FLUABCDE12345", false},
		{"Chuyen khoan", "", false},
	}

	for _, c := range cases {
		got := domain.ContentMatchesReference(c.content, c.reference)
		if got != c.expected {
			t.Errorf("ContentMatchesReference(%q, %q) = %v; want %v", c.content, c.reference, got, c.expected)
		}
	}
}
