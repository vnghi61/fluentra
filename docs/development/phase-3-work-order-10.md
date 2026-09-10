---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-10
---

# Phase 3 — work order 10

**Purpose.** Two faults the deployed system has and the local one does not, the settings
screen, then WP20 — the first two of the four skills.

**Read first.** [phase-3-plan.md](phase-3-plan.md) is the specification.
[phase-3-next-steps.md](phase-3-next-steps.md) §4 holds the traps. §3 below changes
`openapi.yaml`, so §4.2 — the two codegen gates — applies again.

**Follows** [phase-3-work-order-9.md](phase-3-work-order-9.md), merged as PR #76. Its admin
screens, the learner-word queue and withdrawal all shipped.

---

## 1. What is broken

### The worker sleeps and nothing wakes it

`wakeUp()` in `web/src/api/wake.ts` pings `/api/v1/ping`. That is the **API**. The worker is a
second Render service with its own process and its own URL, and nothing in the repository ever
calls it.

The mail path is not a precedent to copy, and it is worth saying why before someone looks for
one: `cmd/api/main.go` builds `newAPIMailSender` and sends over the Resend **HTTP API** from
inside the API process. Mail has never needed the worker, which is exactly why mail kept
working while everything the worker owns quietly stopped.

What actually stops:

| Job | Registered in | Interval |
|---|---|---|
| `vocabulary.verify_uploads` | `vocabulary/module.go` | hourly |
| `vocabulary.enrich_queued` | same | hourly |
| `vocabulary.generate_exercises` | same | twelve-hourly |

Plus every River job. `Submit` enqueues `VerifyUploadArgs` inside the learner's transaction
(BR-JOB-01), so the row is durable — a sleeping worker loses nothing. It just does nothing,
and the learner watches `pending` until something wakes the process.

**Nothing is lost. Everything is late, indefinitely.** That is the fault.

### Signing out leaves the previous account's data in the cache

`authApi.logout()` clears `useAuthStore` and stops there. The TanStack Query cache is never
touched, and `main.tsx` sets `staleTime: 5 * 60 * 1000` for every query.

So the next person to sign in on that browser is served the previous person's cached answers
for up to five minutes. `["account", "me"]` is the one that shows: the header greets them by
the wrong name and draws the wrong avatar until a hard reload. Signing in with Google is where
it is most visible, because that flow navigates rather than reloading the document — a full
page load is what has been hiding this.

**This is a data leak between accounts on a shared browser, not a rendering glitch.** The
severity is not in the header; it is that nothing in the app draws a line between one session's
data and the next.

### An example sentence is written once and never again

`materialise` in `vocabulary/service/upload.go` asks for exactly five (`"ExampleCount": 5`) and
writes them to two places: `word_senses.examples`, and the body of the content version it
publishes for the sense. A learner who reviews a word forty times reads the same five sentences
forty times.

---

## 2. Decisions taken

| Question | Decision |
|---|---|
| Who wakes the worker | The **API**, right after it enqueues work the worker must do |
| Where the extra examples come from | A **background job**, topping each sense up to fifteen |
| How many the flashcard shows | **Three**, shuffled per view |
| Revoking the session you are using | Not offered — **Sign out** is that button |
| Language and timezone pickers | Icons on both; **search on the timezone only** |
| Scope | These, the settings screen, then WP20 — reading and writing |

**Why the API and not the browser.** A server-to-server call needs no CORS and no public
worker URL, and the worker has no business accepting requests from a browser: it is a process
that consumes a queue, and giving it a front door reachable from the internet is a new attack
surface bought for a nudge. The API already knows a job was created; it is the only party that
knows *when* the worker is needed.

**Why a job and not the review request.** `vocabulary/DECISIONS.md` records:

> Generate example sentences at request time? **No** — generate at authoring time and review
> them. Latency, cost, and above all correctness: an unreviewed example can teach a wrong
> collocation to thousands of learners.

