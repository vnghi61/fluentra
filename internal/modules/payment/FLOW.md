---
module: payment
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@backend-team"
schema: billing
tables: [payments, invoices, payment_webhooks, refunds, checkout_sessions]
depends_on: [subscription, audit, job, mailer, storage]
depended_on_by: [subscription, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# payment — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Checkout and activation

```mermaid
sequenceDiagram
    autonumber
    actor U as Learner
    participant P as payment
    participant G as Gateway
    participant S as subscription
    participant M as mailer

    U->>P: POST /billing/checkout { plan } (Idempotency-Key)
    P->>S: validate the plan is purchasable for this learner
    P->>G: create hosted session
    G-->>P: session id + redirect URL
    P->>P: INSERT checkout_sessions
    P-->>U: { redirect_url }
    U->>G: completes payment on the gateway's page
    G->>P: webhook payment.succeeded
    P->>P: verify signature on the raw body
    P->>P: store raw webhook; ack 200 immediately
    P->>P: enqueue processing job
    Note over P: processing is idempotent on provider_event_id
    P->>P: INSERT payments, invoices; generate the PDF
    P->>S: publish payment.succeeded
    S->>S: activate the subscription; invalidate entitlements
    P->>M: receipt email
    U->>P: GET /billing/checkout/{id} → completed
```

## Failed renewal and dunning

```mermaid
flowchart TD
    A[Renewal charge fails] --> B[publish payment.failed]
    B --> C[subscription enters Grace, access continues]
    C --> D[dunning day 1: email]
    D --> E{recovered?}
    E -->|yes| F[payment.succeeded → Active]
    E -->|no| G[dunning day 3: email + in-app]
    G --> H{recovered?}
    H -->|yes| F
    H -->|no| I[dunning day 5: final notice]
    I --> J{recovered?}
    J -->|yes| F
    J -->|no| K[grace expires → Free<br/>no data deleted]
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
