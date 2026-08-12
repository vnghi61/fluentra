package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The endpoints, as defaults. They are constants rather than configuration
// because they are Google's, not ours: a deployment that could point them
// somewhere else could be pointed at a server that mints its own ID tokens, and
// the only thing standing between that and a signed-in session is the JWKS this
// package would then also fetch from the same place. The two values that *are*
// configurable — the JWKS URL and the issuer — are checked against each other by
// the verifier, which is what makes overriding them safe in a test and useless
// to an attacker who cannot also change the issuer we accept.
const (
	defaultAuthEndpoint  = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenEndpoint = "https://oauth2.googleapis.com/token" // #nosec G101 -- a URL, not a credential
)

// scopes are the three this flow needs and no more.
//
// `openid` asks for the ID token that carries the identity, `email` for the
// address and its verification status, and `profile` for the display name a new
// account is opened with. Asking for anything beyond them would be asking a
// learner to grant access to data this product has no use for, and every extra
// scope is one more thing a compromised client secret could reach.
const scopes = "openid email profile"

// Provider is everything this module needs from Google: where to send the
// browser, how to exchange what comes back, and how to verify what that returns.
//
// The Verifier is embedded rather than held, so the three operations read as one
// surface at the call site. They are one flow.
type Provider struct {
	*Verifier

	clientID      string
	clientSecret  string
	redirectURL   string
	authEndpoint  string
	tokenEndpoint string
	client        *http.Client
}

// Options configures the provider. Only the three credential fields are
// required; the endpoints and the clock have working defaults.
type Options struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string

	// Issuer and JWKSURL come from OAUTH_GOOGLE_ISSUER and
	// OAUTH_GOOGLE_JWKS_URL. Empty means Google's own.
	Issuer   string
	JWKSURL  string
	JWKSTTL  time.Duration
	Now      func() time.Time
	Client   *http.Client
	Endpoint Endpoints
}

// Endpoints overrides where the provider talks to. It exists for the tests,
// which stand up an httptest server in place of Google; production leaves it
// zero and gets the constants above.
type Endpoints struct {
	Authorization string
	Token         string
}

// New builds the provider, its JWKS cache and its verifier.
func New(options Options) *Provider {
	client := options.Client
	if client == nil {
		// Bounded, for the reason the JWKS client is: the sign-in path must not
		// be able to hang on a provider that has stopped answering.
		client = &http.Client{Timeout: 10 * time.Second}
	}

	jwksURL := options.JWKSURL
	if jwksURL == "" {
		jwksURL = "https://www.googleapis.com/oauth2/v3/certs"
	}

	verifier := NewVerifier(VerifierOptions{
		JWKS: NewJWKS(JWKSOptions{
			Endpoint: jwksURL,
			Client:   client,
			TTL:      options.JWKSTTL,
			Now:      options.Now,
		}),
		Issuer: options.Issuer,
		// The audience is our client id. It is what stops an ID token minted for
		// some other application — a real, correctly signed Google token — from
		// being replayed into this one.
		Audience: options.ClientID,
		Now:      options.Now,
	})

	authEndpoint := options.Endpoint.Authorization
	if authEndpoint == "" {
		authEndpoint = defaultAuthEndpoint
	}
	tokenEndpoint := options.Endpoint.Token
	if tokenEndpoint == "" {
		tokenEndpoint = defaultTokenEndpoint
	}

	return &Provider{
		Verifier:      verifier,
		clientID:      options.ClientID,
		clientSecret:  options.ClientSecret,
		redirectURL:   options.RedirectURL,
		authEndpoint:  authEndpoint,
		tokenEndpoint: tokenEndpoint,
		client:        client,
	}
}

