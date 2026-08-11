package service_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// otpKey is a test key. It is not a secret and protects nothing — it exists so
// the HMAC has a key of legal length.
const otpKey = "test-otp-hmac-key-at-least-32-bytes-long"

const testSubject = "learner@fluentra.test"

var testNow = time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)

type harness struct {
	service *service.ChallengeService
	repo    *fakeRepository
	limiter *fakeLimiter
	clock   *clock.Fake
	keys    domain.Keyring
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	keys, err := domain.NewKeyring([]byte(otpKey))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}

	repo := newFakeRepository()
	limiter := newFakeLimiter()
	fakeClock := clock.NewFake(testNow)

	return &harness{
		service: service.NewChallengeService(service.ChallengeDeps{
			Repo:    repo,
			Limiter: limiter,
			Keys:    keys,
			Clock:   fakeClock,
			NewID:   func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
			Env:     testEnv,
		}),
		repo:    repo,
		limiter: limiter,
		clock:   fakeClock,
		keys:    keys,
	}
}

func (h *harness) issue(t *testing.T) service.Issued {
	t.Helper()
	issued, err := h.service.Issue(context.Background(), service.IssueRequest{
		Purpose: domain.PurposeVerifyEmail, Subject: testSubject,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return issued
}

func TestIssue_CreatesAUsableChallengeThatExpiresInTenMinutes(t *testing.T) {
	h := newHarness(t)

	issued := h.issue(t)

	if issued.Challenge.Purpose != domain.PurposeVerifyEmail {
		t.Errorf("purpose = %q, want verify_email", issued.Challenge.Purpose)
	}
	if issued.Challenge.MaxAttempts != domain.MaxAttempts {
		t.Errorf("max attempts = %d, want %d", issued.Challenge.MaxAttempts, domain.MaxAttempts)
	}
	if want := testNow.Add(10 * time.Minute); !issued.Challenge.ExpiresAt.Equal(want) {
		t.Errorf("expires at = %s, want %s", issued.Challenge.ExpiresAt, want)
	}
	if !domain.ValidCodeShape(issued.Code.Reveal(), domain.CodeLength) {
		t.Errorf("code is not %d digits", domain.CodeLength)
	}
}

// TestIssue_StoresOnlyAKeyedDigestOfTheSubjectAndCode is BR-AUTH-10 at the
// storage boundary: neither the address nor the code is in the row.
func TestIssue_StoresOnlyAKeyedDigestOfTheSubjectAndCode(t *testing.T) {
	h := newHarness(t)

	issued := h.issue(t)

	if bytes.Contains(issued.Challenge.SubjectHash, []byte(testSubject)) {
		t.Error("the stored subject hash contains the address")
	}
	if bytes.Contains(issued.Challenge.CodeHash, []byte(issued.Code.Reveal())) {
		t.Error("the stored code hash contains the code")
	}
	if !domain.EqualHash(issued.Challenge.SubjectHash, h.keys.SubjectHash(testSubject)) {
		t.Error("the subject hash is not the keyed digest of the subject")
	}
}

func TestIssue_DrawsADifferentCodeEveryTime(t *testing.T) {
	h := newHarness(t)

	seen := make(map[string]bool)
	for range 20 {
		seen[h.issue(t).Code.Reveal()] = true
	}
	// Twenty draws from a million values colliding even once would be
	// remarkable; all twenty colliding means the code is not random at all.
	if len(seen) < 15 {
		t.Errorf("20 issuances produced %d distinct codes", len(seen))
	}
}

func TestIssue_RejectsAnUnknownPurpose(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.Issue(context.Background(), service.IssueRequest{
		Purpose: domain.Purpose("send_money"), Subject: testSubject,
	})
	if err == nil {
		t.Fatal("an unknown purpose was accepted")
	}
}

// TestIssue_AppliesBothIssuanceLimiters covers the per-subject cap and the
// global per-IP cap. The second is the one that catches a distributed campaign
// across many addresses, which the per-challenge attempt cap cannot see at all
// (AGENT.md §11) — so its absence would be invisible without this assertion.
func TestIssue_AppliesBothIssuanceLimiters(t *testing.T) {
	h := newHarness(t)
	ctx := withClientIP(t, "203.0.113.7")

	if _, err := h.service.Issue(ctx, service.IssueRequest{
		Purpose: domain.PurposeVerifyEmail, Subject: testSubject,
	}); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	keys := h.limiter.keys()
	if len(keys) != 2 {
		t.Fatalf("limiter evaluated %d times, want twice: %v", len(keys), keys)
	}
	if !strings.Contains(keys[0], "otp:issue:ip") {
		t.Errorf("first limiter key = %q, want the per-IP cap to be evaluated first", keys[0])
	}
	if !strings.Contains(keys[1], "otp:issue:subject") {
		t.Errorf("second limiter key = %q, want the per-subject cap", keys[1])
	}
}

// TestIssue_LimiterKeysCarryNoPersonalData matters because Redis keys are
// visible to anyone who can run KEYS, and they outlive the request.
func TestIssue_LimiterKeysCarryNoPersonalData(t *testing.T) {
	h := newHarness(t)
	const address = "203.0.113.7"

	if _, err := h.service.Issue(withClientIP(t, address), service.IssueRequest{
		Purpose: domain.PurposeVerifyEmail, Subject: testSubject,
	}); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	for _, key := range h.limiter.keys() {
		if strings.Contains(key, testSubject) {
			t.Errorf("limiter key %q contains the subject address", key)
		}
		if strings.Contains(key, address) {
			t.Errorf("limiter key %q contains the client address", key)
		}
	}
}

func TestIssue_RefusesOnceTheSubjectHasHadItsHourlyQuota(t *testing.T) {
	h := newHarness(t)
	h.limiter.deny(subjectKey(h))

	_, err := h.service.Issue(context.Background(), service.IssueRequest{
		Purpose: domain.PurposeVerifyEmail, Subject: testSubject,
	})
	assertCode(t, err, "OTP_ISSUE_LIMIT_REACHED")
}

func TestIssue_RefusesOnceTheClientAddressHasHadItsQuota(t *testing.T) {
	h := newHarness(t)
	ctx := withClientIP(t, "203.0.113.7")
	h.limiter.deny(ipKey(h, "203.0.113.7"))

	_, err := h.service.Issue(ctx, service.IssueRequest{
		Purpose: domain.PurposeVerifyEmail, Subject: testSubject,
	})
	assertCode(t, err, "OTP_ISSUE_LIMIT_REACHED")
}

// TestIssue_StillWorksWhenTheLimiterIsDown is `cache.Limiter`'s documented
// degradation: a Redis outage must not stop registrations. It is the direction
// a future change is most likely to reverse, so it is asserted explicitly.
func TestIssue_StillWorksWhenTheLimiterIsDown(t *testing.T) {
	h := newHarness(t)
	h.limiter.degraded = true

	if _, err := h.service.Issue(context.Background(), service.IssueRequest{
		Purpose: domain.PurposeVerifyEmail, Subject: testSubject,
	}); err != nil {
		t.Fatalf("a degraded limiter blocked issuance: %v", err)
	}
}

func TestVerify_AcceptsTheRightCodeOnce(t *testing.T) {
	h := newHarness(t)
	issued := h.issue(t)

	consumed, err := h.service.Verify(context.Background(), issued.Challenge.ID, issued.Code.Reveal())
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !consumed.Consumed() {
		t.Error("a verified challenge was not marked consumed")
	}

	// Single use. The same code, immediately again.
	_, err = h.service.Verify(context.Background(), issued.Challenge.ID, issued.Code.Reveal())
	assertCode(t, err, "OTP_ALREADY_USED")
}

// TestVerify_ACodeFromOneChallengeDoesNotVerifyAnother is the acceptance
// criterion, and it is forced rather than sampled: both challenges are given the
// *same* code, so a construction that hashed the code alone would accept it.
// Only binding the challenge id into the HMAC makes this fail.
func TestVerify_ACodeFromOneChallengeDoesNotVerifyAnother(t *testing.T) {
	h := newHarness(t)

	first := h.issue(t)
	second := h.issue(t)

	// Force the collision that would otherwise be one in a million.
	forceCode(t, h, second.Challenge.ID, first.Code.Reveal())

	_, err := h.service.Verify(context.Background(), second.Challenge.ID, first.Code.Reveal())
	if err != nil {
		t.Fatalf("the forced fixture is wrong, the code should verify its own challenge: %v", err)
	}

	third := h.issue(t)
	_, err = h.service.Verify(context.Background(), third.Challenge.ID, first.Code.Reveal())
	if err == nil {
		t.Fatal("a code issued for one challenge verified a different one")
	}
}

// TestVerify_BurnsTheChallengeAfterExactlyFiveWrongCodes is BR-AUTH-12. The
// count is asserted at every step, because "roughly five" is the failure this
// test exists to catch: four would frustrate learners, six is a 20 % larger
// guessing budget.
func TestVerify_BurnsTheChallengeAfterExactlyFiveWrongCodes(t *testing.T) {
	h := newHarness(t)
	issued := h.issue(t)
	wrong := wrongCode(issued.Code.Reveal())

	for attempt := 1; attempt <= domain.MaxAttempts-1; attempt++ {
		_, err := h.service.Verify(context.Background(), issued.Challenge.ID, wrong)
		assertCode(t, err, "OTP_INVALID")
		assertRemaining(t, err, domain.MaxAttempts-attempt)
	}

	// The fifth wrong code spends the last attempt and burns it.
	_, err := h.service.Verify(context.Background(), issued.Challenge.ID, wrong)
	assertCode(t, err, "OTP_ATTEMPTS_EXCEEDED")

	// And a burned challenge cannot be retried, not even with the right code.
	_, err = h.service.Verify(context.Background(), issued.Challenge.ID, issued.Code.Reveal())
	assertCode(t, err, "OTP_ATTEMPTS_EXCEEDED")
}

// TestVerify_ChargesAnAttemptForAMalformedCode closes the loophole where
// submitting one character keeps a challenge alive for its whole window without
// spending any of its budget.
func TestVerify_ChargesAnAttemptForAMalformedCode(t *testing.T) {
	h := newHarness(t)
	issued := h.issue(t)

	_, err := h.service.Verify(context.Background(), issued.Challenge.ID, "x")
	assertCode(t, err, "OTP_INVALID")
	assertRemaining(t, err, domain.MaxAttempts-1)
}

// TestVerify_NeverReachesTheWriteWithAWrongCode is the constant-time claim's
// structural half: the guarded consumption is only attempted after the
// comparison has already succeeded, so a wrong code cannot be distinguished by
// whether a write happened.
func TestVerify_NeverReachesTheWriteWithAWrongCode(t *testing.T) {
	h := newHarness(t)
	issued := h.issue(t)

	for range 3 {
		_, _ = h.service.Verify(context.Background(), issued.Challenge.ID, wrongCode(issued.Code.Reveal()))
	}
	if h.repo.consumeCalls != 0 {
		t.Errorf("consumption was attempted %d times for wrong codes, want none", h.repo.consumeCalls)
	}
}

func TestVerify_RefusesAnExpiredChallenge(t *testing.T) {
	h := newHarness(t)
	issued := h.issue(t)

	h.clock.Advance(10*time.Minute - time.Nanosecond)
	if _, err := h.service.Verify(context.Background(), issued.Challenge.ID, issued.Code.Reveal()); err != nil {
		t.Fatalf("a challenge one nanosecond inside its window was refused: %v", err)
	}

	next := h.issue(t)
	h.clock.Advance(10 * time.Minute)
	_, err := h.service.Verify(context.Background(), next.Challenge.ID, next.Code.Reveal())
	assertCode(t, err, "OTP_EXPIRED")
}

func TestVerify_UnknownChallengeIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.Verify(context.Background(), uuid.New(), "123456")
	assertCode(t, err, "CHALLENGE_NOT_FOUND")
}

// TestResend_ReplacesTheCodeAndClearsAttemptsWithoutMovingTheExpiry is the
// acceptance criterion in one test, because the three halves are only correct
// together: a resend that also moved the expiry would give an attacker an
// indefinitely valid challenge for the price of pressing a button.
func TestResend_ReplacesTheCodeAndClearsAttemptsWithoutMovingTheExpiry(t *testing.T) {
	h := newHarness(t)
	issued := h.issue(t)
	originalExpiry := issued.Challenge.ExpiresAt

	// Spend two attempts, then wait out the cooldown.
	for range 2 {
		_, _ = h.service.Verify(context.Background(), issued.Challenge.ID, wrongCode(issued.Code.Reveal()))
	}
	h.clock.Advance(domain.ResendCooldown)

	resent, err := h.service.Resend(context.Background(), issued.Challenge.ID)
	if err != nil {
		t.Fatalf("Resend: %v", err)
	}

	if !resent.Challenge.ExpiresAt.Equal(originalExpiry) {
		t.Errorf("expiry moved to %s from %s — a resend must not extend it",
			resent.Challenge.ExpiresAt, originalExpiry)
	}
	if resent.Challenge.Attempts != 0 {
		t.Errorf("attempts = %d after a resend, want 0", resent.Challenge.Attempts)
	}
	if resent.Code.Reveal() == issued.Code.Reveal() {
		t.Error("a resend delivered the same code")
	}

	// The superseded code must no longer work, and the new one must.
	_, err = h.service.Verify(context.Background(), issued.Challenge.ID, issued.Code.Reveal())
	assertCode(t, err, "OTP_INVALID")

	if _, err := h.service.Verify(context.Background(), issued.Challenge.ID, resent.Code.Reveal()); err != nil {
		t.Fatalf("the resent code did not verify: %v", err)
	}
}

func TestResend_IsRefusedInsideTheCooldown(t *testing.T) {
	h := newHarness(t)
	issued := h.issue(t)

	h.clock.Advance(domain.ResendCooldown - time.Second)
	_, err := h.service.Resend(context.Background(), issued.Challenge.ID)
	assertCode(t, err, "OTP_RESEND_TOO_SOON")
}

// TestResend_CooldownSurvivesTheLimiterBeingDown is why the database carries its
// own `last_sent_at` guard. `cache.Limiter` allows everything when Redis is
// unreachable; without the second check, a Redis outage would turn resend into
// an email flood aimed at any address the attacker names.
func TestResend_CooldownSurvivesTheLimiterBeingDown(t *testing.T) {
	h := newHarness(t)
	issued := h.issue(t)
	h.limiter.degraded = true

	h.clock.Advance(domain.ResendCooldown - time.Second)
	_, err := h.service.Resend(context.Background(), issued.Challenge.ID)
	assertCode(t, err, "OTP_RESEND_TOO_SOON")

	h.clock.Advance(time.Second)
	if _, err := h.service.Resend(context.Background(), issued.Challenge.ID); err != nil {
		t.Fatalf("Resend past the cooldown: %v", err)
	}
}

// TestResend_DoesNotUnburnAChallenge is BR-AUTH-12: a spent challenge must be
// replaced, so resend cannot be a way around the attempt cap.
func TestResend_DoesNotUnburnAChallenge(t *testing.T) {
	h := newHarness(t)
	issued := h.issue(t)

	for range domain.MaxAttempts {
		_, _ = h.service.Verify(context.Background(), issued.Challenge.ID, wrongCode(issued.Code.Reveal()))
	}
	h.clock.Advance(domain.ResendCooldown)

	_, err := h.service.Resend(context.Background(), issued.Challenge.ID)
	assertCode(t, err, "OTP_ATTEMPTS_EXCEEDED")
}

func TestResend_IsRefusedForAConsumedChallenge(t *testing.T) {
	h := newHarness(t)
	issued := h.issue(t)

	if _, err := h.service.Verify(context.Background(), issued.Challenge.ID, issued.Code.Reveal()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	h.clock.Advance(domain.ResendCooldown)

	_, err := h.service.Resend(context.Background(), issued.Challenge.ID)
	assertCode(t, err, "OTP_ALREADY_USED")
}

// TestTheCodeNeverReachesTheLogs is the acceptance criterion that asks for a
// test which greps the captured log output. It drives every path that has a
// code in scope — issue, a wrong guess, a burn, a resend, a success — with
// logging turned all the way up, and then looks for any of the codes that were
// generated along the way.
//
// It also checks the subject address, because a log line that named the learner
// alongside a challenge id would defeat the point of storing only a digest.
func TestTheCodeNeverReachesTheLogs(t *testing.T) {
	var captured bytes.Buffer
	restore := captureDefaultLogger(t, &captured)
	defer restore()

	h := newHarness(t)
	ctx := withClientIP(t, "203.0.113.7")

	issued := h.issue(t)
	codes := []string{issued.Code.Reveal()}

	// A wrong guess, then four more to burn a second challenge.
	_, _ = h.service.Verify(ctx, issued.Challenge.ID, wrongCode(issued.Code.Reveal()))

	h.clock.Advance(domain.ResendCooldown)
	resent, err := h.service.Resend(ctx, issued.Challenge.ID)
	if err != nil {
		t.Fatalf("Resend: %v", err)
	}
	codes = append(codes, resent.Code.Reveal())

	if _, err := h.service.Verify(ctx, issued.Challenge.ID, resent.Code.Reveal()); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	burned := h.issue(t)
	codes = append(codes, burned.Code.Reveal())
	for range domain.MaxAttempts {
		_, _ = h.service.Verify(ctx, burned.Challenge.ID, wrongCode(burned.Code.Reveal()))
	}

	output := captured.String()
	if output == "" {
		t.Fatal("nothing was captured, so this test proves nothing")
	}
	for _, code := range codes {
		if strings.Contains(output, code) {
			t.Errorf("the log output contains an issued code")
		}
	}
	if strings.Contains(output, testSubject) {
		t.Error("the log output contains the subject address")
	}
	if strings.Contains(output, "203.0.113.7") {
		t.Error("the log output contains the client address")
	}
}

// captureDefaultLogger points slog at buffer at debug level and returns a
// function restoring the previous default. The service logs through the package
// -level slog functions, which is what the rest of the repository does, so
// capturing means replacing the default.
func captureDefaultLogger(t *testing.T, buffer *bytes.Buffer) func() {
	t.Helper()

	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buffer, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return func() { slog.SetDefault(previous) }
}

// withClientIP produces a context carrying address, by running the real
// resolver middleware over a request from that peer.
//
// The context key httpx.ClientIP reads is unexported and there is no public
// setter, which is correct — in production only the middleware ever sets it.
// Going through the middleware rather than adding one is both in scope and more
// honest: the tests then exercise the same path a request does.
func withClientIP(t *testing.T, address string) context.Context {
	t.Helper()

	// No trusted proxies, so the peer address is the client address and no
	// X-Forwarded-For is consulted.
	resolver, err := httpx.NewClientIPResolver(nil)
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}

	var captured context.Context
	resolver.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		captured = request.Context()
	})).ServeHTTP(httptest.NewRecorder(), requestFrom(address))

	if got := httpx.ClientIP(captured); got.String() != address {
		t.Fatalf("client ip = %s, want %s", got, address)
	}
	return captured
}

