#!/usr/bin/env bash
# Starts (or stops) the API and the worker the Playwright suite runs against,
# with every outside service pointed at a local stand-in.
#
#   scripts/e2e-stack.sh start   migrate, seed, build, start api (:8080) and worker (:8081)
#   scripts/e2e-stack.sh stop    stop what `start` started
#
# CI's frontend job and a developer's machine both call this, so the two stacks
# cannot drift. Run `make dev-infra` first: it starts Postgres, Redis, MinIO and
# Mailpit.
#
# Why everything below is set explicitly rather than read from .env: the Go
# commands load .env, and a developer's .env is edited by hand. On 2026-10-12 it
# sent mail through Resend, so a local E2E run registered real accounts and
# tried to email them, and its DB_DSN later pointed at production. An exported
# variable beats .env, so this script exports a safe value for every key that
# reaches outside the machine. Override one only by exporting it yourself, and
# only with something local: the dbtarget guard refuses a remote database here.
set -euo pipefail

cd "$(dirname "$0")/.."

: "${E2E_API_PORT:=8080}"
: "${E2E_WORKER_PORT:=8081}"

# Data services, as deploy/compose/compose.dev.yaml publishes them.
: "${DB_DSN:=postgres://fluentra:fluentra@127.0.0.1:5432/fluentra?sslmode=disable}"
: "${REDIS_URL:=redis://127.0.0.1:6379/0}"
: "${S3_ENDPOINT:=http://127.0.0.1:9000}"
: "${S3_ACCESS_KEY:=minioadmin}"
: "${S3_SECRET_KEY:=minioadmin}"
export DB_DSN REDIS_URL S3_ENDPOINT S3_ACCESS_KEY S3_SECRET_KEY
export S3_USE_SSL=false

# Mail goes to Mailpit, never to a provider.
export MAIL_TRANSPORT=smtp
export SMTP_HOST="${SMTP_HOST:-127.0.0.1}"
export SMTP_PORT="${SMTP_PORT:-1025}"
export SMTP_DEV_MODE=true
# Mailpit offers no AUTH; a non-empty username makes the mailer try PlainAuth
# anyway and every OTP mail dead-letters.
export SMTP_USERNAME='' SMTP_PASSWORD='' RESEND_API_KEY=''

# No model, speech or OAuth provider, and no workflow dispatch to production.
export AI_PROVIDER_1_NAME=mock AI_PROVIDER_1_API_KEY=''
export AI_PROVIDER_2_NAME='' AI_PROVIDER_3_NAME='' AI_PROVIDER_4_NAME=''
export SPEECH_ASR_BASE_URL='' SPEECH_ASR_API_KEY=''
export SPEECH_TTS_DISPATCH_TOKEN='' SPEECH_TTS_DISPATCH_REPOSITORY=''
export OAUTH_GOOGLE_CLIENT_ID='' OAUTH_GOOGLE_CLIENT_SECRET=''

: "${CORS_ALLOWED_ORIGINS:=http://localhost:5173}"
export CORS_ALLOWED_ORIGINS
# Ten journeys across four device projects all register from one address. The
# shipped limits refuse the back half of the matrix, which fails as a timeout
# somewhere unrelated.
export OTP_ISSUE_PER_IP_PER_HOUR=500 RATE_LIMIT_AUTH_PER_MIN=500 RATE_LIMIT_ANON_PER_MIN=2000

start() {
  OTP_HMAC_KEY="$(openssl rand -hex 32)"
  JWT_SIGNING_KEY="$(openssl rand -hex 32)"
  export OTP_HMAC_KEY JWT_SIGNING_KEY

  go run ./cmd/migrate up
  # The learning journeys drive the seeded curriculum; an unseeded database
  # fails them as "element(s) not found" fifteen seconds later.
  go run ./cmd/seed

  # Built, not `go run`: a backgrounded `go run` is a parent whose child is the
  # server, so its pid is not the process that has to be stopped.
  mkdir -p bin
  go build -o ./bin/api ./cmd/api
  go build -o ./bin/worker ./cmd/worker

  HTTP_PORT="$E2E_API_PORT" ./bin/api > api.log 2>&1 &
  echo $! > bin/e2e-api.pid
  # The worker turns an outbox row into an email in Mailpit. Without it every
  # OTP journey times out.
  HTTP_PORT="$E2E_WORKER_PORT" ./bin/worker > worker.log 2>&1 &
  echo $! > bin/e2e-worker.pid

  for _ in $(seq 1 30); do
    if curl -sf "http://127.0.0.1:${E2E_API_PORT}/ready" > /dev/null; then
      echo "API is ready on :${E2E_API_PORT}; mail goes to Mailpit"
      return 0
    fi
    sleep 1
  done
  echo "API never became ready"
  cat api.log
  return 1
}

stop() {
  for name in api worker; do
    pidfile="bin/e2e-${name}.pid"
    if [ -f "$pidfile" ]; then
      kill "$(cat "$pidfile")" 2> /dev/null || true
      rm -f "$pidfile"
    fi
  done
}

case "${1:-}" in
  start) start ;;
  stop) stop ;;
  *)
    echo "usage: $0 start|stop" >&2
    exit 2
    ;;
esac
