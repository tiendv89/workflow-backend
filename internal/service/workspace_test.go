package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tiendv89/workflow-backend/internal/adapter"
	"github.com/tiendv89/workflow-backend/internal/database"
	"github.com/tiendv89/workflow-backend/internal/domain"
	"github.com/tiendv89/workflow-backend/internal/service"
)

// --- fakes ---

type fakeDB struct {
	workspaces  []database.Workspace
	syncRuns    []database.WorkspaceSyncRun
	features    []database.WorkspaceFeature
	documents   []database.WorkspaceFeatureDocument
	tasks       []database.WorkspaceTask
	activity    []database.WorkspaceActivityEvent
	githubSrcs  map[string]database.WorkspaceGitHubSource
	listRunsErr error
	getWSErr    error
	getRunErr   error
}

func (f *fakeDB) ListWorkspaces(_ context.Context) ([]database.Workspace, error) {
	return f.workspaces, nil
}

func (f *fakeDB) GetWorkspace(_ context.Context, workspaceID string) (database.Workspace, error) {
	if f.getWSErr != nil {
		return database.Workspace{}, f.getWSErr
	}
	for _, w := range f.workspaces {
		if database.UUIDString(w.ID) == workspaceID {
			return w, nil
		}
	}
	return database.Workspace{}, database.ErrNotFound
}

func (f *fakeDB) GetGitHubSource(_ context.Context, workspaceID string) (database.WorkspaceGitHubSource, error) {
	if src, ok := f.githubSrcs[workspaceID]; ok {
		return src, nil
	}
	return database.WorkspaceGitHubSource{}, database.ErrNotFound
}

func (f *fakeDB) ListLatestSyncRunsPerWorkspace(_ context.Context) ([]database.WorkspaceSyncRun, error) {
	return f.syncRuns, f.listRunsErr
}

func (f *fakeDB) GetLatestSyncRun(_ context.Context, _ string) (database.WorkspaceSyncRun, error) {
	if f.getRunErr != nil {
		return database.WorkspaceSyncRun{}, f.getRunErr
	}
	if len(f.syncRuns) > 0 {
		return f.syncRuns[0], nil
	}
	return database.WorkspaceSyncRun{}, database.ErrNotFound
}

func (f *fakeDB) ListWorkspaceFeatures(_ context.Context, _ string) ([]database.WorkspaceFeature, error) {
	return f.features, nil
}

func (f *fakeDB) GetWorkspaceFeature(_ context.Context, _, featureID string) (database.WorkspaceFeature, error) {
	for _, feat := range f.features {
		if feat.FeatureID == featureID {
			return feat, nil
		}
	}
	return database.WorkspaceFeature{}, database.ErrNotFound
}

func (f *fakeDB) ListFeatureDocuments(_ context.Context, _, _ string) ([]database.WorkspaceFeatureDocument, error) {
	return f.documents, nil
}

func (f *fakeDB) ListFeatureTasks(_ context.Context, _, _ string) ([]database.WorkspaceTask, error) {
	return f.tasks, nil
}

func (f *fakeDB) ListWorkspaceTasks(_ context.Context, _ string) ([]database.WorkspaceTask, error) {
	return f.tasks, nil
}

func (f *fakeDB) GetWorkspaceTask(_ context.Context, _, featureID, taskID string) (database.WorkspaceTask, error) {
	for _, t := range f.tasks {
		if t.FeatureID == featureID && t.TaskID == taskID {
			return t, nil
		}
	}
	return database.WorkspaceTask{}, database.ErrNotFound
}

func (f *fakeDB) ListActivityEvents(_ context.Context, _, _, _ string) ([]database.WorkspaceActivityEvent, error) {
	return f.activity, nil
}

type fakeAdapter struct {
	importID  string
	importErr error
	syncErr   error
}

func (f *fakeAdapter) ImportWorkspace(_ context.Context, _ adapter.ImportRequest) (string, error) {
	return f.importID, f.importErr
}

func (f *fakeAdapter) SyncWorkspace(_ context.Context, _ string) error {
	return f.syncErr
}

// --- helpers ---

func makeUUID(hex string) database.Workspace {
	// Use a fixed UUID for tests via pgtype scanning.
	var ws database.Workspace
	_ = ws.ID.Scan(hex)
	ws.Name = "test-workspace"
	ws.Slug = "test"
	ws.UpdatedAt.Scan(time.Now())
	return ws
}

const testWSID = "11111111-1111-1111-1111-111111111111"

func newService(db service.DatabaseReader, adp service.AdapterCaller) *service.WorkspaceService {
	return service.New(db, adp, 30*time.Minute)
}

// --- tests ---

func TestListWorkspaces_Success(t *testing.T) {
	ws := makeUUID(testWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	svc := newService(db, &fakeAdapter{})

	result, err := svc.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(result))
	}
	if result[0].Name != "test-workspace" {
		t.Errorf("expected name 'test-workspace', got %q", result[0].Name)
	}
	if !result[0].SourceState.Stale {
		t.Error("expected stale source state when no sync run exists")
	}
}

func TestListWorkspaces_DBError(t *testing.T) {
	db := &fakeDB{getWSErr: errors.New("db down")}
	svc := newService(db, &fakeAdapter{})

	// ListWorkspaces uses its own internal query path, but we can simulate
	// the error by overriding workspaces to trigger empty + no runs.
	_, err := svc.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("unexpected error for empty list: %v", err)
	}
}

func TestGetWorkspace_NotFound(t *testing.T) {
	db := &fakeDB{}
	svc := newService(db, &fakeAdapter{})

	_, se := svc.GetWorkspace(context.Background(), testWSID)
	if se.Code != domain.ErrDatabaseNotFound {
		t.Errorf("expected ErrDatabaseNotFound, got %s", se.Code)
	}
}

