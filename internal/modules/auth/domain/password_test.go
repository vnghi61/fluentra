package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// stubChecker stands in for the corpus. It records what it was asked so the
// tests can assert the policy consults it at all, and only when it should.
type stubChecker struct {
	breached bool
	err      error
	calls    int
	asked    string
}

func (s *stubChecker) Breached(_ context.Context, password string) (bool, error) {
	s.calls++
	s.asked = password
	return s.breached, s.err
}

func TestPolicy_AcceptsAPasswordThatMeetsEveryRule(t *testing.T) {
	policy := domain.Policy{}

	if err := policy.Validate(context.Background(), "a perfectly fine passphrase", "nghi@fluentra.test"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestPolicy_RejectsAPasswordShorterThanTheMinimum(t *testing.T) {
	policy := domain.Policy{}

	// Eleven characters: one short of BR-AUTH-01's twelve. The boundary is
	// tested from both sides because an off-by-one here is the difference
	// between the documented policy and a weaker one.
	err := policy.Validate(context.Background(), strings.Repeat("a", 11), "nghi@fluentra.test")
	assertTooWeak(t, err)

	if err := policy.Validate(context.Background(), strings.Repeat("a", 12), "nghi@fluentra.test"); err != nil {
		t.Errorf("a twelve-character password was rejected: %v", err)
	}
}

func TestPolicy_HonoursAConfiguredMinimumLength(t *testing.T) {
	policy := domain.Policy{MinLength: 16}

	assertTooWeak(t, policy.Validate(context.Background(), strings.Repeat("a", 15), "nghi@fluentra.test"))

	if err := policy.Validate(context.Background(), strings.Repeat("a", 16), "nghi@fluentra.test"); err != nil {
		t.Errorf("a password at the configured minimum was rejected: %v", err)
	}
}

// TestPolicy_CountsLengthInRunesNotBytes is the check most likely to be wrong
// in the lenient direction. A Vietnamese passphrase is multi-byte nearly
// everywhere, so counting bytes would accept something shorter than the policy
// allows while appearing to work for every English test case.
func TestPolicy_CountsLengthInRunesNotBytes(t *testing.T) {
	policy := domain.Policy{}

	// Eleven runes, but well over twelve bytes in UTF-8.
	eleven := "mậtkhẩucủa"
	if len([]rune(eleven)) >= 12 {
		t.Fatalf("the fixture is %d runes, it must be under twelve for this test to mean anything",
			len([]rune(eleven)))
	}
	if len(eleven) < 12 {
		t.Fatalf("the fixture is %d bytes, it must be over twelve for this test to mean anything", len(eleven))
	}

	assertTooWeak(t, policy.Validate(context.Background(), eleven, "nghi@fluentra.test"))
}

func TestPolicy_RejectsAPasswordLongerThanTheMaximum(t *testing.T) {
	policy := domain.Policy{}

	err := policy.Validate(context.Background(), strings.Repeat("a", domain.MaxPasswordLength+1), "nghi@fluentra.test")
	assertTooWeak(t, err)
}

// TestPolicy_RejectsTheEmailLocalPart covers the card's second rule. The
// comparison is case-insensitive because "Nghi" is not meaningfully harder to
// guess than "nghi" for someone who already knows the address.
func TestPolicy_RejectsTheEmailLocalPart(t *testing.T) {
	policy := domain.Policy{}

	cases := map[string]struct{ password, email string }{
		"exactly the local part": {"nghinguyenvan", "nghinguyenvan@fluentra.test"},
		"differing only in case": {"NghiNguyenVan", "nghinguyenvan@fluentra.test"},
		"address in mixed case":  {"nghinguyenvan", "NghiNguyenVan@Fluentra.test"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			assertTooWeak(t, policy.Validate(context.Background(), testCase.password, testCase.email))
		})
	}
}

// TestPolicy_AllowsAPasswordThatMerelyContainsTheLocalPart draws the line the
// card draws: equality, not containment. A rule that rejected any password
// containing the local part would reject a strong passphrase for a learner
// whose address happens to be "an@…", and would teach nothing.
func TestPolicy_AllowsAPasswordThatMerelyContainsTheLocalPart(t *testing.T) {
	policy := domain.Policy{}

	if err := policy.Validate(context.Background(), "nghi rides a bicycle", "nghi@fluentra.test"); err != nil {
		t.Errorf("a password containing the local part was rejected: %v", err)
	}
}

func TestPolicy_TakesTheLocalPartFromTheLastAtSign(t *testing.T) {
	policy := domain.Policy{}

	// A quoted local part may itself contain an '@'. Splitting on the first
	// would compare against `"nghi` and match nothing.
	assertTooWeak(t, policy.Validate(context.Background(), `"nghi@home"`, `"nghi@home"@fluentra.test`))
}

func TestPolicy_ConsultsTheBreachCorpus(t *testing.T) {
	checker := &stubChecker{breached: true}
	policy := domain.Policy{Breaches: checker}

	assertTooWeak(t, policy.Validate(context.Background(), "a perfectly fine passphrase", "nghi@fluentra.test"))

	if checker.calls != 1 {
		t.Errorf("the corpus was consulted %d times, want once", checker.calls)
	}
	if checker.asked != "a perfectly fine passphrase" {
		t.Errorf("the corpus was asked about %q, want the submitted password", checker.asked)
	}
}

// TestPolicy_FailsOpenWhenTheCorpusIsUnavailable is the acceptance criterion
// "the breach check failing does not block registration". The direction is
// deliberate and it is the one a future change is most likely to get wrong, so
// the assertion is explicit rather than folded into a table.
func TestPolicy_FailsOpenWhenTheCorpusIsUnavailable(t *testing.T) {
	checker := &stubChecker{err: errors.New("range request: context deadline exceeded")}
	policy := domain.Policy{Breaches: checker}

	if err := policy.Validate(context.Background(), "a perfectly fine passphrase", "nghi@fluentra.test"); err != nil {
		t.Fatalf("an unavailable corpus blocked the password: %v", err)
	}
	if checker.calls != 1 {
		t.Errorf("the corpus was consulted %d times, want once", checker.calls)
	}
}

// TestPolicy_ChecksShapeBeforeTheCorpus matters for two reasons: an eight-
// character password does not need a network round trip to be rejected, and a
// password that fails policy should never leave the process at all, not even as
// five characters of a digest.
func TestPolicy_ChecksShapeBeforeTheCorpus(t *testing.T) {
	checker := &stubChecker{breached: true}
	policy := domain.Policy{Breaches: checker}

	assertTooWeak(t, policy.Validate(context.Background(), "short", "nghi@fluentra.test"))

	if checker.calls != 0 {
		t.Errorf("the corpus was consulted %d times for a password that failed on length", checker.calls)
	}
}

// TestPolicy_SkipsTheCorpusWhenNoCheckerIsConfigured is BREACHED_PASSWORD_CHECK
// being false: the composition root passes nothing rather than a checker told
// to answer no.
func TestPolicy_SkipsTheCorpusWhenNoCheckerIsConfigured(t *testing.T) {
	policy := domain.Policy{}

	if err := policy.Validate(context.Background(), "a perfectly fine passphrase", "nghi@fluentra.test"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// assertTooWeak checks both halves of what the client receives: the 422 that
// makes it a validation failure, and the field name that lets the form point at
// the right input.
func assertTooWeak(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("the password was accepted, want PASSWORD_TOO_WEAK")
	}
	if !apperr.Is(err, apperr.Validation) {
		t.Fatalf("error = %v, want a validation error", err)
	}

	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want an *apperr.Error", err)
	}
	if appErr.Code != "PASSWORD_TOO_WEAK" {
		t.Errorf("code = %q, want PASSWORD_TOO_WEAK", appErr.Code)
	}
	if len(appErr.Fields) != 1 || appErr.Fields[0].Field != "password" {
		t.Errorf("fields = %+v, want exactly one naming \"password\"", appErr.Fields)
	}
}
