// Package testhelpers provides shared test fixtures and fake implementations
// for the workspace-data-backend integration test suite.
package testhelpers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tiendv89/workflow-backend/internal/adapter"
	database2 "github.com/tiendv89/workflow-backend/internal/database"
)

// FixedTime is a deterministic timestamp for test fixtures.
var FixedTime = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

// NewWorkspace creates a fake Workspace row with the given ID and name.
func NewWorkspace(id, name, slug string) database2.Workspace {
	var ws database2.Workspace
	if err := ws.ID.Scan(id); err != nil {
		panic(fmt.Sprintf("testhelpers.NewWorkspace: invalid UUID %q: %v", id, err))
	}
	ws.Name = name
	ws.Slug = slug
	ws.ManagementRepoID = "management-repo"
	if err := ws.CreatedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	if err := ws.UpdatedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	return ws
}

// NewGitHubSource creates a fake WorkspaceGitHubSource for the given workspace.
func NewGitHubSource(workspaceID, repoURL string) database2.WorkspaceGitHubSource {
	var src database2.WorkspaceGitHubSource
	if err := src.WorkspaceID.Scan(workspaceID); err != nil {
		panic(err)
	}
	src.RepoURL = repoURL
	src.RepoOwner = "testorg"
	src.RepoName = "test-repo"
	defaultBranch := "main"
	src.DefaultBranch = &defaultBranch
	if err := src.CreatedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	if err := src.UpdatedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	return src
}

// NewSyncRun creates a fake WorkspaceSyncRun row.
func NewSyncRun(workspaceID, trigger, mode, status string) database2.WorkspaceSyncRun {
	var sr database2.WorkspaceSyncRun
	if err := sr.ID.Scan("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"); err != nil {
		panic(err)
	}
	if err := sr.WorkspaceID.Scan(workspaceID); err != nil {
		panic(err)
	}
	sr.Trigger = trigger
	sr.Mode = mode
	sr.Status = status
	if err := sr.StartedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	finishedAt := FixedTime.Add(2 * time.Second)
	if err := sr.FinishedAt.Scan(finishedAt); err != nil {
		panic(err)
	}
	return sr
}

// NewFeature creates a fake WorkspaceFeature row.
func NewFeature(workspaceID, featureID, title, status, stage string) database2.WorkspaceFeature {
	var f database2.WorkspaceFeature
	if err := f.ID.Scan("10101010-1010-1010-1010-101010101010"); err != nil {
		panic(err)
	}
	if err := f.WorkspaceID.Scan(workspaceID); err != nil {
		panic(err)
	}
	if err := f.FeatureID.Scan("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"); err != nil {
		panic(err)
	}
	f.FeatureName = featureID
	f.Title = title
	f.FeatureStatus = &status
	f.CurrentStage = &stage
	stagesJSON, _ := json.Marshal([]map[string]string{{"id": stage, "status": status}})
	f.Stages = stagesJSON
	f.SourcePath = fmt.Sprintf("docs/features/%s/status.yaml", featureID)
	if err := f.CreatedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	if err := f.UpdatedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	return f
}

// NewDocument creates a fake WorkspaceFeatureDocument row.
func NewDocument(workspaceID, featureID, docType, sourcePath, url string) database2.WorkspaceFeatureDocument {
	var d database2.WorkspaceFeatureDocument
	if err := d.ID.Scan("cccccccc-cccc-cccc-cccc-cccccccccccc"); err != nil {
		panic(err)
	}
	if err := d.WorkspaceID.Scan(workspaceID); err != nil {
		panic(err)
	}
	if err := d.FeatureID.Scan("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"); err != nil {
		panic(err)
	}
	d.FeatureName = featureID
	d.DocumentType = docType
	d.SourcePath = sourcePath
	d.URL = &url
	if err := d.CreatedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	if err := d.UpdatedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	return d
}

