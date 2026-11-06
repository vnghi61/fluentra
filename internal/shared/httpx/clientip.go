package httpx

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// forwardedForHeader is the only forwarding header this resolver trusts, and
// only from a trusted peer.
const forwardedForHeader = "X-Forwarded-For"

type clientIPKey struct{}

// ClientIPResolver determines the address a request really came from.
//
// It replaces chi's middleware.RealIP, which rewrites r.RemoteAddr from
// X-Forwarded-For, True-Client-IP or X-Real-IP whether or not the deployment
// actually sets them. Anyone may send those headers, so with RealIP a single
// attacker presents a new "client IP" per request — which defeats per-IP login
// lockout (P2.3) and per-IP rate limiting (P2.8) completely. Those two controls
// are the reason this type exists, so it errs towards the socket address:
// forwarding headers count only when the immediate peer is a proxy we run.
type ClientIPResolver struct {
	trusted []netip.Prefix
	header  string
}

// NewClientIPResolver builds a resolver that trusts the given CIDR ranges as
// proxies. An empty list means no header is ever trusted, which is the correct
// default for a service exposed directly.
func NewClientIPResolver(trustedCIDRs []string) (*ClientIPResolver, error) {
	prefixes := make([]netip.Prefix, 0, len(trustedCIDRs))
	for _, entry := range trustedCIDRs {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			// A bare address is a /32 or /128.
			address, addrErr := netip.ParseAddr(entry)
			if addrErr != nil {
				return nil, fmt.Errorf("parse trusted proxy %q: %w", entry, err)
			}
			prefix = netip.PrefixFrom(address, address.BitLen())
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return &ClientIPResolver{trusted: prefixes}, nil
}

// WithClientIPHeader names a header that carries the client address, read
// before X-Forwarded-For and only from a trusted peer. The edge in front of
// that peer must strip it from requests that did not pass the proxy setting it.
func (r *ClientIPResolver) WithClientIPHeader(name string) *ClientIPResolver {
	r.header = http.CanonicalHeaderKey(strings.TrimSpace(name))
	return r
}

// ClientIP returns the client address for request.
//
// When the peer is not a trusted proxy the socket address wins and any
// forwarding header is ignored. When it is, the configured client-IP header
// wins if it holds an address; otherwise the rightmost entry of
// X-Forwarded-For that is not itself a trusted proxy is the client: entries to
// the left of it were appended by hops the client controls and may be forged.
func (r *ClientIPResolver) ClientIP(request *http.Request) netip.Addr {
	peer := parseAddr(request.RemoteAddr)
	if !peer.IsValid() || !r.isTrusted(peer) {
		return peer
	}

	if address, ok := r.fromClientIPHeader(request); ok {
		return address
	}

	forwarded := request.Header.Get(forwardedForHeader)
	if forwarded == "" {
		return peer
	}

	hops := strings.Split(forwarded, ",")
	for index := len(hops) - 1; index >= 0; index-- {
		candidate, err := netip.ParseAddr(strings.TrimSpace(hops[index]))
		if err != nil {
			// A malformed hop means the chain cannot be trusted past this point.
			return peer
		}
		if !r.isTrusted(candidate) {
			return candidate.Unmap()
		}
	}
	return peer
}

func (r *ClientIPResolver) fromClientIPHeader(request *http.Request) (netip.Addr, bool) {
	if r.header == "" {
		return netip.Addr{}, false
	}
	value := request.Header.Get(r.header)
	if value == "" {
		return netip.Addr{}, false
	}
	first, _, _ := strings.Cut(value, ",")
	address, err := netip.ParseAddr(strings.TrimSpace(first))
	if err != nil {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

func (r *ClientIPResolver) isTrusted(address netip.Addr) bool {
	address = address.Unmap()
	for _, prefix := range r.trusted {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

// Middleware records the resolved client address in the request context.
func (r *ClientIPResolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := context.WithValue(request.Context(), clientIPKey{}, r.ClientIP(request))
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

// ClientIP returns the address recorded by ClientIPResolver.Middleware.
// The zero value is invalid, which callers must treat as "unknown" rather than
// as a shared bucket.
func ClientIP(ctx context.Context) netip.Addr {
	address, _ := ctx.Value(clientIPKey{}).(netip.Addr)
	return address
}

func parseAddr(remoteAddr string) netip.Addr {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	address, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return netip.Addr{}
	}
	return address.Unmap()
}
