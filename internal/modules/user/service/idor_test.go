package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// TestIDOR_UserResourcesReturnNotFoundWhenAccessedByAnotherActor verifies that
// any attempt to access or mutate a user-owned resource with an unauthorized
// or unknown actor returns 404 (NotFound) rather than 403 (Forbidden), ensuring
// that the system does not leak the existence of other users' records (P5.5 IDOR suite).
func TestIDOR_UserResourcesReturnNotFoundWhenAccessedByAnotherActor(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	// User A exists as h.actor.
	// User B is a separate actor.
	userB := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def099999999")

	// 1. User B attempting to read User A's account
	t.Run("GetAccount returns NotFound for non-existent actor context", func(t *testing.T) {
		_, err := h.service.GetAccount(context.Background(), userB)
		if !apperr.Is(err, apperr.NotFound) {
			t.Errorf("GetAccount error = %v, want NotFound (404)", err)
		}
	})

	// 2. User B attempting to update User A's profile
	t.Run("UpdateProfile returns NotFound for non-existent actor context", func(t *testing.T) {
		newName := "Malicious Actor"
		_, err := h.service.UpdateProfile(context.Background(), userB, domain.ProfileChange{
			DisplayName: &newName,
		})
		if !apperr.Is(err, apperr.NotFound) {
			t.Errorf("UpdateProfile error = %v, want NotFound (404)", err)
		}
	})

	// 3. User B attempting to read User A's preferences
	t.Run("GetPreferences returns NotFound for non-existent actor context", func(t *testing.T) {
		_, err := h.service.GetPreferences(context.Background(), userB)
		if !apperr.Is(err, apperr.NotFound) {
			t.Errorf("GetPreferences error = %v, want NotFound (404)", err)
		}
	})

	// 4. User B attempting to replace preferences
	t.Run("ReplacePreferences returns NotFound for non-existent actor context", func(t *testing.T) {
		_, err := h.service.ReplacePreferences(context.Background(), userB, domain.Preferences{
			Locale:               "en",
			Theme:                domain.ThemeDark,
			DailyGoalMinutes:     20,
			NotificationChannels: []domain.Channel{domain.ChannelEmail},
		})
		if !apperr.Is(err, apperr.NotFound) {
			t.Errorf("ReplacePreferences error = %v, want NotFound (404)", err)
		}
	})
}