// NewTask creates a fake WorkspaceTask row.
func NewTask(workspaceID, featureID, taskID, title, status string, dependsOn []string) database2.WorkspaceTask {
	var t database2.WorkspaceTask
	if err := t.ID.Scan("20202020-2020-2020-2020-202020202020"); err != nil {
		panic(err)
	}
	if err := t.WorkspaceID.Scan(workspaceID); err != nil {
		panic(err)
	}
	if err := t.FeatureID.Scan("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"); err != nil {
		panic(err)
	}
	t.FeatureName = featureID
	if err := t.TaskID.Scan("dddddddd-dddd-dddd-dddd-dddddddddddd"); err != nil {
		panic(err)
	}
	t.TaskName = taskID
	t.Title = title
	t.Status = &status

	repo := "workflow-backend"
	t.Repo = &repo
	branch := fmt.Sprintf("feature/%s-%s", featureID, taskID)
	t.Branch = &branch

	depJSON, _ := json.Marshal(dependsOn)
	t.DependsOn = depJSON

	execJSON, _ := json.Marshal(map[string]string{
		"actor_type":      "agent",
		"last_updated_by": "test@example.com",
	})
	t.Execution = execJSON

	t.SourcePath = fmt.Sprintf("docs/features/%s/tasks/%s.yaml", featureID, taskID)
	if err := t.CreatedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	if err := t.UpdatedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	return t
}

// NewActivityEvent creates a fake WorkspaceActivityEvent row.
func NewActivityEvent(workspaceID, featureID, taskID, action, actor, note string, seq int32) database2.WorkspaceActivityEvent {
	var e database2.WorkspaceActivityEvent
	if err := e.ID.Scan("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"); err != nil {
		panic(err)
	}
	if err := e.WorkspaceID.Scan(workspaceID); err != nil {
		panic(err)
	}
	scopeType := "task"
	if taskID == "" {
		scopeType = "feature"
	}
	e.ScopeType = scopeType
	if err := e.FeatureID.Scan("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"); err != nil {
		panic(err)
	}
	if taskID != "" {
		if err := e.TaskID.Scan("dddddddd-dddd-dddd-dddd-dddddddddddd"); err != nil {
			panic(err)
		}
	}
	e.Action = &action
	e.Actor = &actor
	e.Note = &note
	at := FixedTime.Format(time.RFC3339)
	e.OccurredAt = &at
	e.Sequence = seq
	e.RawEvent = json.RawMessage(`{}`)
	if err := e.CreatedAt.Scan(FixedTime); err != nil {
		panic(err)
	}
	return e
}

// FakeDB is a configurable in-memory fake of the DatabaseReader interface.
type FakeDB struct {
	Workspaces []database2.Workspace
	SyncRuns   []database2.WorkspaceSyncRun
	Features   []database2.WorkspaceFeature
	Documents  []database2.WorkspaceFeatureDocument
	Tasks      []database2.WorkspaceTask
	Activity   []database2.WorkspaceActivityEvent
	GitHubSrcs map[string]database2.WorkspaceGitHubSource

	// Error injection hooks.
	ListWorkspacesErr error
	GetWorkspaceErr   error
	GetSyncRunErr     error
}

func (f *FakeDB) ListWorkspaces(_ context.Context) ([]database2.Workspace, error) {
	if f.ListWorkspacesErr != nil {
		return nil, f.ListWorkspacesErr
	}
	return f.Workspaces, nil
}

func (f *FakeDB) GetWorkspace(_ context.Context, workspaceID string) (database2.Workspace, error) {
	if f.GetWorkspaceErr != nil {
		return database2.Workspace{}, f.GetWorkspaceErr
	}
	for _, w := range f.Workspaces {
		if database2.UUIDString(w.ID) == workspaceID {
			return w, nil
		}
	}
	return database2.Workspace{}, database2.ErrNotFound
}

func (f *FakeDB) GetGitHubSource(_ context.Context, workspaceID string) (database2.WorkspaceGitHubSource, error) {
	if f.GitHubSrcs != nil {
		if src, ok := f.GitHubSrcs[workspaceID]; ok {
			return src, nil
		}
	}
	return database2.WorkspaceGitHubSource{}, database2.ErrNotFound
}

func (f *FakeDB) ListGitHubSources(_ context.Context) ([]database2.WorkspaceGitHubSource, error) {
	out := make([]database2.WorkspaceGitHubSource, 0, len(f.GitHubSrcs))
	for _, src := range f.GitHubSrcs {
		out = append(out, src)
	}
	return out, nil
}

