#!/usr/bin/env bash
# Runs only the worker on Render; the API runs on the VPS (deploy/vps).
# Build command: go build -o bin/worker ./cmd/worker
# Start command: bash deploy/render/start-worker.sh
set -euo pipefail

export HTTP_PORT="${PORT:-8081}" OTEL_SERVICE_NAME=fluentra-worker
# The VPS API holds 8 of Supabase's ~15 session-pooler slots.
export DB_MAX_CONNS="${WORKER_DB_MAX_CONNS:-5}" GOMEMLIMIT="${WORKER_GOMEMLIMIT:-400MiB}"
exec ./bin/worker
