# feat(user): avatar upload via presigned URL with EXIF stripping and magic-bytes verification

## Summary

Implements P3.1 Avatar Upload in the `user` module:
- `POST /api/v1/me/avatar/upload-intent`: Issues a presigned upload policy for an avatar image (JPEG, PNG, or WebP; max 5 MB, 5-minute expiry).
- `PUT /api/v1/me/avatar`: Confirms and finalizes the avatar:
  1. Validates upload key belongs to actor (`users/{actor_id}/...`).
  2. Verifies raw upload existence, size limit (5 MB), and sniffed magic bytes via `storage.VerifyUpload` (rejecting renamed executables or non-image types).
  3. Decodes the image via Pure-Go library `github.com/disintegration/imaging` (with `golang.org/x/image/webp`), dropping all container EXIF metadata (GPS, camera tags).
  4. Resizes / crops image into three square avatar dimensions (`sm`: 64x64, `md`: 128x128, `lg`: 256x256) via `imaging.Fill(img, width, height, Center, Lanczos)`.
  5. Re-encodes fresh pure-Go JPEG streams (quality 85) and stores the three size variants (`_sm.jpg`, `_md.jpg`, `_lg.jpg`) in `storage.BucketAvatars` (`fluentra-avatars`).
  6. In a single database transaction, updates `core.profiles.avatar_asset_id` and writes `user.profile_updated` outbox event with changed field `avatar_asset_id`.
  7. Cleans up temporary raw upload object.
  8. Deletes previous avatar size variants from storage ONLY AFTER database commit succeeds.

## Architectural & Safety Considerations

### Non-Transactional Storage Lifecycle & Crash Safety
Object storage is non-transactional and cannot participate in database transactions (Rule L4: no cross-module / cross-service transactions). The lifecycle is designed to fail safe:
- **Upload / Verification failure**: DB remains untouched, previous avatar remains active, raw object is cleaned up by orphan GC.
- **Decoding / Resizing failure**: DB remains untouched, previous avatar remains active, raw object cleaned up by GC.
- **New avatar storage Put failure**: DB remains untouched, previous avatar remains active.
- **Database transaction failure / rollback**: Newly uploaded processed avatar object is deleted immediately from storage; DB rolls back; previous avatar remains active.
- **Crash after database commit**: If the process crashes after DB commit but before deleting the old avatar, the old avatar object remains in storage until the periodic storage garbage collection sweeps unreferenced asset IDs. The database and user-facing profile always remain consistent.

### Pure-Go Image Processing Rationale
- Pure-Go decoding via `github.com/disintegration/imaging` and `golang.org/x/image/webp` avoids CGo dependencies (such as `libvips` or `ImageMagick`), preserving single-binary Go compilation, static linking, and cross-platform simplicity across host, Docker, and CI.
- Documented in `DEPENDENCIES.md §1.16` with alternatives evaluated (`bimg`, `imagick`, `govips`).

## Verification & Quality Gates

- `make check`: PASSED (100% success across format, vet, golangci-lint, eslint, spectral OpenAPI linter, boundary enforcement, and race detector).
- `make arch`: PASSED (proven boundary enforcement).
- `make test`: PASSED (all unit tests pass).
- `make test-contract`: PASSED (OpenAPI schema contract tests pass).
- `make test-int`: PASSED (integration tests against PostgreSQL, Redis, and MinIO pass).
- `make cover-check`: PASSED (60.6% test coverage >= 60.0% minimum gate).
- `make docs-check`: PASSED (zero drift, zero markdownlint errors).
