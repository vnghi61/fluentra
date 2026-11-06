#!/usr/bin/env bash
# One-time VPS setup (Ubuntu 22.04, as root). Run from a copy of deploy/vps on the server.
# The Cloudflare Tunnel token is installed separately: cloudflared service install <token>.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"

apt-get update -q
apt-get install -y -q curl ufw qemu-guest-agent
systemctl enable --now qemu-guest-agent || true

if ! command -v cloudflared >/dev/null; then
  install -d -m 0755 /usr/share/keyrings
  curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg -o /usr/share/keyrings/cloudflare-main.gpg
  echo 'deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared any main' \
    > /etc/apt/sources.list.d/cloudflared.list
  apt-get update -q
  apt-get install -y -q cloudflared
fi

id fluentra >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin fluentra
install -d -m 0755 /opt/fluentra
install -d -m 0750 -g fluentra /etc/fluentra
if [ ! -f /etc/fluentra/api.env ]; then
  install -m 0640 -g fluentra "$here/api.env.template" /etc/fluentra/api.env
fi

install -m 0644 "$here/fluentra-api.service" "$here/fluentra-worker-keepalive.service" \
  "$here/fluentra-worker-keepalive.timer" /etc/systemd/system/
install -d -m 0755 /etc/systemd/system/cloudflared.service.d
install -m 0644 "$here/cloudflared-restart.conf" /etc/systemd/system/cloudflared.service.d/restart.conf
systemctl daemon-reload
# The API waits for api.env; the timer can start now.
systemctl enable fluentra-api
systemctl enable --now fluentra-worker-keepalive.timer

# Key-only SSH, but never before a key is installed: that would lock root out.
if [ -s /root/.ssh/authorized_keys ]; then
  install -m 0644 "$here/sshd-key-only.conf" /etc/ssh/sshd_config.d/00-key-only.conf
  sshd -t && systemctl reload ssh
fi

# The API binds every interface and the VPS has a public IPv6: only SSH comes in, the tunnel goes out.
ufw allow 22/tcp
ufw --force enable
