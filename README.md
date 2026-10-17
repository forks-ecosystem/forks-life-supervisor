# Forks Life Supervisor

Docker container management and monitoring service for the Forks Life ecosystem. Provides a REST API to list, inspect, start, stop, restart, and view logs of all running Docker containers.

## Quick Start

```bash
docker compose up -d
```

## Configuration

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

On the server that hosts the PHP admin panel, expose the supervisor API under
the `/sv/` prefix so the panel can reach it. Add this inside the panel's
`server { ... }` block:

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
or change `proxy_pass` to the host/IP and port where the container is published
(`-p 9000:9000`). Then reload nginx:

```bash
nginx -t && systemctl reload nginx
```

## Build from Source

```bash
go build -o supervisor ./cmd/supervisor/
./supervisor
```

## Docker

```bash
docker build -t forks-life-supervisor .
docker run -p 9000:9000 -v /var/run/docker.sock:/var/run/docker.sock:ro forks-life-supervisor
```
