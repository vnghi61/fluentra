# Database Connection Pool Exhausted Runbook

**Alert:** `DatabaseConnectionPoolExhausted`  
**Threshold:** Active connection pool ratio > 90% for 5 minutes  
**Severity:** High

## Diagnosis

```sql
SELECT pid, usename, client_addr, state, query_start, query
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY query_start ASC;
```

## Remediation

1. Terminate long-running idle or blocked queries.
2. Check for connection leaks in recent service code.
3. Increase max pool size if capacity allows.
