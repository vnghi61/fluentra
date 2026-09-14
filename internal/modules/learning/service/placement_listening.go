package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

// PlacementListeningPlays implements contract.PlacementListeningPolicy: a clip
// may be played once, and only while it is the current item of the caller's own
// placement session and the session's time has not run out. A placement is a
// test, so it takes the exam's rule rather than practice's three plays.
func (s *Service) PlacementListeningPlays(ctx context.Context, userID, sessionID, versionID uuid.UUID) (int, error) {
	session, err := s.repo.GetPlacementSession(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	if session == nil || session.UserID != userID || session.Status != domain.PlacementInProgress {
		return 0, domain.ErrPlacementNotFound
	}
	if session.Overdue(s.clock.Now().UTC()) {
		return 0, domain.ErrPlacementExpired
	}
	current := session.CurrentItem()
	if current == nil || current.Skill != domain.SkillListening {
		return 0, domain.ErrPlacementNotCurrentItem
	}
	activity, err := s.resolveActivityHierarchy(ctx, current.ActivityID)
	if err != nil {
		return 0, err
	}
	if activity.ContentVersionID != versionID {
		return 0, domain.ErrPlacementNotCurrentItem
	}
	return domain.PlacementListeningPlays, nil
}
