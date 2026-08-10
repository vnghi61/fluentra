package domain

import (
	"encoding/hex"
	"time"
)

const (
	// LockoutMaxAttempts is BR-AUTH-08: 5 failed attempts lock the account.
	LockoutMaxAttempts = 5
	// LockoutWindow is BR-AUTH-08: 15-minute sliding window.
	LockoutWindow = 15 * time.Minute
	// LockoutEscalationWindow bounds how long prior lockouts influence the
	// next delay. It prevents an old typo from permanently penalising an
	// account while still making repeated attacks progressively more costly.
	LockoutEscalationWindow = 24 * time.Hour
	// LockoutMaxDuration bounds the exponential backoff at one day.
	LockoutMaxDuration = 24 * time.Hour
)

// LockoutDuration returns the lock period after previousLockouts in the
// escalation window. The first lock lasts 15 minutes, then each completed
// lockout doubles the next period up to one day.
func LockoutDuration(previousLockouts int) time.Duration {
	duration := LockoutWindow
	for range previousLockouts {
		if duration >= LockoutMaxDuration/2 {
			return LockoutMaxDuration
		}
		duration *= 2
	}
	return duration
}

// LockoutAccountKey formats the Redis rate-limiter key for an account lockout counter.
func LockoutAccountKey(subjectHash []byte) string {
	return "auth:lockout:account:" + hex.EncodeToString(subjectHash)
}

// LockoutIPKey formats the Redis rate-limiter key for an IP address lockout counter.
func LockoutIPKey(ipHash []byte) string {
	return "auth:lockout:ip:" + hex.EncodeToString(ipHash)
}
