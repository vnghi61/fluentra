# Slow Requests Runbook

**Alert:** `SlowRequests`  
**Threshold:** P95 latency > 2s for 10 minutes  
**Severity:** Warning

## Diagnosis

1. Check Grafana API Overview dashboard for top 10 slowest endpoints.
2. Inspect OpenTelemetry traces for spans with duration > 2s.

## Remediation

1. Identify unindexed database queries on slow endpoints.
2. Check Redis cache hit ratio.
3. Scale API replicas if CPU/Memory pressure is detected.
