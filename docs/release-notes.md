# Release Notes — workspace-data-backend

## Overview

This feature adds a workspace data read API to `workflow-backend` (`api-service`). The service reads workspace snapshots from a shared PostgreSQL database and returns them via stable REST routes. Data is written by `workspace-github-adapter` (`adapter-service`), which is a separate service deployed from a different repository.

`digital-factory-ui` continues to call GitHub directly until `workspace-tabs-data-flow` switches it to these new routes.

---

## Required Environment Variables

### api-service (this repo — workflow-backend)

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | **yes** | — | PostgreSQL DSN. Example: `postgresql://user:pass@localhost:5432/workflow?sslmode=disable`. `api-service` opens a read-only connection pool to this database. |
| `ADAPTER_SERVICE_URL` | no | `http://adapter-service:8080` | Base URL for `adapter-service` RPC calls (import and sync triggers). |
| `PORT` | no | `8081` | Port that `api-service` listens on. |
| `STALE_THRESHOLD_MINUTES` | no | `30` | Number of minutes after which a successful sync run is treated as stale. Set to `0` to disable staleness expiry. |
| `GIN_MODE` | no | `release` | Set to `debug` for verbose request logging during development. |

### adapter-service (workspace-github-adapter repo)

These are listed here for completeness — they must be set in `adapter-service`, not `api-service`.

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | **yes** | Same database as `api-service`. `adapter-service` also runs goose migrations on startup. |
| `GITHUB_TOKEN` | **yes** | GitHub personal access token or App installation token. Used for all GitHub API calls during import and sync. `api-service` makes no GitHub calls. |
| `REDIS_URL` | **yes** | Redis URL for the asynq task queue. Example: `redis://localhost:6379/0`. |
| `ADAPTER_PORT` | no | Port that `adapter-service` RPC endpoints listen on (default `8080`). |

---

## Database Migration

Schema migrations are managed by `goose` and live in the `workspace-github-adapter` repository under `database/migrations/`. Run them from that repo before starting either service.

### Run migrations (from workspace-github-adapter repo)

```bash
# Apply all pending migrations.
DATABASE_URL="postgresql://user:pass@host:5432/workflow?sslmode=disable" \
  goose -dir database/migrations postgres "$DATABASE_URL" up
```

The same `DATABASE_URL` used by the running services must be accessible when running migrations. If the deployment uses a PgBouncer connection pooler, use the direct (non-pooled) URL for migrations — goose uses PostgreSQL advisory locks that are incompatible with transaction-mode poolers.

### Verify migration status

```bash
goose -dir database/migrations postgres "$DATABASE_URL" status
```

### Roll back one migration

```bash
goose -dir database/migrations postgres "$DATABASE_URL" down
```

This is a **greenfield schema** — no existing data needs to be migrated. The first `up` run creates all tables from scratch.

---

## Sync Polling Configuration

`api-service` does not poll GitHub directly. Sync is triggered explicitly:

- **Import**: `POST /api/workspaces/import` — creates a new workspace and triggers a full reconciliation via `adapter-service`.
- **Manual sync**: `POST /api/workspaces/:workspaceId/sync` — triggers a full reconciliation via `adapter-service`.
- **Webhook-driven sync** (on `adapter-service`): GitHub webhook events on the management repo's branches trigger targeted syncs and task queue entries automatically.

The `STALE_THRESHOLD_MINUTES` variable controls how `api-service` classifies a workspace as stale at read time. It does **not** control how frequently syncs are triggered — that is governed by webhook delivery frequency and the asynq worker drain rate on `adapter-service`.

---

## GitHub Token Behavior

- The `GITHUB_TOKEN` is read by `adapter-service` only. `api-service` never calls GitHub.
- The token is used for the GitHub Contents API and Git Trees API calls during import and sync. All requests are authenticated.
- If `GITHUB_TOKEN` is absent or invalid, `adapter-service` will return an auth error to `api-service`, which propagates it as a `GITHUB_UNAUTHORIZED` source error to the UI.
- Public repositories can be imported without a token, but rate limits apply (60 requests/hour per IP). A token raises the limit to 5000 requests/hour per token.
- The token is never stored in the database.

---

## API Routes

All routes are served by `api-service` under the `/api` prefix.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/workspaces` | List all saved workspaces with source state. |
| `POST` | `/api/workspaces/import` | Import a new workspace from a GitHub repo URL. Triggers a full reconciliation in `adapter-service`. |
| `GET` | `/api/workspaces/:workspaceId` | Workspace detail: features and tasks. |
| `GET` | `/api/workspaces/:workspaceId/search/features` | Search workspace features. Optional `?title=`, `?status=`, `?sort=`, and `?limit=` filters. |
| `POST` | `/api/workspaces/:workspaceId/sync` | Trigger a manual full reconciliation. Returns the workspace detail (potentially stale on failure). |
| `GET` | `/api/workspaces/:workspaceId/features/:featureId` | Feature detail: documents, tasks, activity, source state. |
| `GET` | `/api/workspaces/:workspaceId/features/:featureId/tasks` | Task summaries for a feature. |
| `GET` | `/api/workspaces/:workspaceId/features/:featureId/search/tasks` | Search tasks in a feature. Optional `?task_id=`, `?title=`, `?status=`, `?repo=`, `?sort=`, and `?limit=` filters. |
| `GET` | `/api/workspaces/:workspaceId/features/:featureId/tasks/:taskId` | Task detail: dependencies, execution context, PR refs, activity. |
| `GET` | `/api/workspaces/:workspaceId/activity` | Activity events for a workspace. Optional `?featureId=` and `?taskId=` filters. |

Health check (root router, not under `/api`):

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Returns `{"status":"ok"}`. Used by container health checks. |

### Error response shape

Every error response includes:

```json
{
  "code": "DATABASE_NOT_FOUND",
  "message": "workspace not found: <id>",
  "source": "database",
  "retryable": false,
  "cached_data": null
}
```

`code` is machine-readable; `source` is one of `github`, `database`, `adapter`, `validation`; `retryable` signals whether the client should retry.

---

## Backward Compatibility

- **No existing routes are modified.** This feature adds new routes; it does not change or remove existing `workflow-backend` endpoints.
- `digital-factory-ui` continues calling GitHub directly until `workspace-tabs-data-flow` switches it over. Both the existing browser-side GitHub reads and the new backend routes coexist during the transition period.
- The `/healthz` endpoint is unaffected.

---

## sqlc Query Compilation

The `sqlc` generated query code in `internal/database/queries.go` is written by hand (not generated from `.sql` files in this repo) because `workflow-backend` is the read-side service and does not own migrations. Queries are validated via `go build` and `go vet`. If schema changes in `workspace-github-adapter` require adding new queries here, update `internal/database/queries.go` and run `go build ./...` to verify.

---

## Running Locally

```bash
# 1. Ensure PostgreSQL is running and migrations are applied (from workspace-github-adapter repo).
# 2. Start adapter-service (from workspace-github-adapter repo).
# 3. Start api-service:

DATABASE_URL="postgresql://user:pass@localhost:5432/workflow?sslmode=disable" \
ADAPTER_SERVICE_URL="http://localhost:8080" \
GIN_MODE=debug \
go run ./cmd/api-service
```

Or via Docker Compose (defined in `workflow` repo, added in T6):

```bash
docker compose up api-service
```

## Running Tests

```bash
# Unit and integration tests (no external dependencies required):
go test ./...

# With verbose output:
go test -v ./...

# A specific package:
go test -v ./internal/integration/...
```
