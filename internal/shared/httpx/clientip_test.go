package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Documentation-range addresses (RFC 5737) standing in for the three actors in
// these tests: the socket peer we can see, the client address a trusted edge
// proxy reports, and the CIDR our own internal proxies live in.
const (
	peerIP      = "203.0.113.9"
	forwardedIP = "198.51.100.7"
	trustedCIDR = "10.0.0.0/8"
)

func request(remoteAddr, forwardedFor string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	return req
}

// TestClientIP_IgnoresHeadersFromAnUntrustedPeer is the security property.
// chi's RealIP took the header unconditionally, so one attacker could present
// a fresh "client IP" on every request and never trip per-IP lockout or the
// per-IP rate limit.
func TestClientIP_IgnoresHeadersFromAnUntrustedPeer(t *testing.T) {
	t.Parallel()
	resolver, err := NewClientIPResolver(nil)
	if err != nil {
		t.Fatal(err)
	}

	for name, forwarded := range map[string]string{
		"single spoof":  "1.2.3.4",
		"chain spoof":   "1.2.3.4, 5.6.7.8",
		"loopback lie":  "127.0.0.1",
		"garbage":       "not-an-ip",
		"empty element": " , ",
	} {
		got := resolver.ClientIP(request(peerIP+":51234", forwarded))
		if got.String() != peerIP {
			t.Errorf("%s: ClientIP = %s, want the socket address 203.0.113.9", name, got)
		}
	}
}

func TestClientIP_UsesForwardedHeaderFromATrustedProxy(t *testing.T) {
	t.Parallel()
	resolver, err := NewClientIPResolver([]string{trustedCIDR})
	if err != nil {
		t.Fatal(err)
	}

	got := resolver.ClientIP(request("10.0.0.5:443", forwardedIP))
	if got.String() != forwardedIP {
		t.Errorf("ClientIP = %s, want 198.51.100.7", got)
	}
}

// TestClientIP_TakesTheRightmostUntrustedHop is what stops a client from
// prepending forged entries: everything left of the last proxy-appended hop is
// attacker-controlled.
func TestClientIP_TakesTheRightmostUntrustedHop(t *testing.T) {
	t.Parallel()
	resolver, err := NewClientIPResolver([]string{trustedCIDR, "192.168.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}

	// The client forged "1.2.3.4"; the real edge saw 198.51.100.7 and our own
	// internal proxies appended the rest.
	got := resolver.ClientIP(request("10.0.0.5:443", "1.2.3.4, 198.51.100.7, 192.168.1.1"))
	if got.String() != forwardedIP {
		t.Errorf("ClientIP = %s, want 198.51.100.7", got)
	}
}

func TestClientIP_FallsBackToPeerWhenEveryHopIsTrusted(t *testing.T) {
	t.Parallel()
	resolver, _ := NewClientIPResolver([]string{trustedCIDR})
	got := resolver.ClientIP(request("10.0.0.5:443", "10.0.0.1, 10.0.0.2"))
	if got.String() != "10.0.0.5" {
		t.Errorf("ClientIP = %s, want the peer 10.0.0.5", got)
	}
}

func TestNewClientIPResolver_AcceptsBareAddressesAndRejectsGarbage(t *testing.T) {
	t.Parallel()
	resolver, err := NewClientIPResolver([]string{"10.0.0.5", " ", "2001:db8::/32"})
	if err != nil {
		t.Fatalf("bare address should be accepted as a single-host prefix: %v", err)
	}
	if got := resolver.ClientIP(request("10.0.0.5:1", forwardedIP)); got.String() != forwardedIP {
		t.Errorf("ClientIP = %s", got)
	}
	if _, err := NewClientIPResolver([]string{"nonsense"}); err == nil {
		t.Error("expected an error for an unparsable trusted proxy")
	}
}

func TestClientIP_IPv6AndMissingPort(t *testing.T) {
	t.Parallel()
	resolver, _ := NewClientIPResolver(nil)
	if got := resolver.ClientIP(request("[2001:db8::1]:443", "")); got.String() != "2001:db8::1" {
		t.Errorf("IPv6 peer = %s", got)
	}
	if got := resolver.ClientIP(request(peerIP, "")); got.String() != peerIP {
		t.Errorf("peer without a port = %s", got)
	}
	if got := resolver.ClientIP(request("unix", "")); got.IsValid() {
		t.Errorf("unparsable peer should be invalid, got %s", got)
	}
}

func TestClientIP_MiddlewareRecordsAddressInContext(t *testing.T) {
	t.Parallel()
	resolver, _ := NewClientIPResolver(nil)

	var seen string
	handler := resolver.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = ClientIP(r.Context()).String()
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request(peerIP+":51234", "1.2.3.4"))

	if seen != peerIP {
		t.Errorf("context client IP = %s, want 203.0.113.9", seen)
	}
}
