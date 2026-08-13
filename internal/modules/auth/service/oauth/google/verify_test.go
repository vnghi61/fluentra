package google_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/service/oauth/google"
)

const (
	testAudience = "fluentra-client-id.apps.googleusercontent.com"
	testIssuer   = "https://accounts.google.com"
	testNonce    = "the-nonce-this-server-issued"
	testKeyID    = "key-one"
	testSubject  = "1234567890"
)

// The registered claim names these tests build tokens from. They are constants
// because each one appears in the fixture, in the test that tampers with it, and
// in the assertion — and three spellings of a claim name is two chances for a
// test to check something the verifier never reads.
const (
	claimAudience = "aud"
	claimSubject  = "sub"
	claimIssuer   = "iss"
	claimNonce    = "nonce"
	claimExpiry   = "exp"
)

// keyServer is a stand-in for Google's JWKS endpoint that counts requests, so a
// test can prove the cache is a cache rather than a variable that happens to
// hold a key.
type keyServer struct {
	*httptest.Server

	requests atomic.Int64
	keys     map[string]*rsa.PrivateKey
}

func newKeyServer(t *testing.T, keyIDs ...string) *keyServer {
	t.Helper()

	server := &keyServer{keys: map[string]*rsa.PrivateKey{}}
	for _, keyID := range keyIDs {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate key: %v", err)
		}
		server.keys[keyID] = key
	}

	server.Server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		server.requests.Add(1)

		type jwk struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		}
		document := struct {
			Keys []jwk `json:"keys"`
		}{}
		for keyID, key := range server.keys {
			document.Keys = append(document.Keys, jwk{
				Kid: keyID,
				Kty: "RSA",
				N:   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
			})
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(document)
	}))
	t.Cleanup(server.Close)
	return server
}

// rotate replaces the published key set, as Google does periodically.
func (s *keyServer) rotate(t *testing.T, keyID string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	s.keys = map[string]*rsa.PrivateKey{keyID: key}
}

// token mints an ID token the way Google would, with whatever claims the test
// wants to break.
func (s *keyServer) token(t *testing.T, keyID string, mutate func(jwt.MapClaims)) string {
	t.Helper()

	key, ok := s.keys[keyID]
	if !ok {
		t.Fatalf("no signing key %q", keyID)
	}

	claims := jwt.MapClaims{
		claimIssuer:      testIssuer,
		claimAudience:    testAudience,
		claimSubject:     testSubject,
		"email":          "learner@example.com",
		"email_verified": true,
		"name":           "Nghi",
		claimNonce:       testNonce,
		claimExpiry:      time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Unix(),
	}
	if mutate != nil {
		mutate(claims)
	}

	unsigned := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	unsigned.Header["kid"] = keyID
	signed, err := unsigned.SignedString(key)
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}
	return signed
}

func newVerifier(t *testing.T, server *keyServer) (*google.Verifier, *google.JWKS) {
	t.Helper()
	verifier, jwks, _ := newVerifierAtClock(t, server)
	return verifier, jwks
}

// newVerifierAtClock returns the verifier plus a handle that moves its clock,
// so a test can step past the refresh floor rather than pretend it is not there.
func newVerifierAtClock(t *testing.T, server *keyServer) (*google.Verifier, *google.JWKS, func(time.Duration)) {
	t.Helper()

	current := time.Now()
	now := func() time.Time { return current }
	advance := func(by time.Duration) { current = current.Add(by) }

	jwks := google.NewJWKS(google.JWKSOptions{Endpoint: server.URL, TTL: time.Hour, Now: now})
	return google.NewVerifier(google.VerifierOptions{
		JWKS: jwks, Issuer: testIssuer, Audience: testAudience,
	}), jwks, advance
}

