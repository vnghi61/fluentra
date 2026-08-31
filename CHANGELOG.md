# Changelog

All notable changes to Fluentra are recorded here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) ·
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html) ·
Generated from Conventional Commits by `git-cliff`, then **edited by a human** before release —
generated text describes commits; release notes should describe change.

---

## [Unreleased]

### Added

- **Your own vocabulary**: paste a list at `/practice/my-words` — tab, dash, colon, equals,
  semicolon or pipe all separate a word from its meaning, bullets and numbering are
  stripped, and a bare word list is accepted. Submitting stores and returns; an hourly job
  then checks each word against the free dictionary, asks the model whether the learner's
  own wording of the meaning holds and to write example sentences, writes the word into a
  deck of their own, schedules it for review, and publishes `vocabulary.words_verified` so
  `gamification` pays XP per verified word.
  The division of labour is deliberate: whether a word exists is the dictionary's answer,
  not a model's — a model asked will confidently invent an entry for a typo. An unreachable
  dictionary or a rate-limited model leaves the word pending for the next run rather than
  rejecting it, because a network blip must never reject a learner's good word; an item
  that fails three times retires instead of being retried for ever.

- **Practice generation**: a twelve-hourly job turns every word in the dictionary into six
  exercises — flashcard, multiple choice, gap fill, listen-and-type, meaning in context and
  sentence order — plus a matching drill per group of four, published as a `Vocabulary
  Practice` course of its own. The curated course covered 32 activities against 200 words;
  this covers the rest. Deterministic throughout: every shuffle is seeded from the data
  being shuffled, so a re-run converges on the same catalogue instead of rewriting it and
  dropping every cached lesson. No LLM — which word means what, and which distractors are
  plausible, are questions the dictionary already answers.
- **Machine-authoring surfaces**: `content.Author` (one idempotent `EnsurePublished`, keyed
  on slug, rather than the four-step review state machine that models decisions a person
  makes) and `lesson.Author` (course, unit and lesson upserts plus wholesale activity
  replacement). Activities live in `lesson`'s tables and rule L2 forbids writing them from
  a skill module, so the generator asks rather than reaches; `MODULE_INDEX.md` carries the
  new `vocabulary -> lesson` arrow.
- **`rbac.RoleMembers`**: `FirstHolderOf(role)`, so generated content has an owner —
  `content_items.owner_id` is not nullable, and unattributed content is content nobody can
  be asked about. Returns `uuid.Nil` on a database with no administrator yet, and the
  generator stands down quietly rather than failing every twelve hours.

### Fixed

- **Progress read back stale after every graded answer.** Grading writes progress and
  schedules review cards on the server, but nothing invalidated the caches the course,
  dashboard and review screens read — so a learner who answered a lesson and pressed back
  saw the untouched course they had left. The work was always saved; only the reading of it
  was stale, which is indistinguishable from the answer never having been recorded. Both
  the lesson runner and the review session now invalidate on every graded answer, not only
  on finishing.
- **Seeded word senses returned no examples through the API, ever.** The seed wrote
  `examples` as a bare `[]string` while `domain.ExampleSentence`, the `ExampleSentence`
  schema and the reader all expect `{sentence, sentence_vi, audio_url}` objects — and the
  read path discards its unmarshal error, so the mismatch was silent. `sentence_vi` was
  never populated by anything.

### Added

- **Example sentences are bilingual**: every one of the 1,000 curated sentences, and the 40
  on the flashcard activities, now carries its Vietnamese rendering, and the eight
  flashcards carry a Vietnamese gloss of their definition. Translations start hidden behind
  one tap — showing both at once sends the eye to the line it can read and leaves the
  English as decoration.
- **`next_lesson_id` on `LessonDetail`**, resolved server-side across unit boundaries, and a
  **Next lesson** action on the completion screen. The learner-facing route is
  `/learn/lesson/{id}` and carries no course, so a client holding only a lesson id could
  not work this out for itself.

- **`internal/platform/ai`, first slice**: an `ai.Client` interface, a versioned prompt
  registry whose templates carry their own token and temperature settings, an offline
  `mock` provider (the default, so `make dev` needs no key and no internet), and one
  adapter for every OpenAI-compatible server — Ollama, OpenRouter, Groq, LM Studio, vLLM —
  written against `net/http`, so choosing a free or local model is two environment
  variables and adds no Go dependency. Routing, budget, quota, caching, retry, streaming
  and the usage trail are specified but not built; the module's `AGENT.md` §0 says so, and
  nothing on a request path may use it until they are.
