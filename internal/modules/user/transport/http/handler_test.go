package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/modules/user/service"
	userhttp "github.com/fluentra/fluentra/internal/modules/user/transport/http"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

var (
	actorID = uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345678")
	otherID = uuid.MustParse("0199b2d3-4e5f-7a81-8bcd-ef0123456789")
	fixedAt = time.Date(2026, time.August, 9, 4, 21, 7, 0, time.UTC)
)

// testRequestID stands in for the correlation id the telemetry middleware
// generates.
const testRequestID = "01KZGA1FXY6VAHQABK3EBKDN57"

// fakeAccounts records what the handler asked for. The important field is
// seenActor: every assertion about "you cannot read somebody else" comes down
// to which id the handler passed down.
type fakeAccounts struct {
	seenActor       uuid.UUID
	seenChange      domain.ProfileChange
	seenWanted      domain.Preferences
	seenContentType string
	seenObjectKey   string
	err             error
}

func (f *fakeAccounts) GetAccount(_ context.Context, actorID uuid.UUID) (service.Account, error) {
	f.seenActor = actorID
	if f.err != nil {
		return service.Account{}, f.err
	}
	return account(actorID), nil
}

func (f *fakeAccounts) UpdateProfile(
	_ context.Context, actorID uuid.UUID, change domain.ProfileChange,
) (service.Account, error) {
	f.seenActor = actorID
	f.seenChange = change
	if f.err != nil {
		return service.Account{}, f.err
	}
	return account(actorID), nil
}

func (f *fakeAccounts) GetPreferences(_ context.Context, actorID uuid.UUID) (domain.Preferences, error) {
	f.seenActor = actorID
	if f.err != nil {
		return domain.Preferences{}, f.err
	}
	return preferences(), nil
}

func (f *fakeAccounts) ReplacePreferences(
	_ context.Context, actorID uuid.UUID, wanted domain.Preferences,
) (domain.Preferences, error) {
	f.seenActor = actorID
	f.seenWanted = wanted
	if f.err != nil {
		return domain.Preferences{}, f.err
	}
	wanted.UpdatedAt = fixedAt
	return wanted, nil
}

func (f *fakeAccounts) RequestAvatarUploadIntent(
	_ context.Context, actorID uuid.UUID, contentType string,
) (domain.UploadIntent, error) {
	f.seenActor = actorID
	f.seenContentType = contentType
	if f.err != nil {
		return domain.UploadIntent{}, f.err
	}
	return domain.UploadIntent{
		URL:         "https://storage.local/fluentra-avatars/users/" + actorID.String() + "/raw.jpg",
		Method:      "POST",
		ObjectKey:   "users/" + actorID.String() + "/raw.jpg",
		ExpiresAt:   fixedAt.Add(5 * time.Minute),
		MaxBytes:    domain.AvatarMaxBytes,
		ContentType: contentType,
	}, nil
}

func (f *fakeAccounts) ConfirmAvatar(
	_ context.Context, actorID uuid.UUID, objectKey string,
) (service.Account, error) {
	f.seenActor = actorID
	f.seenObjectKey = objectKey
	if f.err != nil {
		return service.Account{}, f.err
	}
	return account(actorID), nil
}

func (f *fakeAccounts) RequestExport(_ context.Context, actorID uuid.UUID) (domain.ExportRequest, error) {
	f.seenActor = actorID
	if f.err != nil {
		return domain.ExportRequest{}, f.err
	}
	return domain.ExportRequest{
		ID:        uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def999999999"),
		UserID:    actorID,
		Status:    domain.ExportStatusPending,
		CreatedAt: fixedAt,
	}, nil
}

func (f *fakeAccounts) GetExportByID(_ context.Context, actorID, exportID uuid.UUID) (domain.ExportRequest, error) {
	f.seenActor = actorID
	if f.err != nil {
		return domain.ExportRequest{}, f.err
	}
	return domain.ExportRequest{
		ID:        exportID,
		UserID:    actorID,
		Status:    domain.ExportStatusCompleted,
		CreatedAt: fixedAt,
	}, nil
}

func (f *fakeAccounts) RequestDeletion(_ context.Context, actorID uuid.UUID) (domain.DeletionRequest, error) {
	f.seenActor = actorID
	if f.err != nil {
		return domain.DeletionRequest{}, f.err
	}
	return domain.DeletionRequest{
		ID:          uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def888888888"),
		UserID:      actorID,
		Status:      domain.DeletionStatusPending,
		RequestedAt: fixedAt,
		ExecuteAt:   fixedAt.Add(domain.DeletionGracePeriod),
	}, nil
}