// TestJWKS_IsCachedAndNotFetchedPerRequest is the acceptance criterion, and the
// reason it matters is availability rather than speed: an outbound call on the
// sign-in path puts Google's uptime in front of every learner's ability to sign
// in, and Google's latency in front of it too.
func TestJWKS_IsCachedAndNotFetchedPerRequest(t *testing.T) {
	server := newKeyServer(t, testKeyID)
	verifier, jwks := newVerifier(t, server)
	ctx := context.Background()

	for i := range 20 {
		if _, err := verifier.Verify(ctx, server.token(t, testKeyID, nil), testNonce); err != nil {
			t.Fatalf("verify %d: %v", i, err)
		}
	}

	if fetched := jwks.Fetches(); fetched != 1 {
		t.Errorf("%d outbound fetches for 20 verifications, want 1", fetched)
	}
	if requests := server.requests.Load(); requests != 1 {
		t.Errorf("the key endpoint saw %d requests, want 1", requests)
	}
}

// TestJWKS_RefreshesOnAnUnknownKeyID is what makes a Google key rotation
// something learners never notice, rather than an outage lasting until the TTL
// expires — six hours, on the default.
//
// The clock is stepped past the refresh floor first, and that is not the test
// working around the implementation: the floor is deliberate (see the amplifier
// test below) and it means a rotation is picked up within a minute rather than
// instantly. That is safe because Google publishes a new key well before it
// signs anything with it, so the overlap covers the minute. A test asserting
// instant pickup would be asserting a property this design does not have and
// should not have.
func TestJWKS_RefreshesOnAnUnknownKeyID(t *testing.T) {
	server := newKeyServer(t, testKeyID)
	verifier, jwks, advance := newVerifierAtClock(t, server)
	ctx := context.Background()

	if _, err := verifier.Verify(ctx, server.token(t, testKeyID, nil), testNonce); err != nil {
		t.Fatalf("verify before rotation: %v", err)
	}

	// Google rotates. The cache is still fresh by TTL, and still wrong.
	server.rotate(t, "key-two")
	advance(2 * time.Minute)

	if _, err := verifier.Verify(ctx, server.token(t, "key-two", nil), testNonce); err != nil {
		t.Fatalf("verify after rotation: %v — an unknown kid must trigger a refresh", err)
	}
	if fetched := jwks.Fetches(); fetched != 2 {
		t.Errorf("%d fetches, want exactly 2: one cold, one for the unknown kid", fetched)
	}

	// The rotated key is now cached, so the next one costs nothing.
	if _, err := verifier.Verify(ctx, server.token(t, "key-two", nil), testNonce); err != nil {
		t.Fatalf("verify after the refresh: %v", err)
	}
	if fetched := jwks.Fetches(); fetched != 2 {
		t.Errorf("%d fetches, want the rotated key to have been cached", fetched)
	}
}

// TestJWKS_AnUnknownKeyIDDoesNotBecomeARequestAmplifier is the other half of
// refresh-on-unknown-kid. Anyone can present a token with a fabricated `kid`,
// and without a floor each one would become an outbound call to Google — an
// amplifier pointed at a third party, from an unauthenticated endpoint.
func TestJWKS_AnUnknownKeyIDDoesNotBecomeARequestAmplifier(t *testing.T) {
	server := newKeyServer(t, testKeyID)
	verifier, jwks := newVerifier(t, server)
	ctx := context.Background()

	// Warm the cache so the floor applies.
	if _, err := verifier.Verify(ctx, server.token(t, testKeyID, nil), testNonce); err != nil {
		t.Fatalf("warm the cache: %v", err)
	}

	for range 50 {
		// A token whose kid names a key that has never existed. Signature
		// verification is irrelevant; the lookup is what is under test.
		unsigned := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{claimSubject: "x"})
		unsigned.Header["kid"] = "a-kid-that-never-existed"
		fabricated, err := unsigned.SignedString(server.keys[testKeyID])
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		if _, err := verifier.Verify(ctx, fabricated, testNonce); err == nil {
			t.Fatal("a token with a fabricated kid verified")
		}
	}

	if fetched := jwks.Fetches(); fetched > 2 {
		t.Errorf("%d fetches for 50 fabricated key ids: the refresh floor is not holding", fetched)
	}
}