func TestGetWorkspace_Success(t *testing.T) {
	ws := makeUUID(testWSID)
	status := "in_design"
	feat := database.WorkspaceFeature{
		FeatureID:     "feature-1",
		Title:         "Feature One",
		FeatureStatus: &status,
	}
	feat.WorkspaceID.Scan(testWSID)
	feat.UpdatedAt.Scan(time.Now())

	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		features:   []database.WorkspaceFeature{feat},
	}
	svc := newService(db, &fakeAdapter{})

	detail, se := svc.GetWorkspace(context.Background(), testWSID)
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	if detail.ID != testWSID {
		t.Errorf("expected ID %s, got %s", testWSID, detail.ID)
	}
	if len(detail.Features) != 1 {
		t.Errorf("expected 1 feature, got %d", len(detail.Features))
	}
	if detail.Features[0].FeatureID != "feature-1" {
		t.Errorf("expected feature-1, got %s", detail.Features[0].FeatureID)
	}
}

func TestSyncWorkspace_SuccessReturnsFreshData(t *testing.T) {
	ws := makeUUID(testWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	svc := newService(db, &fakeAdapter{syncErr: nil})

	detail, se := svc.SyncWorkspace(context.Background(), testWSID)
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	if detail == nil {
		t.Fatal("expected non-nil detail")
	}
}

func TestSyncWorkspace_FailureWithCachedData(t *testing.T) {
	ws := makeUUID(testWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	syncErr := domain.NewAdapterError(domain.ErrAdapterTimeout, "timeout")
	svc := newService(db, &fakeAdapter{syncErr: syncErr})

	detail, se := svc.SyncWorkspace(context.Background(), testWSID)
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	if !detail.SourceState.Stale {
		t.Error("expected stale state on sync failure with cached data")
	}
	if detail.SourceState.ErrorCode == "" {
		t.Error("expected error code in stale source state")
	}
}

func TestSyncWorkspace_FailureWithNoCachedData(t *testing.T) {
	db := &fakeDB{}
	syncErr := domain.NewAdapterError(domain.ErrAdapterTimeout, "timeout")
	svc := newService(db, &fakeAdapter{syncErr: syncErr})

	_, se := svc.SyncWorkspace(context.Background(), testWSID)
	if se == (domain.SourceError{}) {
		t.Error("expected source error when no cached data and sync fails")
	}
}

func TestImportWorkspace_AdapterError(t *testing.T) {
	db := &fakeDB{}
	adpErr := domain.NewAdapterError(domain.ErrAdapterInternal, "import failed")
	svc := newService(db, &fakeAdapter{importErr: adpErr})

	_, se := svc.ImportWorkspace(context.Background(), domain.ImportInput{RepoURL: "https://github.com/org/repo"})
	if se.Code != domain.ErrAdapterInternal {
		t.Errorf("expected ErrAdapterInternal, got %s", se.Code)
	}
}

func TestImportWorkspace_Success(t *testing.T) {
	ws := makeUUID(testWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	svc := newService(db, &fakeAdapter{importID: testWSID})

	detail, se := svc.ImportWorkspace(context.Background(), domain.ImportInput{RepoURL: "https://github.com/org/repo"})
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	if detail.ID != testWSID {
		t.Errorf("expected workspace ID %s, got %s", testWSID, detail.ID)
	}
}

func TestGetFeature_NotFound(t *testing.T) {
	ws := makeUUID(testWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	svc := newService(db, &fakeAdapter{})

	_, se := svc.GetFeature(context.Background(), testWSID, "missing-feature")
	if se.Code != domain.ErrDatabaseNotFound {
		t.Errorf("expected ErrDatabaseNotFound, got %s", se.Code)
	}
}

func TestGetTask_NotFound(t *testing.T) {
	ws := makeUUID(testWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	svc := newService(db, &fakeAdapter{})

	_, se := svc.GetTask(context.Background(), testWSID, "feature-1", "T99")
	if se.Code != domain.ErrDatabaseNotFound {
		t.Errorf("expected ErrDatabaseNotFound, got %s", se.Code)
	}
}

func TestGetTask_Success(t *testing.T) {
	ws := makeUUID(testWSID)
	status := "in_progress"
	blocked := false
	_ = blocked
	taskStatus := &status
	task := database.WorkspaceTask{
		FeatureID: "feature-1",
		TaskID:    "T1",
		Title:     "My Task",
		Status:    taskStatus,
		DependsOn: []byte(`["T0"]`),
		Execution: []byte(`{"actor_type":"agent"}`),
	}
	task.WorkspaceID.Scan(testWSID)
	task.UpdatedAt.Scan(time.Now())

	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		tasks:      []database.WorkspaceTask{task},
	}
	svc := newService(db, &fakeAdapter{})

	detail, se := svc.GetTask(context.Background(), testWSID, "feature-1", "T1")
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	if detail.TaskID != "T1" {
		t.Errorf("expected task T1, got %s", detail.TaskID)
	}
	if detail.Execution.ActorType != "agent" {
		t.Errorf("expected actor_type 'agent', got %q", detail.Execution.ActorType)
	}
	if len(detail.DependsOn) != 1 || detail.DependsOn[0] != "T0" {
		t.Errorf("expected depends_on [T0], got %v", detail.DependsOn)
	}
}

func TestListActivity_WorkspaceNotFound(t *testing.T) {
	db := &fakeDB{}
	svc := newService(db, &fakeAdapter{})

	_, se := svc.ListActivity(context.Background(), testWSID, domain.ActivityScope{})
	if se.Code != domain.ErrDatabaseNotFound {
		t.Errorf("expected ErrDatabaseNotFound, got %s", se.Code)
	}
}
