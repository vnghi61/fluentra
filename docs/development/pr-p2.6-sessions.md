# feat(auth): sessions — list, revoke, logout [P2.6]

Branch `feat/auth-sessions`, cut from `origin/main` (`4aee3ab`). Depends on P2.5, which is merged.

Closes P2.6 in `docs/development/phase-1-plan.md` §5, except for the IP country — see below.

## What this adds

- `GET /api/v1/auth/sessions` — the devices this account is signed in on, with a coarse label, when
  the session started, when it was last used, and which one is the caller's.
- `DELETE /api/v1/auth/sessions/{id}` — sign one device out.
- `POST /api/v1/auth/logout` — sign out the device holding the token.
- `contract.SessionRevoker.RevokeAll` for `user` (account deletion) and `admin` (suspension).
  Published and unconsumed: neither module exists yet, so the integration suite is what proves it.

## The trap, and why the fix is structural

**Another account's session is 404, not 403.** A 403 confirms the id names a real session and turns
the operation into a way to enumerate them; a 404 says only that *the caller* has no such session,
which is all they are entitled to know and is equally true of an id that never existed.

The defence is not an `if`. The ownership lookup puts both the id and the owner in one WHERE clause:

```sql
SELECT ... FROM core.sessions WHERE id = $1 AND user_id = $2;
```

so "no such session" and "not yours" produce the identical empty result and there is no branch where
they can diverge. A malformed uuid takes the same path. The test compares the two RFC 9457 documents
member by member — `instance` excluded, since it only echoes the path the caller sent — rather than
trusting that they were built by the same code.

## Decisions worth a reviewer's attention

**The cache caches ownership, not liveness.** The card asks for a 5-minute session cache busted on
revoke, and the key in AGENT.md §12 is per session id. What is stored is which account owns the
session. That is safe because an owner never changes, so a stale answer cannot become a dangerous
one. Liveness *does* change, and caching it would let a revoked session be treated as live for a
whole TTL — so every revocation stays an `UPDATE … WHERE revoked_at IS NULL` and PostgreSQL remains
the only thing that decides. Entries are still dropped on revoke, and a negative answer is never
cached, because a session created a moment later would otherwise be refused for five minutes.

**Logout denylists the access token; revoking another device cannot.** Logout has the token in hand,
so its `jti` is known. Revoking some other device does not put us in possession of its token and
there is no id to deny — it stops working within one access-token lifetime, which is exactly what
the acceptance criterion asks for. `TokenService.RevokeNow` was added so a caller holding an actor
does not have to know the access TTL: the actor carries a token id but no expiry, and a caller that
guessed would either under-deny or over-deny.

**The denylist write fails open, and the reason it is safe to is new.** ADR-0007 already made the
*read* fail open. Here the write does too — but only because the durable half has already committed
to Postgres. An unreachable Redis costs one access token for at most its remaining fifteen minutes;
refusing the sign-out instead would show a learner a failure for something that has, in every lasting
way, happened. There is a test for it, and it asserts the session died anyway.

**Revocation ends the family before the session, in one transaction.** They commit together, so the
order does not decide correctness — but it means the unreachable state is the harmless one. A live
session with no renewable token expires quietly; a revoked session with a live family would renew
itself indefinitely.

**Device labels are derived at sign-in and stored.** They have to be: the column beside them holds a
digest of the user agent, and nothing can be recovered from that later. The table is ordered
most-specific-first, because every Chrome user agent also contains "Safari", Edge contains both, and
Chrome on iOS says "CriOS" — a matcher in the wrong order labels most of the web "Safari". A user
agent it cannot read produces no label rather than a guess: a learner deciding what to revoke can
reason about a blank, not about a confident lie. Real user-agent strings are pinned in the tests.

## What is deliberately not here

**No IP country.** The card's Do says "IP country from a local database" and its own acceptance says
"`ip_hash` is stored, never the address" — a country cannot be derived from an HMAC. Resolving one
needs it worked out at sign-in and stored in a column of its own, which means a migration (the card
lists no migration), `oschwald/geoip2-golang` in `DEPENDENCIES.md`, a `GEOIP_DATABASE_PATH` key not
in `.env.example`, and a MaxMind licence key wired into CI and Compose. That is an infrastructure
change with its own card, and it was deferred with the human's agreement. The `SessionSummary` schema
description says the field is absent and why — the same pattern handoff §2 decision 2 settled.

**No `POST /admin/users/{id}/sessions/revoke`.** It is in AGENT.md §6 and API.md, but the card names
only the three operations plus the contract method, and `admin` arrives in P4.1.

**No migration.** `core.sessions` already had every column this card reads; P2.5 created them.

## Scope beyond the card's Files list

The card names `auth/service/session.go`. Also touched, all inside `auth` except the last:

- `domain/device.go` — the label parser, pure and table-driven.
- `service/refresh.go` — `Start` now derives and stores the label; `CreateSession` gained a parameter.
- `service/token.go` — `RevokeNow`.
- `repository/session.go`, `repository/session_cache.go`, `db/queries/auth/sessions.sql`.
- `transport/http/` — three handlers, `requireActor`, and the list DTO. The package doc claimed every
  operation in it was unauthenticated; that stopped being true with this card.
- `tools/docgen/data/core.json` — the `SessionRevoker` row, so the generated block in `AGENT.md` is
  not hand-edited.

## Verification — real output

All in the Linux container per handoff §3.

```
==> Step 1: architecture lint on the clean tree
==> Step 2: the violation must not be a compile error
==> Step 3: go-arch-lint must reject the violation
==> Step 4: the tree is clean again
```

`.go-arch-lint.yml` is unchanged — no new package — and `verify-arch-lint.sh` runs the
deliberate-violation probe itself.

```
--- PASS: TestAnotherAccountsSessionIsANotFoundNotAForbidden (0.01s)
--- PASS: TestRevokingASessionStopsItsRefreshFamilyImmediately (0.01s)
--- PASS: TestTheSessionListShowsOnlyLiveSessionsOfTheCallersOwnAccount (0.02s)
--- PASS: TestLogoutDenylistsTheAccessTokenAndStopsTheFamily (0.01s)
--- PASS: TestLogoutStillEndsTheSessionWhenTheDenylistIsUnreachable (0.01s)
--- PASS: TestRevokeAllEndsEverySessionTheAccountHas (0.02s)
ok  github.com/fluentra/fluentra/internal/modules/auth  1.581s
```

Run with `-race`. The P2.5 rotation tests still pass alongside them.

- `make test`, `make test-int`, `make test-contract` — green.
- `golangci-lint v2.12.2` (CI's pin), default and `--build-tags=integration` — `0 issues.` both.
- `make cover-check` — `total coverage: 66.1% of hand-written code (minimum 60.0%)`, run alone per
  the known flake in handoff §3.
- `npx @stoplight/spectral-cli lint api/openapi/openapi.yaml` — `No results with a severity of
  'error' found!`
- `make docs-check` — `Documentation drift check passed.`, markdownlint `0 error(s)`.
- `make gen-check` and `make gen-check-web` — green after committing (handoff trap #4).

## Still carried, not fixed here

- `docs/development/HANDOFF-WP2.md` §1 remains stale — it describes `fix/openapi-login-example` and
  `feat/auth-jwt-middleware` as unpushed and `origin/main` as `aff70f2`. Everything through P2.5 is
  merged and `origin/main` is `4aee3ab`. Worth a one-line follow-up before the next agent reads it
  cold.
- `core.sessions` and `core.refresh_tokens` are never swept. Filed in the module TODO alongside the
  outbox-prune tension from handoff §6.