// TestVerify_RejectsATokenFailingAnyOfTheFiveChecks is BR-AUTH-18. Each row is
// a token Google could plausibly not have issued for this flow, and every one
// must be refused — signature, iss, aud, exp, nonce.
func TestVerify_RejectsATokenFailingAnyOfTheFiveChecks(t *testing.T) {
	server := newKeyServer(t, testKeyID)
	verifier, _ := newVerifier(t, server)
	ctx := context.Background()

	// A second, unpublished key: a token signed with it has a valid structure
	// and a signature nobody can verify, which is what a forgery looks like.
	foreign, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate foreign key: %v", err)
	}

	forged := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		claimIssuer: testIssuer, claimAudience: testAudience, claimSubject: testSubject,
		claimNonce: testNonce, claimExpiry: time.Now().Add(time.Hour).Unix(),
	})
	forged.Header["kid"] = testKeyID
	forgedToken, err := forged.SignedString(foreign)
	if err != nil {
		t.Fatalf("sign with the foreign key: %v", err)
	}

	cases := map[string]string{
		"a signature we cannot verify": forgedToken,
		"another issuer": server.token(t, testKeyID, func(claims jwt.MapClaims) {
			claims[claimIssuer] = "https://accounts.evil.example"
		}),
		"another audience": server.token(t, testKeyID, func(claims jwt.MapClaims) {
			claims[claimAudience] = "some-other-client-id"
		}),
		"expired": server.token(t, testKeyID, func(claims jwt.MapClaims) {
			claims[claimExpiry] = time.Now().Add(-time.Hour).Unix()
		}),
		"a nonce from another flow": server.token(t, testKeyID, func(claims jwt.MapClaims) {
			claims[claimNonce] = "a-nonce-from-somebody-elses-sign-in"
		}),
		"no nonce at all": server.token(t, testKeyID, func(claims jwt.MapClaims) {
			delete(claims, claimNonce)
		}),
		"alg none": func() string {
			unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
				claimIssuer: testIssuer, claimAudience: testAudience, claimSubject: testSubject,
				claimNonce: testNonce, claimExpiry: time.Now().Add(time.Hour).Unix(),
			})
			unsigned.Header["kid"] = testKeyID
			token, signErr := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
			if signErr != nil {
				t.Fatalf("sign alg none: %v", signErr)
			}
			return token
		}(),
		"empty": "",
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			identity, err := verifier.Verify(ctx, token, testNonce)
			if err == nil {
				t.Fatalf("%s was accepted", name)
			}
			if !google.ErrInvalidToken(err) {
				t.Errorf("error = %v, want a token-validation failure", err)
			}
			// Nothing usable comes back, so a caller that ignores the error
			// cannot accidentally create an account from a forged token.
			if identity.Subject != "" || identity.Email != "" {
				t.Errorf("a rejected token still yielded an identity: %+v", identity)
			}
		})
	}
}

// TestVerify_AcceptsAGenuineTokenAndReportsWhatItAsserts is the happy path, and
// it pins that `email_verified` is carried through rather than assumed — the
// linking policy's first branch turns on it.
func TestVerify_AcceptsAGenuineTokenAndReportsWhatItAsserts(t *testing.T) {
	server := newKeyServer(t, testKeyID)
	verifier, _ := newVerifier(t, server)

	identity, err := verifier.Verify(context.Background(), server.token(t, testKeyID, nil), testNonce)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if identity.Subject != testSubject {
		t.Errorf("subject = %q", identity.Subject)
	}
	if identity.Email != "learner@example.com" {
		t.Errorf("email = %q", identity.Email)
	}
	if !identity.EmailVerified {
		t.Error("email_verified was not carried through")
	}

	// And an unverified address is reported as such rather than dropped, so the
	// policy can refuse it.
	unverified, err := verifier.Verify(context.Background(),
		server.token(t, testKeyID, func(claims jwt.MapClaims) { claims["email_verified"] = false }), testNonce)
	if err != nil {
		t.Fatalf("Verify unverified: %v", err)
	}
	if unverified.EmailVerified {
		t.Error("email_verified: false was reported as verified")
	}
}
