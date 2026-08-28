---
adr: 0025
title: "Published curriculum is public; learner state is not"
status: Accepted
date: 2026-08-28
tags: [security, learning, frontend]
---

# ADR-0025: Published curriculum is public; learner state is not

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-28 |
| **Deciders** | Product owner, Tech Lead |
| **Tags** | security, learning, frontend |

## Context

Until now every learning endpoint required an account. `GET /courses` answered `403` to an
anonymous caller, so the product's entire value was behind a registration form: a visitor
could see a login screen and nothing else. The ask is the ordinary one for a learning
product — let someone try a lesson before they sign up, and tell them afterwards that
nothing was kept.

Two facts about the existing system shaped the decision.

**The grader is already pure.** `vocabulary.Grader.Grade` reads `ContentVersionID` and
`Response` and nothing else — not the user, not the attempt. Grading for a visitor with no
account therefore costs no new engine, only a route that declines to persist.

**The answer travelled with the question.** Content bodies are authored as one JSON object
holding the prompt *and* `correct_answer`, `acceptable` and `correct_option_id`, and
`GET /lessons/{id}` returned that object verbatim. No code anywhere redacted it. Every
signed-in learner already held the answer key for a lesson before starting it, and the
renderer read `correct_option_id` straight out of it. Opening the endpoint without fixing
that would have published the answer key for the whole curriculum to anyone with `curl`.

## Decision

**Published curriculum is public.** `GET /courses`, `GET /courses/{slug}` and
`GET /lessons/{id}` no longer consult the authorization guard. A bearer token is still
accepted and still changes the answer — it is what adds the caller's progress and lesson
unlocking — but it is not required.

**The answer no longer travels with the question.** `contentcontract.RedactForLearner`
strips answer-bearing fields from every body on the learner-facing path, and the lesson
service applies it before the DTO enters the cache. The answer is returned by the grading
operations instead, on `GradeResult.CorrectAnswer` and `SubmitAttemptResult.correct_answer`
— after the learner has submitted, which is the only moment it is theirs to know.

**Learner state stays private and stays behind an account.** Attempts, progress, sessions,
review cards, the dashboard and preferences are unchanged. A visitor's work exists in their
browser for the length of the visit and is then gone, and the app says so rather than
implying otherwise.

**`content.read.published` survives.** It still gates the authoring surface and `admin`
still holds it. What it no longer does is stand between a learner and material that is, by
definition, published.

## Alternatives considered

### A. Grade in the browser for anonymous visitors

| | |
|---|---|
| **Pros** | No new endpoint at all — the body already contained the answer |
| **Cons** | Entrenches the leak it depends on; two graders to keep in agreement; the browser's verdict cannot be trusted the moment it matters |
| **Why rejected** | It makes the answer key a permanent public artefact in exchange for saving one handler. |

### B. Preview only — the first lesson of each course, the rest behind sign-up

| | |
|---|---|
| **Pros** | Protects most of the curriculum; conventional freemium shape |
| **Cons** | An arbitrary line to draw and to maintain per course; the paywall Phase 4 needs is an entitlement check, not a lesson-index check |
| **Why rejected** | Deferred rather than refused. With bodies redacted, what is public is the questions, and Phase 4 will gate on entitlements at which point this decision is revisited on its own terms. |

### C. An "anonymous" pseudo-role in RBAC

| | |
|---|---|
| **Pros** | Keeps every read behind a single uniform guard |
| **Cons** | A role nobody holds, granted to callers who do not exist, resolved through a permission cache keyed by a user id that is `uuid.Nil` |
| **Why rejected** | It dresses "this is public" up as authorization and leaves a role in the catalogue whose only purpose is to always say yes. |

## Consequences

**Good.** A visitor can browse the catalogue, open a lesson and answer its activities before
deciding to register. The answer key stops shipping to every client, which was a real
weakness independent of this feature. The three endpoints need no token, so they are also
the cheapest thing to point a cold-start ping at.

**Bad.** The published curriculum is now scrapeable by anyone, questions and all. That is a
deliberate trade for reach, and it is the thing to re-examine in Phase 4 rather than a
detail to forget.

**The denylist has to be maintained.** `RedactForLearner` removes fields it recognises. A
new activity kind that names its answer something new would leak until the name is added.
`TestRedactForLearner_SeededKinds` asserts that no accepted answer survives for every kind
the curriculum authors, so the failure lands in CI rather than in production — but it only
covers kinds that exist.

**AI graders must not follow.** Phase 3's graders cost money per call. Nothing in this
decision extends to them: an anonymous grading path for an AI-graded skill is a funded
denial-of-wallet, and opening one needs its own decision and its own quota.
