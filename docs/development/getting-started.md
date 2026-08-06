---
doc_type: guide
task: getting_started
last_verified: 2026-08-06
---

# Getting started

**Goal: from `git clone` to seeing your own request as a trace, a log line and a metric — in 15 minutes.**

That specific goal is not arbitrary. If you can follow one request through all three signals on
day one, you can debug anything the system does later. If you cannot, fix that before writing
code — it is the highest-value 15 minutes in the project.

---

## Prerequisites

| Tool | Version | Check |
|---|---|---|
| Go | 1.25+ | `go version` |
| Node | 24+ | `node --version` |
| pnpm | 9+ | `pnpm --version` |
| Docker | with Compose v2 | `docker compose version` |
| make | any | `make --version` |

On Windows, use WSL2 or Git Bash. Docker Desktop needs at least 8 GB allocated for the full
stack including observability.

## 1. Clone and configure

```bash
git clone <repo> fluentra && cd fluentra
cp .env.example .env
```

Read `.env` once, top to bottom. It is the complete configuration surface, and knowing what
exists saves you from inventing a key later. Nothing needs changing for local development.

## 2. Install tooling

```bash
make setup
```

This installs `sqlc`, `goose`, `oapi-codegen`, `moq`, `golangci-lint`, `go-arch-lint`,
`govulncheck` and `air`, then the frontend dependencies and the git hooks.

## 3. Start everything

```bash
make dev
```

| Service | URL | Notes |
|---|---|---|
| Web app | http://localhost:5173 | Vite dev server with HMR |
| API | http://localhost:8080 | `air` hot reload |
| API docs | http://localhost:8080/docs | Rendered from the OpenAPI spec |
| Grafana | http://localhost:3000 | `admin` / `admin` |
| Mailpit | http://localhost:8025 | Catches all outbound email |
| MinIO console | http://localhost:9001 | `minioadmin` / `minioadmin` |
| Jaeger (dev only) | http://localhost:16686 | Convenience UI; production uses Tempo |
| River UI | http://localhost:8081 | Job queue inspection |
| Adminer | http://localhost:8082 | Database browsing |

First start takes a few minutes while images build. Subsequent starts are seconds.

## 4. Seed data

```bash
make migrate-up
make seed
```

Demo accounts: `learner@fluentra.dev` / `admin@fluentra.dev`, password `Password123!demo`.
These exist only in the development seed; there is no default password anywhere else.

## 5. The 15-minute exercise — follow one request

1. Sign in as the learner at http://localhost:5173.
2. Open the dashboard. Note the `X-Request-Id` response header in the browser devtools
   network panel.
3. **Trace:** open Grafana → Explore → Tempo, search by that trace ID. You should see the root
   HTTP span, the service spans, and the pgx spans beneath them, with their timings.
4. **Logs:** switch the datasource to Loki, query `{service="fluentra-api"} |= "<trace_id>"`.
   The same request's log lines carry the trace ID. Click it — Grafana links back to the trace.
5. **Metrics:** switch to Prometheus, query
   `histogram_quantile(0.95, sum by (le, route) (rate(http_server_request_duration_seconds_bucket[5m])))`.
   Find the route you just hit.

If any of those three steps does not work, that is a bug in the observability setup and it is
worth more of your attention right now than any feature.

## 6. Verify the toolchain

```bash
make check
```

Runs formatting, vet, linting, **boundary checking** and unit tests — the same things CI runs.
It should be green on a fresh clone. If it is not, the problem is your environment, and fixing
it now is cheaper than discovering it in a pull request.

## 7. Learn the map

Read in this order. Roughly an hour, and it saves days.

| # | Document | Why |
|---|---|---|
| 1 | [`/AGENT.md`](../../AGENT.md) | The rules, the layering, what is forbidden |
| 2 | [`/MODULE_INDEX.md`](../../MODULE_INDEX.md) | Where everything lives |
| 3 | [`/ARCHITECTURE.md`](../../ARCHITECTURE.md) §1–5 | How the pieces fit |
| 4 | [`/GLOSSARY.md`](../../GLOSSARY.md) | The vocabulary — read before naming anything |
| 5 | One module's `AGENT.md` — `internal/modules/srs/AGENT.md` is a good example | The shape of module documentation |
| 6 | [`/CONTRIBUTING.md`](../../CONTRIBUTING.md) | How we work |

Skip the rest until you need it. The documentation is designed for lookup, not for reading
end to end.

## 8. Your first change

Pick something small and real from a module's `TODO.md`. Follow
[`docs/guides/add-an-endpoint.md`](../guides/add-an-endpoint.md). Open a pull request using the
template. Expect the review to be about boundaries and tests, not style — style is automated.

---

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| `make dev` hangs on healthchecks | Docker memory too low; allocate 8 GB+ |
| API restart loop | Missing required config key — `docker compose logs api` names it |
| Migrations fail | A previous crashed run holds the lock; `go run ./cmd/migrate status` |
| No traces in Grafana | Collector endpoint wrong in `.env`, or the Collector is unhealthy; check its own logs |
| Frontend cannot reach the API | `CORS_ALLOWED_ORIGINS` does not include `http://localhost:5173` |
| `make check` fails on generated code | Run `make gen` — someone changed the spec or a query |
| Port already in use | Another project's stack is running; `docker ps` |
| Slow Docker on macOS/Windows | Enable VirtioFS / WSL2 backend |

More: [`troubleshooting.md`](troubleshooting.md).

## Do not

- Do not run `docker compose down -v` or `make db-reset-DANGEROUS` casually — they delete local
  learner data, including anything you were mid-way through testing.
- Do not commit `.env`.
- Do not point local development at a shared or production database. There is no reason to, and
  every reason not to.
