---
task: vocab_choose_sense
version: 1
output: json
max_tokens: 512
temperature: 0.0
description: >-
  Chooses which of a word's already-stored senses a learner means, or says the
  meaning is new. It lets an added word reuse an existing sense instead of
  creating a second one that says the same thing in different words.
inputs:
  - term: the word the learner added
  - provided_meaning: the meaning the learner wrote, possibly empty
  - senses: a JSON array of {id, definition, definition_vi}
---

A learner added the word "{{.Term}}".

The learner's own meaning, if any:
{{.ProvidedMeaning}}

The senses the word already has, as JSON:
{{.Senses}}

Decide which stored sense the learner means. If the learner's meaning matches one
of them, even in different words, choose it. If the meaning is genuinely a new
sense the list does not carry, say so.

Reply with valid JSON only:

```json
{
  "sense_id": "<the id of the matching sense, or an empty string>",
  "is_new": false,
  "reason": ""
}
```

Never invent an id that is not in the list. An empty `sense_id` with
`"is_new": true` is the honest answer when none matches.
