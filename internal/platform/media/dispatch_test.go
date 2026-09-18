package media

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testRepository = "vnghi61/fluentra"

func TestGitHubWorkflowDispatcher_DispatchesTheRenderWorkflow(t *testing.T) {
	var gotRequest, gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest = r.Method + " " + r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	dispatcher, err := NewGitHubWorkflowDispatcher(GitHubDispatchConfig{
		Repository: testRepository, Token: "token-value", APIBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("build dispatcher: %v", err)
	}
	if err := dispatcher.RequestRender(context.Background()); err != nil {
		t.Fatalf("request render: %v", err)
	}

	if want := "POST /repos/" + testRepository + "/actions/workflows/tts-render.yml/dispatches"; gotRequest != want {
		t.Errorf("request = %q, want %q", gotRequest, want)
	}
	if gotAuth != "Bearer token-value" {
		t.Errorf("authorization = %q", gotAuth)
	}
	if gotBody != `{"ref":"main"}` {
		t.Errorf("body = %s, want the main branch", gotBody)
	}
}

// Without a token the hourly schedule is the only trigger, and nothing is sent.
func TestGitHubWorkflowDispatcher_IsOffWithoutAToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a dispatcher with no token sent a request")
	}))
	defer server.Close()

	dispatcher, err := NewGitHubWorkflowDispatcher(GitHubDispatchConfig{
		Repository: testRepository, APIBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("build dispatcher: %v", err)
	}
	if dispatcher.Enabled() {
		t.Error("a dispatcher with no token reports itself enabled")
	}
	if err := dispatcher.RequestRender(context.Background()); err != nil {
		t.Fatalf("request render: %v", err)
	}
}

// A refusal is reported with GitHub's reason, and the token is never in it:
// this error is logged.
func TestGitHubWorkflowDispatcher_ReportsARefusalWithoutTheToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Resource not accessible by personal access token"}`))
	}))
	defer server.Close()

	dispatcher, err := NewGitHubWorkflowDispatcher(GitHubDispatchConfig{
		Repository: testRepository, Token: "secret-token-value", APIBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("build dispatcher: %v", err)
	}
	err = dispatcher.RequestRender(context.Background())
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %v, want GitHub's 403", err)
	}
	if strings.Contains(err.Error(), "secret-token-value") {
		t.Fatalf("the token leaked into the error: %v", err)
	}
}

func TestNewGitHubWorkflowDispatcher_RejectsARepositoryWithoutAnOwner(t *testing.T) {
	if _, err := NewGitHubWorkflowDispatcher(GitHubDispatchConfig{Repository: "fluentra", Token: "t"}); err == nil {
		t.Fatal("a repository without an owner was accepted")
	}
}
