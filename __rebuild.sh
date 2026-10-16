#!/bin/sh
set -e

cd "$(dirname "$0")"

echo '==> Stopping and removing supervisor...'
docker compose down

echo '==> Removing old supervisor image...'
docker rmi -f forks-life-supervisor-supervisor 2>/dev/null || true
docker image prune -f

echo '==> Rebuilding image...'
docker compose build supervisor

echo '==> Starting container...'
docker compose up -d supervisor

echo '==> Done, supervisor rebuilt and running.'