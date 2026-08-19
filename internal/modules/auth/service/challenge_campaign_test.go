package service_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/platform/cache"
)

// countingLimiter is a small in-memory limiter that applies the per-key quota
// the way Redis does, so a simulation can exercise the real counting branch
// rather than a scripted verdict. It is deliberately separate from fakeLimiter,
// whose job is to be told what to answer, not to count.
type countingLimiter struct {
	mu     sync.Mutex
	counts map[string]int
}

func newCountingLimiter() *countingLimiter { return &countingLimiter{counts: make(map[string]int)} }

func (c *countingLimiter) Allow(_ context.Context, key string, limit int, _ time.Duration) (cache.LimitResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[key]++
	if c.counts[key] > limit {
		return cache.LimitResult{Allowed: false, Remaining: 0, ResetIn: time.Hour}, nil
	}
	return cache.LimitResult{Allowed: true, Remaining: limit - c.counts[key], ResetIn: time.Hour}, nil
}

// TestIssue_DistributedCampaignIsStoppedByThePerIPCap is the P5.5 simulation.
//
// A guessing campaign does not need to hit one challenge five times; it can
// issue a fresh challenge for a fresh address and get five new guesses each
// time. The per-challenge attempt counter cannot see that at all, and the
// per-subject cap cannot either — every subject here is used exactly once.
// What stops the campaign is the global per-IP issuance cap, and that is what
// this test proves: the twenty-first issuance from one address is refused while
// every one of the first twenty — each for a different subject — succeeded.
func TestIssue_DistributedCampaignIsStoppedByThePerIPCap(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	counting := newCountingLimiter()
	svc := service.NewChallengeService(service.ChallengeDeps{
		Repo:    h.repo,
		Limiter: counting,
		Keys:    h.keys,
		Clock:   h.clock,
		NewID:   func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
		Env:     testEnv,
	})

	// One address issues against many subjects, so neither the per-subject cap
	// nor the per-challenge attempt counter is what bounds it.
	attacker := withClientIP(t, "203.0.113.7")

	const quota = 20 // DefaultConfig().IssuesPerIPPerHour
	for i := 0; i < quota+5; i++ {
		subject := fmt.Sprintf("victim-%d@fluentra.test", i)
		_, err := svc.Issue(attacker, service.IssueRequest{
			Purpose: domain.PurposeVerifyEmail, Subject: subject,
		})
		if i < quota {
			if err != nil {
				t.Fatalf("issuance %d (subject %q) was refused before the per-IP quota: %v", i, subject, err)
			}
			continue
		}
		assertCode(t, err, "OTP_ISSUE_LIMIT_REACHED")
	}

	// A different address is untouched: the cap is per-IP, not global.
	if _, err := svc.Issue(withClientIP(t, "203.0.113.8"), service.IssueRequest{
		Purpose: domain.PurposeVerifyEmail, Subject: testSubject,
	}); err != nil {
		t.Fatalf("a second address should be unaffected by the first address's quota: %v", err)
	}
}
