package main

import (
	"context"
	"log/slog"

	"github.com/fluentra/fluentra/internal/platform/media"
)

// newMediaRenderDispatcher builds what dispatches the media-render workflow when a
// resource reaches validated status. A bad setting turns it off rather than
// stopping the worker: the workflow's hourly schedule still renders every rendition.
func newMediaRenderDispatcher(ctx context.Context, cfg workerConfig) *media.GitHubWorkflowDispatcher {
	dispatcher, err := media.NewGitHubWorkflowDispatcher(media.GitHubDispatchConfig{
		Repository: cfg.Media.DispatchRepository,
		Workflow:   cfg.Media.DispatchWorkflow,
		Ref:        cfg.Media.DispatchRef,
		Token:      cfg.Media.DispatchToken,
	})
	if err != nil {
		slog.WarnContext(ctx, "media render dispatch is off: its configuration is invalid", "error", err)
		return &media.GitHubWorkflowDispatcher{}
	}
	return dispatcher
}
