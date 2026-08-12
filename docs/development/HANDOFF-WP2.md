# Handoff — P2.5 through P2.10

> **Revision 3, 2026-08-12.** Everything through P2.7 is merged. §1 below is current; the rest of
> the file is still accurate and is where the traps and patterns live. Read all of it.

This file is the single source of truth for the handoff. It is written to be read cold, by an
agent with no memory of the sessions that produced the current state.

Read `/AGENT.md` first. Then this. Then the card you are working on in
`docs/development/phase-1-plan.md` §5.

---

## 1. Exact state right now

**Everything through P2.7 is merged into `main`.** The next card is **P2.8 — rate limiting**, branch
`feat/auth-rate-limiting`.

| | |
|---|---|
| `origin/main` head | see `git log origin/main`; P2.7 was PR #26 |
| Highest migration timestamp | `1700000100`. The next new migration is `1700000110` |

Nothing is unpushed. The credential problem described in earlier revisions is over: branches push
normally over HTTPS, and the only thing an agent still cannot do is open the pull request — `gh` is
read-only on `vnghi61/fluentra`, so write the PR body to a file under `docs/development/` and give
the human the `gh pr create --body-file …` command. There are worked examples in
`pr-p2.5-refresh-rotation.md`, `pr-p2.6-sessions.md` and `pr-p2.7-password-reset.md`.

### What P2.5, P2.6 and P2.7 landed

- **P2.5** — `core.sessions` and `core.refresh_tokens`, rotation, and reuse detection. The claim is
  one guarded `UPDATE … WHERE used_at IS NULL` inside the transaction that writes the replacement;
  a read-then-write passes every sequential test and issues two live tokens under concurrency.
  Presenting a spent token burns the family, revokes the session and raises `refresh_reuse`.
- **P2.6** — the session list, `DELETE /auth/sessions/{id}`, `POST /auth/logout`, and
  `contract.SessionRevoker.RevokeAll`. Another account's session is 404 and not 403, structurally:
  the ownership lookup puts the id and the owner in one `WHERE` clause.
- **P2.7** — `forgot-password` / `reset-password` / `change-password`, on the challenge subsystem
  with `purpose = password_reset`. The challenge TTL is now per purpose.

Three things a reviewer should know were deliberate:

- **`RefreshService` and `SessionService` open READ COMMITTED transactions** through private
  helpers rather than `dbx.InTx`, which is SERIALIZABLE with three retries. Their correctness is a
  row predicate, not a snapshot, and SERIALIZABLE would burn retries colliding on exactly the rows
  a replay storm hammers. An isolation option on `dbx.InTx` is the tidy fix and is filed.
- **No IP country on the session list.** It cannot be derived from `ip_hash` and needs a GeoIP
  database, a config key and a MaxMind licence key in CI. Deferred with the human's agreement; the
  schema description says so.
- **BR-AUTH-10 was amended**: ten minutes unless the purpose sets otherwise, and `password_reset`
  sets thirty.

### What P2.4 landed

`service/token.go` (HS256, `sub`/`sid`/`role`/`jti`/`iat`/`exp`/`aud`/`iss`, no PII, two-key
rotation, 60 s leeway, signing method pinned so `alg: none` cannot verify) · `repository/denylist.go`
over Redis, failing open per ADR-0007 · `transport/http/middleware.go` placing `httpx.Actor` in the
request context · login and email verification both returning an `AuthSession`.

Three things it also corrected, which a reviewer should know were deliberate:

- **`EMAIL_NOT_VERIFIED` is now 403, not 401**, and account suspension is `ACCOUNT_SUSPENDED` 403
  rather than reusing `ACCOUNT_LOCKED` (which means "too many attempts" and clears itself). P2.3
  had both contradicting this module's own `AGENT.md` §12.
- **`cache.ErrMiss`** was added to `platform/cache`, which had been leaking `redis.Nil` as its miss
  sentinel — forcing callers to import a vendor they are not allowed to import. It wraps rather
  than replaces. This touched a module outside P2.4's file list, knowingly.
- The JWT parser was validating `exp` against `time.Now()` while the service issued against the
  injected clock. `jwt.WithTimeFunc` fixes it.

### What is deferred into the cards below

- **No refresh token and no cookie** — P2.5. `AuthSession`'s description in
  `api/openapi/components/auth.yaml` says so.
- **No `/auth/logout`** — P2.6. `TokenService.Revoke` and the denylist are built and tested;
  nothing calls them yet.
- **`sid` identifies nothing** — it is a fresh identifier per login, in the token from the start so
  that P2.6 adding `core.sessions` does not change the token format and invalidate everything
  issued before the deploy.

`internal/modules/auth/TODO.md` has all of this under "Open after P2.4", with the older cards'
open items above it.