func (f *FakeDB) ListLatestSyncRunsPerWorkspace(_ context.Context) ([]database2.WorkspaceSyncRun, error) {
	return f.SyncRuns, nil
}

func (f *FakeDB) GetLatestSyncRun(_ context.Context, _ string) (database2.WorkspaceSyncRun, error) {
	if f.GetSyncRunErr != nil {
		return database2.WorkspaceSyncRun{}, f.GetSyncRunErr
	}
	if len(f.SyncRuns) > 0 {
		return f.SyncRuns[0], nil
	}
	return database2.WorkspaceSyncRun{}, database2.ErrNotFound
}

func (f *FakeDB) ListWorkspaceFeatures(_ context.Context, _ string) ([]database2.WorkspaceFeature, error) {
	return f.Features, nil
}

func (f *FakeDB) SearchWorkspaceFeatures(_ context.Context, _ string, filters database2.FeatureSearchFilters) ([]database2.WorkspaceFeature, error) {
	out := make([]database2.WorkspaceFeature, 0, len(f.Features))
	for _, feature := range f.Features {
		if filters.Title != "" && !strings.Contains(strings.ToLower(feature.Title), strings.ToLower(filters.Title)) {
			continue
		}
		if filters.Status != "" && !statusMatches(feature.FeatureStatus, filters.Status) {
			continue
		}
		out = append(out, feature)
	}

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch filters.Sort {
		case "title_asc":
			return a.Title < b.Title
		case "title_desc":
			return a.Title > b.Title
		case "status_asc":
			return derefString(a.FeatureStatus) < derefString(b.FeatureStatus)
		case "status_desc":
			return derefString(a.FeatureStatus) > derefString(b.FeatureStatus)
		case "updated_at_asc", "time_asc", "createdAt":
			return a.UpdatedAt.Time.Before(b.UpdatedAt.Time)
		case "updated_at_desc", "time_desc", "-createdAt", "":
			fallthrough
		default:
			return a.UpdatedAt.Time.After(b.UpdatedAt.Time)
		}
	})

	return paginateFeatures(out, filters.Page, filters.Limit), nil
}

func (f *FakeDB) ListFeatureTaskCounts(_ context.Context, _ string, featureIDs []string) ([]database2.WorkspaceFeatureTaskCounts, error) {
	counts := make(map[string]database2.WorkspaceFeatureTaskCounts, len(featureIDs))
	for _, featureID := range featureIDs {
		var row database2.WorkspaceFeatureTaskCounts
		if err := row.FeatureID.Scan(featureID); err != nil {
			return nil, err
		}
		counts[featureID] = row
	}

	for _, task := range f.Tasks {
		featureID := database2.UUIDString(task.FeatureID)
		row, ok := counts[featureID]
		if !ok {
			continue
		}
		row.Total++
		if task.Status != nil {
			switch *task.Status {
			case "done":
				row.Done++
			case "in_progress":
				row.InProgress++
			case "blocked":
				row.Blocked++
			case "ready":
				row.Ready++
			case "todo":
				row.Todo++
			}
		}
		counts[featureID] = row
	}

	out := make([]database2.WorkspaceFeatureTaskCounts, 0, len(featureIDs))
	for _, featureID := range featureIDs {
		out = append(out, counts[featureID])
	}
	return out, nil
}

func (f *FakeDB) GetWorkspaceFeature(_ context.Context, _, featureID string) (database2.WorkspaceFeature, error) {
	for _, feat := range f.Features {
		if database2.UUIDString(feat.FeatureID) == featureID {
			return feat, nil
		}
	}
	return database2.WorkspaceFeature{}, database2.ErrNotFound
}

func (f *FakeDB) ListFeatureDocuments(_ context.Context, _, _ string) ([]database2.WorkspaceFeatureDocument, error) {
	return f.Documents, nil
}

func (f *FakeDB) ListFeatureTasks(_ context.Context, _, featureID string) ([]database2.WorkspaceTask, error) {
	out := make([]database2.WorkspaceTask, 0, len(f.Tasks))
	for _, task := range f.Tasks {
		if database2.UUIDString(task.FeatureID) == featureID {
			out = append(out, task)
		}
	}
	return out, nil
}

