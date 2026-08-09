---
doc_type: guideline
scope: database
last_verified: 2026-08-06
---

# DATABASE_GUIDELINE.md

PostgreSQL 17 · pgx v5 · sqlc · goose. One instance, one schema per module.

---

## 1. Ownership rules

| # | Rule |
|---|---|
| DB1 | Every table belongs to exactly one module, in that module's schema |
| DB2 | A module reads and writes **only** its own tables |
| DB3 | Cross-module data is obtained through `contract` interfaces, never through a JOIN |
| DB4 | The only cross-schema foreign key permitted is `→ core.users(id)` (ADR-0004) |
| DB5 | Migrations live in `db/migrations/<module>/`; queries in `db/queries/<module>/` |
| DB6 | No transaction spans two modules (rule L4) |

**Why DB4 is an exception:** almost every table references a user. Denormalising a user ID
without a FK would trade referential integrity for a purity that buys nothing while we are a
monolith. When a module is extracted, that FK becomes the module's own `user_id` column with
integrity maintained by events. This is documented, deliberate, and bounded to one column.

## 2. Naming

| Object | Convention | Example |
|---|---|---|
| Schema | module tier name | `core`, `learn`, `skill`, `assess`, `content`, `comm`, `billing`, `ai`, `ops`, `audit`, `analytics` |
| Table | plural, `snake_case` | `skill.word_senses` |
| Column | singular, `snake_case` | `next_due_at` |
| PK | `id` | `id uuid` |
| FK | `<singular_referenced>_id` | `deck_id`, `user_id` |
| Boolean | `is_`/`has_`/`can_` prefix | `is_published` |
| Timestamp | `_at` suffix | `created_at`, `deleted_at`, `published_at` |
| Date | `_on` suffix | `expires_on` |
| Count | `_count` suffix | `attempt_count` |
| Index | `idx_<table>_<cols>` | `idx_review_cards_user_due` |
| Unique index | `uq_<table>_<cols>` | `uq_decks_user_slug` |
| FK constraint | `fk_<table>_<ref>` | `fk_decks_user` |
| Check | `ck_<table>_<rule>` | `ck_attempts_score_range` |
| Enum type | singular | `content_status` |
| Partition | `<table>_yYYYYmMM` | `attempts_y2026m08` |

## 3. Standard columns

Every table:

```sql
id          uuid        PRIMARY KEY DEFAULT gen_random_uuid()   -- UUIDv7 generated in app code
created_at  timestamptz NOT NULL DEFAULT now()
updated_at  timestamptz NOT NULL DEFAULT now()
```

Add only when justified:

| Column | When |
|---|---|
| `deleted_at timestamptz` | A business reason exists to recover the row. Otherwise hard-delete |
| `version integer` | Optimistic locking on concurrently edited resources |
| `created_by`, `updated_by uuid` | Admin-authored content (audit already logs the event) |
| `metadata jsonb` | Genuinely open-ended attributes only — never as a way to avoid designing columns |

**IDs are UUIDv7 generated in Go** (`shared/id`), not by the database: the app knows the ID
before insert, which makes outbox events, logging, and idempotency simpler. The DB default is a
safety net only.

## 4. Types

| Need | Type | Never |
|---|---|---|
| Identifier | `uuid` | `serial`, `bigserial` (enumerable, leaks volume) |
| Text | `text` | `varchar(n)` unless the limit is a real business rule |
| Money | `numeric(14,2)` + `currency char(3)` | `float`, `double` |
| Score / percentage | `numeric(5,2)` | `float` |
| Timestamp | `timestamptz` | `timestamp`, epoch integers |
| Duration | `interval` or `_ms integer` | ambiguous "duration" numbers |
| Enum (closed, stable) | Postgres `enum` | `text` with a comment |
| Enum (evolving) | lookup table + FK | ad-hoc strings |
| Flexible attributes | `jsonb` + a check constraint or a validated schema | `json`, EAV tables |
| Arrays | `text[]`/`uuid[]` for small, non-queried sets | as a substitute for a join table |
| Binary | Never — object storage + a reference | `bytea` for files |

## 5. Constraints

Push invariants into the database wherever the database can express them:

```sql
CHECK (score >= 0 AND score <= 100)
CHECK (published_at IS NULL OR status = 'published')
CHECK (char_length(lemma) BETWEEN 1 AND 100)
UNIQUE (user_id, slug)
EXCLUDE USING gist (user_id WITH =, active_period WITH &&)   -- e.g. no overlapping subscriptions
FOREIGN KEY (deck_id) REFERENCES skill.decks(id) ON DELETE CASCADE
```

`NOT NULL` is the default posture; nullability must be justified.
`ON DELETE`: `CASCADE` for owned children, `RESTRICT` for referenced masters, `SET NULL` only
when the relationship is genuinely optional.

## 6. Indexing

