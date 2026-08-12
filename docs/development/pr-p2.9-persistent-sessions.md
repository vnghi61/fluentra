# feat(auth): persistent sign-in — sliding window + trusted devices [P2.9]

Branch `feat/auth-persistent-sessions`, cut from `origin/main` (`153d028`). Depends on P2.6, merged.

Closes P2.9 in `docs/development/phase-1-plan.md` §5. Implements ADR-0022.

## What this adds

Rotation is now **sliding**: each renewal starts a fresh idle window from now rather than inheriting
what is left of the old one. Every session carries an **absolute** expiry that activity never moves;
reaching it answers `SESSION_ABSOLUTE_EXPIRED`.

| | idle | absolute |
|---|---|---|
| learner | 30 d | 180 d |
| learner, trusted device | 90 d | **180 d** (unchanged) |
| admin, trusted or not | 12 h | 7 d |

Plus `core.trusted_devices`, `GET /auth/devices`, `DELETE /auth/devices/{id}`, and the
`remember_device` / `device_id` fields that have been on `LoginRequest` since P2.3 and reached
nothing until now.

## The trap, and what writing its test first actually caught

The card is explicit: write the absolute-cap test before the sliding logic, because
sliding-with-no-cap has an identical happy path. ADR-0022 rejected exactly that as alternative C —
"the immortal-token option wearing a different name" — so shipping it by accident would silently
undo a recorded decision.

The test rotates steadily, always well inside the idle window, past the point where the cap falls,
and asserts the session stops. **It failed on the first run for a reason I had not predicted.**

`mint` clamps the last token of a session to expire at *exactly* the cap. At that instant the token
is therefore also idle-expired, and whichever condition `refuse` checked first decided what the
learner was told. It checked idleness first and answered `TOKEN_INVALID` — "your token is broken",
for a session where nothing is broken and the correct answer is "this session reached its maximum
age". The cap is now checked first, with a comment recording why the ordering is load-bearing.

That is the second-order bug the card was warning about: not "I forgot the cap" but "the cap is
present and reports itself as something else".

## Design decisions worth arguing with

**An admin returns before `trusted` is ever read.** `domain.WindowsFor` is a pure function with an
early return for the admin case, precisely because the card names sharing one code path as the way
this gets got wrong. Both a unit table and an end-to-end test assert an admin ticking "remember this
device" still gets 12 h / 7 d.

**Trusting never moves the cap.** A learner can consent to a longer idle window; they cannot consent
their way out of the cap, because the cap is what bounds a theft and the device asking may be the one
an attacker is holding.

**Untrusting revokes the refresh family immediately**, not "demotes to the shorter window".
Untrusting is what somebody does when a laptop is lost.

**`core.sessions` stores its own `idle_window`.** Recomputing it per rotation means resolving the
role again on every refresh; deriving it from the previous token's lifetime would be free and wrong
at the edge, because a token clamped to the cap would shrink the window for the next one. Storing it
also means changing `SESSION_IDLE_WINDOW` does not move the expiry of a session already running.

**A session with no cap is refused, not renewed.** The column is `NOT NULL` so the database cannot
produce one, but that state *is* the immortal token, so it fails closed. There is a test.

**The cap is enforced in the claim's own `WHERE` clause**, so a session past it never spends a token
— the caller is refused either way, and a token burnt on a refusal is one the legitimate client no
longer has.

## Scope

Card's Files list is `auth/service/{session.go,device.go}` and the migration. Also touched, all
inside `auth` except the last two: `domain/window.go` (the pure window selection),
`service/refresh.go` (sliding + clamp + the cap refusal), `service/login.go` (pass the two fields
through), `repository/device.go`, the queries, `transport/http/`, and `cmd/api` for the five
`SESSION_*` keys.

## Verification — real output

```
ARCH=PASS   UNIT=PASS   INT=PASS   CONTRACT=PASS
```

```
--- PASS: TestAContinuouslyActiveSessionStillDiesAtTheAbsoluteCap (0.08s)
--- PASS: TestRotationMovesTheIdleWindowButNeverPastTheCap (0.07s)
--- PASS: TestAnAdminSessionDoesNotReceiveTheExtendedWindow (0.02s)
--- PASS: TestTrustingADeviceLengthensTheIdleWindowAndNothingElse (0.01s)
--- PASS: TestUntrustingADeviceSignsItOutImmediately (0.02s)
--- PASS: TestAnotherAccountsDeviceIsANotFound (0.01s)
--- PASS: TestAPasswordResetUntrustsEveryDevice (0.02s)
```

Run with `-race`, against real PostgreSQL. All 21 P2.5–P2.8 integration tests still pass alongside.

- `golangci-lint v2.12.2` (CI's pin), default and `--build-tags=integration` — `0 issues.` both.
- `make cover-check` — `total coverage: 66.0% of hand-written code (minimum 60.0%)`. It failed once
  when run straight after the other gates and passed alone, which is the flake handoff §3 documents.
- `npx @stoplight/spectral-cli lint api/openapi/openapi.yaml` — `No results with a severity of
  'error' found!`
- `make docs-check` — drift passed, markdownlint `0 error(s)`.
- `make gen-check`, `make gen-check-web` — green after committing.

## Carried, not fixed here

- **Nothing sweeps expired trusted devices.** A device past its cap stops being listed and stops
  lengthening anything — the filter is in the query — but the row stays. Same shape as the
  refresh-token and session sweeps already filed; probably one card for all three.
- **`TRUSTED_DEVICE_LIMIT` is enforced nowhere.** The unique index keeps one row per device id, so
  this is a nuisance rather than a hole, but capping it needs a decision about what happens at the
  limit — refuse, or evict the least recently used.
- **`cmd/worker` gets the default windows.** Nothing in the worker opens a session.

All three are in `internal/modules/auth/TODO.md`.
