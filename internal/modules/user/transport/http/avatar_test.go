package http_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestRequestAvatarUploadIntent_Authenticated(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := authenticated(
		t, server, http.MethodPost, "/api/v1/me/avatar/upload-intent", `{"content_type":"image/png"}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}

	if accounts.seenActor != actorID {
		t.Errorf("service was asked for %s, want actor %s", accounts.seenActor, actorID)
	}
	if accounts.seenContentType != "image/png" {
		t.Errorf("service was asked for content-type %q, want image/png", accounts.seenContentType)
	}

	var response struct {
		UploadURL string `json:"upload_url"`
		Method    string `json:"method"`
		ObjectKey string `json:"object_key"`
		MaxBytes  int64  `json:"max_bytes"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.UploadURL == "" || response.ObjectKey == "" {
		t.Errorf("invalid upload intent response: %+v", response)
	}
}

func TestRequestAvatarUploadIntent_AnonymousIsRefused(t *testing.T) {
	t.Parallel()
	server := newServer(&fakeAccounts{})

	recorder := anonymous(t, server, http.MethodPost, "/api/v1/me/avatar/upload-intent", `{"content_type":"image/jpeg"}`)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestConfirmAvatar_Authenticated(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	rawKey := "users/" + actorID.String() + "/2026/08/avatar-raw.jpg"
	body := `{"object_key":"` + rawKey + `"}`
	recorder := authenticated(t, server, http.MethodPut, "/api/v1/me/avatar", body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}

	if accounts.seenActor != actorID {
		t.Errorf("service was asked for %s, want actor %s", accounts.seenActor, actorID)
	}
	if accounts.seenObjectKey != rawKey {
		t.Errorf("service was asked for key %q, want %q", accounts.seenObjectKey, rawKey)
	}
}

func TestConfirmAvatar_RejectsMissingOrNullObjectKey(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	for _, badBody := range []string{
		`{}`,
		`{"object_key":null}`,
		`{"unknown_field":"value"}`,
	} {
		recorder := authenticated(t, server, http.MethodPut, "/api/v1/me/avatar", badBody)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Errorf("PUT /me/avatar with %q = %d, want 422", badBody, recorder.Code)
		}
	}
}

func TestConfirmAvatar_AnonymousIsRefused(t *testing.T) {
	t.Parallel()
	server := newServer(&fakeAccounts{})

	recorder := anonymous(t, server, http.MethodPut, "/api/v1/me/avatar", `{"object_key":"raw.jpg"}`)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}
