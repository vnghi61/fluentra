package domain_test

import (
	"net/netip"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/resource/domain"
)

func TestValidateURLScheme(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://example.com/article", false},
		{"http://example.com/page", false},
		{"ftp://example.com/file", true},
		{"file:///etc/passwd", true},
		{"gopher://example.com", true},
		{"javascript:alert(1)", true},
		{"not-a-url", true},
		{"https://", true},
	}

	for _, tt := range tests {
		_, err := domain.ValidateURLScheme(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateURLScheme(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
		}
	}
}

func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		ip       string
		isPublic bool
	}{
		// Public IPv4
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"93.184.216.34", true},

		// Loopback
		{"127.0.0.1", false},
		{"127.0.1.1", false},

		// Private IPv4 (RFC 1918)
		{"10.0.0.1", false},
		{"172.16.0.1", false},
		{"192.168.1.1", false},

		// Link-local / Carrier-grade NAT
		{"169.254.1.1", false},
		{"0.0.0.0", false},

		// Multicast
		{"224.0.0.1", false},

		// IPv6 Loopback & Unspecified
		{"::1", false},
		{"::", false},

		// IPv4-mapped IPv6 loopback / private
		{"::ffff:127.0.0.1", false},
		{"::ffff:192.168.1.1", false},
		{"::ffff:8.8.8.8", true},
	}

	for _, tt := range tests {
		addr, err := netip.ParseAddr(tt.ip)
		if err != nil {
			t.Fatalf("failed to parse IP %q: %v", tt.ip, err)
		}
		got := domain.IsPublicIP(addr)
		if got != tt.isPublic {
			t.Errorf("IsPublicIP(%q) = %v, want %v", tt.ip, got, tt.isPublic)
		}
	}
}
