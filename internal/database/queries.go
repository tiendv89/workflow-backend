package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrNotFound is returned when a queried row does not exist.
var ErrNotFound = errors.New("not found")

// Reader executes read-only SQL queries against the shared PostgreSQL schema.
// All UUID parameters are accepted as hex strings and parsed internally.
type Reader struct {
	db *Pool
}

// FeatureSearchFilters contains optional filters and result controls for feature search.
type FeatureSearchFilters struct {
	Title  string
	Status string
	Sort   string
	Limit  int
}

// NewReader creates a new Reader.
func NewReader(db *Pool) *Reader {
	return &Reader{db: db}
}

// ListWorkspaces returns all workspaces ordered by updated_at desc.
func (r *Reader) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	const q = `
		SELECT id, slug, name, management_repo_id, branch_pattern, created_at, updated_at
		FROM workspaces
		ORDER BY updated_at DESC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Workspace
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.Slug, &w.Name, &w.ManagementRepoID, &w.BranchPattern, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetWorkspace returns a single workspace by UUID string.
// Returns ErrNotFound if no row exists.
func (r *Reader) GetWorkspace(ctx context.Context, workspaceID string) (Workspace, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return Workspace{}, err
	}
	const q = `
		SELECT id, slug, name, management_repo_id, branch_pattern, created_at, updated_at
		FROM workspaces WHERE id = $1`
	row := r.db.QueryRow(ctx, q, uid)
	var w Workspace
	if err := row.Scan(&w.ID, &w.Slug, &w.Name, &w.ManagementRepoID, &w.BranchPattern, &w.CreatedAt, &w.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Workspace{}, ErrNotFound
		}
		return Workspace{}, err
	}
	return w, nil
}

// GetGitHubSource returns the GitHub source record for a workspace.
// Returns ErrNotFound if no row exists.
func (r *Reader) GetGitHubSource(ctx context.Context, workspaceID string) (WorkspaceGitHubSource, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return WorkspaceGitHubSource{}, err
	}
	const q = `
		SELECT id, workspace_id, repo_url, repo_owner, repo_name, default_branch, created_at, updated_at
		FROM workspace_github_sources WHERE workspace_id = $1`
	row := r.db.QueryRow(ctx, q, uid)
	var s WorkspaceGitHubSource
	if err := row.Scan(&s.ID, &s.WorkspaceID, &s.RepoURL, &s.RepoOwner, &s.RepoName, &s.DefaultBranch, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkspaceGitHubSource{}, ErrNotFound
		}
		return WorkspaceGitHubSource{}, err
	}
	return s, nil
}

// ListGitHubSources returns all rows from workspace_github_sources.
func (r *Reader) ListGitHubSources(ctx context.Context) ([]WorkspaceGitHubSource, error) {
	const q = `
		SELECT id, workspace_id, repo_url, repo_owner, repo_name, default_branch, created_at, updated_at
		FROM workspace_github_sources`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceGitHubSource
	for rows.Next() {
		var s WorkspaceGitHubSource
		if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.RepoURL, &s.RepoOwner, &s.RepoName, &s.DefaultBranch, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListLatestSyncRunsPerWorkspace returns one sync run per workspace (the most recent).
func (r *Reader) ListLatestSyncRunsPerWorkspace(ctx context.Context) ([]WorkspaceSyncRun, error) {
	const q = `
		SELECT DISTINCT ON (workspace_id)
		       id, workspace_id, trigger, branch, feature_id, task_id, mode, status,
		       commit_sha, changed_paths, started_at, finished_at, error_code, error_message, metadata
		FROM workspace_sync_runs
		ORDER BY workspace_id, started_at DESC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceSyncRun
	for rows.Next() {
		var s WorkspaceSyncRun
		if err := scanSyncRun(rows, &s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetLatestSyncRun returns the most recent sync run for a workspace.
// Returns ErrNotFound if no row exists.
func (r *Reader) GetLatestSyncRun(ctx context.Context, workspaceID string) (WorkspaceSyncRun, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return WorkspaceSyncRun{}, err
	}
	const q = `
		SELECT id, workspace_id, trigger, branch, feature_id, task_id, mode, status,
		       commit_sha, changed_paths, started_at, finished_at, error_code, error_message, metadata
		FROM workspace_sync_runs
		WHERE workspace_id = $1
		ORDER BY started_at DESC
		LIMIT 1`
	row := r.db.QueryRow(ctx, q, uid)
	var s WorkspaceSyncRun
	if err := row.Scan(
		&s.ID, &s.WorkspaceID, &s.Trigger, &s.Branch, &s.FeatureID, &s.TaskID,
		&s.Mode, &s.Status, &s.CommitSha, &s.ChangedPaths,
		&s.StartedAt, &s.FinishedAt, &s.ErrorCode, &s.ErrorMessage, &s.Metadata,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkspaceSyncRun{}, ErrNotFound
		}
		return WorkspaceSyncRun{}, err
	}
	return s, nil
}

