# agentlsp — Go code intelligence for AI agents

Starts the `gopls` MCP server so AI agents (Claude Code, OpenCode/OpenChamber, Cursor…)
navigate Go code by symbol instead of grepping and reading whole files.

The repo already registers it: `.mcp.json` (Claude Code, Cursor) and `opencode.jsonc`
(OpenCode v2 / OpenChamber). Nothing to configure per machine.

| Tool | Use it for |
|---|---|
| `go_search` | find a type/function/method by (fuzzy) name |
| `go_symbol_references` | every use of a symbol, across all modules — run it before changing a shared symbol |
| `go_package_api` | the public API of a package without reading its files |
| `go_file_context` | what a file uses from the rest of its package |
| `go_diagnostics` | build/vet errors after an edit |
| `go_workspace` | modules in the workspace |

`go_rename_symbol` and `go_vulncheck` also exist; the multi-agent workflow keeps its workers
read-only (rename = normal edits; vulncheck needs network).

## Requirements

Only Go (the version in `go.mod`). The first start runs `go install` for the pinned `gopls`
into the **user cache** (`%LocalAppData%\agentlsp` on Windows, `~/Library/Caches/agentlsp` on
macOS, `~/.cache/agentlsp` on Linux): nothing is written to the repo and later starts take
seconds.

## Multi-module repos

With several `go.mod` files and no `go.work`, the launcher writes `.gopls/go.work`
(git-ignored) and passes it to gopls only via `GOWORK`, so references cross modules
(e.g. `db/sqlc` → `sync-service`). Normal `go build` / `go test` never see it and keep their
per-module dependency versions.

Machine budget (measured on the Hilo backend, 19 modules): about 750 MB RAM, ~8 s start with
a warm cache, 2–3 s per reference lookup. To stay lean, gopls runs with `CGO_ENABLED=0` and
skips modules under `tools/`; set `AGENTLSP_CGO=1` or `AGENTLSP_ALL_MODULES=1` to include them.

## Frontend

There is no reliable TypeScript MCP equivalent yet (an LSP bridge was tried: references did
not resolve). For `web/` use the project scripts — `pnpm -C web typecheck`, `pnpm -C web lint`,
`pnpm -C web test` — plus ripgrep.

## Upgrading

Bump `goplsVersion` in `main.go`; the new version installs next to the old one on next start.
