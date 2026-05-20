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
	Page   int
	Limit  int
}

// TaskSearchFilters contains optional filters and result controls for task search.
type TaskSearchFilters struct {
	TaskID string
	Title  string
	Status string
	Repo   string
	Sort   string
	Page   int
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
		SELECT id, workspace_id, feature_id, feature_name, title, feature_status, current_stage, next_action,
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

// SearchWorkspaceFeatures returns workspace features filtered by search/title/status with safe sorting and limiting.
func (r *Reader) SearchWorkspaceFeatures(ctx context.Context, workspaceID string, filters FeatureSearchFilters) ([]WorkspaceFeature, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}

	where, args, argPos := buildFeatureWhere(uid, filters)

	orderBy := "updated_at DESC, feature_name ASC"
	switch filters.Sort {
	case "title_asc":
		orderBy = "title ASC, feature_name ASC"
	case "title_desc":
		orderBy = "title DESC, feature_name ASC"
	case "status_asc":
		orderBy = "feature_status ASC NULLS LAST, feature_name ASC"
	case "status_desc":
		orderBy = "feature_status DESC NULLS LAST, feature_name ASC"
	case "updated_at_asc", "time_asc", "createdAt":
		orderBy = "updated_at ASC, feature_name ASC"
	case "updated_at_desc", "time_desc", "-createdAt", "":
		orderBy = "updated_at DESC, feature_name ASC"
	}

	limitClause := ""
	if filters.Limit > 0 {
		limitClause = fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filters.Limit)
		argPos++
	}
	offsetClause := ""
	if filters.Page > 1 && filters.Limit > 0 {
		offsetClause = fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, (filters.Page-1)*filters.Limit)
	}

	q := fmt.Sprintf(`
		SELECT id, workspace_id, feature_id, feature_name, title, feature_status, current_stage, next_action,
		       stages, source_path, source_hash, created_at, updated_at
		FROM workspace_features
		WHERE %s
		ORDER BY %s%s%s`, strings.Join(where, " AND "), orderBy, limitClause, offsetClause)
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

