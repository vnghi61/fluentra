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

### Notes

The four `/me` operations are specified and implemented but not yet reachable: nothing mounts
them, and there is no authentication to put a caller in the request context. Both arrive in
Phase 1 (P1.5 and P2.4). See [ROADMAP.md](ROADMAP.md).

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
