package service_test

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
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
	listWSErr   error
	listRunsErr error
	listSrcsErr error
	getWSErr    error
	getRunErr   error

	listWorkspaceTasksCalls   int
	searchWorkspaceTasksCalls int
	getWorkspaceTaskCalls     int
	getWorkspaceTaskByIDCalls int
}

func (f *fakeDB) ListWorkspaces(_ context.Context) ([]database.Workspace, error) {
	if f.listWSErr != nil {
		return nil, f.listWSErr
	}
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

func (f *fakeDB) ListGitHubSources(_ context.Context) ([]database.WorkspaceGitHubSource, error) {
	if f.listSrcsErr != nil {
		return nil, f.listSrcsErr
	}
	out := make([]database.WorkspaceGitHubSource, 0, len(f.githubSrcs))
	for _, src := range f.githubSrcs {
		out = append(out, src)
	}
	return out, nil
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

func (f *fakeDB) SearchWorkspaceFeatures(_ context.Context, _ string, _ database.FeatureSearchFilters) ([]database.WorkspaceFeature, error) {
	return f.features, nil
}

func (f *fakeDB) ListFeatureTaskCounts(_ context.Context, _ string, featureIDs []string) ([]database.WorkspaceFeatureTaskCounts, error) {
	counts := make(map[string]database.WorkspaceFeatureTaskCounts, len(featureIDs))
	for _, featureID := range featureIDs {
		var row database.WorkspaceFeatureTaskCounts
		if err := row.FeatureID.Scan(featureID); err != nil {
			return nil, err
		}
		counts[featureID] = row
	}
	for _, task := range f.tasks {
		featureID := database.UUIDString(task.FeatureID)
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
	out := make([]database.WorkspaceFeatureTaskCounts, 0, len(featureIDs))
	for _, featureID := range featureIDs {
		out = append(out, counts[featureID])
	}
	return out, nil
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
	sort.SliceStable(out, func(i, j int) bool {
		switch filters.Sort {
		case "task_id_desc":
			return taskIDGreater(out[i].TaskName, out[j].TaskName)
		case "task_id_asc", "":
			return taskIDLess(out[i].TaskName, out[j].TaskName)
		default:
			return false
		}
	})
	if filters.Limit > 0 && filters.Limit < len(out) {
		out = out[:filters.Limit]
	}
	return out, nil
}

func (f *fakeDB) ListWorkspaceTasks(_ context.Context, _ string) ([]database.WorkspaceTask, error) {
	f.listWorkspaceTasksCalls++
	return f.tasks, nil
}

func (f *fakeDB) SearchWorkspaceTasks(_ context.Context, _ string, filters database.TaskSearchFilters) ([]database.WorkspaceTask, error) {
	f.searchWorkspaceTasksCalls++
	out := make([]database.WorkspaceTask, 0, len(f.tasks))
	for _, task := range f.tasks {
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
		switch filters.Sort {
		case "task_id_desc":
			return taskIDGreater(out[i].TaskName, out[j].TaskName)
		case "task_id_asc", "":
			return taskIDLess(out[i].TaskName, out[j].TaskName)
		default:
			return false
		}
	})
	if filters.Limit > 0 {
		start := (filters.Page - 1) * filters.Limit
		if filters.Page < 1 {
			start = 0
		}
		if start >= len(out) {
			return []database.WorkspaceTask{}, nil
		}
		end := start + filters.Limit
		if end > len(out) {
			end = len(out)
		}
		out = out[start:end]
	}
	return out, nil
}

func (f *fakeDB) GetWorkspaceTask(_ context.Context, _, featureID, taskID string) (database.WorkspaceTask, error) {
	f.getWorkspaceTaskCalls++
	for _, t := range f.tasks {
		if database.UUIDString(t.FeatureID) == featureID && database.UUIDString(t.TaskID) == taskID {
			return t, nil
		}
	}
	return database.WorkspaceTask{}, database.ErrNotFound
}

func (f *fakeDB) GetWorkspaceTaskByID(_ context.Context, workspaceID, taskID string) (database.WorkspaceTask, error) {
	f.getWorkspaceTaskByIDCalls++
	for _, t := range f.tasks {
		if database.UUIDString(t.WorkspaceID) == workspaceID && database.UUIDString(t.TaskID) == taskID {
			return t, nil
		}
	}
	return database.WorkspaceTask{}, database.ErrNotFound
}

func (f *fakeDB) ListActivityEvents(_ context.Context, _, _, _ string) ([]database.WorkspaceActivityEvent, error) {
	return f.activity, nil
}

func (f *fakeDB) CountWorkspaceFeatures(_ context.Context, _ string, _ database.FeatureSearchFilters) (int, error) {
	return len(f.features), nil
}

func (f *fakeDB) CountFeatureTasks(_ context.Context, _, featureID string, _ database.TaskSearchFilters) (int, error) {
	count := 0
	for _, t := range f.tasks {
		if database.UUIDString(t.FeatureID) == featureID {
			count++
		}
	}
	return count, nil
}

func (f *fakeDB) CountWorkspaceTasks(_ context.Context, _ string, _ database.TaskSearchFilters) (int, error) {
	return len(f.tasks), nil
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

func makeSuccessfulSyncRun(workspaceID string) database.WorkspaceSyncRun {
	var run database.WorkspaceSyncRun
	_ = run.ID.Scan("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	_ = run.WorkspaceID.Scan(workspaceID)
	run.Trigger = "api_import"
	run.Mode = "full"
	run.Status = "success"
	_ = run.StartedAt.Scan(time.Now().Add(-time.Second))
	_ = run.FinishedAt.Scan(time.Now())
	return run
}

const testWSID = "11111111-1111-1111-1111-111111111111"
const testFeatureRowID = "33333333-3333-3333-3333-333333333333"
const testTaskRowID = "44444444-4444-4444-4444-444444444444"

func newService(db service.DatabaseReader, adp service.AdapterCaller) *service.WorkspaceService {
	return service.New(db, adp, 30*time.Minute)
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

func TestListWorkspaces_RepoURLBatchLoaded(t *testing.T) {
	ws := makeUUID(testWSID)
	src := database.WorkspaceGitHubSource{RepoURL: "https://github.com/org/repo"}
	src.WorkspaceID.Scan(testWSID)
	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		githubSrcs: map[string]database.WorkspaceGitHubSource{testWSID: src},
	}
	svc := newService(db, &fakeAdapter{})

	result, err := svc.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(result))
	}
	if result[0].RepoURL != "https://github.com/org/repo" {
		t.Errorf("expected RepoURL 'https://github.com/org/repo', got %q", result[0].RepoURL)
	}
}

func TestListWorkspaces_DBError(t *testing.T) {
	db := &fakeDB{listWSErr: errors.New("db down")}
	svc := newService(db, &fakeAdapter{})

	_, err := svc.ListWorkspaces(context.Background())
	if err == nil {
		t.Fatal("expected database error")
	}
}

func TestListWorkspaces_LatestSyncRunsError(t *testing.T) {
	ws := makeUUID(testWSID)
	db := &fakeDB{
		workspaces:  []database.Workspace{ws},
		listRunsErr: errors.New("sync run query failed"),
	}
	svc := newService(db, &fakeAdapter{})

	_, err := svc.ListWorkspaces(context.Background())
	if err == nil {
		t.Fatal("expected database error from latest sync runs query")
	}
}

func TestListWorkspaces_GitHubSourcesError(t *testing.T) {
	ws := makeUUID(testWSID)
	db := &fakeDB{
		workspaces:  []database.Workspace{ws},
		listSrcsErr: errors.New("source query failed"),
	}
	svc := newService(db, &fakeAdapter{})

	_, err := svc.ListWorkspaces(context.Background())
	if err == nil {
		t.Fatal("expected database error from GitHub sources query")
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
		FeatureName:   "feature-1",
		Title:         "Feature One",
		FeatureStatus: &status,
	}
	feat.ID.Scan("77777777-7777-7777-7777-777777777777")
	feat.FeatureID.Scan(testFeatureRowID)
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
	if detail.Features[0].FeatureID != testFeatureRowID {
		t.Errorf("expected feature id %s, got %s", testFeatureRowID, detail.Features[0].FeatureID)
	}
	if detail.Features[0].FeatureName != "feature-1" {
		t.Errorf("expected feature name feature-1, got %s", detail.Features[0].FeatureName)
	}
}

func TestGetWorkspace_FailedSyncIncludesUserFacingErrorMessage(t *testing.T) {
	ws := makeUUID(testWSID)
	errCode := "GITHUB_RATE_LIMIT"
	errMsg := "GitHub API rate limit reached. Try again later."
	run := makeSuccessfulSyncRun(testWSID)
	run.Status = "failed"
	run.ErrorCode = &errCode
	run.ErrorMessage = &errMsg
	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		syncRuns:   []database.WorkspaceSyncRun{run},
	}
	svc := newService(db, &fakeAdapter{})

	detail, se := svc.GetWorkspace(context.Background(), testWSID)
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	if !detail.SourceState.Stale {
		t.Fatal("expected failed sync run to mark source_state stale")
	}
	if detail.SourceState.ErrorCode != errCode {
		t.Fatalf("expected error_code %q, got %q", errCode, detail.SourceState.ErrorCode)
	}
	if detail.SourceState.ErrorMessage != errMsg {
		t.Fatalf("expected error_message %q, got %q", errMsg, detail.SourceState.ErrorMessage)
	}
}

func TestGetWorkspace_TaskCountsUsePublicFeatureID(t *testing.T) {
	ws := makeUUID(testWSID)
	feat := database.WorkspaceFeature{
		FeatureName: "feature-1",
		Title:       "Feature One",
	}
	feat.ID.Scan("77777777-7777-7777-7777-777777777777")
	feat.FeatureID.Scan(testFeatureRowID)
	feat.WorkspaceID.Scan(testWSID)
	feat.UpdatedAt.Scan(time.Now())

	done := "done"
	ready := "ready"
	task1 := database.WorkspaceTask{FeatureName: "feature-1", TaskName: "T1", Title: "Task One", Status: &done}
	task2 := database.WorkspaceTask{FeatureName: "feature-1", TaskName: "T2", Title: "Task Two", Status: &ready}
	task1.FeatureID.Scan(testFeatureRowID)
	task2.FeatureID.Scan(testFeatureRowID)
	task1.WorkspaceID.Scan(testWSID)
	task2.WorkspaceID.Scan(testWSID)
	task1.UpdatedAt.Scan(time.Now())
	task2.UpdatedAt.Scan(time.Now())

	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		features:   []database.WorkspaceFeature{feat},
		tasks:      []database.WorkspaceTask{task1, task2},
	}
	svc := newService(db, &fakeAdapter{})

	detail, se := svc.GetWorkspace(context.Background(), testWSID)
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	if len(detail.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(detail.Features))
	}
	counts := detail.Features[0].TaskCounts
	if counts.Total != 2 || counts.Done != 1 || counts.Ready != 1 {
		t.Fatalf("expected counts to reflect public feature ID, got %+v", counts)
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
	src := database.WorkspaceGitHubSource{RepoURL: "https://github.com/org/repo"}
	src.WorkspaceID.Scan(testWSID)
	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		syncRuns:   []database.WorkspaceSyncRun{makeSuccessfulSyncRun(testWSID)},
		githubSrcs: map[string]database.WorkspaceGitHubSource{testWSID: src},
	}
	svc := newService(db, &fakeAdapter{importID: testWSID})

	detail, se := svc.ImportWorkspace(context.Background(), domain.ImportInput{RepoURL: "https://github.com/org/repo"})
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	if detail.ID != testWSID {
		t.Errorf("expected workspace ID %s, got %s", testWSID, detail.ID)
	}
	if detail.RepoURL != "https://github.com/org/repo" {
		t.Errorf("expected repo_url from persisted source, got %q", detail.RepoURL)
	}
	if detail.SourceState.Stale {
		t.Error("expected fresh source_state after completed import")
	}
}