---

## 2. Decisions already made — do not relitigate these

The human answered these explicitly. They are settled.

| # | Decision |
|---|---|
| 1 | **Two transactions, not one.** The card says user + credential + challenge + outbox in one transaction. `user.Registrar` opens its own and rule L4 forbids a transaction spanning two modules. So: `user` creates the account in its transaction; `auth` writes the credential, the challenge and the outbox row in a second one. A failure in the second leaves an unverified account, which the seven-day sweep removes and which a retry completes. `registerNew`'s doc comment explains this — keep it. |
| 2 | ~~Verification returns no tokens.~~ **Superseded by P2.4**, which added them as planned. Kept here because the pattern is the one to reuse: when a card's acceptance criteria depend on something a later card builds, say so **in the schema description** so a client reading the spec is not surprised when the field appears. |
| 3 | **`user/contract` gained `FindByEmail`** so registration can be enumeration-safe. Approved. |
| 4 | **The seven-day sweep is in P2.2**, not split out. It is an acceptance criterion of this card. |

Two further design corrections were made mid-implementation and are already in the branch:

- **`core.auth_challenges` gained `user_id`** (`1700000070`). P2.1b omitted it on a privacy
  argument that does not survive contact with verification: `subject_hash` is a keyed HMAC and is
  irreversible, so without the column nothing can find the account whose address was proved. The
  migration comment states this.
- **`user/contract` gained `Recipient`** (email, display name, locale). Resend needs an address and
  the challenge row stores only a digest. Flagged to the human as a judgement call; not objected to.

---

## 3. Environment traps — every one has already cost a session

1. **Never run `make check`.** It calls `make fmt`, which runs `goimports-reviser` and prettier and
   reformats ~37 unrelated files. CI enforces neither. Use individual targets, or `make ci`.
2. **`go test -race` does not work on the Windows host.** ThreadSanitizer fails with
   `error code: 87` on unrelated packages. Run the race suite in the Linux container.
3. **`make arch` must run in the Linux container.** go-arch-lint v1.17 has two Windows/Linux
   divergences: `**` globs match nothing on Windows, and the `deepScan` linter (method calls and
   dependency injections) does not report on Windows. deepScan reads `module.go` passing
   `*service.Service` into `NewHandler` as an edge and checks it against the **receiving**
   component's `mayDependOn`. So `m_auth_http` **must** list `m_auth_service` even though the
   handler imports nothing from `service/`. `m_user_http`, `m_rbac_http` and `m_audit_http` all do.
   **After changing `.go-arch-lint.yml`, probe it**: add a deliberately illegal import, confirm
   go-arch-lint names the component, then remove it. A rule that matches nothing passes silently.
4. **`make gen`, not `make gen-backend`,** when `openapi.yaml` changes. `gen-check` only covers Go;
   `web/src/types/api.ts` has its own gate, `make gen-check-web`. Both report "stale" when the file
   is correct but uncommitted — commit, then re-run.
5. **Migration timestamps are global and goose has out-of-order disabled.** Take a number larger
   than every existing one. Current max is `1700000100`.
6. **`gh` has read-only access.** Login is `vppos`, read-only on `vnghi61/fluentra`. You **cannot**
   create a PR. Write the PR body to a file and give the human the
   `https://github.com/vnghi61/fluentra/pull/new/<branch>` URL plus a `gh pr create --body-file …`
   command to run themselves.
7. **Coverage gate** ignores `internal/generated/` and `*.gen.go`. `COVERAGE_MIN = 60.0`, currently
   ~69.5%.
8. Small things: write enums as bare `CREATE TYPE`, never inside `DO $$` (sqlc cannot see inside
   and emits `interface{}`). `from`/`to` are SQL keywords — do not use them as sqlc parameter
   names. `tools/docgen/check-drift.mjs` regex-matches `CREATE TABLE` inside comments and strings,
   so never write those words in a migration comment. markdownlint MD049 wants `_italic_`, not
   `*italic*`. Display names have an anti-impersonation deny-list (`admin`, `support`, `staff`,
   `fluentra`, …) — a fixture using one gets a 422 from `CreateUser`.
9. **The tables in a module's `AGENT.md` are generated.** §4 (contract) and §9 (business rules) sit
   inside `<!-- BEGIN GENERATED -->` markers and are written from `tools/docgen/data/core.json`.
   Hand-editing them passes every test and then `make docs-check` fails with "would change". Edit
   the JSON and run `make docs`. This cost a round trip in both P2.5 and P2.6.

10. **`generate.mjs --check` does not compare front-matter.** It reports "0 files written" while the
   `tables:` list it owns has drifted from `tools/docgen/data/core.json`. `check-drift.mjs` catches
   it only when a migration creates a table the front-matter does not list. If you add a table,
   check both.

