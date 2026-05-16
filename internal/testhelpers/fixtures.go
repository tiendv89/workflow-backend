// Package testhelpers provides shared test fixtures and fake implementations
// for the workspace-data-backend integration test suite.
package testhelpers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tiendv89/workflow-backend/internal/adapter"
	"github.com/tiendv89/workflow-backend/internal/database"
)

// FixedTime is a deterministic timestamp for test fixtures.
var FixedTime = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

// NewWorkspace creates a fake Workspace row with the given ID and name.
func NewWorkspace(id, name, slug string) database.Workspace {
	var ws database.Workspace
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
func NewGitHubSource(workspaceID, repoURL string) database.WorkspaceGitHubSource {
	var src database.WorkspaceGitHubSource
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
func NewSyncRun(workspaceID, trigger, mode, status string) database.WorkspaceSyncRun {
	var sr database.WorkspaceSyncRun
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
func NewFeature(workspaceID, featureID, title, status, stage string) database.WorkspaceFeature {
	var f database.WorkspaceFeature
	if err := f.ID.Scan("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"); err != nil {
		panic(err)
	}
	if err := f.WorkspaceID.Scan(workspaceID); err != nil {
		panic(err)
	}
	f.FeatureID = featureID
	f.Title = title
	f.FeatureStatus = &status
	f.CurrentStage = &stage
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
func NewDocument(workspaceID, featureID, docType, sourcePath, url string) database.WorkspaceFeatureDocument {
	var d database.WorkspaceFeatureDocument
	if err := d.ID.Scan("cccccccc-cccc-cccc-cccc-cccccccccccc"); err != nil {
		panic(err)
	}
	if err := d.WorkspaceID.Scan(workspaceID); err != nil {
		panic(err)
	}
	d.FeatureID = featureID
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
func NewTask(workspaceID, featureID, taskID, title, status string, dependsOn []string) database.WorkspaceTask {
	var t database.WorkspaceTask
	if err := t.ID.Scan("dddddddd-dddd-dddd-dddd-dddddddddddd"); err != nil {
		panic(err)
	}
	if err := t.WorkspaceID.Scan(workspaceID); err != nil {
		panic(err)
	}
	t.FeatureID = featureID
	t.TaskID = taskID
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
func NewActivityEvent(workspaceID, featureID, taskID, action, actor, note string, seq int32) database.WorkspaceActivityEvent {
	var e database.WorkspaceActivityEvent
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
	e.FeatureID = &featureID
	if taskID != "" {
		e.TaskID = &taskID
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
	Workspaces  []database.Workspace
	SyncRuns    []database.WorkspaceSyncRun
	Features    []database.WorkspaceFeature
	Documents   []database.WorkspaceFeatureDocument
	Tasks       []database.WorkspaceTask
	Activity    []database.WorkspaceActivityEvent
	GitHubSrcs  map[string]database.WorkspaceGitHubSource

	// Error injection hooks.
	ListWorkspacesErr error
	GetWorkspaceErr   error
	GetSyncRunErr     error
}

func (f *FakeDB) ListWorkspaces(_ context.Context) ([]database.Workspace, error) {
	if f.ListWorkspacesErr != nil {
		return nil, f.ListWorkspacesErr
	}
	return f.Workspaces, nil
}

func (f *FakeDB) GetWorkspace(_ context.Context, workspaceID string) (database.Workspace, error) {
	if f.GetWorkspaceErr != nil {
		return database.Workspace{}, f.GetWorkspaceErr
	}
	for _, w := range f.Workspaces {
		if database.UUIDString(w.ID) == workspaceID {
			return w, nil
		}
	}
	return database.Workspace{}, database.ErrNotFound
}

func (f *FakeDB) GetGitHubSource(_ context.Context, workspaceID string) (database.WorkspaceGitHubSource, error) {
	if f.GitHubSrcs != nil {
		if src, ok := f.GitHubSrcs[workspaceID]; ok {
			return src, nil
		}
	}
	return database.WorkspaceGitHubSource{}, database.ErrNotFound
}

func (f *FakeDB) ListGitHubSources(_ context.Context) ([]database.WorkspaceGitHubSource, error) {
	out := make([]database.WorkspaceGitHubSource, 0, len(f.GitHubSrcs))
	for _, src := range f.GitHubSrcs {
		out = append(out, src)
	}
	return out, nil
}

func (f *FakeDB) ListLatestSyncRunsPerWorkspace(_ context.Context) ([]database.WorkspaceSyncRun, error) {
	return f.SyncRuns, nil
}

func (f *FakeDB) GetLatestSyncRun(_ context.Context, _ string) (database.WorkspaceSyncRun, error) {
	if f.GetSyncRunErr != nil {
		return database.WorkspaceSyncRun{}, f.GetSyncRunErr
	}
	if len(f.SyncRuns) > 0 {
		return f.SyncRuns[0], nil
	}
	return database.WorkspaceSyncRun{}, database.ErrNotFound
}

func (f *FakeDB) ListWorkspaceFeatures(_ context.Context, _ string) ([]database.WorkspaceFeature, error) {
	return f.Features, nil
}

func (f *FakeDB) GetWorkspaceFeature(_ context.Context, _, featureID string) (database.WorkspaceFeature, error) {
	for _, feat := range f.Features {
		if feat.FeatureID == featureID {
			return feat, nil
		}
	}
	return database.WorkspaceFeature{}, database.ErrNotFound
}

func (f *FakeDB) ListFeatureDocuments(_ context.Context, _, _ string) ([]database.WorkspaceFeatureDocument, error) {
	return f.Documents, nil
}

func (f *FakeDB) ListFeatureTasks(_ context.Context, _, _ string) ([]database.WorkspaceTask, error) {
	return f.Tasks, nil
}

func (f *FakeDB) ListWorkspaceTasks(_ context.Context, _ string) ([]database.WorkspaceTask, error) {
	return f.Tasks, nil
}

func (f *FakeDB) GetWorkspaceTask(_ context.Context, _, featureID, taskID string) (database.WorkspaceTask, error) {
	for _, t := range f.Tasks {
		if t.FeatureID == featureID && t.TaskID == taskID {
			return t, nil
		}
	}
	return database.WorkspaceTask{}, database.ErrNotFound
}

func (f *FakeDB) ListActivityEvents(_ context.Context, _, _, _ string) ([]database.WorkspaceActivityEvent, error) {
	return f.Activity, nil
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
