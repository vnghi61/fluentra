#!/usr/bin/env bash
# Runs the API and the worker in one Render web service. The free plan has no
# background workers, and a second web service would sleep on its own; here the
# worker wakes and sleeps with the API, and the API nudges it on localhost.
#
# Build command: go build -o bin/api ./cmd/api && go build -o bin/worker ./cmd/worker
# Start command: bash deploy/render/start.sh
set -euo pipefail

# The two pools share Supabase's session pooler (about 15 slots on the free plan).
HTTP_PORT=8081 OTEL_SERVICE_NAME=fluentra-worker \
  DB_MAX_CONNS="${WORKER_DB_MAX_CONNS:-5}" GOMEMLIMIT="${WORKER_GOMEMLIMIT:-160MiB}" \
  ./bin/worker &
worker=$!

HTTP_PORT="${PORT:-8080}" WORKER_URL="${WORKER_URL:-http://127.0.0.1:8081}" \
  DB_MAX_CONNS="${DB_MAX_CONNS:-8}" GOMEMLIMIT="${GOMEMLIMIT:-280MiB}" \
  ./bin/api &
api=$!

# Render sends SIGTERM to this script only; pass it on so both drain cleanly.
trap 'kill -TERM "$api" "$worker" 2>/dev/null || true' TERM INT

# If either process exits, stop the other so Render restarts the pair.
set +e
wait -n "$api" "$worker"
status=$?
kill -TERM "$api" "$worker" 2>/dev/null
wait
exit "$status"
