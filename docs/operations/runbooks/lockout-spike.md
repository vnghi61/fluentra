# Login Lockout Spike Runbook

**Alert:** `LoginLockoutSpike`  
**Threshold:** Rate of account lockouts > 50/5m  
**Severity:** Warning

## Diagnosis

Check auth module security logs to see if a specific IP or set of IPs is attempting credential stuffing.

## Remediation

1. Block offending IP addresses at Nginx/gateway layer.
2. Verify rate-limiting middleware is operational.