- **Free dictionary lookup** (`vocabulary/repository`): resolves a word to its IPA, part of
  speech, definition and a link to a human pronunciation from Wikimedia Commons, via
  `api.dictionaryapi.dev` — no key, no account. Audio is referenced by URL and never
  stored, and the recording's source and licence are carried alongside it because most are
  CC BY-SA and playing a file does not satisfy attribution. "Not a word" and "the
  dictionary was unreachable" are distinct outcomes, so a network blip cannot reject a
  learner's good word.

- **`gamification` module, built from its spec**: XP with per-source daily caps and
  diminishing returns, a quadratic level curve, streaks on the learner's own day boundary
  with automatic freeze consumption, an authored badge catalogue with idempotent awards,
  time-boxed quests, and weekly opt-in league leaderboards. It consumes
  `activity.completed`, `lesson.completed` and both session-completed events, and owns two
  cron jobs (hourly streak sweep, quarter-hourly leaderboard build). Six endpoints under
  the `gamification` tag; ten badges and four quests seeded.
  Two departures from the module spec, both documented in its `AGENT.md`: `xp_events` is
  not partitioned, because PostgreSQL cannot give both a monthly partition and the
  `(user_id, source, source_id)` unique constraint idempotency depends on; and the two
  per-learner settings live on `streaks` rather than in a table that would be the same row
  under another name.

- **Four more vocabulary exercise kinds**: `vocab_listen_type` (hear a word, spell it),
  `vocab_match` (pair words with meanings), `vocab_reorder` (rebuild a sentence from
  shuffled words) and `vocab_context_choice` (pick the meaning a sentence uses), each
  seeded twice across the eight lessons and rendered by the lesson runner. Matching is the
  first kind that can be partly right: it scores pair by pair and reports the fraction
  rather than a bare pass or fail. Its answer key (`correct_pairs`) is redacted out of the
  learner-facing body like every other answer field, and grading now schedules one review
  card per word an activity asked about rather than one per activity.

- **Pronunciation on every card**: a shared `PronounceButton` speaks the word, and each
  example sentence, from a recorded asset when the content version carries one and from
  browser speech synthesis otherwise. The previous control rendered only when a body had
  both an `ipa` and an `audio_url`; nothing populates `audio_url`, so it had never appeared
  for a learner. It is now on the lesson-runner flashcard, both faces of the review card,
  and the gap-fill sentence — which speaks the blank as a pause until the answer is in, so
  hearing the sentence cannot give the answer away.
- **Five example sentences per word**: all 200 curated senses and the eight flashcard
  activities now carry five examples instead of one, exposed to clients as a new
  `example_sentences` body key (`example_sentence` still carries the first, for readers of
  content authored before it). The cards show two and collapse the rest, highlighting the
  target word in each.

- **Phase 2 Learning Experience & Core Curriculum (v0.2.0)**:
  - **WP7 Content & Lesson**: Canonical content item authoring, versioning, review & publish state machines (`content`), course catalogue, units, lessons, activities, and prerequisite validation (`lesson`).
  - **WP8 Learning Engine**: Attempt execution engine, startup-validated grader registry, atomic grading claim, multi-level progress rollup (`activity` → `lesson` → `unit` → `course`), learning sessions, and learner dashboard endpoint (`learning`).
  - **WP9 Spaced Repetition & Vocabulary**: FSRS-based review scheduling, due queues, review logs (`srs`), comprehensive dictionary lookup, word senses, curated/user decks (`vocabulary`), and first-party vocabulary exercise grader.
  - **WP10 Learner Web SPA**: Responsive React 19 SPA with Dashboard, Syllabus browser, distraction-free Lesson Runner (Multiple Choice, Gap Fill, Flashcard), Spaced Repetition Review session (with full keyboard controls 1–4 and space), and Progress breakdown.
  - **WP11 Seed, E2E & Observability**: Complete development dataset with 1 A2–B1 course ("Everyday English: A2–B1 Foundations"), 2 units, 8 lessons, and 200 curated word senses with IPA, definitions and examples; Playwright E2E learning journeys; a Grafana learning-funnel and D1 retention dashboard built on new `learning_funnel_events_total` and `learning_cohort_learners` metrics; Prometheus alerts for grading errors and failing scheduled jobs; and runbooks for a stuck attempt and a missing monthly partition.
- WP4 Admin Shell (P4.1): Admin user management endpoints (`GET /api/v1/admin/users`, `GET /api/v1/admin/users/{id}`, `POST /api/v1/admin/users/{id}/suspend`, `POST /api/v1/admin/users/{id}/reinstate`, `POST /api/v1/admin/users/{id}/sessions/revoke`) with cursor pagination, audited reads and writes, and self-administration guards.
- WP4 Feature Flags (P4.2): Feature flags system with stable per-user bucketing via SHA256, percentage rollouts, in-memory caching (30s TTL), and CRUD management endpoints (`/api/v1/admin/flags`).
- WP4 Observability (P4.3): Prometheus alerting rules (`deploy/prometheus/rules/phase1.yml`), Grafana dashboards (API Overview, Database, Jobs, Auth & Security), and operational runbooks (`docs/operations/runbooks/`).

