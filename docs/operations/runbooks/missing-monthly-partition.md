# Missing Monthly Partition Runbook

**Alert:** `ScheduledJobFailing` (`kind="srs.rotate_partitions"` or `kind="learning.rotate_partitions"`)
**Symptom:** `ERROR: no partition of relation "review_logs" found for row` on insert
**Severity:** Page — every graded attempt and every answered review fails until it is fixed

`learn.attempts` and `learn.review_logs` are partitioned monthly by their
timestamp. Both migrations pre-create the current month and three ahead, and a
cron job in the worker extends the window every six hours. An insert fails only
when that job has been failing long enough for the window to run out, which is
why the alert watches the job rather than the error.

## Diagnosis

```sql
-- Which partitions exist? The naming is review_logs_yYYYYmMM — see Remediation.
SELECT c.relname
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'learn'
  AND c.relkind = 'r'
  AND (c.relname ~ '^attempts_y\d{4}m\d{2}$' OR c.relname ~ '^review_logs_y\d{4}m\d{2}$')
ORDER BY c.relname;
```

If the newest partition is the current month, the window has run out and the
next insert dated into next month will fail.

Then find out why the job stopped:

```bash
docker compose logs --tail=200 worker | grep -i "cron job failed"
```

## Common causes

1. The worker has been down or crash-looping, so the rotation never ran.
2. The rotation ran but the function errored — usually a permission change on
   `learn.ensure_partitions` / `learn.ensure_srs_partitions`, which are
   `SECURITY DEFINER` and granted to `fluentra_app`.
3. Someone created a partition by hand under the wrong name. See the warning
   below; this one is self-inflicted and looks like cause 2 in the logs.

## Remediation

Call the functions the migrations created. They are idempotent, they take the
number of months to look ahead, and they return how many partitions they made:

```sql
SELECT learn.ensure_partitions(3);      -- learn.attempts
SELECT learn.ensure_srs_partitions(3);  -- learn.review_logs
```

A return of `0` means nothing was missing. Anything higher is the number of
months that had been lost.

Then confirm an insert dated into next month lands where it should:

```sql
-- Substitute a real card and user; the point is the partition it reports.
INSERT INTO learn.review_logs (card_id, user_id, grade, reviewed_at)
VALUES ('<CARD_ID>', '<USER_ID>', 'good', date_trunc('month', now()) + interval '1 month 14 days')
RETURNING tableoid::regclass::text;
```

> **Do not create the partition with `CREATE TABLE ... PARTITION OF` by hand.**
>
> The rotation functions look for `review_logs_yYYYYmMM` (and
> `attempts_yYYYYmMM`) with `to_regclass` before creating anything. A partition
> added under any other name — `review_logs_2026_10`, say — is invisible to that
> check, so the next scheduled run tries to create the correctly-named one and
> fails on an overlap:
>
> ```
> ERROR: partition "review_logs_y2026m10" would overlap partition "review_logs_2026_10"
> ```
>
> The rotation then fails on every run, silently extending the outage into the
> following months. If this has already happened, drop the wrongly-named
> partition — after copying any rows out of it — and re-run the function.

## Verification

```sql
SELECT learn.ensure_srs_partitions(3);  -- expect 0 on a second call
```

The `ScheduledJobFailing` alert clears within one job interval (six hours) once
the underlying cause is fixed. If it does not, the job is still erroring for a
different reason — go back to the logs.

## Escalation

If the functions themselves are missing, the migration was rolled back. Check
`goose_db_version` and re-apply; do not recreate the functions by hand, because
the advisory-lock ids and the partition naming both live in the migration.

---

*Walked through on 2026-08-25 against a migrated database: the September
partition was dropped, an insert dated 2026-09-15 failed with
`no partition of relation "review_logs" found for row`,
`learn.ensure_srs_partitions(3)` returned `1`, and the retried insert landed in
`learn.review_logs_y2026m09`. The wrong-name warning above was reproduced the
same way.*
