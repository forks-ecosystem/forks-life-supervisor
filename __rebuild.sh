#!/bin/sh
set -e

cd "$(dirname "$0")"

GO="${GO:-/usr/local/go/bin/go}"
BIN=/usr/local/bin/supervisor
SERVICE=supervisor

echo '==> Building supervisor binary...'
"$GO" build -o "$BIN" ./cmd/supervisor/

echo '==> Restarting systemd service...'
systemctl restart "$SERVICE"

echo '==> Status:'
systemctl status "$SERVICE" --no-pager | head -8

echo '==> Done, supervisor rebuilt and running.'
