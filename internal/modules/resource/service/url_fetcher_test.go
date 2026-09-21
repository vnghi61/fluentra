package service_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/resource/domain"
	"github.com/fluentra/fluentra/internal/modules/resource/service"
)

func TestSafeHTTPFetcher_RedirectToPrivateIPRefused(t *testing.T) {
	// Destination server on loopback
	destServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><title>Internal Admin</title></html>"))
	}))
	defer destServer.Close()

	// Initial server that redirects to destServer (loopback 127.0.0.1)
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destServer.URL, http.StatusFound)
	}))
	defer redirectServer.Close()

	fetcher := service.NewSafeHTTPFetcher()

	// Note: Even the initial request to redirectServer will be refused because redirectServer is on 127.0.0.1 (loopback)
	_, err := fetcher.FetchMetadata(context.Background(), redirectServer.URL)
	if err == nil {
		t.Fatalf("expected error for request to loopback server, got nil")
	}
	if !errors.Is(err, domain.ErrResourceURLNotAllowed) {
		t.Errorf("expected ErrResourceURLNotAllowed, got %v", err)
	}
}

func TestExtractTitle(t *testing.T) {
	ogHTML := `<!DOCTYPE html>
<html>
<head>
    <meta property="og:title" content="My OpenGraph Title" />
    <title>Fallback Title</title>
</head>
<body>Hello</body>
</html>`
	if title := service.ExtractTitle(ogHTML); title != "My OpenGraph Title" {
		t.Errorf("expected 'My OpenGraph Title', got %q", title)
	}

	fallbackHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Fallback Title</title>
</head>
<body>Hello</body>
</html>`
	if title := service.ExtractTitle(fallbackHTML); title != "Fallback Title" {
		t.Errorf("expected 'Fallback Title', got %q", title)
	}
}
