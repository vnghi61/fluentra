# Outbox Lag High Runbook

**Alert:** `OutboxLagHigh`  
**Threshold:** Outbox lag > 60s for 2 minutes  
**Severity:** Page

## Diagnosis

1. Check worker process logs for outbox publisher errors (`shared/outbox`).
2. Check database connectivity or transaction locks on the outbox table.
3. Check Redis or event bus listener health.

## Remediation

1. Restart worker instances if outbox publisher goroutine is stalled.
2. Verify outbox table indexes and active worker polling loops.