func TestImportWorkspace_ReturnsErrorWhenAdapterDoesNotPersistWorkspace(t *testing.T) {
	db := &fakeDB{}
	svc := newService(db, &fakeAdapter{importID: testWSID})

	_, se := svc.ImportWorkspace(context.Background(), domain.ImportInput{RepoURL: "https://github.com/org/repo"})
	if se.Code != domain.ErrDatabaseNotFound {
		t.Fatalf("expected DATABASE_NOT_FOUND when import did not persist workspace, got %+v", se)
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

	_, se := svc.GetTask(context.Background(), testWSID, testFeatureRowID, "99999999-9999-9999-9999-999999999999")
	if se.Code != domain.ErrDatabaseNotFound {
		t.Errorf("expected ErrDatabaseNotFound, got %s", se.Code)
	}
}

func TestSearchTasks_TaskIDSortUsesWorkflowNumericOrder(t *testing.T) {
	ws := makeUUID(testWSID)
	feat := database.WorkspaceFeature{
		FeatureName: "feature-1",
		Title:       "Feature One",
	}
	feat.ID.Scan("77777777-7777-7777-7777-777777777777")
	feat.FeatureID.Scan(testFeatureRowID)
	feat.WorkspaceID.Scan(testWSID)
	feat.UpdatedAt.Scan(time.Now())
	status := "ready"
	task1 := database.WorkspaceTask{FeatureName: "feature-1", TaskName: "T1", Title: "Task One", Status: &status}
	task2 := database.WorkspaceTask{FeatureName: "feature-1", TaskName: "T2", Title: "Task Two", Status: &status}
	task10 := database.WorkspaceTask{FeatureName: "feature-1", TaskName: "T10", Title: "Task Ten", Status: &status}
	task1.ID.Scan(testTaskRowID)
	task2.ID.Scan("55555555-5555-5555-5555-555555555555")
	task10.ID.Scan("66666666-6666-6666-6666-666666666666")
	task1.TaskID.Scan(testTaskRowID)
	task2.TaskID.Scan("55555555-5555-5555-5555-555555555555")
	task10.TaskID.Scan("66666666-6666-6666-6666-666666666666")
	task1.FeatureID.Scan(testFeatureRowID)
	task2.FeatureID.Scan(testFeatureRowID)
	task10.FeatureID.Scan(testFeatureRowID)
	task1.WorkspaceID.Scan(testWSID)
	task2.WorkspaceID.Scan(testWSID)
	task10.WorkspaceID.Scan(testWSID)

	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		features:   []database.WorkspaceFeature{feat},
		tasks:      []database.WorkspaceTask{task1, task10, task2},
	}
	svc := newService(db, &fakeAdapter{})

	paged, se := svc.SearchTasks(context.Background(), testWSID, testFeatureRowID, domain.TaskSearchQuery{Sort: "task_id_asc"})
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	got := []string{paged.Items[0].TaskName, paged.Items[1].TaskName, paged.Items[2].TaskName}
	want := []string{"T1", "T2", "T10"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected task order %v, got %v", want, got)
		}
	}
}

