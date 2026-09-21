package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/fluentra/fluentra/internal/modules/resource/domain"
)

var (
	titleRegex   = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	ogTitleRegex = regexp.MustCompile(`(?i)<meta\s+[^>]*property=["']og:title["'][^>]*content=["']([^"']+)["']`)
	ogTitleAlt   = regexp.MustCompile(`(?i)<meta\s+[^>]*content=["']([^"']+)["'][^>]*property=["']og:title["']`)
)

// URLMetadata holds the metadata extracted from an external URL.
type URLMetadata struct {
	FinalURL string
	Title    string
}

// URLFetcher fetches metadata from an external URL while strictly enforcing SSRF protection on every hop.
type URLFetcher interface {
	FetchMetadata(ctx context.Context, targetURL string) (*URLMetadata, error)
}

type safeHTTPFetcher struct {
	client *http.Client
}

// Fetch limits. The body is read only for its title (D17-2), so these are small.
const (
	fetchTimeout        = 10 * time.Second
	fetchDialTimeout    = 5 * time.Second
	fetchMaxRedirects   = 5
	fetchMaxHeaderBytes = 64 << 10
	fetchMaxBodyBytes   = 512 << 10
)

// NewSafeHTTPFetcher constructs a URLFetcher that refuses to connect to any
// non-public address.
//
// The check is made in the dialer, on the address actually being connected to.
// Checking a hostname's DNS answer first and then letting the client connect is
// not a check: the client resolves the name again, and a DNS server that answers
// a public address the first time and 169.254.169.254 the second walks straight
// through. The dialer is also where every redirect hop connects, so one check
// covers the first request, each hop, and rebinding.
func NewSafeHTTPFetcher() URLFetcher {
	dialer := &net.Dialer{
		Timeout: fetchDialTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: unparseable dial address", domain.ErrResourceURLNotAllowed)
			}
			addr, err := netip.ParseAddr(host)
			if err != nil || !domain.IsPublicIP(addr) {
				return fmt.Errorf("%w: refusing to connect to a non-public address", domain.ErrResourceURLNotAllowed)
			}
			return nil
		},
	}

	transport := &http.Transport{
		// No proxy, ever. A proxy is a connection to the proxy, so the dial check
		// would see the proxy's address and approve a request for anything.
		Proxy:                  nil,
		DialContext:            dialer.DialContext,
		TLSHandshakeTimeout:    fetchDialTimeout,
		ResponseHeaderTimeout:  fetchDialTimeout,
		MaxResponseHeaderBytes: fetchMaxHeaderBytes,
		DisableKeepAlives:      true,
	}

	return &safeHTTPFetcher{
		client: &http.Client{
			Timeout:   fetchTimeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= fetchMaxRedirects {
					return fmt.Errorf("%w: too many redirects", ErrURLUnreachable)
				}
				// The scheme is checked here; the address is checked by the dialer.
				_, err := domain.ValidateURLScheme(req.URL.String())
				return err
			},
		},
	}
}

func (f *safeHTTPFetcher) FetchMetadata(ctx context.Context, targetURL string) (*URLMetadata, error) {
	parsed, err := domain.ValidateURLScheme(targetURL)
	if err != nil {
		return nil, err
	}

	// Initial SSRF check before opening connection
	if err := domain.ResolveAndValidateHost(ctx, parsed.Hostname()); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Fluentra-Resource-Intake/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, domain.ErrResourceURLNotAllowed) {
			return nil, domain.ErrResourceURLNotAllowed
		}
		return nil, fmt.Errorf("%w: %w", ErrURLUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("%w: status %d", ErrURLUnreachable, resp.StatusCode)
	case resp.StatusCode >= 400:
		return nil, &URLStatusError{StatusCode: resp.StatusCode}
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, fmt.Errorf("%w: status %d", ErrURLUnreachable, resp.StatusCode)
	}

	// The body is read for its title only and then dropped (D17-2).
	limitReader := io.LimitReader(resp.Body, fetchMaxBodyBytes)
	bodyBytes, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("read response head: %w", err)
	}

	htmlContent := string(bodyBytes)
	title := ExtractTitle(htmlContent)
	if title == "" {
		title = parsed.Hostname()
	}

	return &URLMetadata{
		FinalURL: resp.Request.URL.String(),
		Title:    title,
	}, nil
}

// ErrURLUnreachable is a fetch that failed for reasons on the network or the
// remote server's side: a timeout, a refused connection, a 5xx. It says nothing
// about the resource, so it becomes 'failed', not 'rejected'.
var ErrURLUnreachable = errors.New("url unreachable")

// URLStatusError is a 4xx from the remote server: the page is not there, or not
// for us. That is a fact about the resource, so it becomes 'rejected'.
type URLStatusError struct {
	StatusCode int
}

func (e *URLStatusError) Error() string {
	return fmt.Sprintf("remote server answered status %d", e.StatusCode)
}

// ExtractTitle extracts the OpenGraph og:title or fallback HTML title tag.
func ExtractTitle(html string) string {
	if match := ogTitleRegex.FindStringSubmatch(html); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	if match := ogTitleAlt.FindStringSubmatch(html); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	if match := titleRegex.FindStringSubmatch(html); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}
