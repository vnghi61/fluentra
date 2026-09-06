---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-06
---

# Phase 3 — work order 9

**Purpose.** Give the content and vocabulary an owner. This is WP19 items 1 and 2, and it is
the last of Phase 3 that needs no new money and no new infrastructure.

**Read first.** [phase-3-plan.md](phase-3-plan.md) is the specification.
[phase-3-next-steps.md](phase-3-next-steps.md) §4 holds the traps. One of them decides this
order: §4.2, the two codegen gates, because §4.1 below changes `openapi.yaml` and every screen
here depends on that change.

**Follows** [phase-3-work-order-8.md](phase-3-work-order-8.md). Its Part A shipped, and so did
the grammar grader in §5.1 — `grammar_tense_choice` and `grammar_sentence_transform` are seeded
and graded. Its §5.3, the timed exam, was **not built**; see §6.

---

## 1. The finding that shapes this order

`/admin/content` and its five transitions have been called "backend done" in three work orders,
including one I wrote. They are not done. They are **write-only**:

| Operation | Exists |
|---|---|
| `adminCreateContent`, and the draft / submit / review / publish / archive transitions | yes |
| `adminCreateWord` | yes |
| `adminListContent`, `adminGetContent`, `adminListWords` | **none** |

There is no GET on any admin content or vocabulary path. An admin can create a draft and push
it through the state machine, and cannot see a single thing that exists. A screen has nothing
to render.

So the first task is not a screen. It is the read side of an API that was only ever half
written — and that is an `openapi.yaml` change, which is the one thing in this order that can
cost a CI round.

The permissions, at least, are real: `content.read.published`, `content.create`, `content.edit`,
`content.review` and `content.publish` have existed since migration `1700000180`. The web's
`PERMISSIONS` map simply never learned them — it stops at `systemFlags` and `adminDashboard`.

---

## 2. Decisions taken

| Question | Decision |
|---|---|
| Scope | Admin content **and** the learner-word queue — WP19 items 1 and 2 |
| Editing a body | Raw JSON with validation before save |
| Learner-added words | Visible and correctable **after** publication; no approval gate |
| Reading and writing (WP20) | Next order, written when this one lands |

**Why JSON and not a form per kind.** A content body is JSON keyed by `kind`, and there are
nine kinds today with more coming from work order 8's grammar drills. Nine forms is a week and
a tenth kind is a tenth form; a validated JSON editor is a day and serves every kind, including
the ones nobody has invented yet. It is the worse experience and the honest one — revisit it
when a content author who is not an engineer actually uses this screen.

**Why no approval gate.** Words verified by the model reach the shared dictionary immediately
and the learner reviews them the same day. Putting a human in front of that would mean a
learner waits for someone on duty before practising a word they added themselves. The admin
gets the ability to correct and to withdraw instead, which fixes the same problem without
making every learner pay for it.

---

## 3. The work

### 1. The read side

`GET /admin/content` — a filtered, paginated list. Status is the filter that matters, because
the screen's job is "what is waiting for me": `draft`, `review`, `published`, `archived`.
`GET /admin/content/{id}` — one item with its versions.
`GET /admin/vocabulary/words` — a list, filterable by whether a word came from a learner upload.

**OpenAPI first, then the handler.** Then `make gen-api` **and** `make gen-web`, then
`make gen-check` **and** `make gen-check-web`, and commit what they generate. Both gates compare
`git status`, so regenerating is not enough. This sequence has cost two CI rounds; it is the
single most likely way to lose a third.

### 2. The content screens

A list that opens on what needs attention, and a detail view that carries the state machine.

`content` already owns draft → review → published → archived and refuses invalid transitions,
so the screen's job is to show which transitions are legal from here and to hide the rest.
Buttons that answer 403 are the thing the existing `PERMISSIONS` comment warns against, and the
five content permissions are what decides which ones to draw.