func (f *FakeDB) SearchFeatureTasks(_ context.Context, _, featureID string, filters database2.TaskSearchFilters) ([]database2.WorkspaceTask, error) {
	out := make([]database2.WorkspaceTask, 0, len(f.Tasks))
	for _, task := range f.Tasks {
		if database2.UUIDString(task.FeatureID) != featureID {
			continue
		}
		if filters.TaskID != "" && !strings.Contains(strings.ToLower(task.TaskName), strings.ToLower(filters.TaskID)) {
			continue
		}
		if filters.Title != "" && !strings.Contains(strings.ToLower(task.Title), strings.ToLower(filters.Title)) {
			continue
		}
		if filters.Status != "" && !statusMatches(task.Status, filters.Status) {
			continue
		}
		if filters.Repo != "" && (task.Repo == nil || *task.Repo != filters.Repo) {
			continue
		}
		out = append(out, task)
	}

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch filters.Sort {
		case "task_id_desc":
			return taskIDGreater(a.TaskName, b.TaskName)
		case "title_asc":
			return a.Title < b.Title
		case "title_desc":
			return a.Title > b.Title
		case "status_asc":
			return derefString(a.Status) < derefString(b.Status)
		case "status_desc":
			return derefString(a.Status) > derefString(b.Status)
		case "repo_asc":
			return derefString(a.Repo) < derefString(b.Repo)
		case "repo_desc":
			return derefString(a.Repo) > derefString(b.Repo)
		case "updated_at_asc", "time_asc", "createdAt":
			return a.UpdatedAt.Time.Before(b.UpdatedAt.Time)
		case "updated_at_desc", "time_desc", "-createdAt":
			return a.UpdatedAt.Time.After(b.UpdatedAt.Time)
		case "task_id_asc", "":
			fallthrough
		default:
			return taskIDLess(a.TaskName, b.TaskName)
		}
	})

	return paginateTasks(out, filters.Page, filters.Limit), nil
}

func (f *FakeDB) SearchWorkspaceTasks(_ context.Context, _ string, filters database2.TaskSearchFilters) ([]database2.WorkspaceTask, error) {
	out := make([]database2.WorkspaceTask, 0, len(f.Tasks))
	for _, task := range f.Tasks {
		if filters.TaskID != "" && !strings.Contains(strings.ToLower(task.TaskName), strings.ToLower(filters.TaskID)) {
			continue
		}
		if filters.Title != "" && !strings.Contains(strings.ToLower(task.Title), strings.ToLower(filters.Title)) {
			continue
		}
		if filters.Status != "" && !statusMatches(task.Status, filters.Status) {
			continue
		}
		if filters.Repo != "" && (task.Repo == nil || *task.Repo != filters.Repo) {
			continue
		}
		out = append(out, task)
	}

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch filters.Sort {
		case "task_id_desc":
			return taskIDGreater(a.TaskName, b.TaskName)
		case "title_asc":
			return a.Title < b.Title
		case "title_desc":
			return a.Title > b.Title
		case "status_asc":
			return derefString(a.Status) < derefString(b.Status)
		case "status_desc":
			return derefString(a.Status) > derefString(b.Status)
		case "repo_asc":
			return derefString(a.Repo) < derefString(b.Repo)
		case "repo_desc":
			return derefString(a.Repo) > derefString(b.Repo)
		case "updated_at_asc", "time_asc", "createdAt":
			return a.UpdatedAt.Time.Before(b.UpdatedAt.Time)
		case "updated_at_desc", "time_desc", "-createdAt":
			return a.UpdatedAt.Time.After(b.UpdatedAt.Time)
		case "task_id_asc", "":
			fallthrough
		default:
			return taskIDLess(a.TaskName, b.TaskName)
		}
	})

	return paginateTasks(out, filters.Page, filters.Limit), nil
}

func (f *FakeDB) ListWorkspaceTasks(_ context.Context, _ string) ([]database2.WorkspaceTask, error) {
	return f.Tasks, nil
}

