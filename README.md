# Fluentra

> **AI-first English learning platform.** A Go modular monolith and a React SPA that teach
> vocabulary, grammar, reading, listening, speaking and writing through adaptive lessons,
> spaced repetition, mock exams (IELTS, TOEIC, VSTEP) and AI-graded practice.

[![go](https://img.shields.io/badge/go-1.25-00ADD8)](go.mod)
[![react](https://img.shields.io/badge/react-19-61DAFB)](web/package.json)
[![license](https://img.shields.io/badge/license-proprietary-lightgrey)](DEPENDENCIES.md)

**AI assistants:** read [`AGENT.md`](AGENT.md) first, then [`MODULE_INDEX.md`](MODULE_INDEX.md).

## Quick start

```bash
make dev-infra     # postgres, redis, minio, mailpit
make migrate-up
make seed          # demo accounts, courses, 10k words, mock tests (about 20 min)
make api           # :8080
make worker        # :8081
make web           # :5173
```

Copy `.env.example` to `.env` first. It is the full configuration reference: a key that
is not in it is not read.

## Documentation

| Document | What it answers |
|---|---|
| [ARCHITECTURE.md](ARCHITECTURE.md) | How the system is built and why; project layout; glossary |
| [MODULE_INDEX.md](MODULE_INDEX.md) | Every module, its owner and its allowed dependencies |
| `internal/<group>/<module>/AGENT.md` | One module: contract, schema, endpoints, flows, decisions, backlog |
| [docs/CONVENTIONS.md](docs/CONVENTIONS.md) | Coding, API, database, errors, logging, security and testing rules |
| [docs/OPERATIONS.md](docs/OPERATIONS.md) | Deploying on the free tier (Vercel, Render, Supabase, Upstash, R2), CI, releases, runbooks |
| [docs/AI.md](docs/AI.md) | Working with AI assistants and the product's LLM calls |
| [DECISIONS.md](DECISIONS.md) | Index of the ADRs in `docs/adr/` |
| [DEPENDENCIES.md](DEPENDENCIES.md) | Every library, why it was chosen, third-party notices |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute, including the add-a-module and add-an-endpoint guides |
| [CHANGELOG.md](CHANGELOG.md) | Release history |
| [web/AGENT.md](web/AGENT.md) | The frontend |

## Architecture at a glance

```mermaid
graph TB
    subgraph Client
        W["React SPA<br/>Vite + TS"]
    end

    subgraph Edge
        N["Nginx / Caddy<br/>TLS, static, rate limit"]
    end

    subgraph App["Modular Monolith (Go)"]
        API["cmd/api<br/>HTTP :8080"]
        WRK["cmd/worker<br/>jobs + scheduler"]
        subgraph Modules
            direction LR
            CORE["auth · user · rbac<br/>audit · admin · notification"]
            LEARN["content · lesson · learning · srs<br/>vocabulary · grammar · reading<br/>listening · speaking · writing<br/>exam · questionbank · gamification"]
            COM["payment · subscription · analytics"]
        end
        PLAT["platform: ai · cache · storage<br/>search · job · media · telemetry · mailer"]
    end

    subgraph Data
        PG[("PostgreSQL 17")]
        RD[("Redis 7")]
        MI[("MinIO")]
    end

    subgraph Obs["Observability"]
        OC["OTel Collector"] --> PR["Prometheus"] & LO["Loki"] & TE["Tempo"]
        PR & LO & TE --> GR["Grafana"]
    end

    subgraph Ext["External"]
        LLM["Anthropic · OpenAI<br/>Gemini · OpenRouter"]
        ASR["ASR / TTS"]
        PAY["Payment gateway"]
        SMTP["SMTP"]
    end

    W --> N --> API
    API --> Modules --> PLAT
    WRK --> Modules
    PLAT --> PG & RD & MI
    PLAT --> LLM & ASR & PAY & SMTP
    API -.OTLP.-> OC
    WRK -.OTLP.-> OC
```

## Local services

`make dev` runs everything in containers instead. Then open:

| Service | URL |
|---|---|
| Web app | <http://localhost:5173> |
| API | <http://localhost:8080> |
| API docs (Scalar) | <http://localhost:8080/docs> |
| Grafana | <http://localhost:3000> |
| MinIO console | <http://localhost:9001> |
| Mailpit | <http://localhost:8025> |

## License

Proprietary. All rights reserved.
