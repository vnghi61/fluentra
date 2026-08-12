//go:build integration

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/repository"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/id"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// The windows this suite runs with. They are minutes rather than months so the
// fake clock can step over them, and they keep the same shape as the real ones:
// trusted is longer than default, admin is shorter than both, and the absolute
// cap is reachable by an active session inside the test.
var testWindows = domain.WindowConfig{
	Idle:          10 * time.Minute,
	IdleTrusted:   30 * time.Minute,
	Absolute:      60 * time.Minute,
	IdleAdmin:     2 * time.Minute,
	AbsoluteAdmin: 5 * time.Minute,
}

// TestAContinuouslyActiveSessionStillDiesAtTheAbsoluteCap is the test this card
// exists to make pass, and it is written first because the bug it catches is
// invisible on every other path.
//
// Sliding rotation and sliding-rotation-with-no-cap behave identically for as
// long as anybody normally looks. The learner refreshes, the window moves
// forward, everything works — and it goes on working forever, which is
// ADR-0022's rejected alternative C: a stolen token used regularly renews itself
// indefinitely and the theft becomes permanent and invisible.
//
// So: rotate steadily, well inside the idle window every time, past the point
// where the absolute cap falls. The session must stop, and it must say
// SESSION_ABSOLUTE_EXPIRED rather than pretending the token was bad.
func TestAContinuouslyActiveSessionStillDiesAtTheAbsoluteCap(t *testing.T) {
	h := newPersistentHarness(t, "absolute-cap@fluentra.test", domain.RoleUser)
	ctx := context.Background()

	signedIn := h.startWith(t, service.StartInput{UserID: h.userID})
	token := signedIn.RefreshToken.Reveal()

	// Six rotations, five minutes apart. Every one is comfortably inside the
	// ten-minute idle window, so nothing here is ever refused for idleness —
	// and the sixth lands past the sixty-minute cap.
	const step = 5 * time.Minute
	var refusal error
	for elapsed := step; elapsed <= testWindows.Absolute+step; elapsed += step {
		h.clock.Advance(step)

		rotated, err := h.service.Rotate(ctx, token)
		if err != nil {
			refusal = err
			break
		}
		token = rotated.RefreshToken.Reveal()
	}

	if refusal == nil {
		t.Fatal("an active session renewed itself past its absolute expiry: " +
			"the window is sliding with no cap, which is the immortal-token design ADR-0022 rejected")
	}
	assertCode(t, refusal, "SESSION_ABSOLUTE_EXPIRED")

	// And it stays dead. A cap that can be walked past by trying again is not a
	// cap.
	_, err := h.service.Rotate(ctx, token)
	if err == nil {
		t.Fatal("the session rotated again after reaching its absolute expiry")
	}
}

// TestRotationMovesTheIdleWindowButNeverPastTheCap is the same property one step
// closer to the code: the replacement token's own expiry is clamped.
//
// Without the clamp the last token before the cap would outlive the session it
// belongs to — the session refuses, the token says it is fine, and the two
// disagree about a credential. Reading them apart is how somebody later
// "fixes" the disagreement by trusting the token.
func TestRotationMovesTheIdleWindowButNeverPastTheCap(t *testing.T) {
	h := newPersistentHarness(t, "clamp@fluentra.test", domain.RoleUser)
	ctx := context.Background()

	signedIn := h.startWith(t, service.StartInput{UserID: h.userID})
	absolute := sessionAbsoluteExpiry(t, signedIn.Session.SessionID)

	// Early: the replacement gets a full idle window, and it is nowhere near
	// the cap.
	h.clock.Advance(time.Minute)
	early, err := h.service.Rotate(ctx, signedIn.RefreshToken.Reveal())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	wantEarly := harnessNow.Add(time.Minute).Add(testWindows.Idle)
	if !early.RefreshExpiresAt.Equal(wantEarly) {
		t.Errorf("early renewal expires %s, want a full idle window at %s", early.RefreshExpiresAt, wantEarly)
	}

	// Walk up to just before the cap, staying inside the idle window every
	// time — a single jump would expire the token for idleness and prove
	// nothing about the clamp.
	token := early.RefreshToken.Reveal()
	var late service.SignedIn
	for h.clock.Now().Add(testWindows.Idle).Before(harnessNow.Add(testWindows.Absolute)) {
		h.clock.Advance(testWindows.Idle / 2)
		late, err = h.service.Rotate(ctx, token)
		if err != nil {
			t.Fatalf("Rotate at %s: %v", h.clock.Now(), err)
		}
		token = late.RefreshToken.Reveal()
	}
	if late.RefreshExpiresAt.After(absolute) {
		t.Errorf("a renewal near the cap expires %s, past the session's own %s",
			late.RefreshExpiresAt, absolute)
	}
	if !late.RefreshExpiresAt.Equal(absolute) {
		t.Errorf("a renewal near the cap expires %s, want it clamped to %s", late.RefreshExpiresAt, absolute)
	}
}

