package repository_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/repository"
)

const breachTestPassword = "correct horse battery staple"

// newBreachServer serves one canned response and captures the request, so the
// tests can assert on what was sent as well as on what was returned.
func newBreachServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		captured.path = request.URL.Path
		captured.rawQuery = request.URL.RawQuery
		captured.headers = request.Header.Clone()
		handler(writer, request)
	}))
	t.Cleanup(server.Close)
	return server, captured
}

type capturedRequest struct {
	path     string
	rawQuery string
	headers  http.Header
}

func newChecker(t *testing.T, baseURL string, buffer *bytes.Buffer) *repository.BreachChecker {
	t.Helper()
	return repository.NewBreachChecker(repository.BreachCheckerOptions{
		BaseURL: baseURL,
		Logger:  slog.New(slog.NewTextHandler(buffer, nil)),
	})
}

// TestBreachChecker_SendsOnlyTheFiveCharacterPrefix is the acceptance criterion
// "only the first 5 characters of the SHA-1 hash ever leave the system",
// asserted against a real request rather than against the splitting function.
// It checks the whole request, not just the path: a suffix smuggled into a
// query parameter or a header would satisfy a path-only assertion.
func TestBreachChecker_SendsOnlyTheFiveCharacterPrefix(t *testing.T) {
	server, captured := newBreachServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("0000000000000000000000000000000000A:1"))
	})

	var logs bytes.Buffer
	if _, err := newChecker(t, server.URL, &logs).Breached(context.Background(), breachTestPassword); err != nil {
		t.Fatalf("Breached: %v", err)
	}

	prefix, suffix := domain.PasswordRange(breachTestPassword)
	if captured.path != "/"+prefix {
		t.Errorf("path = %q, want %q", captured.path, "/"+prefix)
	}
	if captured.rawQuery != "" {
		t.Errorf("query = %q, want it empty", captured.rawQuery)
	}
	for name, values := range captured.headers {
		for _, value := range values {
			if strings.Contains(strings.ToUpper(value), suffix) {
				t.Errorf("header %s carries the digest suffix", name)
			}
			if strings.Contains(value, breachTestPassword) {
				t.Errorf("header %s carries the password", name)
			}
		}
	}
}

func TestBreachChecker_AsksForAPaddedResponse(t *testing.T) {
	server, captured := newBreachServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(""))
	})

	var logs bytes.Buffer
	if _, err := newChecker(t, server.URL, &logs).Breached(context.Background(), breachTestPassword); err != nil {
		t.Fatalf("Breached: %v", err)
	}

	// Without padding, the response size narrows down which bucket was asked
	// for, which gives back part of what the prefix-only request protects.
	if got := captured.headers.Get("Add-Padding"); got != "true" {
		t.Errorf("Add-Padding = %q, want \"true\"", got)
	}
}

func TestBreachChecker_ReportsAPasswordThatIsInTheBucket(t *testing.T) {
	_, suffix := domain.PasswordRange(breachTestPassword)
	server, _ := newBreachServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("0000000000000000000000000000000000A:1\r\n" + suffix + ":64361"))
	})

	var logs bytes.Buffer
	breached, err := newChecker(t, server.URL, &logs).Breached(context.Background(), breachTestPassword)
	if err != nil {
		t.Fatalf("Breached: %v", err)
	}
	if !breached {
		t.Error("a password whose suffix is in the bucket was reported as clean")
	}
}

func TestBreachChecker_ReportsAPasswordThatIsNotInTheBucket(t *testing.T) {
	server, _ := newBreachServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("0000000000000000000000000000000000A:1\r\n0000000000000000000000000000000000B:2"))
	})

	var logs bytes.Buffer
	breached, err := newChecker(t, server.URL, &logs).Breached(context.Background(), breachTestPassword)
	if err != nil {
		t.Fatalf("Breached: %v", err)
	}
	if breached {
		t.Error("a password whose suffix is absent was reported as breached")
	}
}

