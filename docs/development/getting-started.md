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
| Web app | <http://localhost:5173> | React frontend running under Vite with HMR |
| API | <http://localhost:8080> | Go API server under `air` hot reload |
| Worker | <http://localhost:8081> | Background worker daemon under `air` hot reload (:8081 health/metrics) |
| API docs | <http://localhost:8080/docs> | Rendered from the OpenAPI spec |
| Grafana | <http://localhost:3000> | `admin` / `admin` (dashboards, logs, traces, metrics) |
| Mailpit | <http://localhost:8025> | Catches all outbound email |
| MinIO console | <http://localhost:9001> | `minioadmin` / `minioadmin` (S3 at :9000) |

First start takes a few minutes while images build. Subsequent starts are seconds.

Alternatively, to run application binaries directly on the host (with data and observability running via Compose):

```bash
make api     # runs API server on host
make worker  # runs worker daemon on host
make web     # runs Vite frontend on host
```

## 4. Seed data

```bash
make migrate-up
make seed
```

Demo accounts: `learner@fluentra.dev` / `admin@fluentra.dev`, password `Password123!demo`.
These exist only in the development seed; there is no default password anywhere else.

## 5. The 15-minute exercise — follow one request

This is the WP0 gate: **one** request must produce three signals that share a trace ID. The
steps below were run end to end and are written from what actually happened, not from what the
setup was meant to do.

There is no sign-in yet and no frontend route that calls the API on load, so the request is made
with `curl` rather than by clicking. `/api/v1/ping` exists precisely for this: it touches
PostgreSQL and Redis so the trace has children worth looking at.

```bash
curl -i http://localhost:8080/api/v1/ping
```

Note the `X-Request-Id` from the response headers. Wait about 15 seconds — the SDK batches, and
Prometheus scrapes every 15s — then:

**1. The log line (Loki).** `trace_id` and `request_id` arrive as OTLP structured metadata, not
as stream labels, so they are filtered *after* the stream selector. `{request_id="..."}` on its
own matches nothing:

```bash
curl -sG http://localhost:3100/loki/api/v1/query_range   --data-urlencode 'query={service_name="fluentra-api"} | request_id=`YOUR_REQUEST_ID`'
```

The line reads `request completed` and carries `trace_id`, `span_id`, `route`, `status` and
`duration_ms`.

**2. The trace (Tempo).** Take the `trace_id` from that log line:

```bash
curl -s http://localhost:3200/api/traces/YOUR_TRACE_ID | jq '[.. | .name? // empty] | unique'
```

Ten spans, and the three that matter are the HTTP root plus one child per dependency:

| Span | What it proves |
|---|---|
| `GET /api/v1/ping` | the HTTP root span, named by route template — no ID in the name |
| `pgx.query`, `query SELECT 1`, `pool.acquire` | PostgreSQL is instrumented and nested under the request |
| `redis.ping` | Redis likewise |

**3. The metric (Prometheus).**

```bash
curl -sG http://localhost:9090/api/v1/query   --data-urlencode 'query=http_server_request_duration_seconds_count{route="/api/v1/ping"}'
```

The counter carries `route`, `method`, `status_class` and `service_name`. Prometheus scrapes the
collector at `otel-collector:8889`, not the application — the application only pushes OTLP.

In Grafana (<http://localhost:3000>, anonymous admin, datasources provisioned from
`deploy/grafana/provisioning/`) the Loki `trace_id` field is a link: clicking it opens the trace
in Tempo. That link is the point of the exercise.

### Two other things this stack guarantees

- **Readiness is real.** `docker stop fluentra-postgres-1` makes `/ready` return **503**
  (`{"status":"unavailable"}`) while `/health` stays **200** — a dependency outage should pull
  the instance out of rotation, not restart the process. Starting Postgres returns `/ready` to
  200.
- **Shutdown flushes.** A request issued milliseconds before `SIGTERM` still lands: its log line
  reaches Loki and all of its spans reach Tempo. Spans buffered in the batch processor are not
  lost on deploy.

If any of the three steps does not work, that is a bug in the observability setup and it is
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
| `port is already allocated` on 5432 or 6379 | Another project's container holds it. `docker stop <name>`, work, then `docker start <name>` — the data survives. Do not `docker rm` it and do not renumber our ports |
| Everything is healthy but nothing connects | You ran `compose.yaml` alone. It publishes no ports by design — use `make dev` or `make dev-infra` |
| Only need the database for tests | `make dev-infra` — postgres, redis and minio with ports published, no application images to build |
| Port already in use | Another project's stack is running; `docker ps` |
| Slow Docker on macOS/Windows | Enable VirtioFS / WSL2 backend |

More: `docs/development/troubleshooting.md` — **not written yet**.

## Do not

- Do not run `docker compose down -v` or `make db-reset-DANGEROUS` casually — they delete local
  learner data, including anything you were mid-way through testing.
- Do not commit `.env`.
- Do not point local development at a shared or production database. There is no reason to, and
  every reason not to.
