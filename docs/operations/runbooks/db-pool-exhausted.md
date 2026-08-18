# Database Connection Pool Exhausted Runbook

**Alert:** `DatabaseConnectionPoolExhausted`
**Threshold:** `db_pool_connections{state="acquired"} / db_pool_connections{state="max"} > 0.9` for 5 minutes
**Severity:** High

The pool is nearly all handed out. Requests are not failing yet — they queue inside
`pgxpool.Acquire` — so the first symptom users see is latency, not errors. If it stays
here, `Acquire` starts timing out and the API returns 503.

## Diagnosis

Which states the pool is actually in — run this first, because it separates "genuinely
busy" from "leaking":

```promql
db_pool_connections
```

`acquired` near `max` with `idle` at zero is real load or a leak. `constructing` high is
a pool still warming up, which is normal shortly after a deploy and resolves itself.

Then find what is holding the connections:

```sql
SELECT pid, usename, client_addr, state, now() - query_start AS age, query
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY query_start ASC
LIMIT 20;
```

A handful of very old rows is a stuck query. Many young rows is load. Rows sitting in
`idle in transaction` are a leak — a transaction opened and never committed or rolled
back.

## Common causes

1. A query with no index doing sequential scans under load — cross-check the
   `SlowQueries` alert and the Database dashboard's p95 panel.
2. A connection leak: a `Begin` without a matching `Commit`/`Rollback` on some path.
   `internal/shared/dbx`'s `InTx` helper exists to make this impossible; a leak usually
   means somewhere took a connection without it.
3. Traffic genuinely above what the pool is sized for.
4. A long-running job holding a connection while it does non-database work.

## Remediation

1. If rows are `idle in transaction` and old, that is the leak. Identify the code path
   from the query text before killing anything — the terminate below frees the pool but
   loses the evidence.
2. Terminate the worst offenders when the pool must be freed now:

   ```sql
   SELECT pg_terminate_backend(pid)
   FROM pg_stat_activity
   WHERE state = 'idle in transaction'
     AND now() - query_start > interval '5 minutes';
   ```

3. If it is load rather than a leak, raise the pool ceiling — but check Postgres'
   own `max_connections` first, because a pool larger than the server allows moves the
   failure rather than fixing it.
4. If it began at a deploy, roll back and diagnose from the previous version.

## Escalation

Page the on-call engineer if `acquired/max` stays above 0.9 for more than 15 minutes, or
if the API begins returning 503 — at that point it is a user-visible outage, not a
warning.
