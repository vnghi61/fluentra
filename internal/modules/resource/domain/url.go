package domain

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// ValidateURLScheme checks that a URL has scheme http or https and a non-empty host.
func ValidateURLScheme(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid URL syntax", ErrResourceURLNotAllowed)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("%w: scheme must be http or https, got %q", ErrResourceURLNotAllowed, parsed.Scheme)
	}

	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("%w: missing host", ErrResourceURLNotAllowed)
	}

	return parsed, nil
}

// nonPublicPrefixes are ranges netip's predicates call global unicast but which
// are not the public internet.
//
// 100.64.0.0/10 is the one that matters most: carrier-grade NAT space, where
// several clouds put internal services (Alibaba's metadata endpoint is
// 100.100.100.200). IsPrivate does not cover it. 64:ff9b::/96 is NAT64, which
// maps an IPv6 address straight onto an IPv4 one, private ranges included.
var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),      // "this network"
	netip.MustParsePrefix("100.64.0.0/10"),  // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),   // IETF protocol assignments
	netip.MustParsePrefix("198.18.0.0/15"),  // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),    // reserved, and broadcast
	netip.MustParsePrefix("64:ff9b::/96"),   // NAT64
	netip.MustParsePrefix("64:ff9b:1::/48"), // local-use NAT64
	netip.MustParsePrefix("2001:db8::/32"),  // documentation
}

// IsPublicIP verifies whether an IP address is a safe, publicly routable address.
// Refuses loopback, private, link-local, unspecified, multicast, and the
// special-purpose ranges above (SSRF prevention).
func IsPublicIP(addr netip.Addr) bool {
	unmapped := addr.Unmap()
	if !unmapped.IsValid() {
		return false
	}
	if unmapped.IsLoopback() ||
		unmapped.IsPrivate() ||
		unmapped.IsLinkLocalUnicast() ||
		unmapped.IsLinkLocalMulticast() ||
		unmapped.IsUnspecified() ||
		unmapped.IsMulticast() {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(unmapped) {
			return false
		}
	}
	return unmapped.IsGlobalUnicast()
}

// ResolveAndValidateHost resolves the hostname and verifies that every resolved IP is a public address.
func ResolveAndValidateHost(ctx context.Context, host string) error {
	// If host is already an IP literal
	if addr, err := netip.ParseAddr(host); err == nil {
		if !IsPublicIP(addr) {
			return fmt.Errorf("%w: IP %s is not a public address", ErrResourceURLNotAllowed, host)
		}
		return nil
	}

	// Resolve hostname
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("%w: failed to resolve host %q: %v", ErrResourceURLNotAllowed, host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: no IP addresses found for host %q", ErrResourceURLNotAllowed, host)
	}

	for _, ip := range ips {
		if !IsPublicIP(ip) {
			return fmt.Errorf("%w: host %q resolves to non-public IP %s", ErrResourceURLNotAllowed, host, ip.String())
		}
	}

	return nil
}