func TestSearchWorkspaceTasks_UsesQueryLayerAndPaginates(t *testing.T) {
	ws := makeUUID(testWSID)
	feat := database.WorkspaceFeature{
		FeatureName: "feature-1",
		Title:       "Feature One",
	}
	feat.FeatureID.Scan(testFeatureRowID)
	feat.WorkspaceID.Scan(testWSID)
	feat.UpdatedAt.Scan(time.Now())
	status := "ready"
	task1 := database.WorkspaceTask{FeatureName: "feature-1", TaskName: "T1", Title: "Task One", Status: &status}
	task2 := database.WorkspaceTask{FeatureName: "feature-1", TaskName: "T2", Title: "Task Two", Status: &status}
	task10 := database.WorkspaceTask{FeatureName: "feature-1", TaskName: "T10", Title: "Task Ten", Status: &status}
	task1.FeatureID.Scan(testFeatureRowID)
	task2.FeatureID.Scan(testFeatureRowID)
	task10.FeatureID.Scan(testFeatureRowID)
	task1.WorkspaceID.Scan(testWSID)
	task2.WorkspaceID.Scan(testWSID)
	task10.WorkspaceID.Scan(testWSID)
	task1.UpdatedAt.Scan(time.Now())
	task2.UpdatedAt.Scan(time.Now())
	task10.UpdatedAt.Scan(time.Now())

	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		features:   []database.WorkspaceFeature{feat},
		tasks:      []database.WorkspaceTask{task1, task10, task2},
	}
	svc := newService(db, &fakeAdapter{})

	paged, se := svc.SearchWorkspaceTasks(context.Background(), testWSID, domain.TaskSearchQuery{Sort: "task_id_asc", Page: 1, Limit: 2})
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	if db.searchWorkspaceTasksCalls != 1 {
		t.Fatalf("expected search workspace tasks query to run once, got %d", db.searchWorkspaceTasksCalls)
	}
	if db.listWorkspaceTasksCalls != 0 {
		t.Fatalf("expected list workspace tasks to stay unused, got %d calls", db.listWorkspaceTasksCalls)
	}
	got := []string{paged.Items[0].TaskName, paged.Items[1].TaskName}
	want := []string{"T1", "T2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected paged task order %v, got %v", want, got)
		}
	}
}