// TestAnAdminSessionDoesNotReceiveTheExtendedWindow is the second thing the card
// singles out, asserted end to end rather than only on the pure function.
//
// The domain test proves WindowsFor returns twelve hours for an admin. This
// proves the value reaches the row — a service that computes the right windows
// and then writes the learner ones is a bug the unit test cannot see.
func TestAnAdminSessionDoesNotReceiveTheExtendedWindow(t *testing.T) {
	learner := newPersistentHarness(t, "window-learner@fluentra.test", domain.RoleUser)
	admin := newPersistentHarness(t, "window-admin@fluentra.test", domain.RoleAdmin)

	learnerSession := learner.startWith(t, service.StartInput{
		UserID: learner.userID, DeviceID: "a-browser", RememberDevice: true,
	})
	adminSession := admin.startWith(t, service.StartInput{
		UserID: admin.userID, DeviceID: "a-browser", RememberDevice: true,
	})

	learnerCap := sessionAbsoluteExpiry(t, learnerSession.Session.SessionID)
	adminCap := sessionAbsoluteExpiry(t, adminSession.Session.SessionID)

	if !learnerCap.Equal(harnessNow.Add(testWindows.Absolute)) {
		t.Errorf("learner cap = %s, want %s", learnerCap, harnessNow.Add(testWindows.Absolute))
	}
	if !adminCap.Equal(harnessNow.Add(testWindows.AbsoluteAdmin)) {
		t.Errorf("admin cap = %s, want the admin window at %s", adminCap, harnessNow.Add(testWindows.AbsoluteAdmin))
	}
	if !adminCap.Before(learnerCap) {
		t.Error("an admin asking to be remembered got a cap no shorter than a learner's")
	}

	// The idle window too: the admin's refresh token must not outlive two
	// minutes even though the request asked to be trusted.
	if !adminSession.RefreshExpiresAt.Equal(harnessNow.Add(testWindows.IdleAdmin)) {
		t.Errorf("admin refresh expires %s, want the 2-minute admin idle window",
			adminSession.RefreshExpiresAt)
	}
	if !learnerSession.RefreshExpiresAt.Equal(harnessNow.Add(testWindows.IdleTrusted)) {
		t.Errorf("trusted learner refresh expires %s, want the 30-minute trusted window",
			learnerSession.RefreshExpiresAt)
	}
}