// ListWorkspaceFeatures returns all features for a workspace.
func (r *Reader) ListWorkspaceFeatures(ctx context.Context, workspaceID string) ([]WorkspaceFeature, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, workspace_id, feature_id, title, feature_status, current_stage, next_action,
		       stages, source_path, source_hash, created_at, updated_at
		FROM workspace_features
		WHERE workspace_id = $1
		ORDER BY updated_at DESC`
	rows, err := r.db.Query(ctx, q, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceFeature
	for rows.Next() {
		var f WorkspaceFeature
		if err := scanFeature(rows, &f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SearchWorkspaceFeatures returns workspace features filtered by title/status with safe sorting and limiting.
func (r *Reader) SearchWorkspaceFeatures(ctx context.Context, workspaceID string, filters FeatureSearchFilters) ([]WorkspaceFeature, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}

	where := []string{"workspace_id = $1"}
	args := []interface{}{uid}
	argPos := 2
	if filters.Title != "" {
		where = append(where, fmt.Sprintf("title ILIKE $%d", argPos))
		args = append(args, "%"+filters.Title+"%")
		argPos++
	}
	if filters.Status != "" {
		where = append(where, fmt.Sprintf("feature_status = $%d", argPos))
		args = append(args, filters.Status)
		argPos++
	}

	orderBy := "updated_at DESC, feature_id ASC"
	switch filters.Sort {
	case "title_asc":
		orderBy = "title ASC, feature_id ASC"
	case "title_desc":
		orderBy = "title DESC, feature_id ASC"
	case "status_asc":
		orderBy = "feature_status ASC NULLS LAST, feature_id ASC"
	case "status_desc":
		orderBy = "feature_status DESC NULLS LAST, feature_id ASC"
	case "updated_at_asc", "time_asc":
		orderBy = "updated_at ASC, feature_id ASC"
	case "updated_at_desc", "time_desc", "":
		orderBy = "updated_at DESC, feature_id ASC"
	}

	limitClause := ""
	if filters.Limit > 0 {
		limitClause = fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filters.Limit)
	}

	q := fmt.Sprintf(`
		SELECT id, workspace_id, feature_id, title, feature_status, current_stage, next_action,
		       stages, source_path, source_hash, created_at, updated_at
		FROM workspace_features
		WHERE %s
		ORDER BY %s%s`, strings.Join(where, " AND "), orderBy, limitClause)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceFeature
	for rows.Next() {
		var f WorkspaceFeature
		if err := scanFeature(rows, &f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// GetWorkspaceFeature returns a single feature.
// Returns ErrNotFound if no row exists.
func (r *Reader) GetWorkspaceFeature(ctx context.Context, workspaceID, featureID string) (WorkspaceFeature, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return WorkspaceFeature{}, err
	}
	const q = `
		SELECT id, workspace_id, feature_id, title, feature_status, current_stage, next_action,
		       stages, source_path, source_hash, created_at, updated_at
		FROM workspace_features
		WHERE workspace_id = $1 AND id::text = $2`
	row := r.db.QueryRow(ctx, q, uid, featureID)
	var f WorkspaceFeature
	if err := row.Scan(
		&f.ID, &f.WorkspaceID, &f.FeatureID, &f.Title, &f.FeatureStatus, &f.CurrentStage,
		&f.NextAction, &f.Stages, &f.SourcePath, &f.SourceHash, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkspaceFeature{}, ErrNotFound
		}
		return WorkspaceFeature{}, err
	}
	return f, nil
}

// ListFeatureDocuments returns documents for a specific feature.
func (r *Reader) ListFeatureDocuments(ctx context.Context, workspaceID, featureID string) ([]WorkspaceFeatureDocument, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, workspace_id, feature_id, feature_name, document_type, source_path, url, created_at, updated_at
		FROM workspace_feature_documents
		WHERE workspace_id = $1 AND feature_id::text = $2
		ORDER BY document_type`
	rows, err := r.db.Query(ctx, q, uid, featureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceFeatureDocument
	for rows.Next() {
		var d WorkspaceFeatureDocument
		if err := rows.Scan(&d.ID, &d.WorkspaceID, &d.FeatureID, &d.FeatureName, &d.DocumentType, &d.SourcePath, &d.URL, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListFeatureTasks returns all tasks for a specific feature.
func (r *Reader) ListFeatureTasks(ctx context.Context, workspaceID, featureID string) ([]WorkspaceTask, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT t.id, t.workspace_id, t.feature_id, t.feature_name, t.task_id, t.title,
		       t.repo, t.status, t.depends_on, t.blocked_reason, t.branch, t.execution,
		       t.pr, t.workspace_pr, t.source_path, t.source_hash, t.created_at, t.updated_at
		FROM workspace_tasks t
		WHERE t.workspace_id = $1 AND t.feature_id::text = $2
		ORDER BY t.task_id`
	rows, err := r.db.Query(ctx, q, uid, featureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceTask
	for rows.Next() {
		var t WorkspaceTask
		if err := scanTask(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListWorkspaceTasks returns all tasks for a workspace.
func (r *Reader) ListWorkspaceTasks(ctx context.Context, workspaceID string) ([]WorkspaceTask, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT t.id, t.workspace_id, t.feature_id, t.feature_name, t.task_id, t.title,
		       t.repo, t.status, t.depends_on, t.blocked_reason, t.branch, t.execution,
		       t.pr, t.workspace_pr, t.source_path, t.source_hash, t.created_at, t.updated_at
		FROM workspace_tasks t
		WHERE t.workspace_id = $1
		ORDER BY t.feature_name, t.task_id`
	rows, err := r.db.Query(ctx, q, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceTask
	for rows.Next() {
		var t WorkspaceTask
		if err := scanTask(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetWorkspaceTask returns a single task by workspace UUID, feature_id, and task row UUID.
// Returns ErrNotFound if no row exists.
func (r *Reader) GetWorkspaceTask(ctx context.Context, workspaceID, featureID, taskID string) (WorkspaceTask, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return WorkspaceTask{}, err
	}
	const q = `
		SELECT t.id, t.workspace_id, t.feature_id, t.feature_name, t.task_id, t.title,
		       t.repo, t.status, t.depends_on, t.blocked_reason, t.branch, t.execution,
		       t.pr, t.workspace_pr, t.source_path, t.source_hash, t.created_at, t.updated_at
		FROM workspace_tasks t
		WHERE t.workspace_id = $1 AND t.feature_id::text = $2 AND t.id::text = $3`
	row := r.db.QueryRow(ctx, q, uid, featureID, taskID)
	var t WorkspaceTask
	if err := row.Scan(
		&t.ID, &t.WorkspaceID, &t.FeatureID, &t.FeatureName, &t.TaskID, &t.Title, &t.Repo, &t.Status,
		&t.DependsOn, &t.BlockedReason, &t.Branch, &t.Execution, &t.Pr, &t.WorkspacePr,
		&t.SourcePath, &t.SourceHash, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkspaceTask{}, ErrNotFound
		}
		return WorkspaceTask{}, err
	}
	return t, nil
}

// ListActivityEvents returns activity events for a workspace filtered by scope.
func (r *Reader) ListActivityEvents(ctx context.Context, workspaceID, featureID, taskID string) ([]WorkspaceActivityEvent, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	var (
		q    string
		args []interface{}
	)
	switch {
	case taskID != "" && featureID != "":
		q = `
			SELECT id, workspace_id, scope_type, feature_id, task_id, action, actor,
			       occurred_at, note, sequence, raw_event, created_at
			FROM workspace_activity_events
			WHERE workspace_id = $1 AND feature_id = $2 AND task_id = $3
			ORDER BY occurred_at DESC, sequence DESC`
		args = []interface{}{uid, featureID, taskID}
	case featureID != "":
		q = `
			SELECT id, workspace_id, scope_type, feature_id, task_id, action, actor,
			       occurred_at, note, sequence, raw_event, created_at
			FROM workspace_activity_events
			WHERE workspace_id = $1 AND feature_id = $2
			ORDER BY occurred_at DESC, sequence DESC`
		args = []interface{}{uid, featureID}
	default:
		q = `
			SELECT id, workspace_id, scope_type, feature_id, task_id, action, actor,
			       occurred_at, note, sequence, raw_event, created_at
			FROM workspace_activity_events
			WHERE workspace_id = $1
			ORDER BY occurred_at DESC, sequence DESC`
		args = []interface{}{uid}
	}

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceActivityEvent
	for rows.Next() {
		var e WorkspaceActivityEvent
		if err := rows.Scan(
			&e.ID, &e.WorkspaceID, &e.ScopeType, &e.FeatureID, &e.TaskID,
			&e.Action, &e.Actor, &e.OccurredAt, &e.Note,
			&e.Sequence, &e.RawEvent, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UUIDString converts a pgtype.UUID to its hex string representation.
func UUIDString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// --- helpers ---

func parseUUID(s string) (pgtype.UUID, error) {
	var uid pgtype.UUID
	if err := uid.Scan(s); err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid UUID %q: %w", s, err)
	}
	return uid, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSyncRun(row rowScanner, s *WorkspaceSyncRun) error {
	return row.Scan(
		&s.ID, &s.WorkspaceID, &s.Trigger, &s.Branch, &s.FeatureID, &s.TaskID,
		&s.Mode, &s.Status, &s.CommitSha, &s.ChangedPaths,
		&s.StartedAt, &s.FinishedAt, &s.ErrorCode, &s.ErrorMessage, &s.Metadata,
	)
}

func scanFeature(row rowScanner, f *WorkspaceFeature) error {
	return row.Scan(
		&f.ID, &f.WorkspaceID, &f.FeatureID, &f.Title, &f.FeatureStatus, &f.CurrentStage,
		&f.NextAction, &f.Stages, &f.SourcePath, &f.SourceHash, &f.CreatedAt, &f.UpdatedAt,
	)
}

func scanTask(row rowScanner, t *WorkspaceTask) error {
	return row.Scan(
		&t.ID, &t.WorkspaceID, &t.FeatureID, &t.FeatureName, &t.TaskID, &t.Title, &t.Repo, &t.Status,
		&t.DependsOn, &t.BlockedReason, &t.Branch, &t.Execution, &t.Pr, &t.WorkspacePr,
		&t.SourcePath, &t.SourceHash, &t.CreatedAt, &t.UpdatedAt,
	)
}
