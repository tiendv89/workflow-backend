package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tiendv89/workflow-backend/internal/adapter"
	"github.com/tiendv89/workflow-backend/internal/database"
	"github.com/tiendv89/workflow-backend/internal/domain"
	"github.com/tiendv89/workflow-backend/internal/handler"
	"github.com/tiendv89/workflow-backend/internal/service"
)

const handlerTestWSID = "22222222-2222-2222-2222-222222222222"
const handlerTestFeatureRowID = "33333333-3333-3333-3333-333333333333"
const handlerTestTaskRowID = "44444444-4444-4444-4444-444444444444"

// --- fakes (identical pattern to service tests) ---

type fakeDB struct {
	workspaces []database.Workspace
	syncRuns   []database.WorkspaceSyncRun
	features   []database.WorkspaceFeature
	documents  []database.WorkspaceFeatureDocument
	tasks      []database.WorkspaceTask
	activity   []database.WorkspaceActivityEvent
	githubSrcs map[string]database.WorkspaceGitHubSource
}

func (f *fakeDB) ListWorkspaces(_ context.Context) ([]database.Workspace, error) {
	return f.workspaces, nil
}
func (f *fakeDB) GetWorkspace(_ context.Context, workspaceID string) (database.Workspace, error) {
	for _, w := range f.workspaces {
		if database.UUIDString(w.ID) == workspaceID {
			return w, nil
		}
	}
	return database.Workspace{}, database.ErrNotFound
}
func (f *fakeDB) GetGitHubSource(_ context.Context, workspaceID string) (database.WorkspaceGitHubSource, error) {
	if f.githubSrcs != nil {
		if src, ok := f.githubSrcs[workspaceID]; ok {
			return src, nil
		}
	}
	return database.WorkspaceGitHubSource{}, database.ErrNotFound
}
func (f *fakeDB) ListGitHubSources(_ context.Context) ([]database.WorkspaceGitHubSource, error) {
	out := make([]database.WorkspaceGitHubSource, 0, len(f.githubSrcs))
	for _, src := range f.githubSrcs {
		out = append(out, src)
	}
	return out, nil
}
func (f *fakeDB) ListLatestSyncRunsPerWorkspace(_ context.Context) ([]database.WorkspaceSyncRun, error) {
	return f.syncRuns, nil
}
func (f *fakeDB) GetLatestSyncRun(_ context.Context, _ string) (database.WorkspaceSyncRun, error) {
	if len(f.syncRuns) > 0 {
		return f.syncRuns[0], nil
	}
	return database.WorkspaceSyncRun{}, database.ErrNotFound
}
func (f *fakeDB) ListWorkspaceFeatures(_ context.Context, _ string) ([]database.WorkspaceFeature, error) {
	return f.features, nil
}
func (f *fakeDB) SearchWorkspaceFeatures(_ context.Context, _ string, _ database.FeatureSearchFilters) ([]database.WorkspaceFeature, error) {
	return f.features, nil
}
func (f *fakeDB) GetWorkspaceFeature(_ context.Context, _, featureID string) (database.WorkspaceFeature, error) {
	for _, feat := range f.features {
		if database.UUIDString(feat.FeatureID) == featureID {
			return feat, nil
		}
	}
	return database.WorkspaceFeature{}, database.ErrNotFound
}
func (f *fakeDB) ListFeatureDocuments(_ context.Context, _, _ string) ([]database.WorkspaceFeatureDocument, error) {
	return f.documents, nil
}
func (f *fakeDB) ListFeatureTasks(_ context.Context, _, featureID string) ([]database.WorkspaceTask, error) {
	out := make([]database.WorkspaceTask, 0, len(f.tasks))
	for _, task := range f.tasks {
		if database.UUIDString(task.FeatureID) == featureID {
			out = append(out, task)
		}
	}
	return out, nil
}
func (f *fakeDB) SearchFeatureTasks(_ context.Context, _, featureID string, filters database.TaskSearchFilters) ([]database.WorkspaceTask, error) {
	out := make([]database.WorkspaceTask, 0, len(f.tasks))
	for _, task := range f.tasks {
		if database.UUIDString(task.FeatureID) != featureID {
			continue
		}
		if filters.Title != "" && task.Title != filters.Title {
			continue
		}
		if filters.Status != "" && (task.Status == nil || *task.Status != filters.Status) {
			continue
		}
		out = append(out, task)
	}
	if filters.Limit > 0 && filters.Limit < len(out) {
		out = out[:filters.Limit]
	}
	return out, nil
}
func (f *fakeDB) ListWorkspaceTasks(_ context.Context, _ string) ([]database.WorkspaceTask, error) {
	return f.tasks, nil
}