That decision holds and this order does not overturn it. Generating on the review path would
make a flashcard wait on a provider, would spend quota per tap, and would put unreviewed
sentences in front of a learner. The learner asked for variety, not for freshness measured in
milliseconds: fifteen sentences shuffled three at a time is five distinct views of a word, and
the job can take all night to build them.

---

## 3. The work

### 1. Wake the worker when there is something for it to do

A new config key, and it is the awkward part rather than the interesting one. `config.Load`
splits an environment variable on its **first** underscore, so `WORKER_URL` is section `worker`,
key `url`. The allowlist is per section, not per key — a section nothing declares is dropped
silently.

Three things must agree or the key does nothing:

- `configOptions()` in `cmd/api/main.go` declares it, with an empty default.
- `.env.example` documents it. `TestConfigOptions_EveryDeclaredKeyIsDocumented` fails
  otherwise, and it reads `.env.example` rather than restating the keys.
- The deployment sets it to the worker's own base URL.

**Empty means disabled, and that is the local default.** Nobody running `docker compose` has a
sleeping worker, and a nudge that fires at nothing on every upload would be noise in the log
and a needless dependency in the tests.

**The nudge itself is fire-and-forget.** `GET {WORKER_URL}/ready` — the worker already serves
it (`cmd/worker/main.go`, beside `/health`). Do not await it on the request path and do not
fail the upload when it fails: the job is already committed, the learner's words are already
saved, and a 202 that turns into a 500 because a *hint* did not land would be worse than the
sleep it was meant to fix. Give it a short timeout, log at debug, and move on.

**Where it goes.** After the transaction that enqueued, not inside it. Inside, a slow ping
holds a database transaction open for the length of a cold boot.

**Done means:** an upload submitted against a sleeping worker is picked up within a cold boot
rather than within an hour; `WORKER_URL` unset changes nothing anywhere; and a worker that is
down does not turn a successful upload into an error.

### 2. Fifteen examples, three at a time

**Check what the flashcard actually reads before writing anything.** This is the whole
difficulty of the task and it is easy to miss:

`FlashcardBack` renders from `flashcardContent(card)`, which reads `card.content.body`. That
body is a **content version**, resolved by `srs.attachContent` through
`content.GetManyVersions` using `review_cards.content_version_id` — the version id frozen into
the card when it was made.

So examples appended to `word_senses.examples` **do not appear on the flashcard**. Nothing
reads that column on the review path. A change that only grows the column will look finished,
pass its own tests, and show the learner the same five sentences.

`EnsurePublished` republishes an existing slug as a **new version**, and `content_items
.current_version_id` moves to it. The old version stays, and so does every card pointing at it.

That leaves a decision, and it belongs to whoever does the work:

- **Repoint the cards.** The job republishes the sense and asks `srs` to move affected cards to
  the new version. `learn.review_cards` has `uq_review_cards_user_content (user_id,
  content_version_id)`, so this is an update, not an upsert — an upsert makes a second card and
  the learner reviews the word twice.
- **Resolve the item's current version.** `attachContent` follows the card's item to whatever
  is published now, rather than the frozen id. Simpler for vocabulary and a real change for
  everything else: a lesson activity may be frozen on purpose, and this would unfreeze all of
  them.

Pick one, and record it in `srs/DECISIONS.md` — it is srs's rule either way, not vocabulary's.

**The job.** A fourth entry in `vocabulary/module.go`'s `CronJobs()`, beside the three that are
there. Take a new lock id. It sweeps senses with fewer than fifteen examples, asks for five
more, and appends. Reuse `vocab_verify`'s template or add a narrower one — a prompt that only
writes sentences does not need the whole verification contract, and asking a model to re-decide
whether a word is real every time it writes a sentence is how a verified word gets rejected on
its second pass.

**Do not duplicate.** Five new sentences that repeat three of the existing five is not variety.
Send the sentences the sense already has and ask for different ones; then compare on
normalised text before appending, because a model told not to repeat will still repeat.

