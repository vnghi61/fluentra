package media

import (
	"bytes"
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

const (
	defaultGitHubAPI      = "https://api.github.com"
	defaultRenderWorkflow = "tts-render.yml"
	defaultRenderRef      = "main"
	dispatchTimeout       = 10 * time.Second
)

// GitHubDispatchConfig names the workflow that renders listening audio.
type GitHubDispatchConfig struct {
	// Repository is owner/name.
	Repository string
	// Workflow is the workflow file name; tts-render.yml when empty.
	Workflow string
	// Ref is the branch the workflow runs on; main when empty.
	Ref string
	// Token is a fine-grained token with Actions read and write on Repository
	// alone. Empty turns dispatching off.
	Token string
	// APIBaseURL is https://api.github.com when empty; tests point it elsewhere.
	APIBaseURL string
	HTTPClient *http.Client
}

// GitHubWorkflowDispatcher asks GitHub Actions to render listening audio now.
//
// Audio is rendered offline by cmd/tts in the tts-render workflow (work order 12
// §4: no engine fits the Render worker). That workflow also runs hourly.
// Dispatching it as soon as the worker publishes listening items means a clip
// exists within minutes rather than at the next hour, without the idle runs a
// tighter schedule would cost.
type GitHubWorkflowDispatcher struct {
	endpoint string
	ref      string
	token    string
	client   *http.Client
}

// NewGitHubWorkflowDispatcher builds a dispatcher. Without a token or a
// repository it is off, and RequestRender does nothing: the hourly schedule is
// then the only trigger.
func NewGitHubWorkflowDispatcher(cfg GitHubDispatchConfig) (*GitHubWorkflowDispatcher, error) {
	token := strings.TrimSpace(cfg.Token)
	repository := strings.TrimSpace(cfg.Repository)
	if token == "" || repository == "" {
		return &GitHubWorkflowDispatcher{}, nil
	}

	owner, name, found := strings.Cut(repository, "/")
	if !found || owner == "" || name == "" || strings.Contains(name, "/") {
		return nil, fmt.Errorf("tts dispatch repository %q is not owner/name", repository)
	}
	workflow := firstNonBlank(cfg.Workflow, defaultRenderWorkflow)
	base := strings.TrimRight(firstNonBlank(cfg.APIBaseURL, defaultGitHubAPI), "/")
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: dispatchTimeout}
	}

	return &GitHubWorkflowDispatcher{
		endpoint: fmt.Sprintf("%s/repos/%s/%s/actions/workflows/%s/dispatches",
			base, url.PathEscape(owner), url.PathEscape(name), url.PathEscape(workflow)),
		ref:    firstNonBlank(cfg.Ref, defaultRenderRef),
		token:  token,
		client: client,
	}, nil
}

// Enabled reports whether a dispatch will be sent.
func (d *GitHubWorkflowDispatcher) Enabled() bool {
	return d != nil && d.endpoint != ""
}

// RequestRender dispatches the render workflow. It does nothing when the
// dispatcher is off. The token never appears in a returned error.
func (d *GitHubWorkflowDispatcher) RequestRender(ctx context.Context) error {
	if !d.Enabled() {
		return nil
	}
	body, err := json.Marshal(map[string]string{"ref": d.ref})
	if err != nil {
		return fmt.Errorf("encode render dispatch: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build render dispatch: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+d.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := d.client.Do(request)
	if err != nil {
		return fmt.Errorf("dispatch audio rendering: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("%w: github returned %d: %s",
			errDispatchRefused, response.StatusCode, strings.TrimSpace(string(detail)))
	}
	return nil
}

var errDispatchRefused = errors.New("dispatch audio rendering refused")

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