// TestBreachChecker_TimesOutAndLogsAWarning is the failure mode the card sizes
// at 800 ms. What matters here is that the call returns an error rather than
// hanging, and that it leaves a record — domain.Policy is what turns the error
// into "allow", and its own test covers that half.
func TestBreachChecker_TimesOutAndLogsAWarning(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	server, _ := newBreachServer(t, func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-release:
		case <-request.Context().Done():
		}
		_, _ = writer.Write([]byte(""))
	})

	var logs bytes.Buffer
	checker := repository.NewBreachChecker(repository.BreachCheckerOptions{
		BaseURL: server.URL,
		Timeout: 20 * time.Millisecond,
		Logger:  slog.New(slog.NewTextHandler(&logs, nil)),
	})

	breached, err := checker.Breached(context.Background(), breachTestPassword)
	if err == nil {
		t.Fatal("a request that never answered returned no error")
	}
	if breached {
		t.Error("a failed check reported the password as breached")
	}
	assertWarnedWithoutTheSecret(t, logs.String())
}

func TestBreachChecker_TreatsANonOKStatusAsAFailure(t *testing.T) {
	server, _ := newBreachServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	})

	var logs bytes.Buffer
	breached, err := newChecker(t, server.URL, &logs).Breached(context.Background(), breachTestPassword)
	if err == nil {
		t.Fatal("a 503 returned no error")
	}
	if breached {
		t.Error("a failed check reported the password as breached")
	}
	assertWarnedWithoutTheSecret(t, logs.String())
}

// TestBreachChecker_RespectsACallerDeadlineShorterThanItsOwn is why the timeout
// is derived from the caller's context instead of set on the http.Client: a
// request already 100 ms from its own deadline must not be made to wait 800.
func TestBreachChecker_RespectsACallerDeadlineShorterThanItsOwn(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	server, _ := newBreachServer(t, func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-release:
		case <-request.Context().Done():
		}
		_, _ = writer.Write([]byte(""))
	})

	var logs bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	started := time.Now()
	if _, err := newChecker(t, server.URL, &logs).Breached(ctx, breachTestPassword); err == nil {
		t.Fatal("a request past the caller's deadline returned no error")
	}
	// Generous, because CI machines are slow — but far below the 800 ms the
	// checker would have waited had it ignored the caller's deadline.
	if elapsed := time.Since(started); elapsed > 400*time.Millisecond {
		t.Errorf("took %s, want the caller's 20 ms deadline to have applied", elapsed)
	}
}

func TestBreachChecker_UsesTheRealEndpointByDefault(t *testing.T) {
	// No request is made: this asserts the constant a deployment relies on,
	// because a checker built with no BaseURL must not silently talk nowhere.
	if !strings.HasPrefix(repository.HIBPBaseURL, "https://") {
		t.Errorf("HIBPBaseURL = %q, want an https endpoint", repository.HIBPBaseURL)
	}
	if repository.BreachCheckTimeout != 800*time.Millisecond {
		t.Errorf("BreachCheckTimeout = %s, want the 800ms the card specifies", repository.BreachCheckTimeout)
	}
}

// assertWarnedWithoutTheSecret checks both halves of the card's "fails open
// with a warn log": that there is a warning at all, and that it carries no
// fragment of the credential it was checking.
func assertWarnedWithoutTheSecret(t *testing.T, output string) {
	t.Helper()

	if !strings.Contains(output, "level=WARN") {
		t.Errorf("log output = %q, want a warning", output)
	}
	if strings.Contains(output, breachTestPassword) {
		t.Error("the log line contains the password")
	}
	prefix, suffix := domain.PasswordRange(breachTestPassword)
	if strings.Contains(output, suffix) {
		t.Error("the log line contains the digest suffix")
	}
	if strings.Contains(output, prefix) {
		t.Error("the log line contains the digest prefix")
	}
}
