# `feat(outbox): prune published event payloads`

## Summary

- adds `OUTBOX_PUBLISHED_RETENTION_DAYS`, defaulting to 30, to the worker;
- runs a daily `platform/job` cron task under advisory lock `1_700_000_050`;
- deletes only published, non-dead-lettered outbox rows older than the configured
  window; and
- adds a matching partial index on `published_at`, avoiding a sweep over the
  historical unpublished/dead-letter paths.

## Design decision

This PR _deletes_ eligible rows rather than retaining rows and nulling their
payloads. Redaction would preserve a longer dispatch trail, but an expired OTP
and the associated email would remain coupled to an event record indefinitely.
Keeping the 30-day row gives operators a bounded delivery trail; deleting it
afterwards actually removes the personal payload. Pending rows remain deliverable
and dead-lettered rows remain available for failure triage.

The sweep is time-bounded and backed by
`idx_outbox_events_published_retention`, a partial index whose predicate exactly
matches the deletion query. This avoids turning a growing event history into a
daily full-table scan.

## Verification

- Linux container (`golang:1.26`, `fluentra-p14` network against PostgreSQL 17, Redis 7.4, MinIO):
  - `make arch` -> clean tree passes go-arch-lint with zero warnings.
  - `make test` -> unit tests with race detector pass.
  - `make test-int` -> integration test `TestOutboxRetention_PrunesOnlyPublishedRowsPastTheWindow` proves against PostgreSQL that published rows past retention are deleted, unpublished rows stay deliverable, and dead-lettered rows stay for triage.
  - `make test-contract` -> contract tests pass against OpenAPI bundle.
  - `make cover-check` -> 66.7% hand-written code coverage (exceeds 60.0% minimum gate).
- Linting:
  - `golangci-lint run ./...` -> 0 issues.
  - `golangci-lint run --build-tags=integration ./...` -> 0 issues.
- Documentation & codegen drift:
  - `node tools/docgen/check-drift.mjs` -> documentation drift check passed.
  - `node tools/docgen/generate.mjs --check` -> 30 modules, 0 files written.
  - `npx markdownlint-cli2@0.20.0` -> 365 files, 0 errors.
