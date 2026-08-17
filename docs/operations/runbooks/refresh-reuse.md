# Refresh Token Reuse Detected Runbook

**Alert:** `RefreshReuseDetected`  
**Threshold:** Any refresh token reuse event > 0  
**Severity:** Critical

## Diagnosis

Refresh token reuse indicates potential session hijacking (a single-use token was presented twice).

## Remediation

1. Automatic security reaction revokes ALL sessions for the affected user.
2. Verify audit events log both session ID and user ID.
3. Contact affected user if unauthorized access is suspected.