- Complete Software Architecture Document ([ARCHITECTURE.md](ARCHITECTURE.md))
- Plan review and optimisation record ([docs/architecture/00-plan-review.md](docs/architecture/00-plan-review.md))
- AI context engineering strategy ([AI_CONTEXT.md](AI_CONTEXT.md)) and root [AGENT.md](AGENT.md)
- 30 module specifications with the nine-file documentation set each
- 20 Architecture Decision Records
- Repository conventions: coding, API, database, errors, logging, testing, security, observability
- Prompt library design, development and runtime ([PROMPT_LIBRARY.md](PROMPT_LIBRARY.md))
- Dependency register with alternatives and rationale ([DEPENDENCIES.md](DEPENDENCIES.md))
- Delivery roadmap through Phase 5 ([ROADMAP.md](ROADMAP.md))
- Module documentation generator (`tools/docgen`) with drift checking
- Module boundary enforcement configuration (`.go-arch-lint.yml`)
- Configuration reference (`.env.example`) and `Makefile`
- The `core` identity schema: `users`, `profiles`, `user_preferences`, `learning_profiles`
- `GET` and `PATCH /api/v1/me` — read and update your own profile
- `GET` and `PUT /api/v1/me/preferences` — read and replace your own settings
- Roles and permissions: the `core` tables, the seeded two-role catalogue, and the guard
- `GET /api/v1/me/permissions` — what the caller is allowed to do
- `GET /api/v1/admin/roles`, and granting and revoking a user's roles
- The append-only audit trail: `audit_logs` and `security_events`, partitioned by month, with
  the application role holding `INSERT` and `SELECT` and nothing else
- `GET /api/v1/admin/audit-logs` and `GET /api/v1/admin/security-events` — search the trail and
  the security feed, filtered and paged
- `POST /api/v1/admin/security-events/{id}/resolve` — mark an event triaged, with a required
  reason
- An outbox consumer that turns the events `user` and `rbac` already publish into audit
  entries, exactly once per event
- Scheduled partition rotation and two-year retention

- `POST /api/v1/auth/register`, `/verify`, `/resend` — user registration & email verification flow
- `POST /api/v1/auth/login` — authentication with Argon2id timing equalisation and per-account/IP lockout protection
- The identity modules wired into the running API and worker: every operation above is now
  mounted, and every audited write reaches `audit_logs` through the worker
- `POST /api/v1/auth/refresh` — exchange the refresh cookie for a new access token. Signing in
  and verifying an address now also set that cookie, so a session outlives the fifteen-minute
  access token without the learner re-entering a password
- Refresh tokens rotate on every use and are single-use. Presenting one that has already been
  spent revokes every token in its family, revokes the session, and raises a `refresh_reuse`
  security event — so a stolen token is detected the moment either party uses it twice, at the
  cost of signing the legitimate learner out alongside the thief
- `core.sessions` and `core.refresh_tokens`. Sessions record a keyed digest of the client
  address, never the address
- `GET /api/v1/auth/sessions` — the devices this account is signed in on, with a coarse label
  ("Chrome on macOS"), when the session started and when it was last used. No IP address appears
  and none is stored
- `DELETE /api/v1/auth/sessions/{id}` — sign one device out. A session belonging to another
  account answers 404 and not 403, so the operation cannot be used to discover which session ids
  exist
- Sign in with Google. `GET /api/v1/auth/oauth/google/start` hands back a consent URL and nothing
  else — the `state`, the `nonce` and the PKCE verifier stay server-side, because a value the page
  can read is one an attacker reading the same page can replay.
  `POST /api/v1/auth/oauth/google/callback` spends the state, redeems the code and verifies
  Google's ID token against its published keys — signature, issuer, audience, expiry and the nonce
  we issued — before writing anything at all
- Google sign-in links to an existing account **only** when Google vouches for the address and a
  local account has proved the same one. An address matching an account that has never completed
  its own verification is refused with `OAUTH_ACCOUNT_CONFLICT` and no link is made: registering an
  address does not prove you own it, so auto-linking there would hand the account to whoever
  claimed the address first. A learner in that position verifies by email once and then links
- A Google account with no local counterpart opens one that is **already verified** — Google has
  performed exactly the check the emailed code would have — and it gets no password. That is why
  `POST /api/v1/auth/oauth/google/link` and `DELETE /api/v1/auth/oauth/google` exist, and why
  unlinking the only remaining way in is refused with `LAST_SIGN_IN_METHOD` rather than leaving an
  account nobody can reach
