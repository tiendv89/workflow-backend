//go:build integration

package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA `+schemaName); err != nil {
		adminPool.Close()
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schemaName+` CASCADE`)
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

	if _, err := pool.Exec(ctx, readerIntegrationSchemaSQL); err != nil {
		t.Fatalf("create test tables: %v", err)
	}

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

	events, err := reader.ListActivityEvents(ctx, workspaceID, featureID, taskID)
	if err != nil {
		t.Fatalf("ListActivityEvents by UUID refs: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 activity event, got %d", len(events))
	}
}

const readerIntegrationSchemaSQL = `
CREATE TABLE workspaces (
	id uuid PRIMARY KEY,
	slug text NOT NULL UNIQUE,
	name text NOT NULL,
	management_repo_id text NOT NULL,
	branch_pattern text,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
);

CREATE TABLE workspace_repos (
	id uuid PRIMARY KEY,
	workspace_id uuid NOT NULL REFERENCES workspaces(id),
	repo_id text NOT NULL,
	base_branch text,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
);

CREATE TABLE workspace_features (
	id uuid PRIMARY KEY,
	workspace_id uuid NOT NULL REFERENCES workspaces(id),
	feature_id uuid NOT NULL,
	feature_name text NOT NULL,
	title text NOT NULL,
	feature_status text,
	current_stage text,
	next_action text,
	stages jsonb,
	source_path text NOT NULL,
	source_hash text,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL,
	UNIQUE (workspace_id, feature_id)
);

CREATE TABLE workspace_feature_documents (
	id uuid PRIMARY KEY,
	workspace_id uuid NOT NULL REFERENCES workspaces(id),
	feature_id uuid NOT NULL,
	feature_name text NOT NULL,
	document_type text NOT NULL,
	source_path text NOT NULL,
	url text,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
);

CREATE TABLE workspace_tasks (
	id uuid PRIMARY KEY,
	workspace_id uuid NOT NULL REFERENCES workspaces(id),
	feature_id uuid NOT NULL,
	feature_name text NOT NULL,
	task_id uuid NOT NULL,
	task_name text NOT NULL,
	title text NOT NULL,
	repo text,
	status text,
	depends_on jsonb NOT NULL,
	blocked_reason text,
	branch text,
	execution jsonb,
	pr jsonb,
	workspace_pr jsonb,
	source_path text NOT NULL,
	source_hash text,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL,
	UNIQUE (workspace_id, feature_id, task_id)
);

CREATE TABLE workspace_activity_events (
	id uuid PRIMARY KEY,
	workspace_id uuid NOT NULL REFERENCES workspaces(id),
	scope_type text NOT NULL,
	feature_id uuid,
	task_id uuid,
	action text,
	actor text,
	occurred_at text,
	note text,
	sequence integer NOT NULL,
	raw_event jsonb NOT NULL,
	created_at timestamptz NOT NULL
);

CREATE TABLE workspace_github_sources (
	id uuid PRIMARY KEY,
	workspace_id uuid NOT NULL REFERENCES workspaces(id),
	repo_url text NOT NULL,
	repo_owner text NOT NULL,
	repo_name text NOT NULL,
	default_branch text,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
);

CREATE TABLE workspace_sync_runs (
	id uuid PRIMARY KEY,
	workspace_id uuid NOT NULL REFERENCES workspaces(id),
	trigger text NOT NULL,
	branch text,
	feature_id text,
	task_id text,
	mode text NOT NULL,
	status text NOT NULL,
	commit_sha text,
	changed_paths jsonb,
	started_at timestamptz NOT NULL,
	finished_at timestamptz,
	error_code text,
	error_message text,
	metadata jsonb
);
`