**Quota is the ordinary state, not the error.** This is decoration on top of a word that
already works. When quota is out the job stops and the word keeps the examples it has — the
same rule §6 of work order 8 set for the topic, and for the same reason.

**The shuffle is the client's.** Three of fifteen, chosen per view. It belongs where the card
is rendered, not in the API: a shuffled response is uncacheable, and the same learner opening
the same card twice in one session should see it move.

**Done means:** a word verified last month has fifteen sentences and shows three; two
consecutive reviews of the same word rarely show the same three; a word verified this morning
still shows the five it has, with no empty slots and no placeholder.

### 3. The account screen

Four things, all in `web/src/features/account`, and one of them is mine.

#### Clear the query cache when the session changes

`logout()` must drop the cache, not only the store. The cache is per-`QueryClient` and the
client lives for the life of the document, so nothing else does it.

Clear on **sign-in as well as sign-out**. Clearing only on the way out looks sufficient and is
not: the Google callback and the boot-time refresh can both establish a session without a
sign-out having happened first, and a browser that was closed mid-session and reopened has a
warm cache and a new person in front of it.

`queryClient.clear()` over `removeQueries` per key: an allowlist of keys to forget is a list
someone has to remember to extend, and the failure is silent and belongs to whoever is
unlucky. There is nothing in the cache worth keeping across an identity change.

The awkward part is reach — `authApi` is a module, and the `QueryClient` is created in
`main.tsx`. Hand the client to the auth layer at composition rather than importing a singleton
into it; a module-level client is also what makes the test suite share state between cases.

**Done means:** signing in as B after A shows B's name and avatar with no reload, and a test
proves it by seeding A's profile into the cache, signing in as B and asserting on the header.

#### Do not offer to revoke the session you are sitting in

`SessionsList` renders Revoke on every row, `session.current` included, behind a raw
`confirm("Revoking your current session will sign you out immediately. Proceed?")` — English,
untranslated, in a browser dialog.

Keep the row: seeing "this device" in the list is how a learner reads the rest of it. Remove
the button, and with it the confirm and the `onLoggedOut` branch that existed only for this
case. Signing out is a button the app already has, and it is in the account menu where people
look for it.

**Done means:** the current session shows as a labelled row with no action; every other session
still revokes; and no `confirm()` string is left in the file.

#### Icons on both pickers, search on the timezone

Language is two options. It gets a flag or a globe and nothing else — a search box over a list
of two is furniture.

Timezone is 418 entries and unusable without one. Two things are needed and they are different:

- **Search.** There is no combobox primitive; `@radix-ui/react-dropdown-menu` is all that is
  installed. `WordAutocomplete` is the in-repo pattern for a searchable listbox and it works,
  but it is vocabulary's — it fetches the dictionary. Extract the shape into
  `components/ui/` or write a second one, and say which in the commit. Do not add `cmdk` for
  this.
- **Something to search by.** A learner types "Vietnam" and `Asia/Ho_Chi_Minh` does not contain
  it. Label each zone with its city, its country and its current UTC offset, and match against
  all three. `Intl.DateTimeFormat(locale, {timeZone, timeZoneName: "shortOffset"})` gives the
  offset without a table to maintain.

`mobile-baseline/touch-target` and `input-font-size` are errors, not warnings: 44 px targets
and a 16 px input, or eslint fails the build.

#### The Vietnam timezone is missing, and that is my bug

`web/src/lib/locales.ts` builds the list from `Intl.supportedValuesOf("timeZone")`. That
returns **canonical** zone names, and on this runtime the canonical name is `Asia/Saigon`:

```text
Ho_Chi_Minh present: false
matching /viet|saigon|ho_chi/i: [ 'Asia/Saigon' ]
```

Everything else in the product says `Asia/Ho_Chi_Minh` — `cmd/seed/main.go`, the validation
message in `user/domain/profile.go`, the OpenAPI example. Go's tzdata resolves both, so the
server accepts either and no test caught it. The learner is the one who notices: they look for
the city they live in and it is not there.

