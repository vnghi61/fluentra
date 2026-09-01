package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
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

// The avatar route is the one the API advertised for months and never served.
//
// `GET /me` has always answered with `avatar_url: /api/v1/storage/avatars/{id}`,
// built by a Sprintf in dto.go, and nothing anywhere mounted that path. The
// integration test that was supposed to cover it asserted
// `strings.HasPrefix(*me.Profile.AvatarURL, "/api/v1/storage/avatars/")` -- it
// checked that the URL *looked* right and never fetched it, which is exactly
// the assertion a URL pointing at nothing passes.
//
// So these ask for the bytes.

// stubImageBody stands in for a stored JPEG. Its only job is to be identical
// coming out as it was going in.
const stubImageBody = "stub image bytes"

func TestGetAvatar_ServesTheStoredImage(t *testing.T) {
	t.Parallel()
	avatars := &fakeAvatars{body: "ÿØÿà jpeg bytes"}
	server := newServerWithAvatars(&fakeAccounts{}, avatars)

	assetID := uuid.New()
	recorder := authenticated(t, server, http.MethodGet, "/api/v1/storage/avatars/"+assetID.String(), "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	if got := recorder.Body.String(); got != avatars.body {
		t.Errorf("body = %q, want %q", got, avatars.body)
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got == "" {
		t.Error("Cache-Control is empty; a stored image never changes and should be cacheable")
	}
}

func TestGetAvatar_DefaultsToTheMediumVariant(t *testing.T) {
	t.Parallel()
	avatars := &fakeAvatars{body: stubImageBody}
	server := newServerWithAvatars(&fakeAccounts{}, avatars)

	authenticated(t, server, http.MethodGet, "/api/v1/storage/avatars/"+uuid.New().String(), "")

	if avatars.gotVariant != domain.AvatarVariantMedium {
		t.Errorf("variant = %q, want md", avatars.gotVariant)
	}
}

func TestGetAvatar_HonoursTheRequestedSize(t *testing.T) {
	t.Parallel()
	avatars := &fakeAvatars{body: stubImageBody}
	server := newServerWithAvatars(&fakeAccounts{}, avatars)

	authenticated(t, server, http.MethodGet, "/api/v1/storage/avatars/"+uuid.New().String()+"?size=lg", "")

	if avatars.gotVariant != domain.AvatarVariantLarge {
		t.Errorf("variant = %q, want lg", avatars.gotVariant)
	}
}

func TestGetAvatar_RefusesAnUnknownSize(t *testing.T) {
	t.Parallel()
	server := newServerWithAvatars(&fakeAccounts{}, &fakeAvatars{body: stubImageBody})

	// Not served as the default. A client asking for "medium" or "256" has a
	// bug, and handing it a picture hides that bug behind pixels nobody checks.
	recorder := authenticated(
		t, server, http.MethodGet, "/api/v1/storage/avatars/"+uuid.New().String()+"?size=medium", "")

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body %s)", recorder.Code, recorder.Body)
	}
}

func TestGetAvatar_UnknownAssetIs404(t *testing.T) {
	t.Parallel()
	server := newServerWithAvatars(&fakeAccounts{}, &fakeAvatars{err: domain.ErrAvatarAssetNotFound})

	recorder := authenticated(t, server, http.MethodGet, "/api/v1/storage/avatars/"+uuid.New().String(), "")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", recorder.Code, recorder.Body)
	}
}

func TestGetAvatar_MalformedAssetIdIs404(t *testing.T) {
	t.Parallel()
	server := newServerWithAvatars(&fakeAccounts{}, &fakeAvatars{body: stubImageBody})

	recorder := authenticated(t, server, http.MethodGet, "/api/v1/storage/avatars/not-a-uuid", "")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", recorder.Code, recorder.Body)
	}
}

func TestGetAvatar_RequiresASession(t *testing.T) {
	t.Parallel()
	server := newServerWithAvatars(&fakeAccounts{}, &fakeAvatars{body: stubImageBody})

	// Any signed-in learner may read any avatar. "Any signed-in" is the half
	// that still has to hold.
	recorder := anonymous(t, server, http.MethodGet, "/api/v1/storage/avatars/"+uuid.New().String(), "")

	if recorder.Code == http.StatusOK {
		t.Fatal("an anonymous request was served an avatar")
	}
}
