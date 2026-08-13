# Handoff — WP3 (P3.1 → P3.3)

> **Revision 1, 2026-08-13.** WP0, WP1 and WP2 are all merged and their gates are proven.
> This file is written to be read cold, by an agent with no memory of the sessions that produced
> the current state.

Read `/AGENT.md` first. Then this. Then the card you are working on in
`docs/development/phase-1-plan.md` §6. `HANDOFF-WP2.md` is still worth reading for §3 (environment
traps) and §4 (patterns) — both still apply, and neither is repeated in full here.

---

## 1. Exact state right now

**Everything through P2.10 is merged into `main`.** WP2 is closed. The next card is **P3.1 — avatar
upload**, and `docs/development/phase-1-plan.md` §1.2 does not fix a branch name for WP3 cards, so
follow the same shape: `feat/user-avatar-upload`.

| | |
|---|---|
| `origin/main` head | `8e03444` — merge of PR #29 (P2.10) |
| Highest migration timestamp | `1700000120`. The next new migration is `1700000130` |
| Coverage | 65.8% of hand-written code, gate is 60.0% |

### The gates, and what proves them

Both are proven by tests that were re-run on `main`, not by assertion:

| WP | Proven by |
|---|---|
| WP1 | `TestModule_RegistrationIsAtomic`, `TestModule_ProfileUpdateIsServedAndRecorded`, `TestModule_PreferencesRoundTripThroughHTTP`, `TestMyPermissionsReflectsRealRoles`, `TestAdminHoldsEverythingAndLearnerHoldsNothing`, `TestLearnerIsRefusedByTheAdminGroup`, and the three audit-trail tests in `cmd/api` (`TestProfileUpdateReachesTheAuditTrail`, `TestPreferenceReplacementIsAuditedToo`, `TestRoleChangeIsAudited`) |
| WP2 | `TestRegisterThenVerifyLeavesTheLearnerSignedIn` (leg 1), the five Google branch tests in `oauth_integration_test.go` (leg 2), `TestPresentingASpentRefreshTokenRevokesTheWholeFamilyAndTheSession` (leg 3), `TestAContinuouslyActiveSessionStillDiesAtTheAbsoluteCap` and `TestRotationMovesTheIdleWindowButNeverPastTheCap` (leg 4) |

Leg 1 of the WP2 gate was the last thing added, on `test/auth-registration-journey`. Everything else
was in place when P2.10 merged.

---

## 2. What WP3 walks into — read this before planning a card

Four things are already true and will save a session each.

### 2.1 `platform/storage` has more than the cards assume

`internal/platform/storage` already ships `PresignPut`, `PresignGet`, `Stat`, `Copy`, `Delete`,
`BuildKey`, and — the one worth knowing about — **`VerifyUpload`, which sniffs magic bytes** and
compares them against the declared content type. P3.1's acceptance criterion "a renamed `.exe` is
rejected by magic-byte sniffing" is therefore mostly a matter of calling it, not of writing it.

Read `internal/platform/storage/AGENT.md` before writing anything that touches an object.

### 2.2 The bucket constants and the config keys disagree — P3.1 hits this immediately

`storage.DefaultBuckets()` declares:

```
BucketAvatars = "fluentra-avatars"
BucketMedia   = "fluentra-media"
BucketExports = "fluentra-exports"
```

`.env.example` declares:

```
S3_BUCKET_MEDIA=fluentra-media
S3_BUCKET_UPLOADS=fluentra-uploads
S3_BUCKET_DERIVED=fluentra-derived
S3_BUCKET_EXPORTS=fluentra-exports
```

**There is no `S3_BUCKET_AVATARS` key, and `uploads`/`derived` have no constants.** P3.1 needs an
avatars bucket, so it needs a config key that does not exist — which is a **stop and ask**, not a
thing to invent (`/AGENT.md` §10, last bullet). Decide with the human whether the avatar flow uses
`avatars` (add the key) or `uploads` + `derived` (add the constants), and record it. Do not resolve
it by writing whichever one makes the code compile.

