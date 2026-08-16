package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

func TestRequestExport_Success(t *testing.T) {
	t.Parallel()

	fake := &fakeAccounts{}
	server := newServer(fake)

	rec := authenticated(t, server, http.MethodPost, "/api/v1/me/export", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got: %d (%s)", rec.Code, rec.Body.String())
	}

	if fake.seenActor != actorID {
		t.Errorf("expected actor %v, got %v", actorID, fake.seenActor)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["status"] != "pending" {
		t.Errorf("expected status 'pending', got: %v", body["status"])
	}
	if body["id"] != "0199a1c2-3d4e-7f80-9abc-def999999999" {
		t.Errorf("unexpected id: %v", body["id"])
	}
}

func TestRequestExport_AlreadyPendingReturns409(t *testing.T) {
	t.Parallel()

	fake := &fakeAccounts{err: domain.ErrExportAlreadyPending}
	server := newServer(fake)

	rec := authenticated(t, server, http.MethodPost, "/api/v1/me/export", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got: %d (%s)", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["code"] != "EXPORT_ALREADY_PENDING" {
		t.Errorf("expected code EXPORT_ALREADY_PENDING, got: %v", body["code"])
	}
}

func TestGetExportByID_Success(t *testing.T) {
	t.Parallel()

	fake := &fakeAccounts{}
	server := newServer(fake)

	exportID := "0199a1c2-3d4e-7f80-9abc-def111111111"
	rec := authenticated(t, server, http.MethodGet, "/api/v1/me/export/"+exportID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d (%s)", rec.Code, rec.Body.String())
	}

	if fake.seenActor != actorID {
		t.Errorf("expected actor %v, got %v", actorID, fake.seenActor)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["status"] != "completed" {
		t.Errorf("expected status 'completed', got: %v", body["status"])
	}
	if body["id"] != exportID {
		t.Errorf("expected id %s, got: %v", exportID, body["id"])
	}
}
