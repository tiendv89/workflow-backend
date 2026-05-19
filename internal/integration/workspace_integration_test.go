// Package integration tests the full HTTP stack: routing → handler → service → DB interface.
// All tests use FakeDB and FakeAdapter from testhelpers; no Docker or PostgreSQL required.
package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tiendv89/workflow-backend/internal/database"
	"github.com/tiendv89/workflow-backend/internal/domain"
	"github.com/tiendv89/workflow-backend/internal/handler"
	"github.com/tiendv89/workflow-backend/internal/service"
	"github.com/tiendv89/workflow-backend/internal/testhelpers"
)

const (
	wsID         = "7967eeca-892d-4d94-8cc0-7a552c2cbe87"
	featureRowID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	taskRowID    = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	featureID    = "workspace-data-backend"
	taskID       = "T1"
	repoURL      = "https://github.com/testorg/test-repo"
)

func newServer(db service.DatabaseReader, adp service.AdapterCaller) *httptest.Server {
	gin.SetMode(gin.TestMode)
	svc := service.New(db, adp, 30*time.Minute)
	h := handler.New(svc)
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	api := r.Group("/api")
	h.RegisterRoutes(api)
	return httptest.NewServer(r)
}

func get(t *testing.T, srv *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func post(t *testing.T, srv *httptest.Server, path, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// --- T5 scenario 1: first import — persisted by adapter and returned as normalized detail ---

func TestImport_FirstImport_200WithWorkspaceDetail(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "My Workspace", "my-workspace")
	feature := testhelpers.NewFeature(wsID, featureID, "Workspace Data Backend", "in_progress", "implementation")
	task := testhelpers.NewTask(wsID, featureID, taskID, "Implement API", "ready", nil)
	src := testhelpers.NewGitHubSource(wsID, repoURL)
	successRun := testhelpers.NewSyncRun(wsID, "api_import", "full", "success")
	if err := successRun.FinishedAt.Scan(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		SyncRuns:   []database.WorkspaceSyncRun{successRun},
		Features:   []database.WorkspaceFeature{feature},
		Tasks:      []database.WorkspaceTask{task},
		GitHubSrcs: map[string]database.WorkspaceGitHubSource{wsID: src},
	}
	adp := &testhelpers.FakeAdapter{ImportedWorkspaceID: wsID}
	srv := newServer(db, adp)
	defer srv.Close()

	resp := post(t, srv, "/api/workspaces/import",
		`{"repo_url":"https://github.com/testorg/test-repo","name":"My Workspace"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var detail domain.WorkspaceDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.ID != wsID {
		t.Errorf("expected workspace ID %s, got %s", wsID, detail.ID)
	}
	if detail.RepoURL != repoURL {
		t.Errorf("expected repo_url %s, got %s", repoURL, detail.RepoURL)
	}
	if detail.SourceState.Stale {
		t.Error("expected fresh source_state after completed import")
	}
	if len(detail.Features) != 1 {
		t.Errorf("expected imported feature summary, got %d features", len(detail.Features))
	}
	if len(detail.Tasks) != 1 {
		t.Errorf("expected imported task summary, got %d tasks", len(detail.Tasks))
	}
}

func TestImport_MissingRepoURL_400(t *testing.T) {
	db := &testhelpers.FakeDB{}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := post(t, srv, "/api/workspaces/import", `{}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestImport_AdapterFailure_500WithErrorShape(t *testing.T) {
	db := &testhelpers.FakeDB{}
	adpErr := domain.NewAdapterError(domain.ErrAdapterInternal, "rpc unavailable")
	srv := newServer(db, &testhelpers.FakeAdapter{ImportErr: adpErr})
	defer srv.Close()

	resp := post(t, srv, "/api/workspaces/import", `{"repo_url":"https://github.com/org/repo"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for adapter error, got %d", resp.StatusCode)
	}
	var apiErr domain.APIError
	json.NewDecoder(resp.Body).Decode(&apiErr)
	if apiErr.Code != domain.ErrAdapterInternal {
		t.Errorf("expected ErrAdapterInternal, got %s", apiErr.Code)
	}
}

// --- T5 scenario 2: sync success — returns fresh (non-stale) data ---

func TestSync_Success_200_FreshSourceState(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	successRun := testhelpers.NewSyncRun(wsID, "manual", "full_reconciliation", "success")
	// Set finished_at to now so it falls within the stale threshold.
	if err := successRun.FinishedAt.Scan(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		SyncRuns:   []database.WorkspaceSyncRun{successRun},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := post(t, srv, "/api/workspaces/"+wsID+"/sync", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on sync success, got %d", resp.StatusCode)
	}
	var detail domain.WorkspaceDetail
	json.NewDecoder(resp.Body).Decode(&detail)
	if detail.SourceState.Stale {
		t.Error("expected stale=false after successful sync with recent run")
	}
}

// --- T5 scenario 3: sync failure with stale cache --- returns 200 + stale data ---

func TestSync_Failure_WithCache_Returns200_StaleData(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	db := &testhelpers.FakeDB{Workspaces: []database.Workspace{ws}}
	syncErr := domain.NewAdapterError(domain.ErrAdapterTimeout, "adapter timeout")
	srv := newServer(db, &testhelpers.FakeAdapter{SyncErr: syncErr})
	defer srv.Close()

	resp := post(t, srv, "/api/workspaces/"+wsID+"/sync", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with stale data, got %d", resp.StatusCode)
	}
	var detail domain.WorkspaceDetail
	json.NewDecoder(resp.Body).Decode(&detail)
	if !detail.SourceState.Stale {
		t.Error("expected stale=true after sync failure with cached data")
	}
	if detail.SourceState.ErrorCode == "" {
		t.Error("expected error_code in stale source state")
	}
}

// --- T5 scenario 4: sync failure without cache — returns structured source error ---

func TestSync_Failure_NoCache_ReturnsSourceError(t *testing.T) {
	db := &testhelpers.FakeDB{} // no workspace in DB
	syncErr := domain.NewAdapterError(domain.ErrAdapterTimeout, "adapter timeout")
	srv := newServer(db, &testhelpers.FakeAdapter{SyncErr: syncErr})
	defer srv.Close()

	resp := post(t, srv, "/api/workspaces/"+wsID+"/sync", "")
	resp.Body.Close()
	if resp.StatusCode < 400 {
		t.Errorf("expected error status for sync failure without cache, got %d", resp.StatusCode)
	}
}

// --- T5 scenario 5: workspace list route ---

func TestListWorkspaces_TwoWorkspaces_WithSourceState(t *testing.T) {
	ws1 := testhelpers.NewWorkspace(wsID, "Workspace One", "workspace-one")
	ws2 := testhelpers.NewWorkspace("22222222-2222-2222-2222-222222222222", "Workspace Two", "workspace-two")
	successRun := testhelpers.NewSyncRun(wsID, "import", "full_reconciliation", "success")
	src := testhelpers.NewGitHubSource(wsID, repoURL)

	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws1, ws2},
		SyncRuns:   []database.WorkspaceSyncRun{successRun},
		GitHubSrcs: map[string]database.WorkspaceGitHubSource{wsID: src},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var workspaces []domain.WorkspaceSummary
	json.NewDecoder(resp.Body).Decode(&workspaces)
	if len(workspaces) != 2 {
		t.Errorf("expected 2 workspaces, got %d", len(workspaces))
	}

	// Find workspace-one and verify source state.
	var ws1Result *domain.WorkspaceSummary
	for i := range workspaces {
		if workspaces[i].ID == wsID {
			ws1Result = &workspaces[i]
		}
	}
	if ws1Result == nil {
		t.Fatalf("workspace %s not found in response", wsID)
	}
	if ws1Result.RepoURL != repoURL {
		t.Errorf("expected repo_url %s, got %s", repoURL, ws1Result.RepoURL)
	}
}

// --- T5 scenario 5: workspace detail route ---

func TestGetWorkspace_Detail_IncludesFeatureAndTaskSummaries(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	feat := testhelpers.NewFeature(wsID, featureID, "Feature", "in_implementation", "in_implementation")
	task := testhelpers.NewTask(wsID, featureID, taskID, "Task One", "done", []string{})
	sr := testhelpers.NewSyncRun(wsID, "import", "full_reconciliation", "success")

	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Features:   []database.WorkspaceFeature{feat},
		Tasks:      []database.WorkspaceTask{task},
		SyncRuns:   []database.WorkspaceSyncRun{sr},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var detail domain.WorkspaceDetail
	json.NewDecoder(resp.Body).Decode(&detail)
	if detail.ID != wsID {
		t.Errorf("expected ID %s, got %s", wsID, detail.ID)
	}
	if len(detail.Features) != 1 || detail.Features[0].FeatureID != featureRowID || detail.Features[0].FeatureName != featureID {
		t.Errorf("expected feature id %s/name %s, got %v", featureRowID, featureID, detail.Features)
	}
	if len(detail.Features[0].Stages) == 0 {
		t.Error("expected feature stages to be included")
	}
	if len(detail.Tasks) != 1 || detail.Tasks[0].TaskID != taskRowID || detail.Tasks[0].TaskName != taskID {
		t.Errorf("expected task id %s/name %s, got %v", taskRowID, taskID, detail.Tasks)
	}
}

func TestGetWorkspace_NotFound_404(t *testing.T) {
	db := &testhelpers.FakeDB{}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	var apiErr domain.APIError
	json.NewDecoder(resp.Body).Decode(&apiErr)
	if apiErr.Code != domain.ErrDatabaseNotFound {
		t.Errorf("expected ErrDatabaseNotFound, got %q", apiErr.Code)
	}
	if apiErr.Source != domain.ErrorSourceDatabase {
		t.Errorf("expected source 'database', got %q", apiErr.Source)
	}
}

// --- T5 scenario 5: feature detail route ---

func TestGetFeature_Detail_IncludesDocumentsTasksActivity(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	feat := testhelpers.NewFeature(wsID, featureID, "Feature", "in_implementation", "in_implementation")
	doc := testhelpers.NewDocument(wsID, featureID, "product_spec",
		"docs/features/workspace-data-backend/product-spec.md",
		"https://github.com/testorg/test-repo/blob/main/docs/features/workspace-data-backend/product-spec.md")
	task := testhelpers.NewTask(wsID, featureID, taskID, "T One", "done", []string{})
	event := testhelpers.NewActivityEvent(wsID, featureID, taskID, "done", "agent@example.com", "done note", 0)

	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Features:   []database.WorkspaceFeature{feat},
		Documents:  []database.WorkspaceFeatureDocument{doc},
		Tasks:      []database.WorkspaceTask{task},
		Activity:   []database.WorkspaceActivityEvent{event},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/features/"+featureRowID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var detail domain.FeatureDetail
	json.NewDecoder(resp.Body).Decode(&detail)
	if detail.FeatureID != featureRowID {
		t.Errorf("expected feature_id %s, got %s", featureRowID, detail.FeatureID)
	}
	if detail.FeatureName != featureID {
		t.Errorf("expected feature_name %s, got %s", featureID, detail.FeatureName)
	}
	if detail.WorkspaceID != wsID {
		t.Errorf("expected workspace_id %s, got %s", wsID, detail.WorkspaceID)
	}
	if len(detail.Documents) != 1 || detail.Documents[0].DocumentType != "product_spec" {
		t.Errorf("expected 1 document with type 'product_spec', got %v", detail.Documents)
	}
	if detail.Documents[0].URL == "" {
		t.Error("expected non-empty document URL")
	}
	if len(detail.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(detail.Tasks))
	}
	if len(detail.Activity) != 1 || detail.Activity[0].Action != "done" {
		t.Errorf("expected 1 'done' activity event, got %v", detail.Activity)
	}
}

func TestGetFeature_NotFound_404(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	db := &testhelpers.FakeDB{Workspaces: []database.Workspace{ws}}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/features/nonexistent")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- T5 scenario 5: feature tasks route ---

func TestListFeatureTasks_ReturnsSummariesWithAllFields(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	feat := testhelpers.NewFeature(wsID, featureID, "Workspace Data Backend", "in_progress", "build")
	t1 := testhelpers.NewTask(wsID, featureID, "T1", "Foundation", "done", []string{})
	t2 := testhelpers.NewTask(wsID, featureID, "T2", "GitHub adapter", "in_progress", []string{"T1"})
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Features:   []database.WorkspaceFeature{feat},
		Tasks:      []database.WorkspaceTask{t1, t2},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/features/"+featureRowID+"/tasks")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var tasks []domain.TaskSummary
	json.NewDecoder(resp.Body).Decode(&tasks)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.TaskID == "" || task.FeatureID == "" || task.Title == "" || task.Status == "" {
			t.Errorf("task summary has empty required fields: %+v", task)
		}
	}
}

func TestSearchFeatures_FiltersSortsAndLimits(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	f1 := testhelpers.NewFeature(wsID, "auth", "Auth UI", "done", "ship")
	f2 := testhelpers.NewFeature(wsID, "adapter", "GitHub Adapter", "in_progress", "build")
	f3 := testhelpers.NewFeature(wsID, "adapter-cleanup", "Adapter Cleanup", "in_progress", "build")
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Features:   []database.WorkspaceFeature{f1, f2, f3},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/features?title=adapter&status=in_progress&sort=title_asc&limit=1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var features []domain.FeatureSummary
	json.NewDecoder(resp.Body).Decode(&features)
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}
	if features[0].Title != "Adapter Cleanup" {
		t.Errorf("expected first sorted feature to be Adapter Cleanup, got %q", features[0].Title)
	}
}

func TestSearchTasks_FiltersSortsAndLimits(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	feat := testhelpers.NewFeature(wsID, featureID, "Workspace Data Backend", "in_progress", "build")
	t1 := testhelpers.NewTask(wsID, featureID, "T1", "Foundation", "done", []string{})
	t2 := testhelpers.NewTask(wsID, featureID, "T2", "Adapter wiring", "in_progress", []string{"T1"})
	t3 := testhelpers.NewTask(wsID, featureID, "T3", "Adapter cleanup", "in_progress", []string{"T2"})
	t1.ID.Scan("dddddddd-dddd-dddd-dddd-dddddddddddd")
	t2.ID.Scan("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	t3.ID.Scan("ffffffff-ffff-ffff-ffff-ffffffffffff")
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Features:   []database.WorkspaceFeature{feat},
		Tasks:      []database.WorkspaceTask{t1, t2, t3},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/features/"+featureRowID+"/tasks?title=adapter&status=in_progress&sort=title_desc&limit=1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var tasks []domain.TaskSummary
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		t.Fatalf("decode tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after limit, got %d", len(tasks))
	}
	if tasks[0].TaskName != "T2" {
		t.Errorf("expected T2 first for title_desc, got %s", tasks[0].TaskName)
	}
}

func TestSearchWorkspaceTasks_FiltersSortsAndLimits(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	feat := testhelpers.NewFeature(wsID, featureID, "Workspace Data Backend", "in_progress", "build")
	t1 := testhelpers.NewTask(wsID, featureID, "T1", "Foundation", "done", []string{})
	t2 := testhelpers.NewTask(wsID, featureID, "T2", "Adapter wiring", "in_progress", []string{"T1"})
	t10 := testhelpers.NewTask(wsID, featureID, "T10", "Final adapter verification", "in_progress", []string{"T2"})
	t1.ID.Scan("dddddddd-dddd-dddd-dddd-dddddddddddd")
	t2.ID.Scan("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	t10.ID.Scan("ffffffff-ffff-ffff-ffff-ffffffffffff")
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Features:   []database.WorkspaceFeature{feat},
		Tasks:      []database.WorkspaceTask{t1, t10, t2},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/tasks?title=adapter&status=in_progress&sort=task_id_asc&limit=1&page=2")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var tasks []domain.TaskSummary
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		t.Fatalf("decode tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected second paged task, got %d", len(tasks))
	}
	if tasks[0].TaskName != "T10" {
		t.Errorf("expected T10 after numeric sort and page 2, got %s", tasks[0].TaskName)
	}
}

func TestSearchTasks_TaskIDSortUsesWorkflowNumericOrder(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	feat := testhelpers.NewFeature(wsID, featureID, "Workspace Data Backend", "in_progress", "build")
	t1 := testhelpers.NewTask(wsID, featureID, "T1", "Foundation", "ready", []string{})
	t2 := testhelpers.NewTask(wsID, featureID, "T2", "Adapter wiring", "ready", []string{"T1"})
	t10 := testhelpers.NewTask(wsID, featureID, "T10", "Final verification", "ready", []string{"T2"})
	t1.ID.Scan("dddddddd-dddd-dddd-dddd-dddddddddddd")
	t2.ID.Scan("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	t10.ID.Scan("ffffffff-ffff-ffff-ffff-ffffffffffff")
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Features:   []database.WorkspaceFeature{feat},
		Tasks:      []database.WorkspaceTask{t1, t10, t2},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/features/"+featureRowID+"/tasks?sort=task_id_asc")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var tasks []domain.TaskSummary
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		t.Fatalf("decode tasks: %v", err)
	}
	got := []string{tasks[0].TaskName, tasks[1].TaskName, tasks[2].TaskName}
	want := []string{"T1", "T2", "T10"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected task order %v, got %v", want, got)
		}
	}
}

func TestSearchTasks_InvalidLimit_400(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	feat := testhelpers.NewFeature(wsID, featureID, "Workspace Data Backend", "in_progress", "build")
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Features:   []database.WorkspaceFeature{feat},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/features/"+featureRowID+"/tasks?limit=abc")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSearchFeatures_InvalidLimit_400(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	db := &testhelpers.FakeDB{Workspaces: []database.Workspace{ws}}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/features?limit=abc")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// --- T5 scenario 5: task detail route ---

func TestGetTask_Detail_UsesUUIDFeatureAndTaskIDs(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	feat := testhelpers.NewFeature(wsID, featureID, "Workspace Data Backend", "in_progress", "build")
	task := testhelpers.NewTask(wsID, featureID, taskID, "Task One", "done", []string{"T0"})
	event := testhelpers.NewActivityEvent(wsID, featureID, taskID, "done", "human@example.com", "Approved", 0)
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Features:   []database.WorkspaceFeature{feat},
		Tasks:      []database.WorkspaceTask{task},
		Activity:   []database.WorkspaceActivityEvent{event},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/features/"+featureRowID+"/tasks/"+taskRowID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var detail domain.TaskDetail
	json.NewDecoder(resp.Body).Decode(&detail)
	if detail.TaskID != taskRowID {
		t.Errorf("expected task_id %s, got %s", taskRowID, detail.TaskID)
	}
	if detail.TaskName != taskID {
		t.Errorf("expected task_name %s, got %s", taskID, detail.TaskName)
	}
	if detail.WorkspaceID != wsID {
		t.Errorf("expected workspace_id %s, got %s", wsID, detail.WorkspaceID)
	}
	if len(detail.DependsOn) != 1 || detail.DependsOn[0] != "T0" {
		t.Errorf("expected depends_on [T0], got %v", detail.DependsOn)
	}
	if detail.Execution.ActorType != "agent" {
		t.Errorf("expected actor_type 'agent', got %q", detail.Execution.ActorType)
	}
	if len(detail.Activity) != 1 {
		t.Errorf("expected 1 activity event, got %d", len(detail.Activity))
	}
}

func TestGetTask_NotFound_404(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	db := &testhelpers.FakeDB{Workspaces: []database.Workspace{ws}}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/features/"+featureRowID+"/tasks/99999999-9999-9999-9999-999999999999")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- T5 scenario 5: activity route ---

func TestListActivity_WorkspaceLevel_ReturnsEvents(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	ev1 := testhelpers.NewActivityEvent(wsID, featureID, taskID, "done", "agent@example.com", "note1", 0)
	ev2 := testhelpers.NewActivityEvent(wsID, featureID, "", "ready", "orchestrator@example.com", "note2", 1)
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Activity:   []database.WorkspaceActivityEvent{ev1, ev2},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/activity")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var events []domain.ActivityEvent
	json.NewDecoder(resp.Body).Decode(&events)
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestListActivity_WithFeatureFilter_200(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	db := &testhelpers.FakeDB{Workspaces: []database.Workspace{ws}}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/activity?featureId="+featureID)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with featureId filter, got %d", resp.StatusCode)
	}
}

func TestListActivity_WithTaskOnlyFilter_ReturnsTaskEvents(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	taskEvent := testhelpers.NewActivityEvent(wsID, featureID, taskID, "done", "agent@example.com", "task note", 0)
	featureEvent := testhelpers.NewActivityEvent(wsID, featureID, "", "approved", "reviewer@example.com", "feature note", 1)
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Activity:   []database.WorkspaceActivityEvent{taskEvent, featureEvent},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/activity?taskId="+taskRowID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with taskId filter, got %d", resp.StatusCode)
	}
	var events []domain.ActivityEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatalf("decode events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 task activity event, got %d", len(events))
	}
	if events[0].TaskID != taskRowID {
		t.Fatalf("expected task_id %s, got %s", taskRowID, events[0].TaskID)
	}
}

func TestListActivity_WithFeatureAndTaskFilters_ReturnsMatchingTaskEvents(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	taskEvent := testhelpers.NewActivityEvent(wsID, featureID, taskID, "done", "agent@example.com", "task note", 0)
	featureEvent := testhelpers.NewActivityEvent(wsID, featureID, "", "approved", "reviewer@example.com", "feature note", 1)
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Activity:   []database.WorkspaceActivityEvent{taskEvent, featureEvent},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/activity?featureId="+featureRowID+"&taskId="+taskRowID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with featureId and taskId filters, got %d", resp.StatusCode)
	}
	var events []domain.ActivityEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatalf("decode events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 filtered activity event, got %d", len(events))
	}
	if events[0].FeatureID != featureRowID || events[0].TaskID != taskRowID {
		t.Fatalf("expected feature/task %s/%s, got %s/%s", featureRowID, taskRowID, events[0].FeatureID, events[0].TaskID)
	}
}

func TestListActivity_WorkspaceNotFound_404(t *testing.T) {
	db := &testhelpers.FakeDB{}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID+"/activity")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for missing workspace, got %d", resp.StatusCode)
	}
}

// --- Error response shape ---

func TestErrorResponse_HasRequiredFields(t *testing.T) {
	db := &testhelpers.FakeDB{}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID)
	defer resp.Body.Close()

	var apiErr domain.APIError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if apiErr.Code == "" {
		t.Error("error response must have a non-empty 'code' field")
	}
	if apiErr.Message == "" {
		t.Error("error response must have a non-empty 'message' field")
	}
	if apiErr.Source == "" {
		t.Error("error response must have a non-empty 'source' field")
	}
}

// --- Backward-compatibility: /healthz not shadowed by workspace routes ---

func TestHealthz_NotShadowedByWorkspaceRoutes(t *testing.T) {
	db := &testhelpers.FakeDB{}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from /healthz, got %d", resp.StatusCode)
	}
}

// --- All eight routes are registered ---

func TestAllWorkspaceRoutes_Registered(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	feat := testhelpers.NewFeature(wsID, featureID, "F", "in_design", "in_design")
	task := testhelpers.NewTask(wsID, featureID, taskID, "T", "done", nil)
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		Features:   []database.WorkspaceFeature{feat},
		Tasks:      []database.WorkspaceTask{task},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{ImportedWorkspaceID: wsID})
	defer srv.Close()

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/workspaces"},
		{http.MethodGet, "/api/workspaces/" + wsID},
		{http.MethodGet, "/api/workspaces/" + wsID + "/features"},
		{http.MethodGet, "/api/workspaces/" + wsID + "/tasks"},
		{http.MethodGet, "/api/workspaces/" + wsID + "/features/" + featureRowID},
		{http.MethodGet, "/api/workspaces/" + wsID + "/features/" + featureRowID + "/tasks"},
		{http.MethodGet, "/api/workspaces/" + wsID + "/features/" + featureRowID + "/tasks/" + taskRowID},
		{http.MethodGet, "/api/workspaces/" + wsID + "/activity"},
	}

	for _, rt := range routes {
		var resp *http.Response
		if rt.method == http.MethodGet {
			resp = get(t, srv, rt.path)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Errorf("route %s %s returned 404 — route not registered", rt.method, rt.path)
		}
	}
}

// --- Staleness signal ---

func TestGetWorkspace_IncludesStaleSourceState_WhenNoSyncRun(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	db := &testhelpers.FakeDB{Workspaces: []database.Workspace{ws}}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID)
	defer resp.Body.Close()

	var detail domain.WorkspaceDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !detail.SourceState.Stale {
		t.Error("expected source_state.stale=true when no sync run exists")
	}
}

func TestGetWorkspace_IncludesFreshSourceState_AfterRecentSuccessfulSync(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	sr := testhelpers.NewSyncRun(wsID, "import", "full_reconciliation", "success")
	if err := sr.FinishedAt.Scan(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		SyncRuns:   []database.WorkspaceSyncRun{sr},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID)
	defer resp.Body.Close()

	var detail domain.WorkspaceDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if detail.SourceState.Stale {
		t.Error("expected source_state.stale=false after recent successful sync")
	}
	if detail.SourceState.LastSyncedAt == nil {
		t.Error("expected source_state.last_synced_at after successful sync")
	}
}

func TestGetWorkspace_IncludesStaleSourceState_AfterFailedSyncRun(t *testing.T) {
	ws := testhelpers.NewWorkspace(wsID, "W", "w")
	failedRun := testhelpers.NewSyncRun(wsID, "manual", "full_reconciliation", "failed")
	errCode := "GITHUB_RATE_LIMIT"
	errMsg := "rate limit reached"
	failedRun.ErrorCode = &errCode
	failedRun.ErrorMessage = &errMsg
	db := &testhelpers.FakeDB{
		Workspaces: []database.Workspace{ws},
		SyncRuns:   []database.WorkspaceSyncRun{failedRun},
	}
	srv := newServer(db, &testhelpers.FakeAdapter{})
	defer srv.Close()

	resp := get(t, srv, "/api/workspaces/"+wsID)
	defer resp.Body.Close()

	var detail domain.WorkspaceDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !detail.SourceState.Stale {
		t.Error("expected source_state.stale=true after failed sync")
	}
	if detail.SourceState.ErrorCode != errCode {
		t.Errorf("expected source_state.error_code %q, got %q", errCode, detail.SourceState.ErrorCode)
	}
}
