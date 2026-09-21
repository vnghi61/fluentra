---
doc_type: handoff
phase: 4
status: planned
last_verified: 2026-09-20
---

# Phase 3 — work order 16

**Purpose.** Build the knowledge spine: one identifier for each piece of English knowledge, a
prerequisite graph over those identifiers, and the shape a Foundation topic takes. This is step P1 of
[the phase 4 plan](phase-4-plan.md), and every later step depends on it.

**This work order deliberately ships no Foundation prose.** It seeds roughly 69 topic *codes* covering
the brief's five strands, the edges between them, and the schema a topic's content must satisfy. Writing
the explanations, examples and exercises is P5 (work order 20), which is a content project with an
editorial owner. Shipping the spine first means P5, P6 and P7 can start in parallel against a stable
vocabulary instead of inventing one each.

**Read first.** [Phase 4 plan](phase-4-plan.md) §§1, 3, 6.
[ADR-0015](../adr/ADR-0015-content-exercise-core.md),
[ADR-0025](../adr/ADR-0025-anonymous-curriculum-access.md), and the `AGENT.md` of `content`, `lesson`
and `grammar`.

**Do not start by writing a module.** This is an extension of `content`, which already owns
`content.taxonomies`, `content_tags`, `content_items` and `content_versions`. No new module, no new
schema.

---

## 1. What exists today

Checked on 2026-09-20 against `feat/phase-3-work-order-15` at `abbe8af`.

| Piece | State | Where |
|---|---|---|
| `content.taxonomies` | **Built.** `id`, `namespace`, `code`, `label`, `parent_id` (self-FK, `ON DELETE SET NULL`), timestamps. `UNIQUE (namespace, code)`. Length checks on all three text columns | `db/migrations/content/1700000190`, lines 133-152 |
| Seeded rows | 20, all `namespace = 'course_topic'`, codes in **kebab-case** (`business-english`) | `1700000746_seed_topic_taxonomies.sql` |
| `content.content_tags` | **Built.** `(item_id, taxonomy_id)` composite PK, both FKs `ON DELETE CASCADE` | `1700000190`, lines 154-165 |
| `content.content_items` / `content_versions` | **Built.** `kind` (free text, 1-50 chars), `slug` (kebab-case CHECK), jsonb `body` with a GIN index, `cefr_level` with a `^(A1\|A2\|B1\|B2\|C1\|C2)$` CHECK, immutable once published | `1700000190` |
| Cycle detection | **Built.** `DetectCycle(adj map[uuid.UUID][]uuid.UUID) bool`, 3-colour DFS, handles self-edges | `internal/modules/lesson/domain/graph.go` |
| Permissions | `content.read.published`, `content.create`, `content.edit` all exist | `rbac/contract/permissions.go` |
| Anonymous published reads | **Built and specified** | ADR-0025 |
| Highest migration | `1700000775` | `db/migrations/studio/` |

**What follows.** The table you need exists, is namespaced, is hierarchical, and has exactly one row type
in it. Adding a second namespace is not a schema change. The only genuinely new table in this work order
is the prerequisite edge list, because a prerequisite is not the relation `parent_id` encodes.

---

## 2. The one distinction that matters

`parent_id` is **containment**. `taxonomy_prerequisites` is **ordering**. They are different relations
over the same nodes, and collapsing them is the mistake this work order exists to prevent.

```text
parent_id (a tree)                 prerequisites (a DAG)

TENSES                             SENTENCE_STRUCTURE
├── PRESENT_SIMPLE                   └── PRESENT_SIMPLE
├── PRESENT_CONTINUOUS                     ├── PRESENT_CONTINUOUS
├── PAST_SIMPLE                            └── PAST_SIMPLE
└── PRESENT_PERFECT                              └── PRESENT_PERFECT
```

Both diagrams describe the same five nodes. The left one says where they live; the right one says what a
learner must meet first. The brief's §1 example —
`Tenses → Present Simple → Present Continuous → Past Simple → Present Perfect → Future forms` — is the
right-hand relation, and its members all share the same parent. A single column cannot carry both, and a
design that tries will produce a learning path that walks the containment tree and teaches the present
perfect before the past simple.

---

## 3. Decisions

