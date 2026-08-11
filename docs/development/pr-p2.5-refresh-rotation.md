# feat(auth): refresh rotation with reuse detection [P2.5]

Branch `feat/auth-refresh-rotation`, cut from `origin/main` (`d3816b2`). Depends on P2.4, which is
already merged — this PR is not stacked on anything unpushed.

Closes P2.5 in `docs/development/phase-1-plan.md` §5.

## What this adds

`POST /api/v1/auth/refresh` exchanges the refresh cookie for a new access token and a new cookie.
Login and email verification now issue the first cookie, so a session outlives the fifteen-minute
access token without a second password prompt.

A refresh token is single use (BR-AUTH-04). **That is what makes theft detectable.** A stolen token
works exactly once; whoever presents it second — the thief or the real client — is presenting one
that has already been spent, and at that moment the server cannot tell which of the two is genuine.
So it trusts neither: the whole family is revoked, the session is revoked, a `refresh_reuse`
security event goes through the outbox to `audit`, and the caller gets `401 SESSION_REVOKED`.

## The trade-off a reviewer should push back on if they disagree

**The legitimate learner is signed out alongside the thief**, on every device sharing that session.
Two honest tabs refreshing at the same instant land there too — the loser of the race is
indistinguishable from a replay, so it is treated as one.

The alternative is a grace window in which a recently spent token is still accepted. That is a
window an attacker holding the stolen token simply refreshes inside, which is to say it is not an
alternative. OAuth 2.0 Security BCP §4.14.2 makes the same call. The cost is documented in the
operation's OpenAPI description so a client author designs around it rather than discovering it.

## The part that matters most

The claim is **one guarded UPDATE inside the transaction that writes the replacement**:

```sql
UPDATE core.refresh_tokens rt SET used_at = $now
FROM core.sessions s
WHERE rt.token_hash = $1 AND rt.used_at IS NULL AND rt.revoked_at IS NULL
  AND rt.expires_at > $now AND s.id = rt.session_id AND s.revoked_at IS NULL
RETURNING ..., s.user_id;
```

A read followed by a write passes every sequential test in this suite and, under two concurrent
refreshes with one token, issues two live tokens from one — the exact state reuse detection exists
to make impossible, reached by the server itself. So the reuse-detection and concurrency tests were
written **before** the implementation, and both run against real PostgreSQL under `-race`:

- `TestPresentingASpentRefreshTokenRevokesTheWholeFamilyAndTheSession` — the assertion carrying the
  weight is the last one: the *legitimate* token, issued moments earlier and never presented by
  anybody, must also stop working. A server that 401s the replay and keeps rotating the real
  client's token has told the attacker to try again and told nobody else anything.
- `TestTwoConcurrentRefreshesWithOneTokenProduceExactlyOneWinner` — eight callers, one token, one
  winner, seven `SESSION_REVOKED`, and **exactly one** security event.

## Two judgement calls

**1. `RefreshService` opens its transactions READ COMMITTED, not through `dbx.InTx`.**
`dbx.InTx` is SERIALIZABLE with three retries. Here the stronger level buys nothing — the claim's
correctness is a row predicate, which READ COMMITTED re-evaluates against the winner's committed row
before deciding it matched nothing — and it costs something real: every concurrent refresh of one
token and every concurrent revocation of one family would collide and burn retries, making the
authentication path least available during exactly the replay storm it exists to detect. Three
attempts is not a budget worth spending there.

The tidy home for this is an isolation-level option on `dbx.InTx`, which is a change to a shared
package and outside this card's file list, so it is a private helper here with the reasoning in its
doc comment and a TODO entry. **Say if you would rather I changed `shared/dbx` instead.**

**2. `RefreshCookie` in `openapi.yaml` is still referenced by nothing.**
`POST /auth/refresh` declares `security: []` like every other `auth` operation. Declaring
`security: [{RefreshCookie: []}]` would be more honest, but the `fluentra-operation-permission`
spectral rule then demands an `x-permission`, and there is no RBAC permission that fits a
cookie-authenticated operation. Closing this means amending the ruleset to exempt cookie-only
schemes; filed in the module TODO.

## Scope beyond the card's Files list

The card names `auth/service/refresh.go` and `db/migrations/auth/`. Also touched, all inside `auth`
except the last two:

- `service/login.go`, `service/register.go` — both now open a session through one `Sessions`
  interface instead of minting an access token directly, so there is exactly one place a refresh
  family is rooted. `LoginResult` and `Verification` embed `SignedIn`, so `result.Session` still
  reads as it did.
- `transport/http/` — the refresh handler, the cookie, and `Set-Cookie` on the two existing sign-in
  responses. Refresh has nothing to rotate unless something issues the first token, and FLOW.md's
  registration diagram already ended `200 { access_token } + refresh cookie`.
- `cmd/api` — `REFRESH_TOKEN_TTL` (already in `.env.example`; no new config key) threaded to the
  module, and `Secure` derived from `APP_ENV`.
- `tools/docgen/data/core.json` — the `auth.SecurityEvent` contract row, so the generated block in
  `AGENT.md` is not hand-edited.

## Schema — `1700000100`

`core.sessions` and `core.refresh_tokens`. `sid` has pointed at nothing since P2.4 deliberately put
it in the token so this migration would not change the token format; it now names a row.

- **Spent rows are kept, not deleted.** A deleted row and a token that never existed are
  indistinguishable, and that difference is the entire detection.
- `used_at` and `revoked_at` are separate: one was exchanged legitimately, the other was taken away,
  and the audit trail has to say which happened to each row in a burnt family.
- `token_hash` is a plain SHA-256, not an HMAC and not a password hash. The token is 256 bits from
  `crypto/rand`; there is no dictionary to attack and nothing a keyed digest or work factor buys.
- Sessions store a keyed digest of the client address and user agent, never the values.
- `idx_refresh_tokens_session` exists because `TestCoreSchema_EveryForeignKeyIsIndexed` in
  `db/migrations/user` caught the foreign key having no index behind it. Good test.

## Verification — real output

All in the Linux container per handoff §3; `go test -race` and `make arch` do not work on the
Windows host.

```
==> Step 1: architecture lint on the clean tree
==> Step 2: the violation must not be a compile error
==> Step 3: go-arch-lint must reject the violation
==> Step 4: the tree is clean again
```

`.go-arch-lint.yml` is unchanged — no new package — but `verify-arch-lint.sh` runs the
deliberate-violation probe itself and it passed.

```
--- PASS: TestPresentingASpentRefreshTokenRevokesTheWholeFamilyAndTheSession (0.09s)
--- PASS: TestTwoConcurrentRefreshesWithOneTokenProduceExactlyOneWinner (0.09s)
--- PASS: TestARefreshTokenOneMillisecondPastExpiryIsRefused (0.03s)
--- PASS: TestAnUnknownRefreshTokenRevokesNothing (0.04s)
--- PASS: TestRotationKeepsTheSessionAndMovesItsLastSeen (0.03s)
--- PASS: TestTheRefreshTokenIsNotStoredAndIsNotPrintedByAccident (0.01s)
ok  github.com/fluentra/fluentra/internal/modules/auth  2.260s
```

- `make test`, `make test-int`, `make test-contract` — green.
- `golangci-lint v2.12.2` (CI's pin), default and `--build-tags=integration` — `0 issues.` both.
- `make cover-check` — `total coverage: 66.0% of hand-written code (minimum 60.0%)`, run alone per
  the known flake in handoff §3.
- `npx @stoplight/spectral-cli lint api/openapi/openapi.yaml` — `No results with a severity of
  'error' found!`
- `make docs-check` — `Documentation drift check passed.`, markdownlint `0 error(s)`.
- `make gen-check` and `make gen-check-web` — green after committing (handoff trap #4).

## Not fixed here

- `docs/development/HANDOFF-WP2.md` §1 is stale — it says `fix/openapi-login-example` and
  `feat/auth-jwt-middleware` are unpushed and that `origin/main` is `aff70f2`. Both were merged
  (PRs #21 and #23) and `origin/main` is `d3816b2`. Left alone to keep this diff to the card; worth
  a one-line follow-up.
- Contract-tag `golangci-lint` reports pre-existing `goconst`/`lll` findings, including in
  `internal/modules/user/transport/http/contract_test.go` which this PR does not touch. CI lints
  only the default and integration tag sets.
- The `ops.outbox_events` retention tension from handoff §6 is unchanged and still filed.