### The container

Docker Desktop must be running.

```bash
docker network create fluentra-p14
docker run -d --name fluentra-p14-pg --network fluentra-p14 -e POSTGRES_PASSWORD=postgres -e POSTGRES_USER=postgres -e POSTGRES_DB=fluentra_test postgres:17-alpine
docker run -d --name fluentra-p14-redis --network fluentra-p14 redis:7.4-alpine
docker run -d --name fluentra-p14-minio --network fluentra-p14 -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin minio/minio:latest server /data
```

```bash
MSYS_NO_PATHCONV=1 docker run --rm --network fluentra-p14 \
  -v fluentra-gocache:/root/.cache/go-build -v fluentra-gomod:/go/pkg/mod \
  -v "C:/Users/HP/Downloads/code/fluentra:/src" -w /src -e GOFLAGS=-buildvcs=false \
  -e TEST_DATABASE_URL='postgres://postgres:postgres@fluentra-p14-pg:5432/fluentra_test?sslmode=disable' \
  -e TEST_S3_ENDPOINT='fluentra-p14-minio:9000' -e TEST_S3_ACCESS_KEY=minioadmin -e TEST_S3_SECRET_KEY=minioadmin \
  golang:1.26 sh -c 'make arch && make test && make test-int && make test-contract && make cover-check'
```

`MSYS_NO_PATHCONV=1` is required. Keep the named volumes — a cold build makes tests flaky.

**Known flake:** `make cover-check` fails when run as the last step immediately after
`arch`+`test`+`test-int`+`test-contract` in one container, and passes when run alone. The coverage
`go test` itself exits 0 with no `FAIL` line either way. This matches `job`/`worker` integration
flakiness under load already known on `main`. Re-run it alone; do not chase it.

---

## 4. Patterns to follow — do not invent alternatives

**Module layout.** `contract/` (the only package other modules may import) · `domain/` (pure, no
I/O) · `repository/` (takes `dbx.Querier`, has `WithTx(pgx.Tx)`) · `service/` (declares its own
`Repository` interface — consumer-defined) · `transport/http/` · `module.go` (`New(Deps)`).

**`module.go` usually needs two adapters** because Go has no covariant returns: a `repositoryAdapter`
for `WithTx`, and an `outboxWriter` for `EventWriter`. Copy from `internal/modules/user/module.go`.

**Consumer-defined interfaces when importing is forbidden.** `audit` needs `rbac` to guard its
endpoints but may not import it; it declares `Guard interface { Require(ctx, permission string) error }`
and `cmd/api` adapts. Use the same shape for any dependency cycle.

**Events go through `shared/outbox` in the same transaction as the business write** (rule L4).
Constants in `contract` are declared **fully qualified** (`auth.security_event`); the writer strips
the aggregate prefix and `Event.Topic()` reassembles it. Payload field-name convention, which
`audit` reads structurally without importing anything: `occurred_at`, `actor_id`, `user_id`,
`changed_fields`, `severity`. **Send field names, not values.**

**Actor** comes from `httpx.ActorFrom(ctx)`. There is **no auth middleware until P2.4** — every
authenticated endpoint returns 401 to a real client until then; tests call `httpx.WithActor(...)`.
`httpx.ClientIP(ctx)` has no public setter on purpose; drive it through
`httpx.NewClientIPResolver(nil).Middleware` in tests, as `auth/service/challenge_test.go` does.

**Integration tests** create their own database in `TestMain` (`CREATE DATABASE` + migrate + drop).
Mandatory — sharing `TEST_DATABASE_URL` makes the suite flaky. Copy `TestMain` from
`internal/modules/auth/repository/integration_test.go`.

**Contract tests**: `//go:build contract`, named `TestContract*`, validate real responses against
`api/openapi/openapi.bundle.yaml` with `kin-openapi`. `make test-contract`.

**sqlc**: one entry per module in `sqlc.yaml`; `schema` must list every migration directory in the
DDL chain (auth FKs to `core.users`, so it lists `db/migrations/user` too). Keep `RETURNING` column
order identical to the table's physical column order or sqlc emits a per-query Row type instead of
reusing the model.

---

## 5. What P2.2's tests must prove

The card's acceptance criteria, each needing a named test:

- A **rolled-back registration sends no code.** Force a failure after the credential write and
  assert no `ops.outbox_events` row and no challenge.
- **Registering an already-verified address is indistinguishable** from a fresh registration: same
  status, same body shape, and — critically — `POST verify` on the returned `challenge_id` returns
  `OTP_INVALID`, **not** `CHALLENGE_NOT_FOUND`. That difference is the enumeration oracle the whole
  path exists to close. The service issues a real challenge nobody is ever given the code for.