| # | Decision | Why |
|---|---|---|
| D16-1 | Exercises, quiz items and review questions are **separate `content_items` tagged to the node**, not embedded in the topic's body | The brief's §12 needs an exercise about `PRESENT_PERFECT` to be drawable by the practice pool and the Question Bank. A body-embedded exercise is invisible to both. "Every Foundation item must contain exercises" becomes a **completeness gate at publish** (BR-FOUNDATION-05) rather than a nesting |
| D16-2 | CEFR lives on `content_versions.cefr_level` (a column that already exists and is already constrained); the prerequisite lives on the edge table. **Neither is repeated in the body** | Two sources of truth for a topic's difficulty is how a topic ends up A2 in one query and B1 in another |
| D16-3 | Codes are `SCREAMING_SNAKE` for the five new namespaces, and the existing kebab-case `course_topic` rows are left alone | A single global format CHECK would fail the migration against 20 live rows. See Trap 1 |
| D16-4 | A node is **deprecated, never renamed or deleted** (`deprecated_at`) | The code is a foreign key in spirit: content, questions and mastery rows point at it. See Trap 6 |
| D16-5 | `PHRASAL_VERBS` is **one node, in `grammar`**, and the vocabulary strand reaches it as a related topic | The brief lists it under both Grammar and Vocabulary. Two nodes would split its content and its mastery in half |
| D16-6 | Browse and read are anonymous per ADR-0025; authoring is behind `content.create` / `content.edit` | The Foundation is the thing that should be reachable before sign-up |

---

## 4. Scope

**In.**

1. Migration: the prerequisite edge table, five new namespaces, four new columns on `content.taxonomies`.
2. Domain: the node, the DAG, cycle refusal, topological ordering for a learning path.
3. Repository and service: browse, read one, path to a target, author a node, author an edge.
4. Six HTTP endpoints, specified in `openapi.yaml` **before** any handler exists.
5. The `foundation_topic` body JSON Schema, versioned.
6. A seed of about 69 nodes and their edges, covering every topic the brief's §1 names.
7. `content/AGENT.md`, `API.md`, docgen data, and this module's `TODO.md`.

**Out.**

- Foundation prose, exercises and quizzes — P5 / work order 20.
- Any web UI. The endpoints are consumed by P5 and P12; a browse screen is its own work order.
- `skill.grammar_points` (D2 of the plan) — the `grammar` module is built in its own work order and
  references these nodes when it is.
- Tagging existing pool content onto nodes — a backfill job, and it belongs with P6.

---

## 5. The migration

`db/migrations/content/1700000780_foundation_spine.sql`

```sql
-- +goose Up
-- +goose StatementBegin

ALTER TABLE content.taxonomies
    ADD COLUMN IF NOT EXISTS description   text        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cefr_level    text,
    ADD COLUMN IF NOT EXISTS position      integer     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS deprecated_at timestamptz;

-- Same enum as content_versions.cefr_level, and nullable: a strand root such as
-- TENSES has no single level, while PRESENT_PERFECT does.
ALTER TABLE content.taxonomies
    ADD CONSTRAINT ck_taxonomies_cefr_level
    CHECK (cefr_level IS NULL OR cefr_level ~ '^(A1|A2|B1|B2|C1|C2)$');

-- Namespaces this system knows. Additive: a new strand is a migration, not a typo.
ALTER TABLE content.taxonomies
    ADD CONSTRAINT ck_taxonomies_namespace
    CHECK (namespace IN ('course_topic', 'grammar', 'vocabulary',
                         'pattern', 'pronunciation', 'skill'));

-- Code format, per namespace. The 20 course_topic rows seeded by 1700000746 are
-- kebab-case ('business-english'); the spine namespaces are SCREAMING_SNAKE. One
-- global pattern would refuse the rows already in the table.
ALTER TABLE content.taxonomies
    ADD CONSTRAINT ck_taxonomies_code_format
    CHECK (
        (namespace = 'course_topic' AND code ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
        OR
        (namespace <> 'course_topic' AND code ~ '^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$')
    );

-- ------------------------------------------------- taxonomy_prerequisites
--
-- "node_id requires requires_node_id first." A DAG, not a tree: PRESENT_PERFECT
-- requires both PAST_SIMPLE and PRESENT_SIMPLE, and REPORTED_SPEECH requires
-- PAST_SIMPLE through a different chain. parent_id cannot express either.
--
-- RESTRICT on requires_node_id, not CASCADE: silently dropping the edge that
-- says "learn this first" is how a learning path quietly starts teaching the
-- present perfect to somebody who has not met the past simple.
CREATE TABLE IF NOT EXISTS content.taxonomy_prerequisites (
    node_id          uuid        NOT NULL,
    requires_node_id uuid        NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (node_id, requires_node_id),
    CONSTRAINT fk_taxonomy_prereq_node
        FOREIGN KEY (node_id) REFERENCES content.taxonomies (id) ON DELETE CASCADE,
    CONSTRAINT fk_taxonomy_prereq_requires
        FOREIGN KEY (requires_node_id) REFERENCES content.taxonomies (id) ON DELETE RESTRICT,
    CONSTRAINT ck_taxonomy_prereq_not_self CHECK (node_id <> requires_node_id)
);

CREATE INDEX IF NOT EXISTS idx_taxonomy_prereq_requires
    ON content.taxonomy_prerequisites (requires_node_id);

CREATE INDEX IF NOT EXISTS idx_taxonomies_namespace_position
    ON content.taxonomies (namespace, position) WHERE deprecated_at IS NULL;

-- +goose StatementEnd
```