func requestFrom(address string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil)
	request.RemoteAddr = net.JoinHostPort(address, "51000")
	return request
}

func subjectKey(h *harness) string {
	return "fluentra:test:auth:otp:issue:subject:" + hexOf(h.keys.SubjectHash(testSubject)) + ":v1"
}

func ipKey(h *harness, address string) string {
	return "fluentra:test:auth:otp:issue:ip:" + hexOf(h.keys.SubjectHash(address)) + ":v1"
}

func hexOf(digest []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(digest)*2)
	for _, b := range digest {
		out = append(out, digits[b>>4], digits[b&0x0f])
	}
	return string(out)
}

// wrongCode returns a code of the right shape that is not the right one.
func wrongCode(code string) string {
	shifted := []byte(code)
	shifted[0] = '0' + (shifted[0]-'0'+1)%10
	return string(shifted)
}

// forceCode overwrites a challenge's stored digest so a test can create a
// collision that would otherwise be one in a million.
func forceCode(t *testing.T, h *harness, challengeID uuid.UUID, code string) {
	t.Helper()

	h.repo.mu.Lock()
	defer h.repo.mu.Unlock()

	challenge, ok := h.repo.challenges[challengeID]
	if !ok {
		t.Fatalf("challenge %s is not in the fake", challengeID)
	}
	challenge.CodeHash = h.keys.CodeHash(challengeID, code)
	h.repo.challenges[challengeID] = challenge
}

func assertCode(t *testing.T, err error, wanted string) {
	t.Helper()

	if err == nil {
		t.Fatalf("no error, want %s", wanted)
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want an *apperr.Error with code %s", err, wanted)
	}
	if appErr.Code != wanted {
		t.Fatalf("code = %q, want %q", appErr.Code, wanted)
	}
}

func assertRemaining(t *testing.T, err error, wanted int) {
	t.Helper()

	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want an *apperr.Error", err)
	}
	got, ok := appErr.Meta["attempts_remaining"]
	if !ok {
		t.Fatalf("meta = %v, want attempts_remaining", appErr.Meta)
	}
	if got != wanted {
		t.Errorf("attempts_remaining = %v, want %d", got, wanted)
	}
}
