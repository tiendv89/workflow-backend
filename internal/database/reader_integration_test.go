//go:build integration

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestReaderIntegration_UUIDReferencesWithNameMetadata(t *testing.T) {
	databaseURL := os.Getenv("WORKFLOW_BACKEND_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("WORKFLOW_BACKEND_TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	schemaName := fmt.Sprintf("reader_integration_%d", time.Now().UnixNano())

	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect admin db: %v", err)
	}
	if err := adminPool.Ping(ctx); err != nil {
		adminPool.Close()
		t.Fatalf("ping admin db: %v", err)
	}
	quotedSchemaName := pgx.Identifier{schemaName}.Sanitize()
	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA `+quotedSchemaName); err != nil {
		adminPool.Close()
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+quotedSchemaName+` CASCADE`)
		adminPool.Close()
	})

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse db config: %v", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schemaName

	rawPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	defer rawPool.Close()
	if err := rawPool.Ping(ctx); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	pool := &Pool{Pool: rawPool}

	applyAdapterMigrations(ctx, t, databaseURL, schemaName)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	featureID := "22222222-2222-2222-2222-222222222222"
	taskID := "33333333-3333-3333-3333-333333333333"

	if _, err := pool.Exec(ctx, `
		INSERT INTO workspaces (id, slug, name, management_repo_id, branch_pattern, created_at, updated_at)
		VALUES ($1, 'test-workspace', 'Test Workspace', 'management-repo', 'feature/{feature_id}-{work_id}', now(), now());

		INSERT INTO workspace_features (
			id, workspace_id, feature_id, feature_name, title, feature_status, current_stage,
			next_action, stages, source_path, source_hash, created_at, updated_at
		)
		VALUES (
			'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', $1, $2, 'workspace-data-backend',
			'Workspace Data Backend', 'ready', 'tasks', 'next', '{}',
			'docs/features/workspace-data-backend/status.yaml', NULL, now(), now()
		);

		INSERT INTO workspace_tasks (
			id, workspace_id, feature_id, feature_name, task_id, task_name, title, repo, status,
			depends_on, blocked_reason, branch, execution, pr, workspace_pr, source_path,
			source_hash, created_at, updated_at
		)
		VALUES (
			'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', $1, $2, 'workspace-data-backend',
			$3, 'T1', 'First task', 'workflow-backend', 'ready', '[]',
			NULL, 'feature/workspace-data-backend-T1', '{}', '{}', '{}',
			'docs/features/workspace-data-backend/tasks/T1.yaml', NULL, now(), now()
		);

		INSERT INTO workspace_activity_events (
			id, workspace_id, scope_type, feature_id, task_id, action, actor, occurred_at,
			note, sequence, raw_event, created_at
		)
		VALUES (
			'cccccccc-cccc-cccc-cccc-cccccccccccc', $1, 'task', $2, $3,
			'ready', 'agent@example.com', $4, 'ready note', 1, '{}', now()
		);
	`, workspaceID, featureID, taskID, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("seed reader data: %v", err)
	}

	reader := NewReader(pool)
	feature, err := reader.GetWorkspaceFeature(ctx, workspaceID, featureID)
	if err != nil {
		t.Fatalf("GetWorkspaceFeature by UUID: %v", err)
	}
	if feature.FeatureName != "workspace-data-backend" {
		t.Fatalf("FeatureName = %q, want workspace-data-backend", feature.FeatureName)
	}

	if _, err := reader.GetWorkspaceFeature(ctx, workspaceID, "workspace-data-backend"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetWorkspaceFeature by name must not resolve references, got %v", err)
	}

	task, err := reader.GetWorkspaceTask(ctx, workspaceID, featureID, taskID)
	if err != nil {
		t.Fatalf("GetWorkspaceTask by UUID: %v", err)
	}
	if task.TaskName != "T1" {
		t.Fatalf("TaskName = %q, want T1", task.TaskName)
	}

	if _, err := reader.GetWorkspaceTask(ctx, workspaceID, featureID, "T1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetWorkspaceTask by name must not resolve references, got %v", err)
	}

	workspaceTask, err := reader.GetWorkspaceTaskByID(ctx, workspaceID, taskID)
	if err != nil {
		t.Fatalf("GetWorkspaceTaskByID by workspace and task UUID: %v", err)
	}
	if workspaceTask.FeatureName != "workspace-data-backend" {
		t.Fatalf("FeatureName = %q, want workspace-data-backend", workspaceTask.FeatureName)
	}

	if _, err := reader.GetWorkspaceTaskByID(ctx, workspaceID, "T1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetWorkspaceTaskByID by name must not resolve references, got %v", err)
	}

	events, err := reader.ListActivityEvents(ctx, workspaceID, featureID, taskID)
	if err != nil {
		t.Fatalf("ListActivityEvents by UUID refs: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 activity event, got %d", len(events))
	}
}

func applyAdapterMigrations(ctx context.Context, t *testing.T, databaseURL, schemaName string) {
	t.Helper()

	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse migration db config: %v", err)
	}
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["search_path"] = schemaName

	migrationDB := stdlib.OpenDB(*cfg)
	migrationDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = migrationDB.Close() })
	if err := migrationDB.PingContext(ctx); err != nil {
		t.Fatalf("ping migration db: %v", err)
	}

	if err := runGooseUp(ctx, migrationDB, adapterMigrationsDir(t)); err != nil {
		t.Fatalf("apply workspace-github-adapter migrations: %v", err)
	}
}

func runGooseUp(ctx context.Context, db *sql.DB, migrationsDir string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	goose.SetBaseFS(os.DirFS(filepath.Dir(migrationsDir)))
	return goose.UpContext(ctx, db, filepath.Base(migrationsDir))
}

func adapterMigrationsDir(t *testing.T) string {
	t.Helper()

	if dir := os.Getenv("WORKSPACE_GITHUB_ADAPTER_MIGRATIONS_DIR"); dir != "" {
		return verifiedMigrationsDir(t, dir)
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	dir := filepath.Join(repoRoot, "..", "workspace-github-adapter", "database", "migrations")
	return verifiedMigrationsDir(t, dir)
}

func verifiedMigrationsDir(t *testing.T, dir string) string {
	t.Helper()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("workspace-github-adapter migrations dir %q is not available: %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("workspace-github-adapter migrations path %q is not a directory", dir)
	}
	return dir
}
