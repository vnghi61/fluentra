package main

import (
	"context"
	"log/slog"

	"github.com/fluentra/fluentra/internal/platform/media"
)

// newRenderDispatcher builds what dispatches the tts-render workflow when a
// top-up publishes listening items. A bad setting turns it off rather than
// stopping the worker: the workflow's hourly schedule still renders every clip.
func newRenderDispatcher(ctx context.Context, cfg workerConfig) *media.GitHubWorkflowDispatcher {
	dispatcher, err := media.NewGitHubWorkflowDispatcher(media.GitHubDispatchConfig{
		Repository: cfg.Speech.TTSDispatchRepository,
		Workflow:   cfg.Speech.TTSDispatchWorkflow,
		Ref:        cfg.Speech.TTSDispatchRef,
		Token:      cfg.Speech.TTSDispatchToken,
	})
	if err != nil {
		slog.WarnContext(ctx, "tts render dispatch is off: its configuration is invalid", "error", err)
		return &media.GitHubWorkflowDispatcher{}
	}
	return dispatcher
}
