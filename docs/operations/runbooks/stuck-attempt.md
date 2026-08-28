# Stuck Exercise Attempt Runbook

**Alert:** `GradingErrorRateHigh`
**Symptom:** rows in `learn.attempts` sitting in `grading` for more than a few minutes
**Severity:** Page if it is more than a handful — a learner in this state has answered and been told nothing

`grading` is a claim, not a queue. `SubmitAttempt` takes it with a conditional
`UPDATE`, grades, and writes the score, the rollup and the status in one
transaction — so an attempt is only ever in `grading` for as long as that
transaction takes. Anything longer means the process holding the claim died
between taking it and committing.

> **Phase 2 has no asynchronous grading.** The exercise engine supports it —
> `GradeResult.Async` leaves the attempt in `grading` for a consumer to settle —
> but the only registered grader is `vocabulary`, and it always returns
> `Async: false`. There is no `learning.grade_attempt` River worker and no DLQ to
> check. If attempts are stuck *and* a grader has started returning `Async: true`,
> that is the bug: the consumer that clears them is Phase 3.

## Diagnosis

```sql
-- Attempts that took the claim and never settled.
SELECT id, user_id, activity_id, status, created_at, updated_at
FROM learn.attempts
WHERE status = 'grading'
  AND updated_at < now() - interval '5 minutes'
ORDER BY updated_at ASC
LIMIT 20;
```

`'grading'` is the only stuck state worth querying: `ck_attempts_status` permits
`in_progress`, `grading`, `graded` and `failed`, and `in_progress` is simply an
attempt the learner has not submitted yet.

Then check whether the API was restarting around `updated_at`:

```bash
docker compose logs --since 30m api | grep -iE "panic|shutting down|signal"
```

## Common causes

1. The API process was killed — deploy, OOM, or a panic — while holding the claim.
2. The grading transaction failed to commit: database failover, or the pool
   exhausted mid-transaction. `DatabaseConnectionPoolExhausted` usually fires too.
3. A grader was registered that returns `Async: true`. See the note above.

## What the learner sees

A resubmission with the **same** `Idempotency-Key` waits briefly for the claim to
settle and then returns the stored result, so a learner who retries the same
answer recovers on their own. A resubmission with a *different* key gets
`ALREADY_GRADED`. So a stuck attempt blocks that one activity for that learner
and nothing else — it is not an outage, and it does not need a 3am fix unless
the count is climbing.

## Remediation

Release the claim so the learner can submit again. Attempts are partitioned
monthly, so scope the update by `created_at` to keep it to one partition:

```sql
UPDATE learn.attempts
SET status = 'in_progress', updated_at = now()
WHERE status = 'grading'
  AND updated_at < now() - interval '15 minutes'
  AND created_at >= date_trunc('month', now());
```

`in_progress`, not `failed`: the learner has not failed anything — their answer
was never graded — and `in_progress` is the state a resubmission expects. Do not
invent a score.

If the count is large, stop the bleeding first: the cause is upstream (cause 1 or
2), and released attempts will re-stick until it is fixed.

## Verification

```sql
SELECT count(*) FROM learn.attempts
WHERE status = 'grading' AND updated_at < now() - interval '15 minutes';
```

Expect `0`, and expect it to stay `0` on a second check five minutes later.

## Escalation

If attempts re-enter `grading` and stay there after the release, the API is
crashing mid-transaction — treat it as an availability incident and page the
on-call engineer rather than repeating the update.

---

*Walked through on 2026-08-25 against a migrated database: an attempt was moved
into `grading` by hand, the diagnosis query found it, the remediation query
returned it to `in_progress`, and the verification query returned 0.*
