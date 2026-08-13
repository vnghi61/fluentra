// Package google verifies Google's ID tokens and exchanges authorization codes.
//
// It speaks net/http directly rather than through a platform facade, for the
// reason repository.BreachChecker does: it is one external read with one
// caching rule of its own, and a facade would hide the rule that matters.
package google

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// DefaultJWKSCacheTTL mirrors OAUTH_JWKS_CACHE_TTL in `.env.example`.
const DefaultJWKSCacheTTL = 6 * time.Hour

// minRefreshInterval floors how often an unknown `kid` may trigger a fetch.
//
// Refreshing on an unknown key id is how a rotation is picked up without waiting
// out the TTL. It is also, unguarded, a request amplifier: anyone can present a
// token with a `kid` that has never existed, and each one would become an
// outbound call to Google. The floor turns an unbounded amplifier into at most
// one fetch a minute, which is far more often than Google rotates and far less
// often than an attacker would like.
const minRefreshInterval = time.Minute

// JWKS caches Google's signing keys.
//
// Never fetched per request: an outbound call on the sign-in path would put
// Google's availability in front of every learner's ability to sign in, and
// their latency in front of it too.
type JWKS struct {
	endpoint string
	client   *http.Client
	ttl      time.Duration
	now      func() time.Time

	mu          sync.RWMutex
	keys        map[string]*rsa.PublicKey
	fetchedAt   time.Time
	lastAttempt time.Time

	// fetches counts outbound calls, so a test can prove the cache is a cache.
	fetches int
}

// JWKSOptions configures the cache. Every field has a working default.
type JWKSOptions struct {
	Endpoint string
	Client   *http.Client
	TTL      time.Duration
	Now      func() time.Time
}

// NewJWKS creates the cache.
func NewJWKS(options JWKSOptions) *JWKS {
	client := options.Client
	if client == nil {
		// A bounded client. The sign-in path must not be able to hang on a
		// provider that has stopped answering.
		client = &http.Client{Timeout: 5 * time.Second}
	}
	ttl := options.TTL
	if ttl <= 0 {
		ttl = DefaultJWKSCacheTTL
	}
	timeNow := options.Now
	if timeNow == nil {
		timeNow = time.Now
	}
	return &JWKS{
		endpoint: options.Endpoint,
		client:   client,
		ttl:      ttl,
		now:      timeNow,
		keys:     map[string]*rsa.PublicKey{},
	}
}

// Fetches reports how many outbound calls have been made. It exists for the
// test that proves this is not fetched per request.
func (j *JWKS) Fetches() int {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.fetches
}

// KeyFor returns the signing key for a key id, refreshing if it is unknown.
//
// Three cases, and the middle one is the interesting one:
//
//   - The key is cached and the cache is fresh: return it, no network.
//   - The key is unknown: Google has rotated, so refresh once — subject to the
//     floor above — and look again. This is what makes a rotation invisible to
//     learners instead of a five-minute outage.
//   - The cache is stale: refresh on the next lookup.
func (j *JWKS) KeyFor(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	if key, fresh := j.cached(keyID); key != nil && fresh {
		return key, nil
	}

	if err := j.refresh(ctx); err != nil {
		// A cached key that is merely stale still verifies a token Google
		// signed, and refusing every sign-in because Google's key endpoint is
		// briefly unreachable is a worse outage than serving from a slightly
		// old key set. An unknown key id with no refresh is a real failure and
		// falls through to the error below.
		if key, _ := j.cached(keyID); key != nil {
			slog.WarnContext(ctx, "jwks refresh failed, using the cached key set",
				"module", "auth", "op", "KeyFor", "error", err)
			return key, nil
		}
		return nil, err
	}

	if key, _ := j.cached(keyID); key != nil {
		return key, nil
	}
	return nil, fmt.Errorf("no google signing key for kid %q", keyID)
}

func (j *JWKS) cached(keyID string) (key *rsa.PublicKey, fresh bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.keys[keyID], j.now().Before(j.fetchedAt.Add(j.ttl))
}

func (j *JWKS) refresh(ctx context.Context) error {
	j.mu.Lock()
	if now := j.now(); now.Before(j.lastAttempt.Add(minRefreshInterval)) && len(j.keys) > 0 {
		// Floored. A token carrying a fabricated `kid` must not become an
		// outbound request.
		j.mu.Unlock()
		return fmt.Errorf("jwks refresh suppressed: last attempt was less than %s ago", minRefreshInterval)
	}
	j.lastAttempt = j.now()
	j.fetches++
	j.mu.Unlock()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, j.endpoint, nil)
	if err != nil {
		return fmt.Errorf("build jwks request: %w", err)
	}
	response, err := j.client.Do(request)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch jwks: status %d", response.StatusCode)
	}

	var document struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	parsed := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, entry := range document.Keys {
		if entry.Kty != "RSA" {
			// Google publishes RSA. Anything else is a key this verifier does
			// not know how to use, and skipping it is better than guessing.
			continue
		}
		key, err := rsaKeyFrom(entry.N, entry.E)
		if err != nil {
			slog.WarnContext(ctx, "skipping an unreadable google signing key",
				"module", "auth", "op", "refresh", "kid", entry.Kid, "error", err)
			continue
		}
		parsed[entry.Kid] = key
	}
	if len(parsed) == 0 {
		// Replacing a working key set with an empty one would turn a malformed
		// response into a total sign-in outage.
		return fmt.Errorf("jwks response carried no usable keys")
	}

	j.mu.Lock()
	j.keys, j.fetchedAt = parsed, j.now()
	j.mu.Unlock()
	return nil
}

func rsaKeyFrom(modulus, exponent string) (*rsa.PublicKey, error) {
	rawModulus, err := base64.RawURLEncoding.DecodeString(modulus)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	rawExponent, err := base64.RawURLEncoding.DecodeString(exponent)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}
	if len(rawExponent) == 0 || len(rawExponent) > 4 {
		return nil, fmt.Errorf("exponent is %d bytes", len(rawExponent))
	}

	value := 0
	for _, digit := range rawExponent {
		value = value<<8 | int(digit)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(rawModulus), E: value}, nil
}