func (f *fakeAccounts) CancelDeletion(_ context.Context, actorID uuid.UUID) (domain.DeletionRequest, error) {
	f.seenActor = actorID
	if f.err != nil {
		return domain.DeletionRequest{}, f.err
	}
	now := fixedAt
	return domain.DeletionRequest{
		ID:          uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def888888888"),
		UserID:      actorID,
		Status:      domain.DeletionStatusCancelled,
		RequestedAt: fixedAt,
		ExecuteAt:   fixedAt.Add(domain.DeletionGracePeriod),
		CancelledAt: &now,
	}, nil
}

func (f *fakeAccounts) GetDeletion(_ context.Context, actorID, deletionID uuid.UUID) (domain.DeletionRequest, error) {
	f.seenActor = actorID
	if f.err != nil {
		return domain.DeletionRequest{}, f.err
	}
	return domain.DeletionRequest{
		ID:          deletionID,
		UserID:      actorID,
		Status:      domain.DeletionStatusPending,
		RequestedAt: fixedAt,
		ExecuteAt:   fixedAt.Add(domain.DeletionGracePeriod),
	}, nil
}

func account(id uuid.UUID) service.Account {
	country := "VN"
	born := time.Date(1998, time.March, 4, 0, 0, 0, 0, time.UTC)
	return service.Account{
		User: domain.User{
			ID: id, Email: "learner@example.com", Status: domain.StatusActive,
			EmailVerifiedAt: &fixedAt, CreatedAt: fixedAt, UpdatedAt: fixedAt,
		},
		Profile: domain.Profile{
			UserID: id, DisplayName: "Nghi", Country: &country,
			Timezone: "Asia/Ho_Chi_Minh", DateOfBirth: &born,
		},
	}
}

func preferences() domain.Preferences {
	return domain.Preferences{
		Locale: "vi", Theme: domain.ThemeDark, DailyGoalMinutes: 30,
		NotificationChannels: []domain.Channel{domain.ChannelInApp, domain.ChannelPush},
		QuietHours: &domain.QuietHours{
			Start: domain.TimeOfDay{Hour: 22}, End: domain.TimeOfDay{Hour: 7},
		},
		UpdatedAt: fixedAt,
	}
}

// newServer mounts the handler the way the composition root will, so the tests
// exercise the real routing rather than calling handler methods directly.
func newServer(accounts userhttp.Accounts) http.Handler {
	return newServerWithAvatars(accounts, &fakeAvatars{})
}

// newServerWithAvatars is newServer for the tests that care what the avatar
// surface returns. It is separate because avatar serving is the one route here
// that reads another learner's data, and most tests have no opinion about it.
func newServerWithAvatars(accounts userhttp.Accounts, avatars userhttp.Avatars) http.Handler {
	router := chi.NewRouter()
	router.Route("/api/v1", func(api chi.Router) {
		userhttp.NewHandler(accounts, avatars).Routes(api)
	})
	return router
}

// fakeAvatars serves one image, or an error, for every asset id.
type fakeAvatars struct {
	body  string
	asset domain.AvatarAsset
	err   error

	// gotVariant records what the handler resolved the ?size= parameter to.
	gotVariant domain.AvatarVariant
}

func (f *fakeAvatars) AvatarBlob(
	_ context.Context, assetID uuid.UUID, variant domain.AvatarVariant,
) (io.ReadCloser, domain.AvatarAsset, error) {
	f.gotVariant = variant
	if f.err != nil {
		return nil, domain.AvatarAsset{}, f.err
	}
	asset := f.asset
	asset.AssetID = assetID
	asset.Variant = variant
	if asset.MimeType == "" {
		asset.MimeType = "image/jpeg"
	}
	asset.ByteSize = int64(len(f.body))
	return io.NopCloser(strings.NewReader(f.body)), asset, nil
}

// authenticated performs a request with actor in the context, standing in for
// the middleware that P2.4 adds.
func authenticated(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, handler, method, path, body, &httpx.Actor{UserID: actorID, Role: "user"})
}

func anonymous(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, handler, method, path, body, nil)
}

func do(
	t *testing.T, handler http.Handler, method, path, body string, actor *httpx.Actor,
) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	// The telemetry middleware puts a request id on every request before a
	// handler sees it. The tests do the same, because `request_id` is a
	// required member of the Problem schema and a fixture without one would
	// let a response that is invalid in production pass here.
	ctx := httpx.WithRequestID(request.Context(), testRequestID)
	if actor != nil {
		ctx = httpx.WithActor(ctx, *actor)
	}
	request = request.WithContext(ctx)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeProblem(t *testing.T, recorder *httptest.ResponseRecorder) problem {
	t.Helper()
	var decoded problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode problem: %v (body %s)", err, recorder.Body.String())
	}
	return decoded
}

type problem struct {
	Type      string           `json:"type"`
	Title     string           `json:"title"`
	Status    int              `json:"status"`
	Code      string           `json:"code"`
	RequestID string           `json:"request_id"`
	Errors    []fieldViolation `json:"errors"`
}

type fieldViolation struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
