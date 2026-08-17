# High Job Failure Rate Runbook

**Alert:** `JobFailureRateHigh`  
**Threshold:** Failed jobs > 10% for 10 minutes  
**Severity:** High

## Diagnosis

Check failed job logs in Loki or stdout of worker binaries.

## Remediation

1. Identify failing job class (e.g. data export, email delivery).
2. Fix underlying dependency issue (e.g. SMTP server rate limits or MinIO permissions).
3. Retry failed jobs via River admin tool or operational endpoint.
