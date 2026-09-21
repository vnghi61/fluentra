---
module: resource
tier: learning
group: modules
status: IMPLEMENTED
phase: 4
owner: "@learning-team"
schema: resource
tables: [resources, renditions, extractions, classifications]
depends_on: [storage, job, user]
depended_on_by: []
spec_version: 1.0.0
last_verified: 2026-09-21
---

# resource — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| One table or two? (D17-1) | One table, resource.resources | One upload is one file. P4 can add a child table when it finds per-item work. |
| Store a URL's body? (D17-2) | No, the title only | Plan D8. A study tool, not a copy of someone else's page. |
| Ever modify the original? (D17-3) | Never | Plan D9. Renditions are new objects in fluentra-derived. |
| Trust the declared type? (D17-4) | No, sniff the bytes | The content type is a string the client chose. |
| Where do limits live? (D17-5) | Go constants in resource/domain | The AvatarMaxBytes convention. UPLOAD_MAX_MB in .env.example is read by nothing. |
| Does delete remove the object? (D17-6) | Yes, in the same operation | Otherwise storage grows without bound and a deleted file still exists. |
| How is ownership enforced? (D17-7) | In the service, answering 404 | A 403 confirms the id exists; a check living in one SQL clause is one refactor from gone. |
| Where is the SSRF check? | In the dialer, on the connected address | Resolving first and connecting later lets a rebinding DNS server answer differently the second time; a proxy would bypass it entirely. |
| Which writes share a transaction with the job enqueue? | Confirm and URL submit, through explicit Tx repository methods | A row written outside the transaction survives a failed enqueue and is never validated. Explicit methods avoid a repository adapter that go-arch-lint deep scan refuses. |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