`timezoneOptions(current)` prepends the stored zone when it is missing, which hides this for
anyone who already has one and hides it from nobody signing up.

Carry the aliases the product actually uses, rather than trusting the canonical set to contain
them, and assert it: a test naming `Asia/Ho_Chi_Minh` explicitly is the only thing that would
have failed here. Decide what a learner whose stored zone is `Asia/Saigon` sees — one entry,
not two that mean the same place.

#### The English that never reached i18next

A census of `web/src`, excluding tests and generated types, with `t(...)` spans blanked first:

| | |
|---|---|
| Files with untranslated user-facing copy | 16 |
| Short strings — labels, placeholders, titles | ~28 |
| Multi-line prose blocks | ~9 |

The heaviest are `DataPrivacySettings`, `CreateFeatureFlagModal`, `ChangePasswordModal`,
`RegisterForm` and `ForgotPasswordForm`. `SessionsList`'s confirm goes away with the button
above it.

**Not everything on that list should be translated, and translating it would be a bug.**
`placeholder="••••••••"` is a mask. `learner@example.com` and `@team` are format examples, not
sentences. `placeholder="DELETE"` is the word a learner must type to confirm erasure, and the
string in the box and the string the code compares against must stay one thing — translate it
and the confirmation stops matching, or it matches in one locale only. Leave those, and say in
the commit which you left and why, so the next census does not re-raise them.

`src/test/i18n-keys.test.ts` already fails on a key that is used and missing, in either locale.
It cannot see a string that never asked for a key, which is what this section is about.

#### What the census cost, and what paid for it

Translating those strings grew `en.json` and `vi.json` by about 290 lines each. Both were
static imports in `src/i18n/index.ts`, so both landed in the entry chunk, and the entry chunk
is what `scripts/check-bundle.mjs` measures against a 200 kB gzipped budget. `main` was already
sitting at 199.5 kB; this work order pushed it to 202.1 kB and the build failed.

Raising the budget was the wrong answer twice over: it would have bought about one work order
of headroom, and it would have left every visitor downloading two languages to read one.

`initI18n` now fetches only the locale being read — `en` for an English reader, `vi` for a
Vietnamese one — and `loadLocale` fetches the other if someone switches. The numbers:

| | before | after |
|---|---|---|
| Entry chunk, gzipped | 202.1 kB | 180.0 kB |
| Real first visit, English reader | 202.1 kB | 190.9 kB |
| Real first visit, Vietnamese reader | 202.1 kB | 192.2 kB |

The middle row is the one that matters. A dynamic import that is awaited before first render
still costs the visitor its bytes, so a split that only moved the translations out of the
measured chunk would have passed the check while changing nothing — the budget would have been
gamed, not met. Loading one language instead of two is what actually made the page smaller, and
both real numbers are under 200 kB, not only the measured one.

Two things follow that a later change has to respect:

- **`fallbackLng` is English while English may not be loaded.** That is safe only because
  `i18n-keys.test.ts` asserts the two bundles carry exactly the same keys, so the fallback has
  nothing left to resolve. Relax that test and this has to load English alongside.
- **`i18n.changeLanguage` on its own now renders raw keys.** `setLocale` loads first and
  switches second; nothing else in `web/src` may call `changeLanguage` directly.
  `i18n-switching.test.ts` holds that, and it lives in its own file because i18next is a
  process-wide singleton that survives `vi.resetModules()` — in a shared file the test would
  pass on a bundle some earlier test happened to fetch.

### 4. WP20 — reading and writing

`internal/modules/reading` and `internal/modules/writing` are documentation and an empty
`doc.go`. There is no code. The boundaries are already declared in `.go-arch-lint.yml`:

```
m_reading: mayDependOn [p_cache, p_telemetry, c_content, c_questionbank, c_vocabulary, c_learning]
m_writing: mayDependOn [p_ai, p_job, p_telemetry, c_content, c_learning, c_notification]
```

