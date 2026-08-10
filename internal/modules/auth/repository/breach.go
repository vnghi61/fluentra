package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
)

// HIBPBaseURL is the Have I Been Pwned range endpoint. The full URL this
// adapter builds is this plus the five-character prefix and nothing else.
const HIBPBaseURL = "https://api.pwnedpasswords.com/range/"

// BreachCheckTimeout bounds the range request. The number is from the P2.1 card
// and it is chosen against registration latency, not against the API's typical
// response time: the check is advisory, so it is better to give up and let the
// password through than to make every learner wait.
const BreachCheckTimeout = 800 * time.Millisecond

// maxRangeResponseBytes caps what is read from the response. A padded bucket is
// on the order of 40 KiB; a megabyte is far above anything legitimate and stops
// a compromised or misconfigured endpoint from being able to exhaust memory on
// an unauthenticated code path.
const maxRangeResponseBytes = 1 << 20

// BreachChecker asks Have I Been Pwned whether a password appears in its corpus,
// using the k-anonymity range API.
//
// Why this speaks net/http directly rather than going through a platform
// facade: `depOnAnyVendor: false` exists to keep provider SDKs out of business
// modules so that swapping one is a change to a facade. There is no SDK here —
// one GET against one documented endpoint, using the standard library — and a
// facade wrapping it would have exactly one implementation and one caller. The
// port that would make swapping cheap already exists as domain.BreachChecker,
// which is what an offline Bloom filter would satisfy instead.
type BreachChecker struct {
	client  *http.Client
	baseURL string
	timeout time.Duration
	logger  *slog.Logger
}

// BreachCheckerOptions configures a BreachChecker. Every field has a working
// default; the zero value produces a checker that talks to the real endpoint at
// the documented timeout.
type BreachCheckerOptions struct {
	// BaseURL overrides HIBPBaseURL. Tests point it at an httptest server.
	BaseURL string
	// Timeout overrides BreachCheckTimeout.
	Timeout time.Duration
	// Client overrides the HTTP client, for tests and for a deployment that
	// needs a proxy.
	Client *http.Client
	// Logger receives the warn line when the check cannot be completed. It is
	// the only record that a password went unchecked.
	Logger *slog.Logger
}

// NewBreachChecker builds a checker from options.
func NewBreachChecker(options BreachCheckerOptions) *BreachChecker {
	checker := &BreachChecker{
		client:  options.Client,
		baseURL: options.BaseURL,
		timeout: options.Timeout,
		logger:  options.Logger,
	}
	if checker.client == nil {
		checker.client = &http.Client{}
	}
	if checker.baseURL == "" {
		checker.baseURL = HIBPBaseURL
	}
	if checker.timeout <= 0 {
		checker.timeout = BreachCheckTimeout
	}
	if checker.logger == nil {
		checker.logger = slog.Default()
	}
	return checker
}

// Breached reports whether password appears in the corpus.
//
// It returns an error when it could not find out. That is not the same as
// "false", and the distinction is the caller's to collapse: domain.Policy
// treats an error as "allow", which is the fail-open behaviour the card
// requires, and this method logs it at warn so the decision is visible rather
// than silent.
//
// What leaves the process is the first five characters of the SHA-1 digest, in
// the URL path, and nothing else. The remaining 35 are compared here against
// the bucket the server returns.
func (c *BreachChecker) Breached(ctx context.Context, password string) (bool, error) {
	prefix, suffix := domain.PasswordRange(password)

	// The deadline is derived from the caller's context rather than set on the
	// client, so a request that is already closer to its own deadline than 800
	// ms is not made to wait past it.
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	body, err := c.fetchRange(ctx, prefix)
	if err != nil {
		c.logger.WarnContext(ctx, "breached password check unavailable, allowing the password",
			slog.String("error", err.Error()))
		return false, err
	}
	return domain.ParseRangeResponse(body, suffix) > 0, nil
}

// fetchRange performs the GET. No error it returns carries the endpoint, and
// therefore none carries the prefix — see stripURL for why that is worth the
// trouble.
func (c *BreachChecker) fetchRange(ctx context.Context, prefix string) (string, error) {
	// url.JoinPath escapes the segment, so the prefix cannot alter the path
	// even though it is always five hex characters by construction.
	endpoint, err := url.JoinPath(c.baseURL, prefix)
	if err != nil {
		return "", fmt.Errorf("build range url: %w", stripURL(err))
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("build range request: %w", stripURL(err))
	}
	// Padding makes every bucket respond at a similar size, so an observer who
	// can see the response length cannot narrow down which bucket was asked for.
	// It costs nothing and the parser already ignores the synthetic zero-count
	// entries it adds.
	request.Header.Set("Add-Padding", "true")
	request.Header.Set("User-Agent", "fluentra")

	response, err := c.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("range request: %w", stripURL(err))
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("range request: unexpected status %d", response.StatusCode)
	}

	content, err := io.ReadAll(io.LimitReader(response.Body, maxRangeResponseBytes))
	if err != nil {
		return "", fmt.Errorf("read range response: %w", stripURL(err))
	}
	return string(content), nil
}

// stripURL replaces a *url.Error with the cause it wraps.
//
// net/http puts the whole request URL into every transport error, and this
// request's URL ends in the five-character prefix. On the wire that prefix is
// k-anonymous — one server sees it once, alongside half a million other
// candidates. In an aggregated, retained log it is something else: a permanent
// record narrowing one learner's password to one bucket, written every time the
// endpoint is slow. The cause is what a reader wants from the line anyway; the
// URL is a constant they already know.
func stripURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}
