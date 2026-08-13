# feat(auth): sign in with Google [P2.10]

Branch `feat/auth-google-oauth`, cut from `origin/main` (`483fb50`). Depends on P2.5 and P2.9, both
merged.

Closes P2.10 in `docs/development/phase-1-plan.md` §5, **and closes WP2**. Implements ADR-0023.

## What this adds

Authorization code + PKCE against Google, a server-side single-use `state`, and ID token
verification against Google's published keys.

| Operation | | |
|---|---|---|
| `GET /auth/oauth/google/start` | public | Returns one consent URL and nothing else |
| `POST /auth/oauth/google/callback` | public | Spend the state, redeem the code, verify the token, then one of five branches |
| `POST /auth/oauth/google/link` | self | Add Google to the account already signed in |
| `DELETE /auth/oauth/google` | self | Unlink, unless it is the last way in |

Plus `core.oauth_states` and `core.oauth_identities` (`1700000120`), a `sweep_oauth_states` cron job,
and the `OAUTH_*` keys wired through `cmd/api`.

## The trap: the branch that must write nothing

The card names the unverified-local-match branch as the account-takeover path, and it is the one
place where the friendly behaviour is the wrong one. Registering an address does not prove you own
it — anyone can type a stranger's address into the signup form — so an unverified local account is a
*claim* on an address, not ownership of it. Auto-linking there means: an attacker registers the
victim's address and never verifies it, the victim later signs in with Google, and the account they
land in is the attacker's, with the attacker's password still on it.

So it is refused with 409 `OAUTH_ACCOUNT_CONFLICT`, and **the test asserts the table rather than the
error**:

```go
assertCode(t, err, "OAUTH_ACCOUNT_CONFLICT")
if rows := identityRows(t, oauthSubject); rows != 0 {
    t.Fatalf("%d identity rows exist after the refusal; the refusal is worth nothing if the "+
        "link it refused is in the table afterwards", rows)
}
```

A 409 with a row written behind it is the same compromise as a 200 — the next request that finds the
row completes the takeover. Same shape for the ID token: `TestAnIDTokenFailingVerificationLeavesEveryTableEmpty`
asserts `core.oauth_identities` **and** `core.users` are empty afterwards, across five different
verification failures, not merely that the call errored.

## The thing I had to change, and why

**The PKCE verifier could not be recovered at callback time.** `domain.NewPKCE` drew it from
`crypto/rand` and only its SHA-256 reached the row — `pkce_verifier_hash bytea` with
`CHECK (octet_length(...) = 32)` — while the exchange must send the verifier itself to Google. The
spec closes both alternative routes: `OAuthStart` returns only `authorization_url`, and
`OAuthCallbackRequest` is `additionalProperties: false` over `[code, state]`. So there was no path
from a stored row back to the value the exchange needs.

Agreed fix: **derive it instead of drawing it.** `Keyring.PKCEFor(state)` is
`base64url(HMAC-SHA256(server_key, "pkce-verifier" ‖ 0x00 ‖ state))`, recomputed at callback from
the `state` the row already holds.

- No migration, no spec change, no new config key — it reuses `OTP_HMAC_KEY`, which every instance
  must already share for the OTP challenges to work at all.
- The security properties are the ones a drawn verifier had: 256 bits from `crypto/rand` in the
  state, keyed with the server key. A dump of `core.oauth_states` still completes no flow, because
  the key is not in the table — which is the property the column was shaped around.
- `pkce_verifier_hash` keeps its job and gains a better one: the callback compares its recomputation
  against it, so a rotated key or a changed derivation is a refused sign-in *here*, with a log line,
  rather than an opaque `invalid_grant` from Google that looks like the learner's fault.

`NewPKCE` is gone rather than left unused. An exported function that draws a random verifier is what
the next contributor would reach for, and it produces a flow that cannot complete.

Four domain tests pin it, including `TestPKCEFor_RecomputesTheSameVerifierFromTheSameState` — if
that equality ever stops holding, every Google sign-in fails at Google.

## Design decisions worth arguing with

