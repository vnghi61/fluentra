---
doc_type: glossary
project: fluentra
last_verified: 2026-08-06
---

# GLOSSARY.md

**Read this before naming anything.** One concept, one word, everywhere — in code, in the
database, in the API, and in the UI. Synonyms in a codebase are a permanent tax.

---

## 1. Learning domain

| Term | Definition | Also called (do **not** use) |
|---|---|---|
| **Learner** | A person with the `user` role who studies on the platform | student, member, customer |
| **Administrator** | A person with the `admin` role | moderator, staff, editor |
| **Course** | The largest learning container; has a CEFR level range and a goal | program, track |
| **Unit** | A group of lessons inside a course | chapter, section, module (never "module" — that means a code module here) |
| **Lesson** | The smallest schedulable learning unit; contains activities | class, session |
| **Activity** | One thing a learner does inside a lesson (read a passage, drill 10 words, record a response) | exercise, task (both mean something else — see below) |
| **Exercise** | The generic mechanism: prompt + expected response + grader | — |
| **Item** | One question or prompt inside an exercise | question (reserve "question" for the question bank) |
| **Task** | ① A writing/speaking prompt (e.g. "IELTS Task 2"), ② an AI task name. Always qualify: "writing task", "AI task" | — |
| **Attempt** | One recorded try at an activity or exam, with a score and a timestamp | try, submission (submission is narrower) |
| **Submission** | An attempt that produces a persisted artefact to be graded (essay, recording) | — |
| **Grader** | The component that scores a response. Deterministic or AI-backed | scorer, evaluator |
| **Rubric** | The criteria and band descriptors used to score productive skills | — |
| **Skill** | One of: vocabulary, grammar, reading, listening, speaking, writing | competency, area |
| **CEFR level** | A1, A2, B1, B2, C1, C2 | difficulty, grade, level (unqualified) |
| **Placement** | The initial assessment that assigns a starting level | onboarding test |
| **Learning path** | The ordered sequence of lessons generated for a learner | curriculum, plan, journey |
| **Progress** | A learner's completion and mastery state for a lesson or skill | — |
| **Enrollment** | The link between a learner and a course | registration (reserve for account creation) |

## 2. Spaced repetition

| Term | Definition |
|---|---|
| **Review card** | The scheduling record for one learnable item for one learner |
| **Review** | One answered card |
| **Grade** | The learner's self-rated recall on a review: `again`, `hard`, `good`, `easy` |
| **Due** | A card whose `due_at` has passed |
| **Interval** | Days until the next review |
| **Stability** | FSRS memory-strength parameter — how slowly the memory decays |
| **Difficulty** | FSRS item-difficulty parameter (0–10) |
| **Retrievability** | Estimated probability of successful recall right now |
| **Lapse** | A review graded `again` after the card had graduated |
| **Retention target** | The desired recall probability (default 0.90) that drives scheduling |
| **FSRS** | Free Spaced Repetition Scheduler — the algorithm we use (ADR-0016) |

## 3. Content

| Term | Definition |
|---|---|
| **Content item** | The canonical unit of learning material (word, grammar point, passage, audio, prompt) |
| **Content version** | An immutable snapshot of a content item; published lessons reference a version |
| **Taxonomy** | A controlled vocabulary used to tag content (topic, CEFR level, skill, exam type) |
| **Media asset** | A file in object storage plus its metadata (audio, image, derived waveform) |
| **Authoring workflow** | draft → in_review → approved → published → archived |
| **Publish** | Making a content version visible to learners; triggers cache invalidation, TTS, reindex |

## 4. Assessment

| Term | Definition |
|---|---|
| **Question** | An item in the question bank, reusable across exams and exercises |
| **Question set** | An ordered, reusable group of questions |
| **Exam** | A timed, structured assessment with sections (e.g. an IELTS mock) |
| **Exam attempt** | One learner's run through an exam |
| **Section** | A timed part of an exam, bound to one skill |
| **Score report** | The learner-facing result: band scores, sub-scores, feedback, comparison |
| **Band** | A score on an exam's scale (IELTS 0–9); distinct from CEFR level |
| **Item difficulty** | A statistical estimate derived from attempt data |

## 5. Platform / architecture

| Term | Definition |
|---|---|
| **Module** | A bounded context in `internal/modules/` or a capability in `internal/platform/`. **Never** used for learning content |
| **Contract** | A module's public interface package — the only thing other modules may import |
| **Platform module** | A technical capability with no business rules |
| **Shared kernel** | `internal/shared/` — primitives with no business meaning |
| **Outbox** | The table that makes event publishing transactional |
| **Event** | A past-tense fact published by a module (`writing.graded`) |
| **Job** | A unit of background work executed by the worker |
| **Queue** | A named lane of jobs (`ai`, `media`, `notify`, `batch`, `default`) |
| **Composition root** | `cmd/api/main.go` — the only place concrete types are wired |
| **Problem Details** | The RFC 9457 error response format |
| **Cursor** | The opaque pagination token |

## 6. AI

| Term | Definition |
|---|---|
| **AI task** | A named capability (`writing.grade_essay`) that business code requests. **Never a model name** |
| **Provider** | A vendor adapter (`anthropic`, `openai`, `gemini`, `openrouter`, `local`, `mock`) |
| **Model tier** | `small`, `mid`, `frontier` — the routing abstraction over actual model IDs |
| **Prompt template** | A versioned Markdown file with typed input and output schemas |
| **Prompt version** | An immutable `vN`; config pins which version is active |
| **Fallback chain** | The ordered list of providers tried for a task |
| **Eval suite** | Golden examples plus scorers and thresholds that gate a prompt change |
| **Golden set** | Human-labelled examples used as the evaluation oracle |
| **Semantic cache** | Cache keyed by embedding similarity rather than exact input |
| **Budget** | The global daily spend cap |
| **Quota** | A per-user limit on AI usage |

## 7. Gamification

| Term | Definition |
|---|---|
| **XP** | Experience points awarded for completed learning actions |
| **Streak** | Consecutive days meeting the daily goal |
| **Freeze** | A consumable that preserves a streak for one missed day |
| **Badge** | A one-off achievement |
| **Quest** | A time-boxed multi-step goal |
| **Daily goal** | The learner's chosen XP target per day |

## 8. Commerce

| Term | Definition |
|---|---|
| **Plan** | A purchasable tier with entitlements |
| **Entitlement** | A capability a plan grants (AI grading quota, exam count, deck limit) |
| **Subscription** | A learner's active plan with a period and status |
| **Invoice** | A billing document |
| **Payment** | A gateway transaction |
| **Grace period** | Time after a failed renewal during which access continues |

## 9. Naming rules

| Rule | Example |
|---|---|
| Use the glossary term exactly, in every layer | `review_cards`, `ReviewCard`, `/reviews`, "Review" in the UI |
| Never introduce a synonym for an existing term | Not `flashcard` if `review card` exists |
| Never overload a term across domains | "module" is code-only; use "unit" for content |
| Abbreviations only when they are the domain's own | `CEFR`, `FSRS`, `XP`, `ASR`, `TTS`, `SRS` are fine; `sub` for submission is not |
| A new concept needs an entry here **before** it appears in code | Add it in the same PR |
