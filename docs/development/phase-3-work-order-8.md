---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-05
---

# Phase 3 — work order 8

**Purpose.** The learner's own vocabulary made usable, and then WP12. It **replaces**
[phase-3-work-order-6.md](phase-3-work-order-6.md) entirely — that order was never started, and
its work is §5 below, unchanged apart from the migration numbers.

**Read first.** [phase-3-plan.md](phase-3-plan.md) is the specification.
[phase-3-next-steps.md](phase-3-next-steps.md) §4 holds the traps, and the two codegen gates in
it matter more here than in any order since: §4.2 changes `openapi.yaml`.

**Depends on** [phase-3-work-order-7.md](phase-3-work-order-7.md) being merged. That branch adds
the named provider chain, verify-on-submit, and `vocab_verify.v2` — and §4.1 below bumps that
same template to v3.

---

## 1. Where this leaves off

Work order 7's branch carries four changes beyond its own brief, all found by using the feature
rather than reading it:

| Found | State |
|---|---|
| Reading your own upload list was charged as an upload | fixed; the page's own polling exhausted a learner's hourly budget in fifteen minutes |
| `WordAutocomplete` written since WP13, rendered by nothing | mounted above the paste box |
| A pasted word with no meaning got no Vietnamese at all | `vocab_verify.v2` asks for the gloss |
| Seventeen cron jobs against a pool of four | semaphore plus a configurable pool floor |

**The review screen already works and nothing says so.** `/practice/review` is reachable from
the dashboard and the practice page; a verified upload gets a card with `InitialGrade: "again"`,
so it is due immediately; and `FlashcardBack` already renders the definition, the IPA, the
Vietnamese and the example sentences. A learner who has just added thirty words is told none of
this on the screen where they added them.

---

## 2. Decisions taken

| Question | Decision |
|---|---|
| The review button | Review **one deck at a time**, which is why the topics come first |
| WP12 | Kept. Vocabulary first, because it is what is being used today |
| Who assigns a topic | The model suggests it; the learner can change it |
| Meanings in the list | Yes, in this order — with both codegen gates |

---

## 3. Why the topics come first

The review button was the request, and it is last in the dependency order rather than first.
"Review the words I added" is one link. "Review my food words" needs a deck per topic, and a
deck per topic needs something to put words in them. So: topics, then the list that shows them,
then the button that uses them.

---

## 4. Part A — the learner's own words

### 1. A topic on every word

**The column exists and nothing writes to it.** `skill.word_senses.domain` has been there since
migration `1700000230`, `WordSense.Domain *string` is in the Go model and in
`vocabulary/contract`, and `module.go` already maps it through. The upload path has simply never
set it. No migration is needed for the topic itself.

Ask for it in `vocab_verify.v3`, beside the Vietnamese v2 added: one call already happens per
word, so a `topic` field costs nothing extra. Keep the set small and closed — food, home,
science, work, travel, study, health, nature, art, other — because a free-text topic becomes
forty near-duplicates ("food", "Food", "eating") and a deck list nobody can use.

**The learner can change it.** A model guessing "science" for a word somebody added from a film
is exactly the case the decision above exists for, and a wrong topic that cannot be corrected is
the "value nobody checked" fault this project has caught three times.

**Decks are how the learner's choice is stored.** `ensureDeck` currently puts every word into
one deck with the fixed slug `my-words`. `uq_decks_owner_slug` is `UNIQUE NULLS NOT DISTINCT
(owner_id, slug)`, so `my-words-food` and `my-words-science` are ordinary rows in a table that
already supports them. The `domain` on the sense is what the model said, shared across learners;
the deck membership is what *this* learner decided. They are different facts and they belong in
different places.

**Done means:** a learner pastes thirty words and finds them in several named decks; moving one
to another deck sticks; and a word the model could not place lands somewhere honest rather than
in whichever topic came first alphabetically.

### 2. The list shows what was found

`VocabUploadItem` carries `term`, `provided_meaning`, `status`, `reason`, `word_sense_id` and
`verified_at`. There is no definition and no examples, so the screen can only show the learner
their own typing back. Add the definition, the Vietnamese and the examples.

**OpenAPI first, then the handler.** Then `make gen-api` **and** `make gen-web`, then
`make gen-check` **and** `make gen-check-web`, and commit the generated files. Both gates
compare `git status`, so regenerating is not enough. This sequence has cost two CI rounds
already and it is the single most likely way to lose a round here.

**Done means:** the list shows, for a verified word, what the dictionary and the model actually
decided — not a repeat of what was typed.

### 3. Review one deck at a time

`ListDueCards` filters on `user_id`, `suspended_at IS NULL` and `due_at` and nothing else.
`learn.review_cards` has no deck column and should not grow one: the link already exists through
`content_version_id` → `skill.word_senses.content_version_id` → `skill.deck_items.word_sense_id`
→ `skill.decks`. A filtered query, an optional API parameter, no migration.