// ListFeatureTaskCounts returns aggregated task counts for the requested features.
func (r *Reader) ListFeatureTaskCounts(ctx context.Context, workspaceID string, featureIDs []string) ([]WorkspaceFeatureTaskCounts, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	if len(featureIDs) == 0 {
		return []WorkspaceFeatureTaskCounts{}, nil
	}

	args := make([]interface{}, 1, len(featureIDs)+1)
	args[0] = uid
	placeholders := make([]string, 0, len(featureIDs))
	for _, featureID := range featureIDs {
		fid, err := parseUUID(featureID)
		if err != nil {
			return nil, err
		}
		args = append(args, fid)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}

	const qTemplate = `
		SELECT feature_id,
		       count(*) AS total,
		       count(*) FILTER (WHERE status = 'done') AS done,
		       count(*) FILTER (WHERE status = 'in_progress') AS in_progress,
		       count(*) FILTER (WHERE status = 'blocked') AS blocked,
		       count(*) FILTER (WHERE status = 'ready') AS ready,
		       count(*) FILTER (WHERE status = 'todo') AS todo
		FROM workspace_tasks
		WHERE workspace_id = $1 AND feature_id IN (%s)
		GROUP BY feature_id`
	q := fmt.Sprintf(qTemplate, strings.Join(placeholders, ", "))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkspaceFeatureTaskCounts
	for rows.Next() {
		var row WorkspaceFeatureTaskCounts
		if err := rows.Scan(&row.FeatureID, &row.Total, &row.Done, &row.InProgress, &row.Blocked, &row.Ready, &row.Todo); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// GetWorkspaceFeature returns a single feature by feature UUID.
// Returns ErrNotFound if no row exists.
func (r *Reader) GetWorkspaceFeature(ctx context.Context, workspaceID, featureID string) (WorkspaceFeature, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return WorkspaceFeature{}, err
	}
	fid, err := parseUUID(strings.TrimSpace(featureID))
	if err != nil {
		return WorkspaceFeature{}, ErrNotFound
	}
	const q = `
		SELECT id, workspace_id, feature_id, feature_name, title, feature_status, current_stage, next_action,
		       stages, source_path, source_hash, created_at, updated_at
		FROM workspace_features
		WHERE workspace_id = $1 AND feature_id = $2`
	row := r.db.QueryRow(ctx, q, uid, fid)
	var f WorkspaceFeature
	if err := row.Scan(
		&f.ID, &f.WorkspaceID, &f.FeatureID, &f.FeatureName, &f.Title, &f.FeatureStatus, &f.CurrentStage,
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
	fid, err := parseUUID(strings.TrimSpace(featureID))
	if err != nil {
		return nil, ErrNotFound
	}
	const q = `
		SELECT id, workspace_id, feature_id, feature_name, document_type, source_path, url, created_at, updated_at
		FROM workspace_feature_documents
		WHERE workspace_id = $1 AND feature_id = $2
		ORDER BY document_type`
	rows, err := r.db.Query(ctx, q, uid, fid)
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

// SearchFeatureTasks returns tasks for a feature filtered by task_id/title/status/repo with safe sorting and limiting.
func (r *Reader) SearchFeatureTasks(ctx context.Context, workspaceID, featureID string, filters TaskSearchFilters) ([]WorkspaceTask, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	fid, err := parseUUID(strings.TrimSpace(featureID))
	if err != nil {
		return nil, ErrNotFound
	}
	return r.searchTasks(ctx, []string{"t.workspace_id = $1", "t.feature_id = $2"}, []interface{}{uid, fid}, 3, filters)
}

// SearchWorkspaceTasks returns tasks for a workspace filtered by task_id/title/status/repo with safe sorting and limiting.
func (r *Reader) SearchWorkspaceTasks(ctx context.Context, workspaceID string, filters TaskSearchFilters) ([]WorkspaceTask, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}

	return r.searchTasks(ctx, []string{"t.workspace_id = $1"}, []interface{}{uid}, 2, filters)
}

// ListFeatureTasks returns all tasks for a specific feature.
func (r *Reader) ListFeatureTasks(ctx context.Context, workspaceID, featureID string) ([]WorkspaceTask, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	fid, err := parseUUID(strings.TrimSpace(featureID))
	if err != nil {
		return nil, ErrNotFound
	}
	q := fmt.Sprintf(`
		SELECT t.id, t.workspace_id, t.feature_id, t.feature_name, t.task_id, t.task_name, t.title,
		       t.repo, t.status, t.depends_on, t.blocked_reason, t.branch, t.execution,
		       t.pr, t.workspace_pr, t.source_path, t.source_hash, t.created_at, t.updated_at
		FROM workspace_tasks t
		WHERE t.workspace_id = $1 AND t.feature_id = $2
		ORDER BY %s`, taskIDOrderAsc("t"))
	rows, err := r.db.Query(ctx, q, uid, fid)
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
	q := fmt.Sprintf(`
		SELECT t.id, t.workspace_id, t.feature_id, t.feature_name, t.task_id, t.task_name, t.title,
		       t.repo, t.status, t.depends_on, t.blocked_reason, t.branch, t.execution,
		       t.pr, t.workspace_pr, t.source_path, t.source_hash, t.created_at, t.updated_at
		FROM workspace_tasks t
		WHERE t.workspace_id = $1
		ORDER BY t.feature_name, %s`, taskIDOrderAsc("t"))
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

// GetWorkspaceTask returns a single task by workspace, feature, and task UUIDs.
// Returns ErrNotFound if no row exists.
func (r *Reader) GetWorkspaceTask(ctx context.Context, workspaceID, featureID, taskID string) (WorkspaceTask, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return WorkspaceTask{}, err
	}
	fid, err := parseUUID(strings.TrimSpace(featureID))
	if err != nil {
		return WorkspaceTask{}, ErrNotFound
	}
	tid, err := parseUUID(strings.TrimSpace(taskID))
	if err != nil {
		return WorkspaceTask{}, ErrNotFound
	}
	const q = `
		SELECT t.id, t.workspace_id, t.feature_id, t.feature_name, t.task_id, t.task_name, t.title,
		       t.repo, t.status, t.depends_on, t.blocked_reason, t.branch, t.execution,
		       t.pr, t.workspace_pr, t.source_path, t.source_hash, t.created_at, t.updated_at
		FROM workspace_tasks t
		WHERE t.workspace_id = $1 AND t.feature_id = $2 AND t.task_id = $3`
	row := r.db.QueryRow(ctx, q, uid, fid, tid)
	var t WorkspaceTask
	if err := row.Scan(
		&t.ID, &t.WorkspaceID, &t.FeatureID, &t.FeatureName, &t.TaskID, &t.TaskName, &t.Title, &t.Repo, &t.Status,
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

// GetWorkspaceTaskByID returns a single task by workspace and task UUID.
// Returns ErrNotFound if no row exists.
func (r *Reader) GetWorkspaceTaskByID(ctx context.Context, workspaceID, taskID string) (WorkspaceTask, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return WorkspaceTask{}, err
	}
	tid, err := parseUUID(strings.TrimSpace(taskID))
	if err != nil {
		return WorkspaceTask{}, ErrNotFound
	}
	const q = `
		SELECT t.id, t.workspace_id, t.feature_id, t.feature_name, t.task_id, t.task_name, t.title,
		       t.repo, t.status, t.depends_on, t.blocked_reason, t.branch, t.execution,
		       t.pr, t.workspace_pr, t.source_path, t.source_hash, t.created_at, t.updated_at
		FROM workspace_tasks t
		WHERE t.workspace_id = $1 AND t.task_id = $2`
	row := r.db.QueryRow(ctx, q, uid, tid)
	var t WorkspaceTask
	if err := scanTask(row, &t); err != nil {
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
	args := []interface{}{uid}
	filterClause, filterArgs, _, err := activityFilterClause(featureID, taskID, 2)
	if err != nil {
		return nil, err
	}
	args = append(args, filterArgs...)

	q := fmt.Sprintf(`
		SELECT id, workspace_id, scope_type, feature_id, task_id, action, actor,
		       occurred_at, note, sequence, raw_event, created_at
		FROM workspace_activity_events
		WHERE workspace_id = $1%s
		ORDER BY occurred_at DESC, sequence DESC`, filterClause)

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
		return pgtype.UUID{}, ErrNotFound
	}
	return uid, nil
}

// buildFeatureWhere constructs the WHERE clause conditions for feature search/count queries.
func buildFeatureWhere(workspaceUID pgtype.UUID, filters FeatureSearchFilters) ([]string, []interface{}, int) {
	where := []string{"workspace_id = $1"}
	args := []interface{}{workspaceUID}
	argPos := 2
	if filters.Title != "" {
		where = append(where, fmt.Sprintf("title ILIKE $%d", argPos))
		args = append(args, "%"+filters.Title+"%")
		argPos++
	}
	if filters.Status != "" {
		statuses := splitCSV(filters.Status)
		if len(statuses) == 1 {
			where = append(where, fmt.Sprintf("feature_status = $%d", argPos))
			args = append(args, statuses[0])
			argPos++
		} else if len(statuses) > 1 {
			where = append(where, fmt.Sprintf("feature_status = ANY($%d)", argPos))
			args = append(args, statuses)
			argPos++
		}
	}
	return where, args, argPos
}

// buildTaskWhere constructs the WHERE clause conditions for task search/count queries.
func buildTaskWhere(baseWhere []string, baseArgs []interface{}, argPos int, filters TaskSearchFilters) ([]string, []interface{}, int) {
	where := make([]string, len(baseWhere), len(baseWhere)+4)
	copy(where, baseWhere)
	args := make([]interface{}, len(baseArgs), len(baseArgs)+4)
	copy(args, baseArgs)
	if filters.TaskID != "" {
		where = append(where, fmt.Sprintf("t.task_name ILIKE $%d", argPos))
		args = append(args, "%"+filters.TaskID+"%")
		argPos++
	}
	if filters.Title != "" {
		where = append(where, fmt.Sprintf("t.title ILIKE $%d", argPos))
		args = append(args, "%"+filters.Title+"%")
		argPos++
	}
	if filters.Status != "" {
		statuses := splitCSV(filters.Status)
		if len(statuses) == 1 {
			where = append(where, fmt.Sprintf("t.status = $%d", argPos))
			args = append(args, statuses[0])
			argPos++
		} else if len(statuses) > 1 {
			where = append(where, fmt.Sprintf("t.status = ANY($%d)", argPos))
			args = append(args, statuses)
			argPos++
		}
	}
	if filters.Repo != "" {
		where = append(where, fmt.Sprintf("t.repo = $%d", argPos))
		args = append(args, filters.Repo)
		argPos++
	}
	return where, args, argPos
}

// CountWorkspaceFeatures returns the total number of features matching the filters (ignores Page/Limit).
func (r *Reader) CountWorkspaceFeatures(ctx context.Context, workspaceID string, filters FeatureSearchFilters) (int, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return 0, err
	}
	where, args, _ := buildFeatureWhere(uid, filters)
	q := fmt.Sprintf(`SELECT COUNT(*) FROM workspace_features WHERE %s`, strings.Join(where, " AND "))
	var count int
	if err := r.db.QueryRow(ctx, q, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// CountWorkspaceTasks returns the total number of tasks in a workspace matching the filters.
func (r *Reader) CountWorkspaceTasks(ctx context.Context, workspaceID string, filters TaskSearchFilters) (int, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return 0, err
	}
	where, args, _ := buildTaskWhere([]string{"t.workspace_id = $1"}, []interface{}{uid}, 2, filters)
	q := fmt.Sprintf(`SELECT COUNT(*) FROM workspace_tasks t WHERE %s`, strings.Join(where, " AND "))
	var count int
	if err := r.db.QueryRow(ctx, q, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// CountFeatureTasks returns the total number of tasks in a feature matching the filters.
func (r *Reader) CountFeatureTasks(ctx context.Context, workspaceID, featureID string, filters TaskSearchFilters) (int, error) {
	uid, err := parseUUID(workspaceID)
	if err != nil {
		return 0, err
	}
	fid, err := parseUUID(strings.TrimSpace(featureID))
	if err != nil {
		return 0, ErrNotFound
	}
	where, args, _ := buildTaskWhere([]string{"t.workspace_id = $1", "t.feature_id = $2"}, []interface{}{uid, fid}, 3, filters)
	q := fmt.Sprintf(`SELECT COUNT(*) FROM workspace_tasks t WHERE %s`, strings.Join(where, " AND "))
	var count int
	if err := r.db.QueryRow(ctx, q, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func taskIDOrderAsc(alias string) string {
	return fmt.Sprintf("%s ASC NULLS LAST, %s.task_name ASC", taskIDNumberExpr(alias), alias)
}

func taskIDOrderDesc(alias string) string {
	return fmt.Sprintf("%s DESC NULLS LAST, %s.task_name DESC", taskIDNumberExpr(alias), alias)
}

func taskIDNumberExpr(alias string) string {
	col := alias + ".task_name"
	return fmt.Sprintf("NULLIF(regexp_replace(%s, '^T([0-9]+)$', '\\1'), %s)::int", col, col)
}

func activityFilterClause(featureID, taskID string, firstArg int) (string, []interface{}, int, error) {
	var where []string
	var args []interface{}
	argPos := firstArg
	if featureID != "" {
		fid, err := parseUUID(featureID)
		if err != nil {
			return "", nil, argPos, err
		}
		where = append(where, fmt.Sprintf("feature_id = $%d", argPos))
		args = append(args, fid)
		argPos++
	}
	if taskID != "" {
		tid, err := parseUUID(taskID)
		if err != nil {
			return "", nil, argPos, err
		}
		where = append(where, fmt.Sprintf("task_id = $%d", argPos))
		args = append(args, tid)
		argPos++
	}
	if len(where) == 0 {
		return "", nil, argPos, nil
	}
	return " AND " + strings.Join(where, " AND "), args, argPos, nil
}

func (r *Reader) searchTasks(ctx context.Context, baseWhere []string, baseArgs []interface{}, argPos int, filters TaskSearchFilters) ([]WorkspaceTask, error) {
	where, args, argPos := buildTaskWhere(baseWhere, baseArgs, argPos, filters)

	orderBy := taskIDOrderAsc("t")
	switch filters.Sort {
	case "task_id_desc":
		orderBy = taskIDOrderDesc("t")
	case "title_asc":
		orderBy = "t.title ASC, " + taskIDOrderAsc("t")
	case "title_desc":
		orderBy = "t.title DESC, " + taskIDOrderAsc("t")
	case "status_asc":
		orderBy = "t.status ASC NULLS LAST, " + taskIDOrderAsc("t")
	case "status_desc":
		orderBy = "t.status DESC NULLS LAST, " + taskIDOrderAsc("t")
	case "repo_asc":
		orderBy = "t.repo ASC NULLS LAST, " + taskIDOrderAsc("t")
	case "repo_desc":
		orderBy = "t.repo DESC NULLS LAST, " + taskIDOrderAsc("t")
	case "updated_at_asc", "time_asc", "createdAt":
		orderBy = "t.updated_at ASC, " + taskIDOrderAsc("t")
	case "updated_at_desc", "time_desc", "-createdAt":
		orderBy = "t.updated_at DESC, " + taskIDOrderAsc("t")
	case "task_id_asc", "":
		orderBy = taskIDOrderAsc("t")
	}

	limitClause := ""
	if filters.Limit > 0 {
		limitClause = fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filters.Limit)
		argPos++
	}
	offsetClause := ""
	if filters.Page > 1 && filters.Limit > 0 {
		offsetClause = fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, (filters.Page-1)*filters.Limit)
	}

	q := fmt.Sprintf(`
		SELECT t.id, t.workspace_id, t.feature_id, t.feature_name, t.task_id, t.task_name, t.title,
		       t.repo, t.status, t.depends_on, t.blocked_reason, t.branch, t.execution,
		       t.pr, t.workspace_pr, t.source_path, t.source_hash, t.created_at, t.updated_at
		FROM workspace_tasks t
		WHERE %s
		ORDER BY %s%s%s`, strings.Join(where, " AND "), orderBy, limitClause, offsetClause)
	rows, err := r.db.Query(ctx, q, args...)
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

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
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
		&f.ID, &f.WorkspaceID, &f.FeatureID, &f.FeatureName, &f.Title, &f.FeatureStatus, &f.CurrentStage,
		&f.NextAction, &f.Stages, &f.SourcePath, &f.SourceHash, &f.CreatedAt, &f.UpdatedAt,
	)
}

func scanTask(row rowScanner, t *WorkspaceTask) error {
	return row.Scan(
		&t.ID, &t.WorkspaceID, &t.FeatureID, &t.FeatureName, &t.TaskID, &t.TaskName, &t.Title, &t.Repo, &t.Status,
		&t.DependsOn, &t.BlockedReason, &t.Branch, &t.Execution, &t.Pr, &t.WorkspacePr,
		&t.SourcePath, &t.SourceHash, &t.CreatedAt, &t.UpdatedAt,
	)
}