The body editor is a JSON field validated **before** the request goes out: parseable, and
carrying the keys the kind's grader reads. A body that saves and then fails at a learner's
submission is the failure this screen exists to prevent — `vocabulary/service/grader.go` and
the new `grammar` grader are what define "the keys it reads", and the seed's own
`assertActivityIsGradable` is the closest thing to a specification of them.

### 3. Teach the web its permissions

`PERMISSIONS` gains the five content entries. They exist server-side and the guard already
enforces them; this is the half that decides what a screen offers.

### 4. The learner-word queue

A list of what learners have added — the word, the meaning they wrote, the model's verdict,
the topic it was filed under, and which deck it went into.

Two actions, both after the fact: **correct** a definition, gloss, topic or example that the
model got wrong, and **withdraw** a word that should not be in the shared dictionary.

Withdrawal is the one that needs thought. A word is not the learner's copy: `materialise`
shares one `word_senses` row across everyone who added it, and a review card points at its
content version. Removing it must not leave a card pointing at nothing — `srs` already has
`CardContentUnavailable` for a card whose content will not load, so the honest options are to
archive the content version and let that path handle it, or to suspend the affected cards.
Decide which, and record it in `vocabulary/DECISIONS.md`.

**Done means:** an admin can find a word a learner added this morning, see what the model
decided about it, fix a wrong Vietnamese gloss, and take a bad entry out of the shared
dictionary without breaking anyone's review queue.

---

## 4. What must not change

**A learner still practises the word they added, the same day.** No approval step, no pending
state between verification and the flashcard.

**The state machine stays in `content`.** The screen asks which transitions are legal; it does
not decide. A UI that knows the rules is a second copy of them.

**Nothing here needs the model.** This whole order works against a `mock` provider — it is
about seeing and correcting what is already stored. That is deliberate: it is the last piece of
Phase 3 that does not wait on a real provider being configured.

---

## 5. Migration numbers

`1700000470` is the highest used. **Take `1700000480` and above.**

This order should need no migration at all: the permissions exist, the tables exist, and the
read side is a query. Write one only if withdrawal turns out to need a column, and say in the
file what it is.

---

## 6. The exam, and what comes after

Work order 8 §5.3 was not built. `learn.attempts.duration_ms` and the lesson-scope
`learn.progress` score both exist and `CompletionScreen` still shows neither, counting both in
the browser — so a learner who closes the tab and returns has a client count of zero beside a
server score that is right. The decision was recorded in `learning/DECISIONS.md`; the work was
not done.

It stays deferred here, deliberately: WP19's fourth item is an exam admin, and there is no
point administering something that does not exist yet. It comes back with the exam.

After this order: **reading and writing** (WP20), which need the AI provider and nothing else.
Then listening and speaking (WP21), which need `platform/media` and Azure Speech and are the
only part of the plan that costs real money.

---

## 7. What keeps being found in review

**Built and never wired**, five times now: the gamification widgets, the avatar route,
`AdminAIUsage`, `WordAutocomplete`, and the review session that has worked the whole time with
nothing pointing at it. §1 above is the same fault in a new shape — an API called done that
cannot be read from. Before calling something done, open the screen or call the URL.

**A test that agrees with itself proves nothing.** The Google sign-in test passed `onSuccess`
straight to the button, so it stayed green while the form dropped it and every learner had to
reload. When a test needs a fixture, ask whether the fixture is the case that ships.

**Generated blocks are generated.** `TODO.md` and `DECISIONS.md` are written from
`tools/docgen/data/*.json`; an edit between the markers is reverted by the next `make docs`,
and `generate.mjs --check` is the gate that catches it — not `check-drift.mjs`, which passes
regardless. Completed work goes in the hand-written section below the markers, because docgen
renders every generated item unticked.

**Run it in the shape CI runs it.** `go-arch-lint`'s deepScan gives different answers on
Windows and Linux: `make arch` was green locally and red in CI. Check boundary changes in a
Linux container.