**Optional, and absent must mean everything.** A review session with no deck named is today's
session over every due card, and that is the behaviour the dashboard's "Reviews Due" count
already promises. A filter that silently narrowed the default would make that number wrong.

**And put the button where the words are.** The My Words screen is where a learner finishes
adding and has nothing to do next.

**Done means:** a learner can review one deck, or everything; the due count on the dashboard
still counts everything; and the button is on the screen where the words were added.

---

## 5. Part B — WP12, unchanged from work order 6

### 1. A second grader, and the first that is not vocabulary's

`vocabularycontract.GradedKinds()` returns seven kinds and the seed and the runner cover all
seven — the plan's "every activity is a word quiz" expired some time ago. But all seven are
graded by vocabulary, so the condition the plan actually set is still unmet:

> **Done when** the seeded course contains at least two activity kinds the vocabulary grader
> does not handle.

ADR-0015 settles the shape: *"A seventh skill is a grader, not a module rebuild"*, and under
Compliance, *"A skill module that defines its own attempt table fails review."* So a `grammar`
module implementing `learning.ExerciseGrader` with its own `GradedKinds()`, no attempt table, no
progress table, no score. `content` already carries a `grammar_rule` type, so it likely needs no
table at all.

Pick two kinds that are honestly not word knowledge. A gap-fill with a hole where a word goes,
graded by a grammar grader, satisfies the letter of the condition and leaves the registry as
untested as it is now.

**Done means:** a kind removed from the grader map fails `cmd/api` at boot with that kind named,
proved through the composition root rather than through the registry's own unit test, which has
always passed.

### 2. `skill_focus` must be exactly `grammar`

`learn.lessons.skill_focus` is free text; `learn.skill_mastery.skill` is a CHECK over six
values; `updateSkillMastery` skips an unrecognised skill without an error, so that a content
author's typo cannot abort a learner's submission. The cost is that a lesson seeded as
`"Grammar"` records no mastery at all, for ever, with every test still green.

### 3. The exam-shaped lesson

Server-side timing already exists — `learn.attempts.duration_ms`, from `safeDurationMs`. A
server-side lesson score already exists — `rollupLessonAndAbove` writes `learn.progress` at
`scope = 'lesson'`. **`CompletionScreen` shows neither**, counting both in the browser instead.
An exam is where those two numbers stop agreeing: a learner who closes the tab has a client
count of zero and a server score that is right.

Decide which number the exam reports and record it in `learning/DECISIONS.md`. The
recommendation is the server's. A time limit has to be enforced where the learner cannot edit
it, which means the deadline is a property of the lesson or the attempt rather than a
`setTimeout`.

### 4. Two small things in the same file

`startTime` is `useState(() => Date.now())` and `onRetryLesson` resets seven pieces of state
without touching it, so a retried lesson reports the first run plus the second. And
`vocabulary_quiz` sits in `GradedKinds()` with nothing seeding it — the seed's bidirectional
check compares the seed against the *runner*, so this one slips through.

---

## 6. What must not change

**An explanation still yields silently.** WP17 decided that "no explanation yet" is a normal
state: quota exhaustion returns the grade with the explanation absent and no error on the
screen. Adding a topic to the same template must not change that.

**A word saved while quota is out is still kept** — `status = 'queued'`, no CEFR level nobody
assigned, still reviewable. A word with no topic is the same case: leave it unset and let the
state say why, rather than filing it under "other" as though something decided.

**The learner's own wording wins.** `definition_vi` takes the learner's note first and the
model's gloss only when they wrote none. The same rule applies to the topic.

---

## 7. Migration numbers

`1700000470` is used by work order 7. **Take `1700000480` and above.** `1700000460` was claimed
by work order 6, which this replaces; it was never taken, and leaving the gap is cheaper than
checking whether some branch still holds it.

Neither §4.1 nor §4.3 should need a migration at all — the topic lives in a column that exists,
and the deck filter joins tables that exist. Write one only when something is genuinely missing,
and say in the file what it is.

---

## 8. What keeps being found in review

**Built and never wired**, four times: the gamification widgets, the avatar route,
`AdminAIUsage`, and `WordAutocomplete` — written, exported, unit-tested and rendered by nothing
for two work orders. Before calling something done, open the screen. The review session in §4.3
is the fifth candidate: it has worked the whole time and no learner has been told.

**A fabricated value is worse than a missing one.** A score invented as a perfect 100, a word
marked valid by nobody, a CEFR level assigned by no model. The topic is the next one in line.

**A test that agrees with itself proves nothing.** Every router fallback test passed while the
provider chain silently collapsed, because each fixture used providers with distinct names —
the one thing the real configuration did not do. When a test needs a fixture, ask whether the
fixture is the case that ships.

**Run it.** Every defect in the last two orders came from executing the code or opening the
screen: the rate limit from a learner's own screenshot, the collapsed chain from comparing two
pointers, the empty Vietnamese from removing a fallback and reading `<nil>`.