| Rule | Detail |
|---|---|
| Every FK gets an index | Postgres does not create one |
| Index the actual query | Column order = equality columns, then range, then sort |
| Partial indexes for skewed predicates | `WHERE deleted_at IS NULL`, `WHERE status = 'published'` |
| Covering indexes | `INCLUDE (…)` when it turns a heap fetch into an index-only scan |
| GIN | `jsonb` containment and full-text (`tsvector`) |
| BRIN | Very large append-only tables ordered by time |
| Unique constraints | Express business uniqueness — do not rely on application checks |
| Never | Index everything "just in case"; every index costs write throughput and space |

Every PR that adds a query must include the `EXPLAIN (ANALYZE, BUFFERS)` output in the
description if the query touches a table expected to exceed 100k rows.

## 7. Migrations

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE skill.decks (…);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE skill.decks;
-- +goose StatementEnd
```

| Rule | Detail |
|---|---|
| Naming | `<unix_ts>_<verb>_<object>.sql` — the timestamp orders migrations across module folders |
| Reversible | Every migration has a working `Down`. If it truly cannot be reversed, say so explicitly in a comment and get review |
| One concern | One logical change per migration |
| Idempotent guards | `IF NOT EXISTS` on creates where safe |
| Locks | `CREATE INDEX CONCURRENTLY` on tables > 100k rows (and therefore **outside** a transaction — goose `--no-transaction`) |
| Data backfills | Separate migration, batched, restartable; large backfills run as a job, not a migration |
| Destructive changes | Never in one release. **Expand → migrate → contract**: |

```
Release N   : add the new nullable column; write to both; read from old
Release N+1 : backfill; read from new; still write both
Release N+2 : stop writing old; drop it
```

This is what makes rollback safe (§17.5 of ARCHITECTURE.md).

## 8. Queries (sqlc)

```sql
-- name: ListDueReviewCards :many
SELECT rc.id, rc.content_item_id, rc.stability, rc.difficulty, rc.due_at
FROM learn.review_cards rc
WHERE rc.user_id = $1
  AND rc.due_at <= $2
  AND rc.suspended_at IS NULL
ORDER BY rc.due_at ASC, rc.id ASC
LIMIT $3;
```

| Rule | Detail |
|---|---|
| Annotations | `:one`, `:many`, `:exec`, `:execrows`, `:batchexec` |
| Naming | Verb + subject: `GetDeckByID`, `ListDueReviewCards`, `CountUserDecks` |
| No `SELECT *` | Explicit columns, so adding a column cannot silently change a struct |
| No dynamic SQL | Need three filter combinations? Write three queries, or use `sqlc.narg` and `COALESCE` |
| Batch | Use `:batchexec`/`copyfrom` for bulk inserts (seeding, imports) |
| Always bounded | Every `:many` has a `LIMIT` |

## 9. Transactions

| Rule | Detail |
|---|---|
| Opened in | The service layer only |
| Isolation | `READ COMMITTED` default; `REPEATABLE READ` for read-modify-write on counters; document any use of `SERIALIZABLE` |
| Duration | Short. No LLM call, no HTTP call, no S3 upload inside a transaction |
| Locking | `SELECT … FOR UPDATE` on the aggregate root; `SKIP LOCKED` for queue-like scans |
| Retries | Serialisation failures (`40001`) retried up to 3 times with backoff |
| Scope | Never across modules (rule L4) — use the outbox |

## 10. Partitioning

Partition by month on `created_at` from day one for high-growth tables:

`learn.attempts` · `learn.review_logs` · `ai.ai_requests` · `audit.audit_logs` ·
`analytics.analytics_events`

| Rule | Detail |
|---|---|
| Creation | A monthly job creates the next three partitions in advance |
| Retention | Old partitions detached and archived per the retention table (ARCHITECTURE §8.5) |
| Indexes | Defined on the parent so partitions inherit them |
| Queries | Must include the partition key in the `WHERE` clause, or accept a full scan |

## 11. Performance budget

| Rule | Threshold |
|---|---|
| Any query in a request path | p95 < 50 ms |
| Any query anywhere | logged with its plan above 100 ms |
| CI performance test | fails above 500 ms on the seeded dataset |
| Connection pool | max 25 per API replica, 10 per worker; `pool_waiting` alerted |
| N+1 | Detected by an integration-test query counter around each service method |

## 12. Operations

| Concern | Approach |
|---|---|
| Backups | Nightly `pg_dump` + continuous WAL archiving to MinIO; 30-day retention |
| Restore drill | Monthly, timed, documented in `docs/operations/runbooks/restore.md` |
| Monitoring | `postgres_exporter`: connections, locks, replication lag, bloat, slow queries, cache hit ratio |
| Vacuum | Autovacuum tuned per hot table; monitored for wraparound risk |
| Extensions | `pgcrypto`, `pg_stat_statements`, `pg_trgm`, `btree_gin`, `citext` (case-insensitive email, created by `db/migrations/user/`); `pgvector` when semantic caching ships |
| Access | The app uses a least-privilege role; migrations use a separate owner role; no superuser at runtime |
| PII | Encrypted columns for MFA secrets and refresh-token hashes; anonymisation script for deleted users |
