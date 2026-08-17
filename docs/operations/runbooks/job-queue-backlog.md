# Job Queue Backlog Runbook

**Alert:** `JobQueueBacklog`  
**Threshold:** River queue depth > 1000 for 15 minutes  
**Severity:** Warning

## Diagnosis

Inspect River queue depth metrics and worker process CPU/Memory utilization.

## Remediation

1. Scale background worker replicas (`cmd/worker`).
2. Verify worker concurrency settings in environment configuration.