func (f *fakeDB) GetWorkspaceTask(_ context.Context, _, featureID, taskID string) (database.WorkspaceTask, error) {
	for _, t := range f.tasks {
		if database.UUIDString(t.FeatureID) == featureID && database.UUIDString(t.TaskID) == taskID {
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

// --- setup helpers ---

func makeTestWorkspace(id string) database.Workspace {
	var ws database.Workspace
	ws.ID.Scan(id)
	ws.Name = "Test Workspace"
	ws.Slug = "test-workspace"
	ws.UpdatedAt.Scan(time.Now())
	return ws
}

func makeSuccessfulSyncRun(workspaceID string) database.WorkspaceSyncRun {
	var run database.WorkspaceSyncRun
	run.ID.Scan("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	run.WorkspaceID.Scan(workspaceID)
	run.Trigger = "api_import"
	run.Mode = "full"
	run.Status = "success"
	run.StartedAt.Scan(time.Now().Add(-time.Second))
	run.FinishedAt.Scan(time.Now())
	return run
}

func newTestRouter(db service.DatabaseReader, adp service.AdapterCaller) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := service.New(db, adp, 30*time.Minute)
	h := handler.New(svc)
	r := gin.New()
	api := r.Group("/api")
	h.RegisterRoutes(api)
	return r
}

// --- tests ---

func TestListWorkspaces_200(t *testing.T) {
	ws := makeTestWorkspace(handlerTestWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []domain.WorkspaceSummary
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(result))
	}
}

func TestGetWorkspace_200(t *testing.T) {
	ws := makeTestWorkspace(handlerTestWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces/"+handlerTestWSID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var detail domain.WorkspaceDetail
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if detail.ID != handlerTestWSID {
		t.Errorf("expected workspace ID %s, got %s", handlerTestWSID, detail.ID)
	}
	if !detail.SourceState.Stale {
		t.Error("expected stale source state when no sync run exists")
	}
}

func TestGetWorkspace_404(t *testing.T) {
	db := &fakeDB{}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces/"+handlerTestWSID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	var apiErr domain.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if apiErr.Code != domain.ErrDatabaseNotFound {
		t.Errorf("expected ErrDatabaseNotFound, got %s", apiErr.Code)
	}
}

func TestImportWorkspace_200WithWorkspaceDetail(t *testing.T) {
	ws := makeTestWorkspace(handlerTestWSID)
	feat := database.WorkspaceFeature{
		FeatureName: "workspace-data-backend",
		Title:       "Workspace Data Backend",
	}
	feat.ID.Scan("77777777-7777-7777-7777-777777777777")
	feat.FeatureID.Scan(handlerTestFeatureRowID)
	feat.WorkspaceID.Scan(handlerTestWSID)
	feat.UpdatedAt.Scan(time.Now())
	src := database.WorkspaceGitHubSource{RepoURL: "https://github.com/org/repo"}
	src.WorkspaceID.Scan(handlerTestWSID)
	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		syncRuns:   []database.WorkspaceSyncRun{makeSuccessfulSyncRun(handlerTestWSID)},
		features:   []database.WorkspaceFeature{feat},
		githubSrcs: map[string]database.WorkspaceGitHubSource{
			handlerTestWSID: src,
		},
	}
	r := newTestRouter(db, &fakeAdapter{importID: handlerTestWSID})

	body := `{"repo_url":"https://github.com/org/repo"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/workspaces/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var detail domain.WorkspaceDetail
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if detail.ID != handlerTestWSID {
		t.Errorf("expected workspace ID %s, got %s", handlerTestWSID, detail.ID)
	}
	if detail.RepoURL != "https://github.com/org/repo" {
		t.Errorf("expected repo_url from persisted source, got %q", detail.RepoURL)
	}
	if detail.SourceState.Stale {
		t.Error("expected fresh source_state after completed import")
	}
	if len(detail.Features) != 1 {
		t.Errorf("expected persisted feature summary, got %d features", len(detail.Features))
	}
}

func TestImportWorkspace_400_MissingBody(t *testing.T) {
	db := &fakeDB{}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/workspaces/import", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing repo_url, got %d", w.Code)
	}
}

func TestImportWorkspace_AdapterError(t *testing.T) {
	db := &fakeDB{}
	adpErr := domain.NewAdapterError(domain.ErrAdapterInternal, "rpc failed")
	r := newTestRouter(db, &fakeAdapter{importErr: adpErr})

	body := `{"repo_url":"https://github.com/org/repo"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/workspaces/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for adapter error, got %d", w.Code)
	}
}

func TestSyncWorkspace_200_Success(t *testing.T) {
	ws := makeTestWorkspace(handlerTestWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/workspaces/"+handlerTestWSID+"/sync", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on sync success, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSyncWorkspace_200_StaleOnAdapterFailure(t *testing.T) {
	ws := makeTestWorkspace(handlerTestWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	syncErr := domain.NewAdapterError(domain.ErrAdapterTimeout, "timeout")
	r := newTestRouter(db, &fakeAdapter{syncErr: syncErr})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/workspaces/"+handlerTestWSID+"/sync", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with stale data, got %d: %s", w.Code, w.Body.String())
	}

	var detail domain.WorkspaceDetail
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !detail.SourceState.Stale {
		t.Error("expected stale=true in response")
	}
}

func TestGetFeature_200(t *testing.T) {
	ws := makeTestWorkspace(handlerTestWSID)
	status := "in_design"
	feat := database.WorkspaceFeature{
		FeatureName:   "feature-1",
		Title:         "My Feature",
		FeatureStatus: &status,
	}
	feat.ID.Scan("77777777-7777-7777-7777-777777777777")
	feat.FeatureID.Scan(handlerTestFeatureRowID)
	feat.WorkspaceID.Scan(handlerTestWSID)
	feat.UpdatedAt.Scan(time.Now())

	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		features:   []database.WorkspaceFeature{feat},
	}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces/"+handlerTestWSID+"/features/"+handlerTestFeatureRowID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetFeature_404(t *testing.T) {
	ws := makeTestWorkspace(handlerTestWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces/"+handlerTestWSID+"/features/missing", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestListFeatureTasks_200(t *testing.T) {
	ws := makeTestWorkspace(handlerTestWSID)
	status := "ready"
	feat := database.WorkspaceFeature{
		FeatureName: "feature-1",
		Title:       "Feature One",
	}
	feat.ID.Scan(handlerTestFeatureRowID)
	feat.FeatureID.Scan(handlerTestFeatureRowID)
	feat.WorkspaceID.Scan(handlerTestWSID)
	feat.UpdatedAt.Scan(time.Now())
	task := database.WorkspaceTask{
		FeatureName: "feature-1",
		TaskName:    "T1",
		Title:       "Task One",
		Status:      &status,
	}
	task.ID.Scan("88888888-8888-8888-8888-888888888888")
	task.TaskID.Scan(handlerTestTaskRowID)
	task.FeatureID.Scan(handlerTestFeatureRowID)
	task.WorkspaceID.Scan(handlerTestWSID)
	task.UpdatedAt.Scan(time.Now())

	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		features:   []database.WorkspaceFeature{feat},
		tasks:      []database.WorkspaceTask{task},
	}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces/"+handlerTestWSID+"/features/"+handlerTestFeatureRowID+"/tasks", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var tasks []domain.TaskSummary
	if err := json.Unmarshal(w.Body.Bytes(), &tasks); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

func TestGetTask_200(t *testing.T) {
	ws := makeTestWorkspace(handlerTestWSID)
	status := "in_progress"
	feat := database.WorkspaceFeature{
		FeatureName: "feature-1",
		Title:       "Feature One",
	}
	feat.ID.Scan("99999999-9999-9999-9999-999999999999")
	feat.FeatureID.Scan(handlerTestFeatureRowID)
	feat.WorkspaceID.Scan(handlerTestWSID)
	feat.UpdatedAt.Scan(time.Now())
	task := database.WorkspaceTask{
		FeatureName: "feature-1",
		TaskName:    "T1",
		Title:       "Task One",
		Status:      &status,
		DependsOn:   []byte(`[]`),
		Execution:   []byte(`{"actor_type":"agent"}`),
	}
	task.ID.Scan("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	task.TaskID.Scan(handlerTestTaskRowID)
	task.FeatureID.Scan(handlerTestFeatureRowID)
	task.WorkspaceID.Scan(handlerTestWSID)
	task.UpdatedAt.Scan(time.Now())

	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		features:   []database.WorkspaceFeature{feat},
		tasks:      []database.WorkspaceTask{task},
	}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces/"+handlerTestWSID+"/features/"+handlerTestFeatureRowID+"/tasks/"+handlerTestTaskRowID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetTask_404(t *testing.T) {
	ws := makeTestWorkspace(handlerTestWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces/"+handlerTestWSID+"/features/"+handlerTestFeatureRowID+"/tasks/T99", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestListActivity_200(t *testing.T) {
	ws := makeTestWorkspace(handlerTestWSID)
	db := &fakeDB{workspaces: []database.Workspace{ws}}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces/"+handlerTestWSID+"/activity", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListActivity_404_WorkspaceNotFound(t *testing.T) {
	db := &fakeDB{}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces/"+handlerTestWSID+"/activity", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestErrorResponseShape(t *testing.T) {
	db := &fakeDB{}
	r := newTestRouter(db, &fakeAdapter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces/"+handlerTestWSID, nil)
	r.ServeHTTP(w, req)

	var apiErr domain.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if apiErr.Code == "" {
		t.Error("expected non-empty error code")
	}
	if apiErr.Message == "" {
		t.Error("expected non-empty error message")
	}
	if apiErr.Source == "" {
		t.Error("expected non-empty error source")
	}
}