// AuthorizationURL is where the browser is sent.
//
// The `state` and the `nonce` are carried here and checked in two different
// places, which is why there are two of them: the state comes back on the
// redirect and proves this server started the flow, and the nonce comes back
// inside the ID token and proves the token was minted for this flow rather than
// obtained elsewhere. One value could not do both jobs, because the redirect and
// the token travel by different routes.
//
// The PKCE challenge goes out here and its verifier does not, which is the whole
// mechanism (see domain.NewPKCE).
func (p *Provider) AuthorizationURL(state, nonce, challenge string) string {
	query := url.Values{
		"client_id":             {p.clientID},
		"redirect_uri":          {p.redirectURL},
		"response_type":         {"code"},
		"scope":                 {scopes},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return p.authEndpoint + "?" + query.Encode()
}

// Exchange trades the authorization code for an ID token.
//
// The verifier is sent here and nowhere else. It is the half of the PKCE pair
// that never travelled over the channel the code did, so an attacker holding an
// intercepted code has nothing to send with it.
//
// Only the ID token is returned. Google also issues an access token for calling
// its APIs, and this product calls none of them — keeping it would mean storing
// or logging a credential with no use, which is a liability with no benefit.
func (p *Provider) Exchange(ctx context.Context, code, verifier string) (string, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {p.clientID},
		"client_secret": {p.clientSecret},
		"redirect_uri":  {p.redirectURL},
		"grant_type":    {"authorization_code"},
		"code_verifier": {verifier},
	}

	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, p.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := p.client.Do(request)
	if err != nil {
		// We could not ask. That is our outage to report as one, not the
		// caller's credential to blame — telling a learner their sign-in was
		// invalid when Google was simply unreachable sends them to reset a
		// password that is not the problem.
		return "", fmt.Errorf("%w: %s", ErrProviderUnavailable, err)
	}
	defer func() { _ = response.Body.Close() }()

	// Bounded. The body is a small JSON document and this is an unauthenticated
	// path, so a provider — or something impersonating one at the network level
	// — must not be able to make it allocate without limit.
	body, err := io.ReadAll(io.LimitReader(response.Body, maxTokenResponseBytes))
	if err != nil {
		return "", fmt.Errorf("%w: read token response: %s", ErrProviderUnavailable, err)
	}

	if response.StatusCode != http.StatusOK {
		// Google looked at the code and refused it: expired, already redeemed,
		// or issued for a different client. All three are the caller's problem
		// and none of them is ours to retry.
		//
		// The body is deliberately not included. It is provider text about a
		// credential, and this error reaches a log line.
		return "", fmt.Errorf("%w: status %d", errExchangeRejected, response.StatusCode)
	}

	var document struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return "", fmt.Errorf("%w: decode token response: %s", ErrProviderUnavailable, err)
	}
	if document.IDToken == "" {
		// A 200 with no ID token is not something Google does for a request
		// carrying `openid`. Treating it as a rejection rather than an outage
		// stops it being retried against a provider that answered fine.
		return "", fmt.Errorf("%w: response carried no id_token", errExchangeRejected)
	}
	return document.IDToken, nil
}

// maxTokenResponseBytes bounds the token endpoint's reply. Google's is well
// under a kilobyte; 64 KiB is generous and still finite.
const maxTokenResponseBytes = 64 << 10

var (
	// errExchangeRejected is Google declining the code itself.
	errExchangeRejected = errors.New("google rejected the authorization code")

	// ErrProviderUnavailable is not being able to ask. It is kept apart from a
	// rejection because the two have different HTTP statuses and different
	// advice: one is 401 and "start again", the other is 503 and "this is us".
	//
	// It is the one sentinel here that is exported, and only because a fake
	// provider in another package has to be able to produce it. The distinction
	// between an outage and a refusal is a branch in the service, and a branch
	// no test can reach is a branch that rots.
	ErrProviderUnavailable = errors.New("google's token endpoint could not be reached")
)

// ErrRejected reports whether err is Google refusing the exchange.
func ErrRejected(err error) bool { return errors.Is(err, errExchangeRejected) }

// ErrUnavailable reports whether err is a failure to reach Google at all.
func ErrUnavailable(err error) bool { return errors.Is(err, ErrProviderUnavailable) }
