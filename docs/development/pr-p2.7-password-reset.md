# feat(auth): password reset and change [P2.7]

Branch `feat/auth-password-reset`, cut from `origin/main` (`f0fc715`). Depends on P2.6, merged.

Closes P2.7 in `docs/development/phase-1-plan.md` §5.

## What this adds

- `POST /api/v1/auth/forgot-password` — **always 202**, in comparable time, whether or not the
  address has an account.
- `POST /api/v1/auth/reset-password` — consume the code, set the password, revoke every session.
- `POST /api/v1/auth/change-password` — signed in, current password required, revoke every *other*
  session.

Mechanism is the existing challenge subsystem with `purpose = password_reset`, as handoff §7
directed. No migration: `core.auth_challenges` already had the purpose.

## The decision made before implementing

**The challenge TTL is now per purpose**, and this is the one thing I stopped and asked about.

P2.7's acceptance wants a thirty-minute reset window. The subsystem gave every purpose the same
ten minutes (`domain.ChallengeTTL`, no override on `IssueRequest`) and BR-AUTH-10 said so in
writing. Meeting the criterion meant widening a subsystem the card's Files list does not name and
adding a config key — two stop-and-ask triggers — so it was agreed first.

`Config.TTLByPurpose` overrides the default for the purposes that name one, `PASSWORD_RESET_TTL=30m`
is in `.env.example` and wired through `cmd/api`, and BR-AUTH-10 now reads "ten minutes unless the
purpose sets otherwise". An override of zero or less is ignored rather than honoured — a
misconfigured key must not issue challenges that expire on arrival.

## Enumeration safety, tested two ways

`forgot-password` for an unknown address writes a real challenge with a real code that is never
delivered. Same insert, same limiter calls, same body shape.

- **Structurally**: both paths return a live handle with the same purpose and attempts, and
  verifying the unknown one answers `OTP_INVALID` — not `CHALLENGE_NOT_FOUND`, which is the
  difference that would give the game away.
- **On the clock**: fifteen samples each, medians compared. `median known=4.19ms unknown=3.12ms`.
  The bound is deliberately loose (unknown must not be under a quarter of known). What it catches
  is the regression that actually happens — an early return, two orders of magnitude faster — and a
  tight bound on a shared database would flake on a busy neighbour rather than on a bug.

The residual difference is real and expected: the known path additionally reads the recipient and
writes an outbox row. That is roughly a millisecond against a 4ms baseline, and it is the price of
actually sending the email.

## Reset versus change

| | reset | change |
|---|---|---|
| Sessions | every one | every one **but this** |
| Current password | not held | **required** |
| Signs the caller in | no | already is |
| Refresh cookie | cleared | untouched |

`change-password` requires the current password even though the caller holds a valid token: the
token proves the session was *opened* by somebody who knew the password, not that whoever holds it
now does. Without it, a token from an unlocked laptop locks the owner out of their own account. An
account with no password — P2.10's Google-only accounts — gets the byte-identical refusal, because
telling those apart says how somebody signs in.

## Two orderings that are load-bearing

**A second request burns the first challenge**, in the same transaction as the new one. Burned by
spending the attempt budget, not by a new column: this table already derives "burned" from
`attempts >= max_attempts` (AGENT.md §5), and marking it `consumed_at` would claim somebody used it
when nobody did. Superseding is a separate `SupersedeIn` call rather than folded into `IssueIn`,
because a resent *verification* code deliberately keeps its challenge (BR-AUTH-13) and burning on
issue would break the resend cooldown's whole model.

**The password policy is checked before the code is consumed.** A learner whose first choice trips
the breach corpus can try another with the code they already have, instead of waiting on a second
email for a mistake they can fix in the form in front of them.

## A defect the tests caught

`domain.Code` is a `secret.Redacted[string]`. The first draft used `.String()` for the outbox
payload, which renders the redaction marker — **every reset email would have carried `[redacted]`
and no learner could ever have reset a password.** Three integration tests failed on it before it
went anywhere. `.Reveal()` is the one place the code is meant to escape, and there is now a comment
saying so, matching what `register.go` already does.

