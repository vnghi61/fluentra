---
module: telemetry
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: []
depended_on_by: [ai, cache, storage, job, media, search, mailer]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# telemetry — Testing

Global policy: [`/TESTING_GUIDELINE.md`](../../../TESTING_GUIDELINE.md).

## Targets

| Layer | Target |
|---|---|
| `domain/` | 90 % |
| `service/` | 80 % |
| `repository/` | every query exercised by an integration test |
| `transport/http/` | every endpoint has a contract test |

## What to test

<!-- BEGIN GENERATED: test-focus -->
- Trace context propagates from HTTP through service, repository, job and AI call
- The redaction handler blocks a non-allowlisted attribute
- `/ready` fails correctly when Postgres or Redis is down
- Shutdown flushes pending spans
- Span names contain no IDs
<!-- END GENERATED: test-focus -->

## Edge cases that have bitten similar modules

<!-- BEGIN GENERATED: test-edges -->
_Add them here as you find them. This list is the module's institutional memory._
<!-- END GENERATED: test-edges -->

## Fixtures and test data

<!-- BEGIN GENERATED: test-fixtures -->
- Builders in `test/fixtures/builders/telemetry.go`
- Golden files in `internal/platform/telemetry/testdata/`
<!-- END GENERATED: test-fixtures -->

## Mocks

<!-- BEGIN GENERATED: test-mocks -->
This module has no outbound dependencies to mock.
<!-- END GENERATED: test-mocks -->

## Running

```bash
go test ./internal/platform/telemetry/...
go test -tags=integration ./internal/platform/telemetry/...
go test -run TestXxx -race -v ./internal/platform/telemetry/...
```

## AI-assisted test generation

Use `docs/prompts/dev/testing/generate-unit-test.md`. Give the agent this module's
`AGENT.md` §9 (business rules) as the oracle — **never the implementation**, or the test will
inherit the implementation's bugs.
