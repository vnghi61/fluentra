package domain_test

import (
	"errors"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// TestStateMachineTransitions covers all 5x5 = 25 transition pairs in the authoring state machine.
// Legal moves must succeed without error; illegal moves must return ErrInvalidStateTransition
// (HTTP 409 Conflict), never a 500 or any other error.
func TestStateMachineTransitions(t *testing.T) {
	t.Parallel()

	// 5 states
	statuses := []domain.AuthoringStatus{
		domain.StatusDraft,
		domain.StatusInReview,
		domain.StatusApproved,
		domain.StatusPublished,
		domain.StatusArchived,
	}

	// 8 legal transitions
	legal := map[domain.AuthoringStatus]map[domain.AuthoringStatus]bool{
		domain.StatusDraft: {
			domain.StatusDraft:    true,
			domain.StatusInReview: true,
		},
		domain.StatusInReview: {
			domain.StatusApproved: true,
			domain.StatusDraft:    true,
		},
		domain.StatusApproved: {
			domain.StatusPublished: true,
			domain.StatusDraft:     true,
		},
		domain.StatusPublished: {
			domain.StatusPublished: true,
			domain.StatusArchived:  true,
		},
		domain.StatusArchived: {},
	}

	pairCount := 0
	for _, from := range statuses {
		for _, to := range statuses {
			pairCount++
			from := from
			to := to
			isLegal := legal[from][to]

			t.Run(string(from)+"_to_"+string(to), func(t *testing.T) {
				t.Parallel()

				err := domain.ValidateTransition(from, to)
				can := domain.CanTransition(from, to)

				if isLegal {
					if err != nil {
						t.Errorf("ValidateTransition(%q, %q) returned error %v, want nil", from, to, err)
					}
					if !can {
						t.Errorf("CanTransition(%q, %q) returned false, want true", from, to)
					}
				} else {
					if err == nil {
						t.Fatalf("ValidateTransition(%q, %q) succeeded, want ErrInvalidStateTransition", from, to)
					}
					if can {
						t.Errorf("CanTransition(%q, %q) returned true, want false", from, to)
					}

					var appErr *apperr.Error
					if !errors.As(err, &appErr) {
						t.Fatalf("error %v is not an *apperr.Error", err)
					}
					if appErr.Code != "INVALID_STATE_TRANSITION" {
						t.Errorf("error code = %q, want INVALID_STATE_TRANSITION", appErr.Code)
					}
					if appErr.Kind != apperr.Conflict {
						t.Errorf("error kind = %v, want Conflict (409)", appErr.Kind)
					}
				}
			})
		}
	}

	if pairCount != 25 {
		t.Fatalf("expected 25 pairs tested, got %d", pairCount)
	}
}
