# Changelog

All notable changes to Fluentra are recorded here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) ·
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html) ·
Generated from Conventional Commits by `git-cliff`, then **edited by a human** before release —
generated text describes commits; release notes should describe change.

---

## [Unreleased]

### Added

- Complete Software Architecture Document ([ARCHITECTURE.md](ARCHITECTURE.md))
- Plan review and optimisation record ([docs/architecture/00-plan-review.md](docs/architecture/00-plan-review.md))
- AI context engineering strategy ([AI_CONTEXT.md](AI_CONTEXT.md)) and root [AGENT.md](AGENT.md)
- 30 module specifications with the nine-file documentation set each
- 20 Architecture Decision Records
- Repository conventions: coding, API, database, errors, logging, testing, security, observability
- Prompt library design, development and runtime ([PROMPT_LIBRARY.md](PROMPT_LIBRARY.md))
- Dependency register with alternatives and rationale ([DEPENDENCIES.md](DEPENDENCIES.md))
- Delivery roadmap through Phase 5 ([ROADMAP.md](ROADMAP.md))
- Module documentation generator (`tools/docgen`) with drift checking
- Module boundary enforcement configuration (`.go-arch-lint.yml`)
- Configuration reference (`.env.example`) and `Makefile`
- The `core` identity schema: `users`, `profiles`, `user_preferences`, `learning_profiles`
- `GET` and `PATCH /api/v1/me` — read and update your own profile
- `GET` and `PUT /api/v1/me/preferences` — read and replace your own settings
- Roles and permissions: the `core` tables, the seeded two-role catalogue, and the guard
- `GET /api/v1/me/permissions` — what the caller is allowed to do
- `GET /api/v1/admin/roles`, and granting and revoking a user's roles
- The append-only audit trail: `audit_logs` and `security_events`, partitioned by month, with
  the application role holding `INSERT` and `SELECT` and nothing else
- `GET /api/v1/admin/audit-logs` and `GET /api/v1/admin/security-events` — search the trail and
  the security feed, filtered and paged
- `POST /api/v1/admin/security-events/{id}/resolve` — mark an event triaged, with a required
  reason
- An outbox consumer that turns the events `user` and `rbac` already publish into audit
  entries, exactly once per event
- Scheduled partition rotation and two-year retention

- `POST /api/v1/auth/register`, `/verify`, `/resend` — user registration & email verification flow
- `POST /api/v1/auth/login` — authentication with Argon2id timing equalisation and per-account/IP lockout protection
- The identity modules wired into the running API and worker: every operation above is now
  mounted, and every audited write reaches `audit_logs` through the worker
- `POST /api/v1/auth/refresh` — exchange the refresh cookie for a new access token. Signing in
  and verifying an address now also set that cookie, so a session outlives the fifteen-minute
  access token without the learner re-entering a password
- Refresh tokens rotate on every use and are single-use. Presenting one that has already been
  spent revokes every token in its family, revokes the session, and raises a `refresh_reuse`
  security event — so a stolen token is detected the moment either party uses it twice, at the
  cost of signing the legitimate learner out alongside the thief
- `core.sessions` and `core.refresh_tokens`. Sessions record a keyed digest of the client
  address, never the address

### Fixed

- Outbox events were published under a doubled topic (`user.user.profile_updated`), so no
  consumer could ever match one. Because an event with no handlers is accepted rather than
  retried, every event published since the `user` module landed was marked delivered and
  discarded. Nothing had subscribed yet, so nothing noticed.

- Outbox events carry the producing transaction's W3C `traceparent`, so work done in the worker
  continues the trace of the request that caused it. An audit entry now records the trace of
  the action rather than of the worker that filed it (BR-AUDIT-07)

### Notes

Authentication is live: P2.4 added the bearer middleware and P2.5 the refresh cookie behind it,
so the operations above are usable by a real client. What is still missing is `POST
/auth/logout` and the session list — P2.6. See [ROADMAP.md](ROADMAP.md).

A refresh token is deliberately **not** in any response body. It exists only as an `HttpOnly`
cookie scoped to `/api/v1/auth`: a value the page can read is a value an injected script can
steal, and unlike an access token it is renewable indefinitely.

Audit entries record **which fields changed, not what they changed to**, and redact anything on
the PII deny-list if a value is supplied. An audit log holding a copy of every old display name
would be a second store of personal data with a longer retention period than the first. Client
addresses are stored as a keyed HMAC and never in the clear.

---

## How to write an entry

| Section | Use for |
|---|---|
| **Added** | New features and capabilities |
| **Changed** | Changes to existing behaviour |
| **Deprecated** | Soon-to-be-removed features, with a sunset date |
| **Removed** | Features removed in this release |
| **Fixed** | Bug fixes |
| **Security** | Vulnerability fixes — always call these out explicitly |
| **Breaking** | Anything requiring action from a client or operator |
| **Migration notes** | What an operator must do when deploying this version |

Write from the reader's point of view: *"Essay feedback now streams as it is generated"*,
not *"refactor writing grading to use SSE"*. The commit log already says the second thing.

Every user-visible change gets an entry under `Unreleased` **in the same pull request** that
makes the change. Adding them at release time means half of them are missed.
