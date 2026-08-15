# `feat(deploy): put application services in the dev compose stack`

## Summary

- Adds development Dockerfiles for Go (`deploy/docker/Dockerfile.go.dev` with `air` hot reload) and Node (`deploy/docker/Dockerfile.node.dev` running `vite`).
- Adds `.air.toml` for `cmd/api` and `.air.worker.toml` for `cmd/worker`.
- Configures `api`, `worker`, and `web` services in `deploy/compose/compose.dev.yaml` without touching the base `compose.yaml` or `compose.prod.yaml`.
- Configures Vite server dev proxy for `/api` in `web/vite.config.ts` and binds host `0.0.0.0` for containerized dev.
- Adds host targets `make api`, `make worker`, and `make web` in `Makefile` and updates `make dev` output summary.
- Rewrites `DOCKER_GUIDE.md` §9 and `docs/development/getting-started.md` §3 to document the full containerized dev stack and host commands.

## Design decisions and trade-offs

1. **Overlay isolation (production safety):**
   The application services (`api`, `worker`, `web`) are added strictly to `deploy/compose/compose.dev.yaml`. The base `compose.yaml` and `compose.prod.yaml` contain zero references to dev images or dev commands. Verified via `docker compose -f deploy/compose/compose.yaml -f deploy/compose/compose.prod.yaml -f deploy/compose/compose.observability.yaml config` that `api`, `worker`, and `web` are absent.

2. **Network access and port exposure:**
   `compose.dev.yaml` overrides `backend` to `internal: false` so published ports (Postgres 5432, Redis 6379, MinIO 9000/9001, Mailpit 8025) are accessible on `127.0.0.1` for local host tooling, migrations, and tests. Containerized services use Docker DNS names (`postgres:5432`, `redis:6379`, `minio:9000`, `mailpit:1025`, `otel-collector:4317`).

3. **Vite API proxy vs CORS:**
   Configured Vite dev server with `host: "0.0.0.0"` and a proxy on `/api` forwarding to `VITE_API_TARGET` (`http://api:8080` in Compose, `http://localhost:8080` on host). This matches production Nginx routing while preserving CORS support on the Go API server for direct cross-origin browser requests.

4. **Node volume mounting on Windows:**
   `compose.dev.yaml` mounts `../../web:/app` alongside an anonymous volume `/app/node_modules` so container Linux binaries in `node_modules` are preserved and not clobbered by the host filesystem.

## Verification

- `docker compose -f deploy/compose/compose.yaml -f deploy/compose/compose.dev.yaml config` -> valid syntax.
- `docker compose -f deploy/compose/compose.yaml -f deploy/compose/compose.prod.yaml -f deploy/compose/compose.observability.yaml config` -> verified `api`, `worker`, `web` are absent.
- `docker compose -f deploy/compose/compose.yaml -f deploy/compose/compose.dev.yaml -f deploy/compose/compose.observability.yaml -f deploy/compose/compose.observability.dev.yaml config` -> full dev stack configuration valid.
- `node tools/docgen/check-drift.mjs` -> documentation drift check passed.
- `node tools/docgen/generate.mjs --check` -> 30 modules, 0 files written.
- `npx markdownlint-cli2@0.20.0` -> 366 files, 0 errors.
