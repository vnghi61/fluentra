package http_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

func TestHandler_RequestDeletion_202Accepted(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	rec := authenticated(t, server, http.MethodDelete, "/api/v1/me", "")

	if rec.Code != http.StatusAccepted {
		t.Fatalf("got status %d, want 202; body: %s", rec.Code, rec.Body.String())
	}
	if accounts.seenActor != actorID {
		t.Errorf("got actor %s, want %s", accounts.seenActor, actorID)
	}

	var body struct {
		ID        uuid.UUID             `json:"id"`
		UserID    uuid.UUID             `json:"user_id"`
		Status    domain.DeletionStatus `json:"status"`
		ExecuteAt time.Time             `json:"execute_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.UserID != actorID {
		t.Errorf("got user_id %s, want %s", body.UserID, actorID)
	}
	if body.Status != domain.DeletionStatusPending {
		t.Errorf("got status %s, want pending", body.Status)
	}
}

func TestHandler_RequestDeletion_409Conflict(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{err: domain.ErrDeletionAlreadyPending}
	server := newServer(accounts)

	rec := authenticated(t, server, http.MethodDelete, "/api/v1/me", "")

	if rec.Code != http.StatusConflict {
		t.Fatalf("got status %d, want 409; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_CancelDeletion_200OK(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	rec := authenticated(t, server, http.MethodPost, "/api/v1/me/deletion/cancel", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if accounts.seenActor != actorID {
		t.Errorf("got actor %s, want %s", accounts.seenActor, actorID)
	}

	var body struct {
		Status domain.DeletionStatus `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Status != domain.DeletionStatusCancelled {
		t.Errorf("got status %s, want cancelled", body.Status)
	}
}

func TestHandler_CancelDeletion_409Conflict(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{err: domain.ErrDeletionNotCancellable}
	server := newServer(accounts)

	rec := authenticated(t, server, http.MethodPost, "/api/v1/me/deletion/cancel", "")

	if rec.Code != http.StatusConflict {
		t.Fatalf("got status %d, want 409; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_GetDeletion_200OK(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	deletionID := uuid.New()
	rec := authenticated(t, server, http.MethodGet, "/api/v1/me/deletion/"+deletionID.String(), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		ID     uuid.UUID             `json:"id"`
		UserID uuid.UUID             `json:"user_id"`
		Status domain.DeletionStatus `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.ID != deletionID {
		t.Errorf("got id %s, want %s", body.ID, deletionID)
	}
}

func TestHandler_GetDeletion_404NotFound(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{err: domain.ErrDeletionNotFound}
	server := newServer(accounts)

	deletionID := uuid.New()
	rec := authenticated(t, server, http.MethodGet, "/api/v1/me/deletion/"+deletionID.String(), "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_GetDeletion_InvalidUUID_422(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	rec := authenticated(t, server, http.MethodGet, "/api/v1/me/deletion/not-a-uuid", "")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got status %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
}
