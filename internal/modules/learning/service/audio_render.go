package service

import (
	"context"
	"log/slog"
)

// AudioRenderRequester asks for listening audio to be rendered now, rather than
// at the render workflow's next scheduled run.
type AudioRenderRequester interface {
	RequestRender(ctx context.Context) error
}

// requestAudioRender runs after a listening slot gains items. A failure is only
// logged: the render workflow also runs on a schedule, so a clip is late, not lost.
func (s *Service) requestAudioRender(ctx context.Context, added int) {
	if added == 0 || s.audioRender == nil {
		return
	}
	if err := s.audioRender.RequestRender(ctx); err != nil {
		slog.WarnContext(ctx, "could not request listening audio rendering; the scheduled render will pick it up",
			"error", err)
	}
}
