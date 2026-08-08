---
module: ai
tier: platform
group: platform
status: PLANNED
phase: 3
owner: "@ai-team"
schema: ai
tables: [ai_requests, ai_usage, prompt_versions, ai_cache_entries, ai_budgets]
depends_on: [cache, telemetry, job]
depended_on_by: [writing, speaking, grammar, questionbank, content, reading, media, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# ai — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## A task call, end to end

```mermaid
flowchart TD
    A["ai.Client.Run(task, input)"] --> B{Task in routing config?}
    B -->|no| Z1[startup error — cannot happen at runtime]
    B -->|yes| C{User opted out?}
    C -->|yes| Z2[AI_OPTED_OUT]
    C -->|no| D{Quota + budget OK?}
    D -->|no| Z3[AI_QUOTA_EXCEEDED / AI_BUDGET_EXCEEDED]
    D -->|yes| E[Redact PII]
    E --> F[Wrap untrusted content]
    F --> G[Render prompt vN]
    G --> H{Cache hit?}
    H -->|exact| I[Return cached + record usage]
    H -->|semantic within threshold| I
    H -->|miss| J[Provider 1 via breaker]
    J -->|429/5xx/timeout| K{Retries left?}
    K -->|yes| J
    K -->|no| L{Next provider?}
    L -->|yes| J
    L -->|no| Z4[AI_UNAVAILABLE]
    J -->|response| M[Validate against output schema]
    M -->|invalid| N[One repair attempt]
    N -->|still invalid| Z5[AI_OUTPUT_INVALID]
    M -->|valid| O[Clamp + consistency check]
    O -->|out of bounds| Z5
    O --> P[Cache, record ai_requests + ai_usage, emit metrics]
    P --> Q[Return TaskResult]
```

## Provider fallback

Fallback is per task, not global — a cheap task may have no fallback at all, while grading has two.

```mermaid
sequenceDiagram
    autonumber
    participant C as Caller
    participant R as Resilience chain
    participant P1 as anthropic
    participant P2 as openai
    participant M as Metrics

    C->>R: Run(writing.grade_essay)
    R->>P1: request (timeout 30s)
    P1-->>R: 503
    R->>R: retry 1 after 1s ± jitter
    R->>P1: request
    P1-->>R: 503
    R->>R: breaker opens for anthropic
    R->>M: ai_provider_degraded, ai_fallback_total
    R->>P2: request with the same rendered prompt
    P2-->>R: 200
    R->>M: record provider=openai, fallback_from=anthropic
    R-->>C: TaskResult
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
## State machine

Prompt version lifecycle. A version never returns to an earlier state; rollback means activating the previous version, not editing this one.

```mermaid
stateDiagram-v2
    [*] --> Draft: author vN+1
    Draft --> Shadow: eval thresholds met + human approval
    Shadow --> Draft: comparison worse
    Shadow --> Rollout10: shadow results acceptable
    Rollout10 --> Active: metrics stable
    Rollout10 --> Draft: regression detected
    Active --> Deprecated: superseded by vN+2
    Deprecated --> [*]
```

<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
| Failure | Detected by | Behaviour |
|---|---|---|
| All providers down | `ai_requests_total{result="unavailable"}` | Job retries with backoff; the learner sees "still grading", not an error; pages after 5 minutes |
| Provider silently changes a model behind an alias | Nightly eval score shift | Alert; models are pinned to exact IDs to make this rare |
| Schema violations spike after a prompt change | `ai_schema_violation_total` | Automatic rollback to the previous active version at > 5 % over 30 minutes |
| Cost spike | `ai_cost_usd_total` rate | 80 % budget alert; at 100 % non-critical tasks are shed while grading continues |
<!-- END GENERATED: failures -->