The `Down` drops the table, the index and the four constraints, and drops the four columns.

**The CHECK constraint cannot catch a cycle.** Postgres has no cheap way to reject one declaratively, so
the DAG invariant is enforced in the service (step 3) and asserted by an integration test that tries to
build one. That is a known, stated limit, not an oversight — write it in `DECISIONS.md`.

---

## 6. The topic body

`content_items.kind = 'foundation_topic'`. `content_versions.body`:

```json
{
  "schema_version": 1,
  "objective": "Use the present perfect for experience and for unfinished time.",
  "explanation": { "en": "...", "vi": "..." },
  "examples": [
    { "text": "I have lived here for ten years.", "note": "unfinished time" }
  ],
  "related": ["PAST_SIMPLE", "PRESENT_PERFECT_CONTINUOUS"],
  "common_mistakes": [
    { "wrong": "I have seen him yesterday.", "right": "I saw him yesterday.", "why": "..." }
  ]
}
```

Seven of the brief's nine required fields are here or adjacent:

| Brief's field | Where it lives |
|---|---|
| learning objective | `body.objective` |
| explanation | `body.explanation` |
| examples | `body.examples` |
| difficulty / CEFR | `content_versions.cefr_level` (D16-2) |
| prerequisite | `content.taxonomy_prerequisites` (D16-2) |
| related topics | `body.related`, codes in the same namespace |
| exercises | `content_items` of an exercise kind tagged to this node (D16-1) |
| quiz | same, `kind = 'foundation_quiz'` |
| review questions | same, `kind = 'foundation_review'` |

Validate the body against a schema in `internal/modules/content/domain/`, versioned by
`schema_version`, in the same style the pools validate a generated item. An unknown `schema_version` is
refused, not ignored.

---

## 7. The API

**Write `api/openapi/openapi.yaml` first.** Tag every operation `content` — the drift check
(`tools/docgen/check-drift.mjs`) maps paths to modules by tag, and a `foundation` tag would send it
looking for a module that does not exist. Spectral requires `x-permission` (or `security: []`), a
description and a response example on every operation.

| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/foundation/topics` | `content.read.published` | Browse the spine. Filters: `namespace`, `cefr`, `parent`, `q`. Paginated |
| `GET` | `/api/v1/foundation/topics/{code}` | `content.read.published` | One topic: body, prerequisites, dependants, related, and the counts of exercises, quiz items and review questions attached |
| `GET` | `/api/v1/foundation/path` | `content.read.published` | `?target=PRESENT_PERFECT` returns the ordered prerequisite chain. `?namespace=grammar` returns a full ordering of the strand |
| `POST` | `/api/v1/admin/foundation/topics` | `content.create` | Create a node |
| `PATCH` | `/api/v1/admin/foundation/topics/{code}` | `content.edit` | Label, description, CEFR, position, deprecation. **Never the code** |
| `PUT` | `/api/v1/admin/foundation/topics/{code}/prerequisites` | `content.edit` | Replace a node's prerequisite set; refuses a cycle with 422 |

The three read endpoints follow ADR-0025 for anonymous access, exactly as the published-curriculum reads
already do. Deprecated nodes are excluded from browse by default and still resolve by code, because
content published against them must keep rendering.

New error codes owned by `content`:

| Code | Status | Meaning |
|---|---|---|
| `TAXONOMY_NODE_NOT_FOUND` | 404 | Unknown namespace/code pair |
| `TAXONOMY_CYCLE` | 422 | The proposed prerequisite set would create a cycle; the response names the offending chain |
| `TAXONOMY_CODE_IMMUTABLE` | 422 | An attempt to change a code (D16-4) |

---

## 8. Steps

1. **Migration** (§5). Run it up and down twice against a real database before writing Go.
2. **OpenAPI** (§7), then `make gen` and `make gen-check`. The generated types are what the handlers use.
3. **Domain.** `content/domain/taxonomy.go` already exists — extend it. Add the node, the edge, and:
   - `DetectCycle` over the **proposed** graph, meaning the existing edges plus the ones being written,
     evaluated before the write. Reuse the 3-colour algorithm in `lesson/domain/graph.go`; copy it into
     `content`'s domain rather than importing across a module boundary (rule L1).
   - `TopologicalOrder(nodes, edges) ([]Node, error)` — a stable ordering, ties broken by
     `(position, code)` so the same input always produces the same path.
   - `PathTo(target)` — the transitive closure of the target's prerequisites, topologically ordered.
4. **Repository.** `db/queries/content/taxonomy.sql`, sqlc-generated. Match the physical column order of
   `content.taxonomies` in every projection so sqlc reuses the table row type instead of emitting a row
   type per query.
5. **Service.** Browse, read, path, create, update, replace-prerequisites. The cycle check and the code
   immutability check live here.
6. **Transport.** Handlers, DTOs, routes. Guard each admin route with its permission.
7. **Seed** (§9). A migration, not `cmd/seed`: these codes are referenced by later migrations and by the
   Question Bank, so they must exist in every environment including CI.
8. **Docs.** `tools/docgen/data/*.json` for the endpoint table, then `make docs`. Update `content`'s
   `AGENT.md` §5 and §9, `API.md`, `DECISIONS.md` (the cycle-check-in-service decision), and `TODO.md`.
9. **Verify.** `make check`, then `make lint` — Spectral over `openapi.yaml` is the step that gets
   skipped and fails CI.

---

## 9. The seed

`db/migrations/content/1700000781_seed_foundation_spine.sql`. About 69 nodes. This list is the brief's §1
in full — it is not a sample, and nothing in it is invented.

**`grammar`** (36). Roots: `SENTENCE_STRUCTURE`, `PARTS_OF_SPEECH`, `TENSES`, `CLAUSES`.

`SENTENCE_STRUCTURE`, `SUBJECT_VERB_OBJECT`, `PARTS_OF_SPEECH`, `NOUNS`, `ARTICLES`, `PRONOUNS`,
`ADJECTIVES`, `ADVERBS`, `PREPOSITIONS`, `CONJUNCTIONS`, `MODAL_VERBS`, `QUESTIONS_AND_NEGATIVES`,
`COMPARATIVES_AND_SUPERLATIVES`, `PASSIVE_VOICE`, `CONDITIONALS`, `REPORTED_SPEECH`, `RELATIVE_CLAUSES`,
`NOUN_CLAUSES`, `ADVERBIAL_CLAUSES`, `GERUNDS_AND_INFINITIVES`, `PARTICIPLES`, `PHRASAL_VERBS`,
`COMMON_GRAMMAR_MISTAKES`, `CLAUSES`.

Under `TENSES`, all twelve — the brief says "all major English tenses" and twelve is the closed set:
`PRESENT_SIMPLE`, `PRESENT_CONTINUOUS`, `PRESENT_PERFECT`, `PRESENT_PERFECT_CONTINUOUS`, `PAST_SIMPLE`,
`PAST_CONTINUOUS`, `PAST_PERFECT`, `PAST_PERFECT_CONTINUOUS`, `FUTURE_SIMPLE`, `FUTURE_CONTINUOUS`,
`FUTURE_PERFECT`, `FUTURE_PERFECT_CONTINUOUS`.

**`vocabulary`** (8): `ESSENTIAL_EVERYDAY`, `COMMON_VERBS`, `HIGH_FREQUENCY_WORDS`, `TOPIC_VOCABULARY`,
`COLLOCATIONS`, `SYNONYMS_AND_ANTONYMS`, `ACADEMIC_VOCABULARY`, `WORKPLACE_VOCABULARY`.
Phrasal verbs are `grammar.PHRASAL_VERBS` (D16-5), reached from `TOPIC_VOCABULARY` as a related topic.

**`pattern`** (13): `INTRODUCING_YOURSELF`, `ASKING_QUESTIONS`, `ANSWERING_QUESTIONS`, `REQUESTING`,
`OFFERING`, `SUGGESTING`, `AGREEING_AND_DISAGREEING`, `GIVING_OPINIONS`, `DESCRIBING`, `COMPARING`,
`EXPLAINING`, `ASKING_FOR_CLARIFICATION`, `DAILY_CONVERSATIONS`.

**`pronunciation`** (8): `ENGLISH_SOUNDS`, `IPA_BASICS`, `WORD_STRESS`, `SENTENCE_STRESS`, `LINKING`,
`REDUCTIONS`, `INTONATION`, `COMMON_PRONUNCIATION_MISTAKES`.

**`skill`** (4): `LISTENING`, `READING`, `SPEAKING`, `WRITING`.

**The edges the brief names explicitly**, which are the acceptance test for §11:

```text
SENTENCE_STRUCTURE → PRESENT_SIMPLE → PRESENT_CONTINUOUS → PAST_SIMPLE
                  → PRESENT_PERFECT → FUTURE_SIMPLE
SENTENCE_STRUCTURE → RELATIVE_CLAUSES → NOUN_CLAUSES → ADVERBIAL_CLAUSES
```

Every other edge is a judgement call. Seed the obvious ones (`PARTS_OF_SPEECH` before everything that
names a part of speech; `PRESENT_PERFECT` requires `PAST_SIMPLE`; `PASSIVE_VOICE` requires `PARTICIPLES`;
`REPORTED_SPEECH` requires `PAST_SIMPLE`; `CONDITIONALS` requires `PAST_SIMPLE` and `MODAL_VERBS`;
`IPA_BASICS` before `ENGLISH_SOUNDS`), and leave the rest empty rather than guessing. A missing edge
makes a path shorter than ideal; a wrong edge makes it wrong, and nobody will notice for months.

---

## 10. Business rules

1. **BR-FOUNDATION-01** — A node's `code` is immutable. Correcting one means deprecating the node and
   adding a new one; the old one keeps resolving so published content keeps rendering.
2. **BR-FOUNDATION-02** — The prerequisite graph is acyclic. A write that would close a cycle is refused
   with `TAXONOMY_CYCLE` and names the chain.
3. **BR-FOUNDATION-03** — A prerequisite edge joins two nodes in the **same namespace**. Cross-strand
   ordering ("learn IPA before the present perfect") is a claim nobody can defend; use `related` instead.
4. **BR-FOUNDATION-04** — A node with dependants cannot be deleted, only deprecated. The FK is `RESTRICT`
   so the database holds the line when the service forgets.
5. **BR-FOUNDATION-05** — A `foundation_topic` cannot be **published** unless at least one exercise, one
   quiz item and one review question are tagged to its node. This is the brief's "every Foundation item
   must contain…" enforced at the gate rather than by nesting (D16-1).
6. **BR-FOUNDATION-06** — A learning path returns nodes in topological order, and the order is stable
   across calls for the same input.
7. **BR-FOUNDATION-07** — Deprecated nodes are excluded from browse and from generated paths, and still
   resolve by code.

---

## 11. Testing

The acceptance test for this work order is the brief's own example:

```text
GET /api/v1/foundation/path?target=PRESENT_PERFECT
  → SENTENCE_STRUCTURE, PRESENT_SIMPLE, PRESENT_CONTINUOUS, PAST_SIMPLE, PRESENT_PERFECT

GET /api/v1/foundation/path?target=ADVERBIAL_CLAUSES
  → SENTENCE_STRUCTURE, RELATIVE_CLAUSES, NOUN_CLAUSES, ADVERBIAL_CLAUSES
```

Also required:

- **Unit.** `DetectCycle` on the proposed graph: self-edge, two-node cycle, three-node cycle, a diamond
  (which is legal and must pass). `TopologicalOrder` stability: same input, same output, twice.
- **Unit.** Body schema validation: every required field missing in turn; an unknown `schema_version`
  refused.
- **Integration.** The migration up, down, up. The seed is idempotent (`ON CONFLICT DO NOTHING`) and the
  second run changes no rows.
- **Integration.** `PUT .../prerequisites` closing a cycle returns 422 and **writes nothing** — assert the
  edge count is unchanged, not just the status code.
- **Integration.** `DELETE` on a node with dependants is refused by the FK.
- **Integration.** Publishing a topic with no exercises is refused (BR-FOUNDATION-05).
- **Contract.** Every one of the six endpoints against the generated types.

---

## 12. Traps

1. **The kebab-case rows.** `content.taxonomies` holds 20 `course_topic` codes like `business-english`.
   A global `SCREAMING_SNAKE` CHECK fails the migration on existing data. The constraint is
   namespace-conditional (§5) — and test the migration against a database that has been seeded, not an
   empty one.
2. **Checking the cycle after inserting it.** Insert, then check, then roll back leaves a window where a
   concurrent read sees a cyclic graph, and a topological sort over a cyclic graph does not return an
   error unless it is written to. Build the proposed adjacency in memory, check it, then write inside the
   same transaction.
3. **`parent_id` used as a prerequisite.** See §2. If a reviewer sees a path built by walking parents,
   the work order has been misread.
4. **`content_tags` cascades.** Its FK to `taxonomies` is `ON DELETE CASCADE` — deleting a node silently
   untags every piece of content pointing at it. BR-FOUNDATION-04 and `deprecated_at` exist because of
   this; do not "fix" it by loosening the restriction on the prerequisite FK.
5. **sqlc row types.** Widening a projection without matching the physical column order makes sqlc emit
   a per-query row type instead of reusing `ContentTaxonomy`, and the compile errors land far from the
   cause.
6. **Renaming a code during review** because a better name occurs to somebody. BR-FOUNDATION-01 and
   `TAXONOMY_CODE_IMMUTABLE` exist for the moment this feels harmless. It is harmless today and expensive
   in three months.
7. **Tagging by `taxonomy_id` in the seed.** The ids are generated; the codes are stable. Every seed
   statement resolves the id by `(namespace, code)` in a subquery so re-running against a database where
   a node already exists does not create a second one.
8. **The drift check.** Tag the operations `content`. A new tag sends `check-drift.mjs` looking for an
   `API.md` that does not exist, and the failure message does not say that.

---

## 13. Risks

| Risk | Mitigation |
|---|---|
| **The edges are wrong** and nobody notices until P12 builds paths on them | Seed only the edges the brief names plus the obvious ones (§9). An empty edge set is recoverable; a wrong one is invisible |
| **69 nodes is the wrong granularity** — too coarse to target a drill, too fine to teach | The tree is additive: a node can gain children later without moving its content. This is why codes are immutable and nodes are never deleted |
| **BR-FOUNDATION-05 blocks P5** — no topic can publish until it has exercises, and P5 writes both | Expected and correct. P5 publishes a topic and its exercises together; the gate is what stops a half-written Foundation reaching learners |
| **Cross-namespace prerequisites get requested** (BR-FOUNDATION-03 forbids them) | If the need is real it is a plan-level change: relax the rule with a written reason, do not special-case one edge |

---

## 14. What this work order does not answer

- Which CEFR level each node sits at. The column exists and is nullable; P5 fills it while writing the
  content, because the level of an explanation is a property of the explanation.
- How a learner's **mastery** of a node is measured. That is P12, and it consumes this spine rather than
  extending it.
- Whether `skill.grammar_points` (plan D2) duplicates anything here. It does not — it holds rule
  statements and error tags and points at these nodes — but the `grammar` work order is where that is
  proven.
