---
doc_type: folder_index
folder: docs/security
last_verified: 2026-08-06
---

# docs/security — Security

## Purpose

Threat model, controls, and privacy operations.

## Contents

- `threat-model.md` — STRIDE analysis with per-threat controls
- `authentication.md` — token design, rotation, MFA, OAuth linking
- `authorization.md` — the three enforcement layers and why all three are needed
- `data-protection.md` — classification, encryption, retention, erasure
- `asvs-mapping.md` — OWASP ASVS L2 checklist and current status
- `secrets.md` — where secrets live and how each is rotated
- `ai-safety.md` — prompt injection, output validation, moderation, provider terms

## How AI agents should use this folder

Read before touching authentication, authorization, uploads, payments, or anything handling learner content. Security changes require a second reviewer.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
