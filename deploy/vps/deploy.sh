#!/usr/bin/env bash
# Builds the API for linux/amd64 and swaps it in on the VPS. Usage: deploy/vps/deploy.sh [ssh-host]
set -euo pipefail
host="${1:-fluentra-vps}"
cd "$(dirname "$0")/../.."

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/linux/api ./cmd/api
scp bin/linux/api "$host:/opt/fluentra/api.new"
ssh "$host" 'chmod 0755 /opt/fluentra/api.new && mv -f /opt/fluentra/api.new /opt/fluentra/api \
  && systemctl restart fluentra-api \
  && curl -fsS --retry 15 --retry-connrefused --retry-delay 1 http://127.0.0.1:8080/health && echo'
