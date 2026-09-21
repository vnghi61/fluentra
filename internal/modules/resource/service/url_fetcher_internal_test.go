package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/resource/domain"
)

// TestSafeHTTPFetcher_DialerRefusesNonPublicAddress is the SSRF guard that
// holds under DNS rebinding.
//
// FetchMetadata resolves the host and checks it before connecting, but that
// check alone is not one: the HTTP client resolves the name again to connect,
// and a DNS server that answers a public address the first time and a private
// one the second walks through. This test goes round the pre-check entirely -
// it drives the client, as a rebinding attacker effectively does - and asserts
// the connection itself is refused. Every redirect hop dials through the same
// transport, so this is also the redirect guard.
func TestSafeHTTPFetcher_DialerRefusesNonPublicAddress(t *testing.T) {
	loopback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<title>internal admin</title>"))
	}))
	defer loopback.Close()

	fetcher, ok := NewSafeHTTPFetcher().(*safeHTTPFetcher)
	if !ok {
		t.Fatal("NewSafeHTTPFetcher did not return the safe fetcher")
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, loopback.URL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := fetcher.client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("the client connected to a loopback address")
	}
	if !errors.Is(err, domain.ErrResourceURLNotAllowed) {
		t.Fatalf("expected ErrResourceURLNotAllowed from the dialer, got %v", err)
	}
}

// TestSafeHTTPFetcher_IgnoresProxyEnvironment. A proxy is a connection to the
// proxy: with one configured, the dial check would inspect the proxy's address
// and approve a request for anything behind it.
func TestSafeHTTPFetcher_IgnoresProxyEnvironment(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://203.0.113.10:3128")
	t.Setenv("HTTPS_PROXY", "http://203.0.113.10:3128")

	fetcher, ok := NewSafeHTTPFetcher().(*safeHTTPFetcher)
	if !ok {
		t.Fatal("NewSafeHTTPFetcher did not return the safe fetcher")
	}
	transport, ok := fetcher.client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected an *http.Transport")
	}
	if transport.Proxy != nil {
		t.Fatal("the fetcher honours proxy settings, which bypasses the dial check")
	}
}

func TestIsPublicIP_SpecialPurposeRanges(t *testing.T) {
	refused := []string{
		"100.100.100.200", // carrier-grade NAT; a cloud metadata address
		"0.1.2.3",         // "this network"
		"198.18.0.1",      // benchmarking
		"240.0.0.1",       // reserved
		"64:ff9b::a00:1",  // NAT64 onto 10.0.0.1
		"::ffff:127.0.0.1",
		"169.254.169.254",
	}
	for _, raw := range refused {
		if domain.IsPublicIP(netip.MustParseAddr(raw)) {
			t.Errorf("%s was treated as public", raw)
		}
	}
	if !domain.IsPublicIP(netip.MustParseAddr("93.184.216.34")) {
		t.Error("a public address was refused")
	}
}
