# Forks Life Supervisor

Docker container management and monitoring service for the Forks Life ecosystem. Provides a REST API to list, inspect, start, stop, restart, and view logs of all running Docker containers.

## Quick Start

```bash
# Build and install the binary, then restart the systemd service
./__rebuild.sh
```

The service is managed by systemd as `supervisor.service`:

```bash
systemctl status supervisor
systemctl restart supervisor
journalctl -u supervisor -f
```

## Configuration

Configuration is read from `/app/forks-life-supervisor/.env` (loaded by the
systemd unit via `EnvironmentFile`).

| Variable | Default | Description |
|---|---|---|
| `SUPERVISOR_PORT` | `9000` | API listen port |
| `SUPERVISOR_HOST` | `127.0.0.1` | API listen host |

## API Endpoints

| Endpoint | Description |
|---|---|
| `GET /api/system` | System overview (container count, Docker version, OS) |
| `GET /api/containers` | List all containers with status and ports |
| `GET /api/containers?all=true` | Include stopped containers |
| `GET /api/container/{name}` | Detailed container info (env, mounts, ports) |
| `POST /api/container/{name}/start` | Start a container |
| `POST /api/container/{name}/stop` | Stop a container |
| `POST /api/container/{name}/restart` | Restart a container |
| `POST /api/container/{name}/remove` | Remove a container |
| `GET /api/container/{name}/logs?lines=100` | View container logs |
| `GET /api/ports` | Port mapping across all containers |

## Examples

```bash
# System overview
curl http://localhost:9000/api/system

# List running containers
curl http://localhost:9000/api/containers

# Container details
curl http://localhost:9000/api/container/forkex-api

# Restart a container
curl -X POST http://localhost:9000/api/container/forkex-api/restart

# View last 50 log lines
curl "http://localhost:9000/api/container/forkex-api/logs?lines=50"

# Port map
curl http://localhost:9000/api/ports
```

## Nginx Reverse Proxy (PHP admin panel)

The PHP admin panel is [forks-life-control](https://github.com/forks-ecosystem/forks-life-control).
On the server that hosts it, expose the supervisor API under the `/sv/` prefix
so the panel can reach it. Add this inside the panel's `server { ... }` block:

```nginx
# LegacyCoin Supervisor admin panel
location /sv/api/ {
    proxy_pass http://127.0.0.1:9000/api/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

The panel calls `/sv/api/...`; because both `location` and `proxy_pass` end with
a trailing slash, nginx strips `/sv` and forwards `/api/...` to the supervisor
listening on `127.0.0.1:9000` (matching `SUPERVISOR_HOST` / `SUPERVISOR_PORT`).
The supervisor must be reachable from the nginx host: run it on the same machine,
or change `proxy_pass` to the host/IP and port where it listens. Then reload
nginx:

```bash
nginx -t && systemctl reload nginx
```

## Build from Source

```bash
go build -o /usr/local/bin/supervisor ./cmd/supervisor/
```

## Deployment

The supervisor runs natively on the host (not in a container) because it needs
direct access to host resources:

- `/etc/nginx/sites-available`, `/etc/nginx/sites-enabled` — nginx sites
- `/etc/systemd/system`, `/lib/systemd/system` and `systemctl` — system services
- `/var/run/docker.sock` — Docker containers
- `/home/coin/go/bin/legacycoin-cli`, `/home/coin/.legacycoin` — LegacyCoin RPC

### systemd unit

`/etc/systemd/system/supervisor.service`:

```ini
[Unit]
Description=Forks Life Supervisor
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/app/forks-life-supervisor
EnvironmentFile=/app/forks-life-supervisor/.env
ExecStart=/usr/local/bin/supervisor
Restart=always
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
systemctl daemon-reload
systemctl enable --now supervisor
```
