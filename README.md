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
