#!/usr/bin/env bash
# Builds the API for linux/amd64 and swaps it in on the VPS, keeping the previous binary.
#   deploy/vps/deploy.sh [ssh-host]              deploy the checked-out commit
#   deploy/vps/deploy.sh --rollback [ssh-host]   swap back to the previous binary
set -euo pipefail
cd "$(dirname "$0")/../.."

# Swaps api and api.prev, restarts, and undoes the swap if /health does not answer.
remote_swap='
set -eu
cd /opt/fluentra
swap() {
  if [ -e api ]; then mv -f api api.swap && mv -f "$1" api && mv -f api.swap "$1"; else mv -f "$1" api; fi
}
swap "$1"
chmod 0755 api
systemctl restart fluentra-api
if ! curl -fs --retry 15 --retry-connrefused --retry-delay 1 http://127.0.0.1:8080/health >/dev/null; then
  echo "health check failed; restoring the previous binary" >&2
  swap "$1"
  systemctl restart fluentra-api
  exit 1
fi
curl -fsS http://127.0.0.1:8080/version && echo
'

if [ "${1:-}" = "--rollback" ]; then
  host="${2:-fluentra-vps}"
  ssh "$host" "test -f /opt/fluentra/api.prev" || { echo "no previous binary on $host" >&2; exit 1; }
  ssh "$host" bash -s -- api.prev <<<"$remote_swap"
  exit 0
fi

host="${1:-fluentra-vps}"
version="$(git describe --tags --always --dirty)"
commit="$(git rev-parse HEAD)"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w -X main.version=${version} -X main.commitSHA=${commit}" \
  -o bin/linux/api ./cmd/api
scp bin/linux/api "$host:/opt/fluentra/api.new"
ssh "$host" bash -s -- api.new <<<"$remote_swap"
# The swap left the replaced binary in api.new; keep it as the rollback target.
ssh "$host" 'cd /opt/fluentra && if [ -e api.new ]; then mv -f api.new api.prev; fi'
