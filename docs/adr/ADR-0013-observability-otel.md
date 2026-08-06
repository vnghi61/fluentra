---
adr: 0013
title: "OpenTelemetry SDK with a Collector; Tempo over Jaeger"
status: Accepted
date: 2026-08-06
tags: [observability]
---

# ADR-0013: OpenTelemetry SDK with a Collector; Tempo over Jaeger

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | observability |

## Context

We need traces, metrics and logs, correlated, from Phase 0. The original brief listed both Jaeger and Tempo as trace backends.

## Decision

The application emits only OTLP to an OpenTelemetry Collector. The Collector routes to Prometheus, Loki and Tempo, and performs tail sampling and PII redaction. Jaeger runs only in the local development profile; production uses Tempo.

## Alternatives considered

### A. Direct exporters from the application

| | |
|---|---|
| **Pros** | One fewer component |
| **Cons** | Backend changes, sampling and redaction all become code changes and deploys |
| **Why rejected** | The Collector converts those into configuration. |

### B. Run both Jaeger and Tempo in production

| | |
|---|---|
| **Pros** | Two UIs |
| **Cons** | Two trace stores to operate and pay for, storing the same data; roughly 700 MB additional memory; no team benefit |
| **Why rejected** | Redundant. Tempo shares MinIO for storage and integrates natively with Grafana. |

### C. A commercial APM

| | |
|---|---|
| **Pros** | Least operational work; excellent UX |
| **Cons** | Per-host or per-GB pricing at a scale we cannot predict; vendor lock-in on instrumentation |
| **Why rejected** | OTLP keeps this reversible — we can point the Collector at a vendor later without touching code. |

## Consequences

### Positive

- Backend changes are Collector configuration
- One ingest point for redaction and sampling policy
- Metrics, logs and traces correlate in one Grafana instance
- A commercial APM remains a configuration change away

### Negative — accepted knowingly

- The Collector is another component to run and monitor
- Tail sampling costs memory in the Collector
- Grafana's trace UI is less polished than Jaeger's for some workflows — hence Jaeger in dev

## Compliance

A direct exporter in application code fails review. Cardinality is checked in review against the 10,000-series budget.

## Revisit when

If self-hosting the Grafana stack costs more engineering time than a commercial APM would cost in licence fees.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