Read those two lines before designing either module. `reading` may **not** reach `p_ai` — it is
a comprehension exercise over authored passages, graded from an answer key, and the boundary
says so. `writing` may, because a written answer needs a judgement no key can hold.

**`c_questionbank` is also empty.** `reading` is declared to depend on a module that does not
exist yet. Decide early whether WP20 builds it, stubs it, or drops the edge — do not discover
this half way through.

ADR-0015 governs both, as it governed the grammar grader in work order 8: *"A seventh skill is
a grader, not a module rebuild"*, and *"A skill module that defines its own attempt table fails
review."* The attempt lifecycle stays in `learning`.

**And the seed's three-way check applies.** `cmd/seed/main_test.go` requires every seeded kind
to appear in `GradedKinds()` and to match `runnerKinds` **in both directions** — a renderer
with nothing seeded fails just as a seeded kind with no renderer does. A new kind is four edits
landing together: the grader, the `GradedKinds()` entry, the seed activity, and the
`LessonPage.tsx` dispatch with its `Exercise*.tsx`.

---

## 4. What must not change

**A learner still practises a word the day they add it.** No approval step, and no waiting for
fifteen examples before a card is usable.

**The review path makes no model call.** §3.2 is a background job precisely so this stays true.
If the work drifts toward "just one call when the card opens", stop and re-read
`vocabulary/DECISIONS.md`.

**An unset `WORKER_URL` is a working configuration.** Local development and the test suite must
not need it.

**The state machine stays in `content`.** Republishing a sense goes through `EnsurePublished`
like everything else; the job does not write `content_versions` itself.

**A learner can still end their other sessions.** §3.3 removes one button, not the feature: the
row for a device someone else is holding keeps its Revoke, and that is the one that matters
after a laptop is lost.

**The stored timezone stays an IANA name.** The label a learner reads may say "Vietnam
(UTC+7)"; what reaches `PATCH /me/profile` is still the zone id, and `ValidateTimezone` still
resolves it against tzdata.

---

## 5. Migration numbers

`1700000480` is the highest used, by work order 9's `user.delete` permission.
**Take `1700000490` and above.**

§3.1, §3.2 and §3.3 should need no migration: the config key is not a table,
`word_senses.examples` is an existing JSON column, and the settings work is all client-side.
WP20 will need its own. Write one only when
something is genuinely missing, and say in the file what it is.

---

## 6. What keeps being found in review

**The gate you did not run is the gate that fails.** Work order 9 went to CI three times: once
for integration tests that need Postgres and are invisible to `go test ./...`, once for a
CodeQL alert that only runs on CI. Both were reproducible locally in under ten minutes. Run
`make lint` and the integration suite before pushing, not after CI says no.

**A permission is three edits.** `rbac/contract/permissions.go` says so in a comment — "a new
one is a line here, a row in a migration, and a test" — and the migration went in without the
line. The integration test caught it, which is what it is for.

**The module usually already has the helper.** `domain.NormaliseLimit` and `NormaliseOffset`
existed, carried the same reasoning, and a third clamp was written beside them anyway. Before
writing a bounds check, a paging helper or a shuffle, grep for one.

**Built and never wired**, six times now: the gamification widgets, the avatar route,
`AdminAIUsage`, `WordAutocomplete`, the review session, and the account menu that drew a generic
icon while the avatar URL sat in a response it had already fetched. §3.2's version of this trap
is examples appended to a column nothing on the review path reads.

**Check what the screen reads, not what the write touches.** Same trap, stated as a rule.

**A platform list is not the list you assumed.** `Intl.supportedValuesOf("timeZone")` returns
canonical names, so the zone the whole product is written around was absent from its own picker
and every gate passed. When a list comes from the runtime, assert that the entries the product
depends on are in it.

**Clearing one half of the session state is not clearing it.** The auth store and the query
cache both hold identity. Emptying either alone leaves the other answering for the person who
just left.