- **Registering an unverified address replaces the credential.** Assert the *new* password verifies
  and the old one does not. This is the account-takeover path: without it, whoever claims an
  address first keeps the password after its real owner verifies.
- **Successful verification marks the address verified** (`email_verified_at` set).
- **Unverified accounts are swept after seven days**, and verified ones are not, and suspended ones
  are not.
- The **code appears in no response body**. `auth/service/challenge_test.go` already has
  `TestTheCodeNeverReachesTheLogs`; extend the same technique to the HTTP layer.
- A **mailer outage does not fail registration** — the email goes through the outbox, so assert the
  outbox row exists and the response is 201 even with the sender erroring.

Coverage target for this module is elevated: **90% domain, 85% service.**

---

## 6. Open issue to carry, not to fix here

`auth.verification_requested`'s outbox payload **carries the six-digit code and the email address in
plaintext**. There is nowhere else for them: the challenge row stores only an HMAC, so by the time
the consumer runs the code cannot be recovered from anything else. The exposure ought to be bounded
by the code's ten-minute life — but `ops.outbox_events` marks published rows with `published_at`
and **never deletes them**, so the row outlives its usefulness indefinitely.

That is a real tension with BR-AUTH-10. The fix belongs in `shared/outbox` (prune published rows,
or null their payload), which is a card of its own. Record it in
`internal/modules/auth/TODO.md` and in the PR body. Do not widen P2.2 to fix it.

---

## 7. The rest of WP2, after P2.2

One card = one PR, branch cut from `origin/main` (not local `main`, which goes stale).
Branch names are fixed by `docs/development/phase-1-plan.md` §1.2. Commit subject
`<type>(<scope>): <subject>` with footer `Refs: <task id>`; PR title adds `[<task id>]`.

**Every WP2 card requires two reviewers (rule S11).**

| Card | Branch | The thing most likely to go wrong |
|---|---|---|
| P2.3 login, lockout, timing | `feat/auth-login-lockout` | The **dummy Argon2id verify for an unknown email is not optional** — without it a timing side channel enumerates the whole user base. The acceptance test measures unknown-email vs wrong-password over 100 samples and asserts the distributions are indistinguishable. Per-account and per-IP counters must be **independent**. Redis down must not prevent login. |
| P2.4 JWT + middleware | `feat/auth-jwt-middleware` | Two-key rotation (`JWT_SIGNING_KEY` + `JWT_PREVIOUS_KEY`). No PII in claims. `TOKEN_EXPIRED` and `TOKEN_INVALID` must be distinguishable by the client. **This is also where `VerifiedChallenge` gains its tokens — go back and finish P2.2's deferred half.** |
| P2.5 refresh rotation | `feat/auth-refresh-rotation` | **Write the reuse-detection test before the implementation.** Presenting a used token revokes the entire family and raises a security event. Two concurrent refreshes with one valid token: exactly one succeeds. |
| P2.6 sessions | `feat/auth-sessions` | Another user's session is a **404, not a 403**. Store `ip_hash`, never the address. |
| P2.7 password reset | `feat/auth-password-reset` | `forgot-password` **always** 202, in comparable time for known and unknown addresses. Reuses the challenge subsystem — `purpose = password_reset`, already built. Reset revokes every session. |
| P2.8 rate limiting | `feat/auth-rate-limiting` | Redis down degrades to **allow-with-warn**, never deny-all. The per-IP challenge cap must block a script issuing challenges against many different addresses. |
| P2.9 persistent sign-in | `feat/auth-persistent-sessions` | **Write the absolute-cap test before the sliding logic** or you will ship sliding-with-no-cap and not notice — the happy path is identical. Admin accounts get 12 h idle / 7 d absolute and must **not** share a code path with the learner windows. |
| P2.10 Google OAuth | `feat/auth-google-oauth` | **Test the unverified-local-match branch first.** A Google email matching an unverified local account must be refused (409), never auto-linked — auto-linking there is the account-takeover path this ADR exists to prevent. PKCE is mandatory. JWKS cached, refreshed on unknown `kid`, never fetched per request. |

The WP2 gate (§1.5): register → OTP → auto sign-in works; Google sign-in works for all five
linking branches; refresh reuse revokes the family; a session survives a browser restart but dies
at the absolute cap. Integration tests prove all four.

### Stop and ask the human when

- you need to touch a module not named in the card,
- you need a config key that is not in `.env.example`,
- a rule in `/AGENT.md` §5 blocks the obvious solution,
- or the card's acceptance criteria contradict a dependency that does not exist yet.

P2.2 hit all four. Asking cost one round trip; guessing would have cost a rewrite.