- A Google callback carrying a `state` this server did not issue, has already spent, or issued more
  than ten minutes ago is refused — all three the same way, since telling them apart tells a prober
  how the check works — and each one raises an `oauth_state_invalid` security event, because a
  refused callback leaves no other trace and the rate is the whole signal

- Persistent sign-in. Refresh rotation is now **sliding**: each renewal starts a fresh idle window,
  so a learner who keeps using the app never sees the login form. Every session also carries an
  **absolute** expiry that activity never moves — reaching it answers `SESSION_ABSOLUTE_EXPIRED` and
  requires signing in again. Without that cap, a stolen token used regularly would renew itself
  forever
- `remember_device` on login trusts the browser and lengthens the idle window from 30 days to 90. It
  does not touch the absolute cap, and an administrator gets neither extension — 12 hours idle,
  7 days absolute
- `GET /api/v1/auth/devices` and `DELETE /api/v1/auth/devices/{id}` — see the devices you have
  trusted, with both expiries, and untrust one. Untrusting revokes its refresh family immediately
  rather than demoting it to a shorter window
- A password change or reset untrusts every device as well as revoking every session

- Rate limiting at the HTTP boundary, in the classes `API_GUIDELINE.md` §11 sets out: 60/min per
  address for anonymous callers, 600/min per account once signed in, 5/min per address **and** per
  account on the operations that hand out or reset a credential, and a per-address hourly cap on
  challenge issuance that catches a script asking for one code each against many different
  addresses. Responses carry `RateLimit-Limit`, `RateLimit-Remaining` and `RateLimit-Reset`; a 429
  adds `Retry-After`
- When the rate limiter's backing store is unreachable the request is **allowed**, not refused, and
  no budget is advertised — a limiter that denies during a cache outage turns it into a total
  outage, and a `RateLimit-Remaining` derived from a budget nobody checked is a number a client
  would pace itself against

- `POST /api/v1/auth/forgot-password`, `/reset-password` and `/change-password` — the reset flow.
  `forgot-password` always answers 202, in comparable time, whether or not the address has an
  account: an unknown address still has a real challenge issued that nobody is given a code for,
  so neither the body nor the clock reveals who is registered
- A reset revokes every session; a change revokes every session but the one it was made from, and
  requires the current password even though the caller is already signed in
- Reset codes live thirty minutes rather than the ten a signup code gets, and asking for a second
  one kills the first

- `POST /api/v1/auth/logout` — sign out of this device: the session and its refresh family are
  revoked, and the access token is denylisted so it stops working immediately rather than at its
  expiry

### Fixed

- Outbox events were published under a doubled topic (`user.user.profile_updated`), so no
  consumer could ever match one. Because an event with no handlers is accepted rather than
  retried, every event published since the `user` module landed was marked delivered and
  discarded. Nothing had subscribed yet, so nothing noticed.

- Outbox events carry the producing transaction's W3C `traceparent`, so work done in the worker
  continues the trace of the request that caused it. An audit entry now records the trace of
  the action rather than of the worker that filed it (BR-AUDIT-07)

### Notes

Authentication is live: P2.4 added the bearer middleware and P2.5 the refresh cookie behind it,
so the operations above are usable by a real client. What is still missing is `POST
/auth/logout` and the session list — P2.6. See [ROADMAP.md](ROADMAP.md).

A refresh token is deliberately **not** in any response body. It exists only as an `HttpOnly`
cookie scoped to `/api/v1/auth`: a value the page can read is a value an injected script can
steal, and unlike an access token it is renewable indefinitely.

Audit entries record **which fields changed, not what they changed to**, and redact anything on
the PII deny-list if a value is supplied. An audit log holding a copy of every old display name
would be a second store of personal data with a longer retention period than the first. Client
addresses are stored as a keyed HMAC and never in the clear.

---

## How to write an entry

| Section | Use for |
|---|---|
| **Added** | New features and capabilities |
| **Changed** | Changes to existing behaviour |
| **Deprecated** | Soon-to-be-removed features, with a sunset date |
| **Removed** | Features removed in this release |
| **Fixed** | Bug fixes |
| **Security** | Vulnerability fixes — always call these out explicitly |
| **Breaking** | Anything requiring action from a client or operator |
| **Migration notes** | What an operator must do when deploying this version |

Write from the reader's point of view: *"Essay feedback now streams as it is generated"*,
not *"refactor writing grading to use SSE"*. The commit log already says the second thing.

Every user-visible change gets an entry under `Unreleased` **in the same pull request** that
makes the change. Adding them at release time means half of them are missed.
