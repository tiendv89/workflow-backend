# workflow-backend

REST API service for workspace management and activity tracking. It reads from a shared PostgreSQL database and calls the [`workspace-github-adapter`](https://github.com/tiendv89/workspace-github-adapter) RPC endpoints for write operations (sync triggers, workspace imports).

## Architecture

```
                        ┌──────────────────────┐
                        │  workspace-github-    │
Client ──► api-service ─┤  adapter (RPC calls) │
               │        └──────────────────────┘
               │                  │
               └──── PostgreSQL ◄─┘
```

- **api-service** — this repo; serves the REST API (port 8080 by default)
- **adapter-service** — separate repo; syncs GitHub data into PostgreSQL
- Shared PostgreSQL database; this repo owns and runs its own migrations

## Requirements

- Go 1.25+
- PostgreSQL
- A running `adapter-service` instance (for sync/import endpoints)

## Configuration

Configuration is loaded from a YAML file passed via `-c <path>`. Viper maps environment variables to config keys using `_` as the separator (e.g. `DB_HOST` overrides `db.host`).

| Key | Default | Description |
|---|---|---|
| `log.level` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `api.http.address` | `:8081` | HTTP listen address |
| `api.http.mode` | `release` | Gin mode; set to `debug` for verbose request logging |
| `api.stale_threshold_minutes` | `30` | Minutes before a workspace sync is considered stale; `0` disables |
| `api.adapter_service_url` | `http://adapter-service:8080` | adapter-service base URL |
| `db.host` | *(required)* | PostgreSQL host |
| `db.port` | — | PostgreSQL port |
| `db.db_name` | — | Database name |
| `db.user` | — | Database user |
| `db.password` | — | Database password |
| `db.auto_migration` | `false` | Run migrations automatically on startup |
| `db.migration_dir` | — | Goose migration directory (e.g. `file://migrations`) |
| `db.max_open_conns` | — | Max open DB connections |
| `db.max_idle_conns` | — | Max idle DB connections |
| `db.conn_life_time_seconds` | — | Connection max lifetime in seconds |

See `configs/config.yaml` for a full example.

## Running locally

```bash
# Start PostgreSQL via Docker Compose (binds to localhost:25432)
docker compose up -d

# Run the API service
go run ./cmd -c configs/config.yaml api
```

The service starts on `http://localhost:8080`. Health check: `GET /healthz`.

## Migrations

Migrations use [Goose](https://github.com/pressly/goose) and live in `migrations/`.

```bash
# Apply all pending migrations
make migrate-up

# Roll back the last migration
make migrate-down-1

# Create a new migration
make new-migration NAME=<name>
```

Migrations also run automatically on startup when `db.auto_migration: true` is set in the config.

## API overview

All routes are prefixed with `/api`.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/workspaces` | List workspaces |
| `POST` | `/api/workspaces/import` | Import a new workspace |
| `GET` | `/api/workspaces/:workspaceId` | Get workspace details |
| `POST` | `/api/workspaces/:workspaceId/sync` | Trigger a workspace sync |
| `GET` | `/api/workspaces/:workspaceId/features` | Search features in a workspace |
| `GET` | `/api/workspaces/:workspaceId/features/:featureId` | Get feature details |
| `GET` | `/api/workspaces/:workspaceId/features/:featureId/tasks` | Search tasks in a feature |
| `GET` | `/api/workspaces/:workspaceId/features/:featureId/tasks/:taskId` | Get a task in a feature |
| `GET` | `/api/workspaces/:workspaceId/tasks` | Search tasks in a workspace |
| `GET` | `/api/workspaces/:workspaceId/tasks/:taskId` | Get a task in a workspace |
| `GET` | `/api/workspaces/:workspaceId/activity` | List workspace activity |
| `GET` | `/healthz` | Health check |

Paginated endpoints accept `?page=` and `?limit=` query parameters. Error responses include a `code`, `message`, `source`, and `retryable` flag.

See [docs/frontend-api.md](docs/frontend-api.md) for full frontend integration docs.

## Testing

```bash
# Unit and integration tests
make test

# With a real test database (enables database reader contract tests)
WORKFLOW_BACKEND_TEST_DATABASE_URL="postgres://..." go test -tags=integration ./internal/database/...
```

## Building

```bash
# Binary
go build -o server ./cmd

# Docker image
docker build -t workflow-backend:latest .
```

## Releases

Releases are triggered manually via the GitHub Actions `release` workflow, which builds and pushes the Docker image to `asia.gcr.io/production-476602/workflow-backend` and creates a GitHub release with a git tag.