**`DecideLink` stays a pure function, separate from the code that writes rows.** The service reads
the rows, calls it, and acts on the answer. The five branches are exhaustive, ordered, and testable
with no database — which is the only reason the third one can be read off the page.

**An unverified provider email is refused before anything is matched.** Not a tidiness point: if the
address were matched first, anyone able to make Google emit an unverified claim for an arbitrary
address would walk into the link-to-verified branch. `TestAnUnverifiedGoogleEmailIsRefusedBeforeAnythingIsMatched`
puts a *verified* local account in the way and asserts it is never consulted.

**Identities are keyed on `(provider, subject)`, never on email**, and the known-identity branch
never reads the address. `TestGoogleSignInWithAKnownIdentityDoesNotConsultTheAddress` changes what
Google asserts between two sign-ins and asserts the same account, no second identity row, and no new
account for the new address.

**A refused state raises an event that names nobody.** `oauth_state_invalid`, severity `medium`,
with a nil user id and nothing from the request — an attacker who can raise an event must not choose
what is stored in a table nobody can `UPDATE`. `medium` and not `high` because one of these is
ordinary: a consent screen left open past ten minutes produces one. The rate is the signal, and there
is no rate to see unless each is written. Forged, spent and expired all return the same code, since
telling them apart tells a prober how the check works.

**The state consume is one guarded `UPDATE`**, the same shape as the refresh claim.
`TestTwoConcurrentCallbacksWithOneStateProduceExactlyOneSignIn` is the test that a read-then-write
implementation fails and every sequential test passes.

**The unlink count and delete are one transaction.** Read-then-delete lets two concurrent unlinks
each see two methods and each remove one, leaving an account nobody can sign into — and that is not
a state any later operation repairs, because `forgot-password` cannot help an account with no
password and the identity it would need is gone.

**A disabled provider loses its credentials, not its routes.** `OAUTH_GOOGLE_ENABLED=false` builds a
provider with no client id; the operations stay mounted and refuse. Unmounting them would make the
API surface depend on configuration and turn a missing key into a 404 that reads as a version
mismatch. Missing credentials while enabled warns at startup rather than refusing to boot — it is one
optional sign-in method, and failing the boot would take password sign-in down with it.

**`redirect_to` only survives if it is a same-site path.** Storing it server-side stops it being
rewritten in flight, which is what the schema comment claims, but it does not make the caller's own
value safe — the caller is who started the flow. Absolute, protocol-relative and bare-host values are
dropped.

## Scope

Card's Files list, all inside `auth`, plus:

- `.go-arch-lint.yml` — a new `m_auth_oauth` component for `service/oauth/google`. It is the one
  package in the module that holds an HTTP client pointed at a third party. **Probed** as handoff §3
  requires: an illegal import into it was rejected by name —
  `Component m_auth_oauth shouldn't depend on .../internal/modules/auth/repository` — then removed.
  A rule that matches nothing passes silently.
- `contract/contract.go` — `SecurityKindOAuthStateInvalid` beside `SecurityKindRefreshReuse`.
- `cmd/api` — the eight `OAUTH_*` keys, all of which were already in `.env.example`.
- `domain/oauth.go` — `NewPKCE` replaced by `Keyring.PKCEFor` (see above).

No other module was touched.

## Verification — real output

```
ARCH=PASS   UNIT=PASS   INT=PASS   CONTRACT=PASS
```

The 21 new integration tests, `-race`, against real PostgreSQL:

