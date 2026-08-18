# Slow Queries Runbook

**Alert:** `SlowQueries`  
**Threshold:** Rate of slow queries > 10 per 5 minutes  
**Severity:** Warning

## Diagnosis

```sql
SELECT query, calls, total_exec_time, mean_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;
```

## Remediation

1. Run EXPLAIN ANALYZE on identified queries.
2. Add missing composite or keyset indices.
