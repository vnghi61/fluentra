---
module: vocabulary
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: skill
tables: [words, word_senses, word_relations, decks, deck_items, user_word_state]
depends_on: [content, srs, media, ai, search]
depended_on_by: [learning, reading, writing, grammar]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# vocabulary — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `vocabulary`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/vocabulary/words/{lemma}` | `content.read.published` | Dictionary lookup with senses, IPA, audio, examples |
| `GET` | `/api/v1/vocabulary/search` | `content.read.published` | Search the dictionary |
| `GET` | `/api/v1/vocabulary/decks` | `self` | The learner's decks plus curated ones |
| `POST` | `/api/v1/vocabulary/decks` | `self` | Create a deck |
| `POST` | `/api/v1/vocabulary/decks/{id}/words` | `self` | Add a word sense to a deck |
| `DELETE` | `/api/v1/vocabulary/decks/{id}/words/{sense_id}` | `self` | Remove |
| `POST` | `/api/v1/vocabulary/words/{sense_id}/state` | `self` | Mark known or ignored |
| `POST` | `/api/v1/admin/vocabulary/words` | `content.create` | Create a word entry |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/vocabulary/words/{lemma}`

Dictionary lookup with senses, IPA, audio, examples

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |


### `GET /api/v1/vocabulary/search`

Search the dictionary

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |


### `GET /api/v1/vocabulary/decks`

The learner's decks plus curated ones

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/vocabulary/decks`

Create a deck

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | `DECK_LIMIT_REACHED` |


### `POST /api/v1/vocabulary/decks/{id}/words`

Add a word sense to a deck

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | `WORD_ALREADY_IN_DECK` |


### `DELETE /api/v1/vocabulary/decks/{id}/words/{sense_id}`

Remove

| | |
|---|---|
| Permission | `self` |
| Success | 204 |
| Errors | standard set |


### `POST /api/v1/vocabulary/words/{sense_id}/state`

Mark known or ignored

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/admin/vocabulary/words`

Create a word entry

| | |
|---|---|
| Permission | `content.create` |
| Success | 201 |
| Errors | standard set |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `DECK_LIMIT_REACHED` | 409 | Plan limit on deck count |
| `DECK_SIZE_LIMIT_REACHED` | 409 | Plan limit on words per deck |
| `WORD_ALREADY_IN_DECK` | 409 | Duplicate sense in the deck |
| `WORD_NOT_FOUND` | 404 | No entry for that lemma |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