// TestTrustingADeviceLengthensTheIdleWindowAndNothingElse is BR-AUTH-23 and
// BR-AUTH-24 together.
func TestTrustingADeviceLengthensTheIdleWindowAndNothingElse(t *testing.T) {
	h := newPersistentHarness(t, "trust@fluentra.test", domain.RoleUser)

	untrusted := h.startWith(t, service.StartInput{UserID: h.userID})
	trusted := h.startWith(t, service.StartInput{
		UserID: h.userID, DeviceID: "the-laptop", RememberDevice: true,
	})

	if !trusted.RefreshExpiresAt.After(untrusted.RefreshExpiresAt) {
		t.Errorf("trusting did not lengthen the idle window: %s vs %s",
			untrusted.RefreshExpiresAt, trusted.RefreshExpiresAt)
	}

	untrustedCap := sessionAbsoluteExpiry(t, untrusted.Session.SessionID)
	trustedCap := sessionAbsoluteExpiry(t, trusted.Session.SessionID)
	if !trustedCap.Equal(untrustedCap) {
		t.Errorf("trusting moved the absolute cap: %s vs %s", untrustedCap, trustedCap)
	}

	// The device is listed, with both expiries, so the learner can see when it
	// stops being trusted.
	devices, err := h.devices.List(context.Background(), httpx.Actor{UserID: h.userID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("%d devices listed, want 1", len(devices))
	}
	if !devices[0].AbsoluteExpiresAt.Equal(trustedCap) {
		t.Errorf("device cap = %s, want the session's %s", devices[0].AbsoluteExpiresAt, trustedCap)
	}
}

// TestUntrustingADeviceSignsItOutImmediately is the acceptance criterion, and
// "immediately" is the word doing the work. Untrusting is what a learner reaches
// for when a laptop is lost, and a laptop demoted to a shorter window is still a
// laptop somebody else is signed in on.
func TestUntrustingADeviceSignsItOutImmediately(t *testing.T) {
	h := newPersistentHarness(t, "untrust@fluentra.test", domain.RoleUser)
	ctx := context.Background()

	onDevice := h.startWith(t, service.StartInput{
		UserID: h.userID, DeviceID: "the-lost-laptop", RememberDevice: true,
	})
	elsewhere := h.startWith(t, service.StartInput{UserID: h.userID})

	devices, err := h.devices.List(ctx, httpx.Actor{UserID: h.userID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("%d devices, want 1", len(devices))
	}

	actor := httpx.Actor{UserID: h.userID, SessionID: elsewhere.Session.SessionID}
	if err := h.devices.Untrust(ctx, actor, devices[0].ID); err != nil {
		t.Fatalf("Untrust: %v", err)
	}

	// The lost laptop cannot renew.
	if _, err := h.service.Rotate(ctx, onDevice.RefreshToken.Reveal()); err == nil {
		t.Error("the untrusted device can still rotate")
	}
	if !sessionIsRevoked(t, onDevice.Session.SessionID) {
		t.Error("the untrusted device's session survived")
	}

	// The session the learner is holding is untouched — they untrusted a
	// laptop, not their whole account.
	if sessionIsRevoked(t, elsewhere.Session.SessionID) {
		t.Error("untrusting one device revoked another")
	}
	if _, err := h.service.Rotate(ctx, elsewhere.RefreshToken.Reveal()); err != nil {
		t.Errorf("the other session stopped working: %v", err)
	}

	// And it leaves the list.
	remaining, err := h.devices.List(ctx, httpx.Actor{UserID: h.userID})
	if err != nil {
		t.Fatalf("List after untrust: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("%d devices still listed, want 0", len(remaining))
	}
}

// TestAnotherAccountsDeviceIsANotFound is the same boundary the session list
// draws, for the same reason: 403 would confirm the id names a real device.
func TestAnotherAccountsDeviceIsANotFound(t *testing.T) {
	mine := newPersistentHarness(t, "device-owner@fluentra.test", domain.RoleUser)
	theirs := newPersistentHarness(t, "device-stranger@fluentra.test", domain.RoleUser)
	ctx := context.Background()

	theirs.startWith(t, service.StartInput{
		UserID: theirs.userID, DeviceID: "their-laptop", RememberDevice: true,
	})
	victim, err := theirs.devices.List(ctx, httpx.Actor{UserID: theirs.userID})
	if err != nil || len(victim) != 1 {
		t.Fatalf("seed the victim's device: %v (%d devices)", err, len(victim))
	}

	actor := httpx.Actor{UserID: mine.userID, SessionID: uuid.New()}
	assertCode(t, mine.devices.Untrust(ctx, actor, victim[0].ID), "RESOURCE_NOT_FOUND")
	assertCode(t, mine.devices.Untrust(ctx, actor, uuid.New()), "RESOURCE_NOT_FOUND")

	// Untouched.
	remaining, err := theirs.devices.List(ctx, httpx.Actor{UserID: theirs.userID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(remaining) != 1 {
		t.Error("one account untrusted another account's device")
	}
}

// TestAPasswordResetUntrustsEveryDevice is BR-AUTH-25. A reset is what somebody
// does when they think an attacker is in their account, and a device that stays
// trusted through it is a ninety-day window the attacker keeps.
func TestAPasswordResetUntrustsEveryDevice(t *testing.T) {
	h := newPersistentHarness(t, "reset-devices@fluentra.test", domain.RoleUser)
	ctx := context.Background()

	h.startWith(t, service.StartInput{UserID: h.userID, DeviceID: "laptop", RememberDevice: true})
	h.startWith(t, service.StartInput{UserID: h.userID, DeviceID: "phone", RememberDevice: true})

	before, err := h.devices.List(ctx, httpx.Actor{UserID: h.userID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(before) != 2 {
		t.Fatalf("%d devices before the reset, want 2", len(before))
	}

	if _, err := h.sessions.RevokeAll(ctx, h.userID); err != nil {
		t.Fatalf("RevokeAll: %v", err)
	}

	after, err := h.devices.List(ctx, httpx.Actor{UserID: h.userID})
	if err != nil {
		t.Fatalf("List after revocation: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("%d devices survived a full revocation, want 0", len(after))
	}
}

// sessionAbsoluteExpiry reads the cap straight out of the row, because the
// column is the thing under test and a value the service reported back would
// prove only that it can echo itself.
func sessionAbsoluteExpiry(t *testing.T, sessionID uuid.UUID) time.Time {
	t.Helper()

	var expiry time.Time
	err := pool.QueryRow(context.Background(),
		`SELECT absolute_expires_at FROM core.sessions WHERE id = $1`, sessionID).Scan(&expiry)
	if err != nil {
		t.Fatalf("read absolute_expires_at: %v", err)
	}
	return expiry.UTC()
}

// ------------------------------------------------------------------ harness

type persistentHarness struct {
	*refreshHarness

	devices  *service.DeviceService
	sessions *service.SessionService
}

// stubRoles answers the one question the window calculation asks.
type stubRoles struct{ role string }

func (s stubRoles) RoleOf(context.Context, uuid.UUID) (string, error) { return s.role, nil }

type deviceAdapter struct {
	*repository.Repository
}

func (a deviceAdapter) WithTx(tx pgx.Tx) service.DeviceRepo {
	return deviceAdapter{Repository: a.Repository.WithTx(tx)}
}

func newPersistentHarness(t *testing.T, email, role string) *persistentHarness {
	t.Helper()

	base := newRefreshHarness(t, email, newStubDenylist())
	repo := repository.New(pool)

	keys, err := domain.NewKeyring([]byte(otpKey))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}

	// Rebuilt rather than reused, because the windows and the role resolver are
	// what this suite is about and newRefreshHarness knows about neither.
	base.service = service.NewRefreshService(service.RefreshDeps{
		Pool:    pool,
		Repo:    refreshAdapter{Repository: repo},
		Tokens:  base.tokens,
		Events:  eventWriter{Writer: outbox.NewWriter()},
		Keys:    keys,
		Clock:   base.clock,
		NewID:   id.NewUUIDv7,
		Roles:   stubRoles{role: role},
		Windows: testWindows,
	})

	return &persistentHarness{
		refreshHarness: base,
		devices: service.NewDeviceService(service.DeviceDeps{
			Pool: pool, Repo: deviceAdapter{Repository: repo}, Clock: base.clock,
		}),
		sessions: service.NewSessionService(service.SessionDeps{
			Pool: pool, Repo: sessionAdapter{Repository: repo},
			Tokens: base.tokens, Clock: base.clock,
		}),
	}
}

func (h *persistentHarness) startWith(t *testing.T, input service.StartInput) service.SignedIn {
	t.Helper()

	signedIn, err := h.service.Start(context.Background(), input)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return signedIn
}
