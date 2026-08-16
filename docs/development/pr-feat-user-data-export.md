# feat(user): GDPR user data export with multi-module aggregation, ZIP packaging, and MinIO delivery

## Summary

Implements WP3 P3.2 User Data Export (GDPR Article 20 / Right to Data Portability) across backend monolith and workers:

- `POST /api/v1/me/export`: Requests personal data export (202 Accepted). Returns `ExportResponse` with `id`, `user_id`, `status: pending`, and timestamps. Enforces single pending/processing export rule per user, returning `409 Conflict` (`EXPORT_ALREADY_PENDING`) if an active request exists.
- `GET /api/v1/me/export/{id}`: Returns status of the data export request. Restricts access to the owning user.
- **Cross-Module Contract (`contract.Exportable`)**: Decoupled interface `ExportUserData(ctx context.Context, userID string) (map[string]interface{}, error)` implemented by:
  - `user`: Identity, status, profile, preferences, learning profiles, creation & update timestamps.
  - `auth`: Active & past sessions, linked OAuth identities (Google), MFA / security preferences.
  - `rbac`: Assigned roles, granular permissions, assignments metadata.
  - `audit`: Audit trail logs and events initiated by or concerning the user.
- **River Worker (`user.data_export`)**: Background worker executing in River `batch` queue:
  1. Updates export status to `processing` (`started_at = now()`).
  2. Aggregates JSON documents from all registered `Exportable` modules (`user.json`, `auth.json`, `rbac.json`, `audit.json`).
  3. Constructs a ZIP archive containing individual module JSON files and `metadata.json` (export ID, exported_at timestamp, system version).
  4. Uploads ZIP archive to private MinIO/S3 bucket `fluentra-exports` (`storage.BucketExports`) with object key `users/{user_id}/{yyyy}/{mm}/{export_id}.zip`.
  5. Generates a 24-hour presigned GET download URL (`storage.PresignGet`).
  6. Dispatches bilingual transactional email (`data_export` template in `en` and `vi`) containing learner display name, download link, and expiration notice.
  7. Updates export request to `completed`, recording `completed_at`, `object_key`, and `expires_at = now() + 7 days`.
  8. Idempotency: re-running a completed job is a safe no-op.
  9. Error Handling: failures mark status `failed` with recorded `error_message`.
- **Scheduled Retention Cleanup**: Daily cron job (`ops.user_export_cleaner`) identifies expired exports (`expires_at < now()`), purges the ZIP object from S3 storage, and hard-deletes the database record.

## Architecture & Boundaries

- **Boundary Compliance**: Adheres to `AGENT.md` rules L1–L20 and `.go-arch-lint.yml`.
  - `user/service` uses an enqueuer adapter to schedule River jobs within database transactions (`dbx.InTx`), completely decoupling domain service from background queue vendor code.
  - Cross-module data access strictly relies on `user/contract.Exportable` interfaces; modules never touch other modules' internal repositories or tables.
- **Storage & Security**:
  - ZIP archives are stored encrypted and private in MinIO/S3, never made public.
  - Download access is guarded via time-bounded 24-hour presigned URLs.
  - Retention is capped at 7 days, after which data is completely purged from both storage and database.

## Verification & Quality Gates

- `make check`: PASSED (vet, lint, format, boundary checks).
- `make arch`: PASSED (zero architecture violations).
- `make test`: PASSED (unit tests across service, job, domain, repository, and transport).
- `make test-contract`: PASSED (OpenAPI schema contract tests pass for `/me/export` and `/me/export/{id}`).
- `make test-int`: PASSED (integration tests against PostgreSQL, River, and MinIO).
- `make cover-check`: PASSED ($\ge 60\%$ coverage threshold met).
- `make docs-check`: PASSED (zero drift).