### 2.3 P3.1 needs a new dependency, which means an ADR conversation

There is **no image library in `go.mod`** — nothing that decodes JPEG/PNG, strips EXIF, or encodes
WebP. Go's standard library has `image/jpeg` and `image/png` but no WebP encoder.

Rule L12 requires a row in `DEPENDENCIES.md` with rationale and alternatives considered, and
`/AGENT.md` §10 forbids introducing a framework without an ADR. Resizing and re-encoding images is a
big enough choice — pure-Go (`golang.org/x/image`, `disintegration/imaging`) versus a cgo binding to
libvips (`h2non/bimg`) is a build-complexity and deployment decision, not a preference — that it
should be agreed before the card starts, not discovered in review.

### 2.4 `auth` does not consume `user.deletion_requested`, though its docs say it does

`internal/modules/auth/AGENT.md` §4 lists:

| Event | Direction | Payload summary |
|---|---|---|
| `user.deletion_requested` | consumes | Revoke all sessions and credentials immediately |

**It does not.** `auth.Module.Subscribe` registers three topics, all of them mail:
`auth.verification_requested`, `auth.registration_attempted`, `auth.password_reset_requested`. The
machinery it would need exists and is tested — `contract.SessionRevoker.RevokeAll` is published and
has an integration test (`TestRevokeAllEndsEverySessionTheAccountHas`) — but **nothing calls it**,
which the contract's own doc comment says outright.

That is P3.3's most load-bearing dependency and it is a documentation-versus-reality gap, so it will
not show up as a compile error. Two things follow:

- P3.3 must add the subscription **and** the purge handler, in `auth`, as part of the card. That is
  a second module beyond `user`, which the card's Files list does not name — flag it rather than
  doing it silently.
- Whoever fixes it should correct the AGENT.md row at the same time, or the next agent believes it
  twice.

`audit` already consumes both `user.deletion_requested` and `user.deleted`, and `rbac` already has
`ForgetUser` reacting to `user.deleted` with an integration test. So of the three modules holding
personal data, two are ready and `auth` is the one that is not.

---

## 3. Decisions already made — do not relitigate

| # | Decision |
|---|---|
| 1 | **Deletion is not a cascade from `user`.** Each module erases its own data in response to `user.deleted`. The card says so and the architecture exists to prevent the alternative. `rbac.ForgetUser` is the pattern to copy. |
| 2 | **Cross-module reads go through `contract`, including for the export.** P3.2 assembles the ZIP from each module's contract, never by reading its tables (rule L2). |
| 3 | **Events cross module boundaries, transactions do not** (rule L4). Every WP2 card that tried to open one transaction across two modules ended up splitting it; the reasoning is in `registerNew`'s doc comment and in `OAuthService.createAccount`'s. |
| 4 | **`user_deletion_requests` and `user_exports` are already in `user/AGENT.md`'s `tables:` front matter.** They are specification, not schema — no migration creates them yet. Front matter listing a table that does not exist passes `check-drift.mjs`; the reverse does not. |

---

## 4. Environment — unchanged from WP2, with one addition

`HANDOFF-WP2.md` §3 is still accurate in full. The short version:

- **Never run `make check`** — it reformats ~37 unrelated files with two formatters CI does not
  enforce. Use individual targets or `make ci`.
- **`make arch`, `go test -race` and the integration suite run in the Linux container**, not on the
  Windows host. The command is in `HANDOFF-WP2.md` §3.
- **The `fluentra-p14-*` containers stop themselves.** `hostname resolving error: lookup
  fluentra-p14-pg` means exactly that: run `docker start fluentra-p14-pg fluentra-p14-redis
  fluentra-p14-minio`. Start, never recreate. This was mistaken for a real test failure once.
- **Lint with the CI pin**: `golangci-lint v2.12.2`, default tags **and**
  `--build-tags=integration`. Both must be 0 issues.
- **Generated doc tables come from `tools/docgen/data/core.json`.** Hand-editing them passes every
  test and then fails `make docs-check` with "would change". Edit the JSON, run `make docs`. Editing
  the JSON with a script that re-serialises it will reformat all 30 modules — make the edit
  textually.