```
--- PASS: TestAGoogleEmailMatchingAnUnverifiedLocalAccountWritesNoIdentity (0.30s)
--- PASS: TestGoogleSignInWithNoLocalAccountCreatesOneAlreadyVerified (0.02s)
--- PASS: TestGoogleSignInLinksToAVerifiedLocalAccount (0.31s)
--- PASS: TestGoogleSignInWithAKnownIdentityDoesNotConsultTheAddress (0.31s)
--- PASS: TestAnUnverifiedGoogleEmailIsRefusedBeforeAnythingIsMatched (0.01s)
--- PASS: TestAForgedStateIsRefusedAndRaisesASecurityEvent (0.00s)
--- PASS: TestAReusedStateIsRefusedAndRaisesASecurityEvent (0.01s)
--- PASS: TestAnExpiredStateIsRefusedAndRaisesASecurityEvent (0.01s)
--- PASS: TestTwoConcurrentCallbacksWithOneStateProduceExactlyOneSignIn (0.01s)
--- PASS: TestAnIDTokenFailingVerificationLeavesEveryTableEmpty (0.02s)
--- PASS: TestUnlinkingTheOnlySignInMethodIsRefused (0.01s)
--- PASS: TestUnlinkingSucceedsWhenAPasswordRemains (0.25s)
--- PASS: TestUnlinkingNothingSucceeds (0.24s)
--- PASS: TestLinkingRefusesAGoogleAccountWithADifferentAddress (0.23s)
--- PASS: TestLinkingRefusesAnIdentityAlreadyOnAnotherAccount (0.45s)
```

41 tests pass in `internal/modules/auth` — the 21 new ones alongside every P2.5–P2.9 test.

- `golangci-lint v2.12.2` (CI's pin), default tags and `--build-tags=integration` — `0 issues.` both.
- `make cover-check` — `total coverage: 65.8% of hand-written code (minimum 60.0%)`. It failed once
  immediately after the other gates and passed alone, which is the flake handoff §3 documents.
- `npx @stoplight/spectral-cli lint api/openapi/openapi.yaml` — `No results with a severity of
  'error' found!`
- `make docs-check` — drift passed, markdownlint `0 error(s)`. The generated AGENT.md tables came
  from `tools/docgen/data/core.json` via `make docs`, not by hand.
- `make gen-check`, `make gen-check-web` — green after committing.
- `cd web && pnpm run lint` — clean.

`make check` was **not** run: it reformats ~37 unrelated files with two formatters CI does not
enforce (handoff §3.1).

## The WP2 gate (`phase-1-plan.md` §1.5)

| | |
|---|---|
| Register → OTP → auto sign-in | Covered, but at unit and contract level rather than by a DB-backed test: `TestVerifyEmail_MarksAddressVerified` proves the address is marked and the flow returns a `Verification`, and `TestContract_VerifyMatchesTheSpec` proves the `AuthSession` inside it matches the published schema. **There is no integration test that walks register → verify → session against Postgres.** Worth adding; it is the one leg of the gate not proven end-to-end. |
| Google sign-in, all five branches | The five integration tests above. |
| Refresh reuse revokes the family | `TestPresentingASpentRefreshTokenRevokesTheWholeFamilyAndTheSession` — re-run on this branch, passes. |
| Session survives a restart, dies at the cap | `TestAContinuouslyActiveSessionStillDiesAtTheAbsoluteCap` and `TestRotationMovesTheIdleWindowButNeverPastTheCap` — re-run on this branch, both pass. |

## Carried, not fixed here

- **`user.Registrar` cannot open an account already verified.** Creating an account for a new Google
  learner is three writes and cannot be one transaction (rule L4). If `MarkEmailVerified` fails after
  `CreateAccount`, the retry finds an unverified local account and takes the conflict branch — so a
  transient failure locks that learner out of Google sign-in for the seven days the sweep needs. The
  window is two consecutive calls on one pool; the fix is a `Verified` field on
  `user/contract.NewUser`, which is another module's contract.
- **A suspended account is caught by address, not by id.** `checkUsable` resolves the account from
  the address Google asserted, so a learner suspended after changing their Google address is not
  matched. `service.Accounts` has no lookup by id. Nothing can suspend an account today — `admin`
  does not exist — and BR-AUTH-09's revocation still covers every session they already hold.
- **Nothing prunes consumed `oauth_states` before the hourly sweep**, and **`redirect_to` is stored
  and never read** — the SPA route that will consume it is P3's.

All four are in `internal/modules/auth/TODO.md`.

## Review focus

Two reviewers, per rule S11. The two things worth the most scrutiny are the PKCE derivation (is
deriving the verifier from the state under the server key an acceptable trade against storing it?)
and the ordering inside `OAuthService.redeem` and `resolve` — everything is checked before anything
is written, and that ordering is the whole of BR-AUTH-18.