## Scope beyond the card's Files list

The card names `auth/service/password.go`. Also touched, all inside `auth` except the last two:

- `service/challenge.go` — `TTLByPurpose` and `SupersedeIn` (the agreed widening).
- `service/session.go`, `repository/session.go`, `db/queries/auth/sessions.sql` —
  `RevokeAllExcept`, which a change needs and P2.6 had no caller for.
- `repository/challenge.go`, `db/queries/auth/challenges.sql` — the burn query.
- `contract/contract.go` — `auth.password_reset_requested` and `auth.password_changed`.
- `module.go` — the service, and the mailer consumer for the reset topic.
- `cmd/api` — `PASSWORD_RESET_TTL`.
- `docs/development/HANDOFF-WP2.md` — see below.

## Verification — real output

```
==> Step 1: architecture lint on the clean tree
==> Step 2: the violation must not be a compile error
==> Step 3: go-arch-lint must reject the violation
==> Step 4: the tree is clean again
GATE=PASS
```

```
--- PASS: TestForgotPasswordAnswersTheSameWayForAnAddressWithNoAccount (0.09s)
--- PASS: TestASecondResetRequestKillsTheFirstCode (0.03s)
--- PASS: TestResetRevokesEverySessionAndTheNewPasswordIsTheOnlyOneThatWorks (0.04s)
--- PASS: TestAResetCodeIsSingleUseAndDiesAtThirtyMinutes (0.03s)
--- PASS: TestChangePasswordKeepsThisDeviceAndSignsOutTheRest (0.05s)
--- PASS: TestChangePasswordRefusesTheWrongCurrentPasswordAndChangesNothing (0.02s)
    password_integration_test.go:480: median known=4.194747ms unknown=3.12482ms
--- PASS: TestForgotPasswordTakesComparableTimeForAKnownAndUnknownAddress (0.12s)
```

Run with `-race`. All P2.5 and P2.6 integration tests still pass alongside.

- `make arch`, `make test`, `make test-int`, `make test-contract` — green (`GATE=PASS`).
- `golangci-lint v2.12.2` (CI's pin), default and `--build-tags=integration` — `0 issues.` both.
- `make cover-check` — `total coverage: 66.4% of hand-written code (minimum 60.0%)`, run alone.
- `npx @stoplight/spectral-cli lint api/openapi/openapi.yaml` — `No results with a severity of
  'error' found!`
- `make docs-check` — drift passed, markdownlint `0 error(s)`.
- `make gen-check`, `make gen-check-web` — green after committing.

One note on the run: the first full-gate attempt failed everywhere with
`hostname resolving error: lookup fluentra-p14-pg`. The test containers had stopped, not the code.
`docker start fluentra-p14-pg fluentra-p14-redis fluentra-p14-minio` (not recreate — the network and
volumes survived) and everything passed.

## Housekeeping folded in

`docs/development/HANDOFF-WP2.md` §1 was three cards stale: it described `fix/openapi-login-example`
and `feat/auth-jwt-middleware` as unpushed and `origin/main` as `aff70f2`, when everything through
P2.6 was merged. I flagged it on the last two PRs without a steer and have now rewritten it, plus
added one trap to §3 that cost a round trip in both P2.5 and P2.6: **the tables in a module's
`AGENT.md` are generated from `tools/docgen/data/core.json`** — hand-editing them passes every test
and then fails `make docs-check` with "would change".

## Carried, not fixed here

- **Two topics now put a live code in an outbox payload**, and the reset one is worth something for
  thirty minutes rather than ten. `ops.outbox_events` still keeps published rows forever. The fix
  belongs in `shared/outbox` and is now twice as worth doing.
- **`auth.password_changed` is published and nothing consumes it.** The mailer has no
  `password_changed` template. A change the learner did not make is the first sign of a takeover and
  the notification is often the only way they find out — the event is in place so the consumer is a
  small card.
- **`PASSWORD_RESET_TTL` is read by `cmd/api` and not `cmd/worker`.** Nothing in the worker issues a
  challenge, so it does not matter today.

All three are in `internal/modules/auth/TODO.md`.