- **`make gen-check` and `make gen-check-web` report "stale" for correct-but-uncommitted output.**
  Commit first, then re-run.

**New for WP3:** the storage suite needs MinIO, so the container run needs `TEST_S3_ENDPOINT`,
`TEST_S3_ACCESS_KEY` and `TEST_S3_SECRET_KEY` — they are already in the documented command, and
until now nothing in WP1 or WP2 read them. If a storage test hangs rather than failing, check
`fluentra-p14-minio` is up before debugging the test.

---

## 5. What each card's tests must prove

Written the way the WP2 traps were, because the same failure mode applies: a criterion with no named
test is a criterion nobody checks.

### P3.1 — avatar upload

- **A renamed executable is refused.** Upload bytes whose magic number is not an image, with
  `Content-Type: image/png`, and assert the refusal. `storage.VerifyUpload` does the sniffing; the
  test is that the service calls it and acts on the answer.
- **EXIF GPS data is absent from the output.** Take a fixture with GPS tags in it, run it through,
  and assert the tag is gone from the derived object. Asserting "we called a strip function" is not
  the same test.
- **The old object is deleted only after the new one is verified.** Order matters: a failure between
  the two must leave the learner with their old avatar, not with none.
- **The size and type limits are enforced server-side**, not only in the presign parameters. A
  presigned PUT is a URL somebody can reuse with different bytes.

### P3.2 — data export

- **A second request while one is pending is 409.** One export at a time, per account.
- **The link expires.** Assert against the signed URL's own TTL, not against a comment.
- **The export contains data from every module that holds personal data**, gathered through
  contracts. The test that earns its place is the one that fails when a new module starts holding
  personal data and is not added — a list checked against something, rather than a hard-coded set.
- **The job is restartable.** Kill it mid-way and re-run; the artefact must be correct and not
  doubled.

### P3.3 — account deletion

- **Sessions die immediately on request**, before the 30-day grace begins. That is the `auth`
  subscription from §2.4 — and the test is that a token issued before the request stops working,
  not that a function was called.
- **Cancellable before the deadline, irreversible after.** Both halves, with the clock moved.
- **Every module holding personal data has an idempotent purge handler.** Run each twice and assert
  the second is a no-op rather than an error.
- **Aggregate statistics survive anonymisation.** Whatever counts the product keeps must still be
  countable after the PII is gone.

---

## 6. Open items carried into WP3

From `internal/modules/auth/TODO.md`, in the order they are likely to bite:

- **`user.Registrar` cannot open an account already verified.** Google sign-in creates an account in
  three writes that cannot be one transaction; a failure between them refuses that learner's Google
  sign-in for seven days. The fix is a `Verified` field on `user/contract.NewUser` — which is a
  `user` module change, so **WP3 is the natural place to do it**.
- **`ops.outbox_events` never deletes published rows**, and two payloads carry a plaintext OTP code
  bounded only by the code's ten-minute life. The fix belongs in `shared/outbox`. P3.2 and P3.3 both
  add payloads carrying personal data, so this stops being a curiosity and starts being an erasure
  problem: **an export or deletion event in the outbox outlives the deletion it describes.**
- **Nothing sweeps expired refresh tokens, sessions, trusted devices, or consumed OAuth states.**
  Four filings of one card.
- **A suspended account is caught by address, not by id** in the Google callback, because
  `service.Accounts` has no lookup by id. Also a `user` contract change.

---

## 7. Stop and ask the human when

Unchanged from WP2, and WP3 trips the first two immediately:

- you need to touch a module the card does not name — **P3.3 needs `auth`**;
- you need a config key that is not in `.env.example` — **P3.1 needs an avatars bucket**;
- you need a new dependency — **P3.1 needs an image library**;
- a rule in `/AGENT.md` §5 blocks the obvious solution;
- or the card's acceptance criteria depend on something that does not exist yet.

Asking cost one round trip in every WP2 card that did it. Guessing cost a rewrite in the one that
did not.