func paginateFeatures(features []database2.WorkspaceFeature, page, limit int) []database2.WorkspaceFeature {
	if limit <= 0 {
		return features
	}
	start := pageOffset(page, limit)
	if start >= len(features) {
		return []database2.WorkspaceFeature{}
	}
	end := start + limit
	if end > len(features) {
		end = len(features)
	}
	return features[start:end]
}

func paginateTasks(tasks []database2.WorkspaceTask, page, limit int) []database2.WorkspaceTask {
	if limit <= 0 {
		return tasks
	}
	start := pageOffset(page, limit)
	if start >= len(tasks) {
		return []database2.WorkspaceTask{}
	}
	end := start + limit
	if end > len(tasks) {
		end = len(tasks)
	}
	return tasks[start:end]
}

func pageOffset(page, limit int) int {
	if page < 1 {
		page = 1
	}
	return (page - 1) * limit
}

func statusMatches(status *string, filter string) bool {
	if status == nil {
		return false
	}
	for _, allowed := range strings.Split(filter, ",") {
		if strings.TrimSpace(allowed) == *status {
			return true
		}
	}
	return false
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func taskIDLess(a, b string) bool {
	an, aok := taskNumber(a)
	bn, bok := taskNumber(b)
	if aok && bok && an != bn {
		return an < bn
	}
	if aok != bok {
		return aok
	}
	return a < b
}

func taskIDGreater(a, b string) bool {
	an, aok := taskNumber(a)
	bn, bok := taskNumber(b)
	if aok && bok && an != bn {
		return an > bn
	}
	if aok != bok {
		return aok
	}
	return a > b
}

func taskNumber(taskID string) (int, bool) {
	if !strings.HasPrefix(taskID, "T") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(taskID, "T"))
	return n, err == nil
}

func (f *FakeDB) GetWorkspaceTask(_ context.Context, _, featureID, taskID string) (database2.WorkspaceTask, error) {
	for _, t := range f.Tasks {
		if database2.UUIDString(t.FeatureID) == featureID && database2.UUIDString(t.TaskID) == taskID {
			return t, nil
		}
	}
	return database2.WorkspaceTask{}, database2.ErrNotFound
}

func (f *FakeDB) GetWorkspaceTaskByID(_ context.Context, workspaceID, taskID string) (database2.WorkspaceTask, error) {
	for _, t := range f.Tasks {
		if database2.UUIDString(t.WorkspaceID) == workspaceID && database2.UUIDString(t.TaskID) == taskID {
			return t, nil
		}
	}
	return database2.WorkspaceTask{}, database2.ErrNotFound
}

func (f *FakeDB) ListActivityEvents(_ context.Context, _, featureID, taskID string) ([]database2.WorkspaceActivityEvent, error) {
	out := make([]database2.WorkspaceActivityEvent, 0, len(f.Activity))
	for _, event := range f.Activity {
		if featureID != "" && database2.UUIDString(event.FeatureID) != featureID {
			continue
		}
		if taskID != "" && database2.UUIDString(event.TaskID) != taskID {
			continue
		}
		out = append(out, event)
	}
	return out, nil
}

func (f *FakeDB) CountWorkspaceFeatures(_ context.Context, _ string, _ database2.FeatureSearchFilters) (int, error) {
	return len(f.Features), nil
}

func (f *FakeDB) CountFeatureTasks(_ context.Context, _, featureID string, _ database2.TaskSearchFilters) (int, error) {
	count := 0
	for _, t := range f.Tasks {
		if database2.UUIDString(t.FeatureID) == featureID {
			count++
		}
	}
	return count, nil
}

func (f *FakeDB) CountWorkspaceTasks(_ context.Context, _ string, _ database2.TaskSearchFilters) (int, error) {
	return len(f.Tasks), nil
}

// FakeAdapter is a configurable fake of the AdapterCaller interface.
type FakeAdapter struct {
	ImportedWorkspaceID string
	ImportErr           error
	SyncErr             error
}

func (f *FakeAdapter) ImportWorkspace(_ context.Context, _ adapter.ImportRequest) (string, error) {
	return f.ImportedWorkspaceID, f.ImportErr
}

func (f *FakeAdapter) SyncWorkspace(_ context.Context, _ string) error {
	return f.SyncErr
}
