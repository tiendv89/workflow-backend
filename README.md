# workflow-backend

Read-only REST API service for workspace management and activity tracking. It reads from a shared PostgreSQL database written to by [`workspace-github-adapter`](https://github.com/tiendv89/workspace-github-adapter) and calls that service's RPC endpoints for write operations (sync triggers, workspace imports).

## Architecture

```
                        ┌──────────────────────┐
                        │  workspace-github-    │
Client ──► api-service ─┤  adapter (RPC calls) │
               │        └──────────────────────┘
               │                  │
               └──── PostgreSQL ◄─┘
```

- **api-service** — this repo; serves the REST API (port 8081 by default)
- **adapter-service** — separate repo; syncs GitHub data into PostgreSQL
- Shared PostgreSQL database; migrations live in the adapter repo

## Requirements

- Go 1.25+
- PostgreSQL (with migrations from `workspace-github-adapter` applied)
- A running `adapter-service` instance (for sync/import endpoints)

## Configuration

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | *(required)* | PostgreSQL connection string |
| `PORT` | `8081` | HTTP listen port |
| `ADAPTER_SERVICE_URL` | `http://adapter-service:8080` | adapter-service base URL |
| `STALE_THRESHOLD_MINUTES` | `30` | Minutes before a workspace sync is considered stale; `0` disables |
| `GIN_MODE` | `release` | Set to `debug` for verbose request logging |

Copy `.env.example` to get started:

```bash
cp .env.example .env
```

## Running locally

```bash
# Start PostgreSQL (and optionally adapter-service) via Docker Compose
docker compose up -d

# Run the API service
DATABASE_URL="postgres://workspace_adapter:workspace_adapter@localhost:5432/workspace_adapter?sslmode=disable" \
ADAPTER_SERVICE_URL="http://localhost:8080" \
GIN_MODE=debug \
go run ./cmd/api-service
```

The service starts on `http://localhost:8081`. Health check: `GET /healthz`.

## API overview

All routes are prefixed with `/api`.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/workspaces` | List workspaces |
| `POST` | `/api/workspaces/import` | Import a new workspace |
| `GET` | `/api/workspaces/:slug` | Get workspace details |
| `POST` | `/api/workspaces/:slug/sync` | Trigger a workspace sync |
| `GET` | `/api/workspaces/:slug/features` | List features in a workspace |
| `GET` | `/api/workspaces/:slug/features/:featureID` | Get feature details |
| `GET` | `/api/workspaces/:slug/features/:featureID/tasks` | List tasks in a feature |
| `GET` | `/api/workspaces/:slug/tasks` | Search tasks in a workspace |
| `GET` | `/api/workspaces/:slug/activity` | List workspace activity |
| `GET` | `/healthz` | Health check |

Paginated endpoints accept `?page=` and `?limit=` query parameters. Error responses include a `code`, `message`, `source`, and `retryable` flag.

See [docs/frontend-api.md](docs/frontend-api.md) for full frontend integration docs.

## Testing

```bash
# Unit and integration tests
go test ./...

# With a real test database (enables database reader contract tests)
WORKFLOW_BACKEND_TEST_DATABASE_URL="postgres://..." go test -tags=integration ./internal/database/...
```

## Building

```bash
# Binary
go build -o api-service ./cmd/api-service

# Docker image
docker build -t workflow-backend:latest .
```

## Releases

Releases are triggered manually via the GitHub Actions `release` workflow, which builds and pushes the Docker image to `asia.gcr.io/production-476602/workflow-backend` and creates a GitHub release with a git tag.