func TestGetTask_Success(t *testing.T) {
	ws := makeUUID(testWSID)
	status := "in_progress"
	blocked := false
	_ = blocked
	taskStatus := &status
	repo := "workflow-backend"
	feat := database.WorkspaceFeature{
		FeatureName: "feature-1",
		Title:       "Feature One",
	}
	feat.ID.Scan("99999999-9999-9999-9999-999999999999")
	feat.FeatureID.Scan(testFeatureRowID)
	feat.WorkspaceID.Scan(testWSID)
	feat.UpdatedAt.Scan(time.Now())
	task := database.WorkspaceTask{
		FeatureName: "feature-1",
		TaskName:    "T1",
		Title:       "My Task",
		Status:      taskStatus,
		Repo:        &repo,
		DependsOn:   []byte(`["T0"]`),
		Execution:   []byte(`{"actor_type":"agent"}`),
		Pr:          []byte(`{"url":"https://github.com/tiendv89/workflow-backend/pull/3","status":"open"}`),
	}
	task.ID.Scan("88888888-8888-8888-8888-888888888888")
	task.TaskID.Scan(testTaskRowID)
	task.FeatureID.Scan(testFeatureRowID)
	task.WorkspaceID.Scan(testWSID)
	task.UpdatedAt.Scan(time.Now())

	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		features:   []database.WorkspaceFeature{feat},
		tasks:      []database.WorkspaceTask{task},
	}
	svc := newService(db, &fakeAdapter{})

	detail, se := svc.GetTask(context.Background(), testWSID, testFeatureRowID, testTaskRowID)
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	if detail.TaskName != "T1" {
		t.Errorf("expected task name T1, got %s", detail.TaskName)
	}
	if detail.TaskID != testTaskRowID {
		t.Errorf("expected task id %s, got %s", testTaskRowID, detail.TaskID)
	}
	if detail.Execution.ActorType != "agent" {
		t.Errorf("expected actor_type 'agent', got %q", detail.Execution.ActorType)
	}
	if len(detail.DependsOn) != 1 || detail.DependsOn[0] != "T0" {
		t.Errorf("expected depends_on [T0], got %v", detail.DependsOn)
	}
	if len(detail.PRRefs) != 1 {
		t.Fatalf("expected one PR ref, got %d", len(detail.PRRefs))
	}
	if detail.PRRefs[0].Repo != "workflow-backend" {
		t.Fatalf("expected PR ref repo workflow-backend, got %q", detail.PRRefs[0].Repo)
	}
}

