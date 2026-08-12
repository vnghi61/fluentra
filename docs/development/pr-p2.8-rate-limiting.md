# feat(httpx): rate limiting [P2.8]

Branch `feat/auth-rate-limiting`, cut from `origin/main` (`1cfc28d`). Depends on P0.8 and P2.7, both
merged.

Closes P2.8 in `docs/development/phase-1-plan.md` §5.

## What this adds

`internal/shared/httpx/ratelimit.go` — middleware over `cache.Limiter`, applying the classes in
`API_GUIDELINE.md` §11:

| Class | Budget | Counted by |
|---|---|---|
| Anonymous | 60/min | address |
| Authenticated | 600/min | account |
| Credential (`login`, `register`, `forgot-password`, `reset-password`, `change-password`, `refresh`) | 5/min | address **and** account |
| Challenge issuance (`register`, `forgot-password`, `resend`) | 20/hour | address |
| Upload | 30/hour | account |

Responses carry `RateLimit-Limit`, `RateLimit-Remaining` and `RateLimit-Reset`; 429 adds
`Retry-After`. The four `RATE_LIMIT_*` keys and `OTP_ISSUE_PER_IP_PER_HOUR` were already in
`.env.example` and read by nothing — they are wired through `cmd/api` now.

## The spec diff is three lines of substance

Two acceptance criteria were **already satisfied**: `Retry-After` on 429 lives on the shared
`TooManyRequests` response, which every operation references. Added: the `RateLimit-*` trio on that
same response, and a paragraph in `info.description` saying every response carries them.

I did **not** repeat the three headers on ~25 operations' success responses. OpenAPI has no way to
express a global response header, and a 75-line repetitive block that every new endpoint silently
drifts from is worse than documenting the rule once. Say if you want the exhaustive version; it is
mechanical.

## Placement, which I got wrong twice

**The limiter runs after the auth middleware.** Before it there is no actor in the context, so every
signed-in caller would be charged the 60/min anonymous class — ten times tighter than promised, and
shared by everybody behind one office NAT. Auth is applied inside `identity.Routes`, so the limiter
goes there too.

**`/health`, `/ready` and `/version` are never limited.** They are registered outside that group. An
instance that answers 429 to a liveness probe is an instance that gets killed for being busy, during
exactly the traffic spike that made it busy. My first attempt mounted the middleware on the root
router with a comment asserting the probes were exempt — `router.Use` applies to every route
registered after it, so they were not. `TestRouter_TheProbesAreNotRateLimited` now pins it.

**go-arch-lint caught the third one.** `s_httpx` was importing `platform/cache` for `LimitResult`:

```
Component s_httpx shouldn't depend on github.com/fluentra/fluentra/internal/platform/cache
  in /src/internal/shared/httpx/ratelimit.go:11
```

The shared kernel sits below the platform packages. `httpx.LimitResult` is declared locally and
`cmd/api` adapts the two in four lines — the alternative was an edge in `.go-arch-lint.yml` that
inverts the layering for the convenience of one struct. Second card running where that linter has
paid for itself.

## Design notes a reviewer should push back on if they disagree

**Fail open, and advertise nothing.** An unreachable store allows the request. The headers are
withheld in that case, because a `RateLimit-Remaining` derived from a budget nobody evaluated is a
number a client will pace itself against. Nothing is logged either: `cache.RedisLimiter` already
warns and increments `cache_unavailable_total`, and a second line per request would double the noise
of an outage in the logs being read to diagnose it. A *malformed reply* still logs — that is a bug in
this stack rather than an outage, and it arrives with `Degraded` unset.

**Every budget is charged even when an earlier one already refused.** Stopping at the first refusal
would let a caller who exhausted the cheap per-IP budget stop accruing against the per-account one,
then come back from a new address with a full allowance.

**Zero is never a limit.** Every config field falls back to its documented default, so a
partially-populated config cannot produce a budget of zero — the one value that must not be
reachable by forgetting to set something, because it refuses everything.

**A nil limiter disables the middleware** rather than refusing everything, the same direction as the
outage case.

## Also here

The `Limiter: nil` and `Env: ""` placeholders that have sat in `auth/module.go` since P2.1b are
filled in. The per-subject issuance caps (BR-AUTH-13) have been coded against that limiter since
P2.1b and doing nothing; they now count, namespaced by environment.

## Verification — real output

```
ARCH=PASS
UNIT=PASS
INT=PASS
CONTRACT=PASS
```

The arch stage includes its own deliberate-violation probe (steps 1–4 of `verify-arch-lint.sh`).

```
--- PASS: TestRateLimit_AnUnreachableStoreAllowsTheRequest (0.01s)
--- PASS: TestRateLimit_ThePerIPChallengeCapBlocksASpreadAcrossManyAddresses (0.00s)
--- PASS: TestRateLimit_ClassesAreCountedIndependently (0.00s)
--- PASS: TestRateLimit_TheBoundaryIsAtTheLimitAndNotBeforeIt (0.00s)
--- PASS: TestRateLimit_ASignedInCallerIsCountedPerAccountNotPerAddress (0.00s)
--- PASS: TestRateLimit_TheCredentialClassCountsTheAddressAndTheAccountSeparately (0.00s)
--- PASS: TestRouter_TheProbesAreNotRateLimited (0.00s)
```

Run with `-race`. The boundary test asserts at the limit *and* one over, so an off-by-one in either
direction fails.

- `golangci-lint v2.12.2` (CI's pin), default and `--build-tags=integration` — `0 issues.` both.
- `make cover-check` — `total coverage: 66.9% of hand-written code (minimum 60.0%)`.
- `npx @stoplight/spectral-cli lint api/openapi/openapi.yaml` — `No results with a severity of
  'error' found!`
- `make docs-check` — drift passed, markdownlint `0 error(s)`.
- `make gen-check`, `make gen-check-web` — green after committing.

One flake worth recording: `make test-int` failed once with exit 1 and produced no `FAIL` line, then
passed on an immediate re-run and on the clean gate above. That matches the `job`/`worker`
integration flakiness handoff §3 already documents on `main`. I did not chase it.

## Carried, not fixed here

- **`/api/v1/ping` is not limited.** It sits outside the authenticated group with the probes, and it
  does real work — a `SELECT 1` and a Redis `PING` per call. Limiting it needs a second middleware
  instance for the anonymous class alone, or moving the probe endpoints; neither is worth doing
  blind.
- **The upload class has no route to guard yet.** `/uploads` and `/avatar` arrive with P3.1, so the
  path matcher is written and untested against a real route.
- **The limiter is a fixed window.** A caller can spend a full budget at the end of one window and
  another at the start of the next — twice the nominal rate across the boundary. Fine for these
  classes, and worth knowing before somebody sizes an alert off the numbers.

All three are in `internal/modules/auth/TODO.md`.
