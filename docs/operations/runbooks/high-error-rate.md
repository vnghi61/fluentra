# High API Error Rate Runbook

**Alert:** `HighErrorRate`  
**Threshold:** Error rate > 5% for 5 minutes  
**Severity:** Warning

## Diagnosis

```sql
-- Check recent HTTP 5xx errors
SELECT status, path, count(*)
FROM http_requests
WHERE occurred_at > now() - interval '10 minutes'
  AND status >= 500
GROUP BY status, path
ORDER BY count DESC
LIMIT 10;
```

## Common Causes

1. Database connection pool exhausted or PostgreSQL unresponsive
2. Redis connection timeout
3. External dependency (e.g. Google OAuth) failure
4. Recent application deployment regression

## Remediation

1. Check database pool: `SELECT * FROM pg_stat_activity WHERE state != 'idle';`
2. Check Redis connectivity: `redis-cli ping`
3. Inspect application logs: `kubectl logs -l app=fluentra-api --tail=100`
4. Rollback deployment if recent release: `kubectl rollout undo deployment/fluentra-api`

## Escalation

If error rate > 20% or lasts > 30 minutes, page the on-call engineer.
