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

	// 5. User B attempting to read User A's export. The resource is addressed
	// by its own id, so this is a real cross-user IDOR: a 403 here would confirm
	// to user B that the export id names a real row.
	t.Run("GetExportByID returns NotFound for another user's export", func(t *testing.T) {
		exportID := uuid.New()
		h.repo.exports[exportID] = domain.ExportRequest{
			ID:        exportID,
			UserID:    h.actor,
			Status:    domain.ExportStatusCompleted,
			CreatedAt: testNow,
		}

		_, err := h.service.GetExportByID(context.Background(), userB, exportID)
		if !apperr.Is(err, apperr.NotFound) {
			t.Errorf("GetExportByID error = %v, want NotFound (404)", err)
		}
	})

	// 6. User B attempting to read User A's deletion request, same reasoning as
	// the export case above.
	t.Run("GetDeletion returns NotFound for another user's deletion", func(t *testing.T) {
		request, err := h.service.RequestDeletion(context.Background(), h.actor)
		if err != nil {
			t.Fatalf("RequestDeletion: %v", err)
		}

		_, err = h.service.GetDeletion(context.Background(), userB, request.ID)
		if !apperr.Is(err, apperr.NotFound) {
			t.Errorf("GetDeletion error = %v, want NotFound (404)", err)
		}
	})
}