func TestGetWorkspaceTask_Success(t *testing.T) {
	ws := makeUUID(testWSID)
	status := "in_progress"
	taskStatus := &status
	repo := "workflow-backend"
	feat := database.WorkspaceFeature{
		FeatureName: "feature-1",
		Title:       "Feature One",
	}
	feat.ID.Scan("99999999-9999-9999-9999-999999999999")
	feat.FeatureID.Scan(testFeatureRowID)
	feat.WorkspaceID.Scan(testWSID)
	feat.UpdatedAt.Scan(time.Now())
	task := database.WorkspaceTask{
		FeatureName: "feature-1",
		TaskName:    "T1",
		Title:       "My Task",
		Status:      taskStatus,
		Repo:        &repo,
		DependsOn:   []byte(`["T0"]`),
		Execution:   []byte(`{"actor_type":"agent"}`),
		Pr:          []byte(`{"url":"https://github.com/tiendv89/workflow-backend/pull/3","status":"open"}`),
	}
	task.ID.Scan("77777777-7777-7777-7777-777777777777")
	task.TaskID.Scan(testTaskRowID)
	task.FeatureID.Scan(testFeatureRowID)
	task.WorkspaceID.Scan(testWSID)
	task.UpdatedAt.Scan(time.Now())

	db := &fakeDB{
		workspaces: []database.Workspace{ws},
		features:   []database.WorkspaceFeature{feat},
		tasks:      []database.WorkspaceTask{task},
	}
	svc := newService(db, &fakeAdapter{})

	detail, se := svc.GetWorkspaceTask(context.Background(), testWSID, testTaskRowID)
	if se != (domain.SourceError{}) {
		t.Fatalf("unexpected error: %v", se)
	}
	if detail.TaskID != testTaskRowID {
		t.Errorf("expected task id %s, got %s", testTaskRowID, detail.TaskID)
	}
	if detail.FeatureID != testFeatureRowID {
		t.Errorf("expected feature id %s, got %s", testFeatureRowID, detail.FeatureID)
	}
	if len(detail.PRRefs) != 1 {
		t.Fatalf("expected one PR ref, got %d", len(detail.PRRefs))
	}
	if detail.PRRefs[0].Repo != "workflow-backend" {
		t.Fatalf("expected PR ref repo workflow-backend, got %q", detail.PRRefs[0].Repo)
	}
	if db.getWorkspaceTaskByIDCalls != 1 {
		t.Fatalf("expected GetWorkspaceTaskByID to run once, got %d", db.getWorkspaceTaskByIDCalls)
	}
	if db.getWorkspaceTaskCalls != 0 {
		t.Fatalf("expected feature-scoped task lookup to stay unused, got %d calls", db.getWorkspaceTaskCalls)
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
