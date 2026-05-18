// Package service implements the WorkspaceService that orchestrates read queries
// and RPC calls to adapter-service.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/tiendv89/workflow-backend/internal/adapter"
	"github.com/tiendv89/workflow-backend/internal/database"
	"github.com/tiendv89/workflow-backend/internal/domain"
)

// DatabaseReader is the read-only interface over the shared PostgreSQL schema.
type DatabaseReader interface {
	ListWorkspaces(ctx context.Context) ([]database.Workspace, error)
	GetWorkspace(ctx context.Context, workspaceID string) (database.Workspace, error)
	GetGitHubSource(ctx context.Context, workspaceID string) (database.WorkspaceGitHubSource, error)
	ListGitHubSources(ctx context.Context) ([]database.WorkspaceGitHubSource, error)
	ListLatestSyncRunsPerWorkspace(ctx context.Context) ([]database.WorkspaceSyncRun, error)
	GetLatestSyncRun(ctx context.Context, workspaceID string) (database.WorkspaceSyncRun, error)
	ListWorkspaceFeatures(ctx context.Context, workspaceID string) ([]database.WorkspaceFeature, error)
	SearchWorkspaceFeatures(ctx context.Context, workspaceID string, filters database.FeatureSearchFilters) ([]database.WorkspaceFeature, error)
	GetWorkspaceFeature(ctx context.Context, workspaceID, featureID string) (database.WorkspaceFeature, error)
	ListFeatureDocuments(ctx context.Context, workspaceID, featureID string) ([]database.WorkspaceFeatureDocument, error)
	ListFeatureTasks(ctx context.Context, workspaceID, featureID string) ([]database.WorkspaceTask, error)
	SearchFeatureTasks(ctx context.Context, workspaceID, featureID string, filters database.TaskSearchFilters) ([]database.WorkspaceTask, error)
	ListWorkspaceTasks(ctx context.Context, workspaceID string) ([]database.WorkspaceTask, error)
	GetWorkspaceTask(ctx context.Context, workspaceID, featureID, taskID string) (database.WorkspaceTask, error)
	ListActivityEvents(ctx context.Context, workspaceID, featureID, taskID string) ([]database.WorkspaceActivityEvent, error)
}

// AdapterCaller issues RPC calls to adapter-service for write operations.
type AdapterCaller interface {
	ImportWorkspace(ctx context.Context, req adapter.ImportRequest) (string, error)
	SyncWorkspace(ctx context.Context, workspaceID string) error
}

// WorkspaceService orchestrates reads from PostgreSQL and calls to adapter-service.
type WorkspaceService struct {
	db             DatabaseReader
	adapter        AdapterCaller
	staleThreshold time.Duration
}

// New creates a new WorkspaceService.
func New(db DatabaseReader, adapterClient AdapterCaller, staleThreshold time.Duration) *WorkspaceService {
	return &WorkspaceService{
		db:             db,
		adapter:        adapterClient,
		staleThreshold: staleThreshold,
	}
}

// ListWorkspaces returns summary rows for all saved workspaces with source state.
func (s *WorkspaceService) ListWorkspaces(ctx context.Context) ([]domain.WorkspaceSummary, error) {
	rows, err := s.db.ListWorkspaces(ctx)
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	// Batch-load latest sync run per workspace to avoid N+1 queries.
	allRuns, err := s.db.ListLatestSyncRunsPerWorkspace(ctx)
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}
	runMap := make(map[string]database.WorkspaceSyncRun, len(allRuns))
	for _, run := range allRuns {
		runMap[database.UUIDString(run.WorkspaceID)] = run
	}

	// Batch-load GitHub sources to avoid N+1 queries.
	allSrcs, err := s.db.ListGitHubSources(ctx)
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}
	srcMap := make(map[string]string, len(allSrcs))
	for _, src := range allSrcs {
		srcMap[database.UUIDString(src.WorkspaceID)] = src.RepoURL
	}

	out := make([]domain.WorkspaceSummary, 0, len(rows))
	for _, r := range rows {
		wsID := database.UUIDString(r.ID)
		srcRun, ok := runMap[wsID]
		var ss domain.SourceState
		if ok {
			ss = toSourceState(&srcRun, s.staleThreshold)
		} else {
			ss = domain.DeriveSourceState(nil, s.staleThreshold)
		}

		repoURL := srcMap[wsID]
		out = append(out, domain.WorkspaceSummary{
			ID:          wsID,
			Name:        r.Name,
			Slug:        r.Slug,
			RepoURL:     repoURL,
			SourceState: ss,
			UpdatedAt:   r.UpdatedAt.Time,
		})
	}
	return out, nil
}

// ImportWorkspace calls adapter-service to import a workspace then returns its summary.
func (s *WorkspaceService) ImportWorkspace(ctx context.Context, input domain.ImportInput) (*domain.WorkspaceSummary, domain.SourceError) {
	workspaceID, err := s.adapter.ImportWorkspace(ctx, adapter.ImportRequest{
		RepoURL:       input.RepoURL,
		DefaultBranch: input.DefaultBranch,
		Name:          input.Name,
	})
	if err != nil {
		if se, ok := err.(domain.SourceError); ok {
			return nil, se
		}
		return nil, domain.NewAdapterError(domain.ErrAdapterInternal, err.Error())
	}

	summary, dbErr := s.GetWorkspaceSummary(ctx, workspaceID)
	if dbErr != (domain.SourceError{}) {
		return nil, dbErr
	}
	return summary, domain.SourceError{}
}

// GetWorkspaceSummary returns only basic workspace information without loading features/tasks.
func (s *WorkspaceService) GetWorkspaceSummary(ctx context.Context, workspaceID string) (*domain.WorkspaceSummary, domain.SourceError) {
	ws, err := s.db.GetWorkspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("workspace", workspaceID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	wsID := database.UUIDString(ws.ID)
	syncRun, err := s.db.GetLatestSyncRun(ctx, wsID)
	var ss domain.SourceState
	if errors.Is(err, database.ErrNotFound) || err != nil {
		ss = domain.DeriveSourceState(nil, s.staleThreshold)
	} else {
		ss = toSourceState(&syncRun, s.staleThreshold)
	}

	return &domain.WorkspaceSummary{
		ID:          wsID,
		Name:        ws.Name,
		Slug:        ws.Slug,
		RepoURL:     s.githubRepoURL(ctx, wsID),
		SourceState: ss,
		UpdatedAt:   ws.UpdatedAt.Time,
	}, domain.SourceError{}
}

// GetWorkspace returns full workspace detail with features, tasks, and source state.
func (s *WorkspaceService) GetWorkspace(ctx context.Context, workspaceID string) (*domain.WorkspaceDetail, domain.SourceError) {
	ws, err := s.db.GetWorkspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("workspace", workspaceID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	wsID := database.UUIDString(ws.ID)

	features, err := s.db.ListWorkspaceFeatures(ctx, wsID)
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	tasks, err := s.db.ListWorkspaceTasks(ctx, wsID)
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	// Count tasks per feature for FeatureSummary.
	taskCountsByFeature := make(map[string]map[string]int)
	for _, t := range tasks {
		featureID := database.UUIDString(t.FeatureID)
		if _, ok := taskCountsByFeature[featureID]; !ok {
			taskCountsByFeature[featureID] = make(map[string]int)
		}
		taskCountsByFeature[featureID]["total"]++
		if t.Status != nil {
			taskCountsByFeature[featureID][*t.Status]++
		}
	}

	syncRun, err := s.db.GetLatestSyncRun(ctx, wsID)
	var ss domain.SourceState
	if errors.Is(err, database.ErrNotFound) || err != nil {
		ss = domain.DeriveSourceState(nil, s.staleThreshold)
	} else {
		ss = toSourceState(&syncRun, s.staleThreshold)
	}

	featureSummaries := make([]domain.FeatureSummary, 0, len(features))
	for _, f := range features {
		counts := taskCountsByFeature[database.UUIDString(f.ID)]
		featureSummaries = append(featureSummaries, domain.FeatureSummary{
			ID:           database.UUIDString(f.ID),
			FeatureID:    f.FeatureID,
			Title:        f.Title,
			Status:       derefStr(f.FeatureStatus),
			CurrentStage: derefStr(f.CurrentStage),
			Stages:       rawJSONOrNil(f.Stages),
			UpdatedAt:    f.UpdatedAt.Time,
			TaskCounts: domain.TaskCounts{
				Total:      counts["total"],
				Done:       counts["done"],
				InProgress: counts["in_progress"],
				Blocked:    counts["blocked"],
				Ready:      counts["ready"],
				Todo:       counts["todo"],
			},
		})
	}

	taskSummaries := make([]domain.TaskSummary, 0, len(tasks))
	for _, t := range tasks {
		taskSummaries = append(taskSummaries, toTaskSummary(t))
	}

	repoURL := s.githubRepoURL(ctx, wsID)
	return &domain.WorkspaceDetail{
		WorkspaceSummary: domain.WorkspaceSummary{
			ID:          wsID,
			Name:        ws.Name,
			Slug:        ws.Slug,
			RepoURL:     repoURL,
			SourceState: ss,
			UpdatedAt:   ws.UpdatedAt.Time,
		},
		Features: featureSummaries,
		Tasks:    taskSummaries,
	}, domain.SourceError{}
}

// SyncWorkspace calls adapter-service to re-sync a workspace then returns updated detail.
// On sync failure with cached data, returns cached data marked stale.
// On sync failure without cached data, returns a structured source error.
func (s *WorkspaceService) SyncWorkspace(ctx context.Context, workspaceID string) (*domain.WorkspaceDetail, domain.SourceError) {
	if err := s.adapter.SyncWorkspace(ctx, workspaceID); err != nil {
		// Sync failed — try to return cached data with stale marker.
		detail, dbErr := s.GetWorkspace(ctx, workspaceID)
		if dbErr != (domain.SourceError{}) {
			// No cached data — return the adapter error.
			if se, ok := err.(domain.SourceError); ok {
				return nil, se
			}
			return nil, domain.NewAdapterError(domain.ErrAdapterInternal, err.Error())
		}
		// Return stale cached data.
		detail.SourceState.Stale = true
		if se, ok := err.(domain.SourceError); ok {
			detail.SourceState.ErrorCode = string(se.Code)
		}
		return detail, domain.SourceError{}
	}

	return s.GetWorkspace(ctx, workspaceID)
}

// GetFeature returns feature detail with documents, tasks, and activity.
func (s *WorkspaceService) GetFeature(ctx context.Context, workspaceID, featureID string) (*domain.FeatureDetail, domain.SourceError) {
	// Verify workspace exists.
	ws, err := s.db.GetWorkspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("workspace", workspaceID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}
	wsID := database.UUIDString(ws.ID)

	feat, err := s.db.GetWorkspaceFeature(ctx, wsID, featureID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("feature", featureID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	featureUUID := database.UUIDString(feat.ID)

	docs, err := s.db.ListFeatureDocuments(ctx, wsID, featureUUID)
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	tasks, err := s.db.ListFeatureTasks(ctx, wsID, featureUUID)
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	activity, err := s.db.ListActivityEvents(ctx, wsID, featureUUID, "")
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	syncRun, err := s.db.GetLatestSyncRun(ctx, wsID)
	var ss domain.SourceState
	if errors.Is(err, database.ErrNotFound) || err != nil {
		ss = domain.DeriveSourceState(nil, s.staleThreshold)
	} else {
		ss = toSourceState(&syncRun, s.staleThreshold)
	}

	counts := countTasks(tasks)
	docLinks := make([]domain.DocumentLink, 0, len(docs))
	for _, d := range docs {
		docLinks = append(docLinks, domain.DocumentLink{
			DocumentType: d.DocumentType,
			SourcePath:   d.SourcePath,
			URL:          derefStr(d.URL),
		})
	}

	taskSummaries := make([]domain.TaskSummary, 0, len(tasks))
	for _, t := range tasks {
		taskSummaries = append(taskSummaries, toTaskSummary(t))
	}

	activityEvents := make([]domain.ActivityEvent, 0, len(activity))
	for _, a := range activity {
		activityEvents = append(activityEvents, toActivityEvent(a))
	}

	return &domain.FeatureDetail{
		FeatureSummary: domain.FeatureSummary{
			ID:           database.UUIDString(feat.ID),
			FeatureID:    feat.FeatureID,
			Title:        feat.Title,
			Status:       derefStr(feat.FeatureStatus),
			CurrentStage: derefStr(feat.CurrentStage),
			Stages:       rawJSONOrNil(feat.Stages),
			UpdatedAt:    feat.UpdatedAt.Time,
			TaskCounts:   counts,
		},
		WorkspaceID: workspaceID,
		Documents:   docLinks,
		Tasks:       taskSummaries,
		Activity:    activityEvents,
		SourceState: ss,
	}, domain.SourceError{}
}

// ListFeatureTasks returns task summaries for all tasks in a feature.
func (s *WorkspaceService) ListFeatureTasks(ctx context.Context, workspaceID, featureID string) ([]domain.TaskSummary, domain.SourceError) {
	ws, err := s.db.GetWorkspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("workspace", workspaceID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}
	wsID := database.UUIDString(ws.ID)

	feat, err := s.db.GetWorkspaceFeature(ctx, wsID, featureID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("feature", featureID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	tasks, err := s.db.ListFeatureTasks(ctx, wsID, database.UUIDString(feat.ID))
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	out := make([]domain.TaskSummary, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toTaskSummary(t))
	}
	return out, domain.SourceError{}
}

// SearchTasks returns task summaries for a feature filtered by task_id, title, status, and/or repo.
func (s *WorkspaceService) SearchTasks(ctx context.Context, workspaceID, featureID string, query domain.TaskSearchQuery) ([]domain.TaskSummary, domain.SourceError) {
	if query.Limit < 0 {
		return nil, domain.NewValidationError(domain.ErrValidationInvalidQuery, "limit must be greater than or equal to 0")
	}
	if query.Sort != "" && !isValidTaskSearchSort(query.Sort) {
		return nil, domain.NewValidationError(domain.ErrValidationInvalidQuery, "sort must be one of task_id_asc, task_id_desc, title_asc, title_desc, status_asc, status_desc, repo_asc, repo_desc, updated_at_asc, updated_at_desc, time_asc, time_desc")
	}

	ws, err := s.db.GetWorkspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("workspace", workspaceID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}
	wsID := database.UUIDString(ws.ID)

	feat, err := s.db.GetWorkspaceFeature(ctx, wsID, featureID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("feature", featureID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	tasks, err := s.db.SearchFeatureTasks(ctx, wsID, database.UUIDString(feat.ID), database.TaskSearchFilters{
		TaskID: query.TaskID,
		Title:  query.Title,
		Status: query.Status,
		Repo:   query.Repo,
		Sort:   query.Sort,
		Limit:  query.Limit,
	})
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	out := make([]domain.TaskSummary, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toTaskSummary(t))
	}
	return out, domain.SourceError{}
}

// SearchFeatures returns feature summaries for a workspace filtered by title and/or status.
func (s *WorkspaceService) SearchFeatures(ctx context.Context, workspaceID string, query domain.FeatureSearchQuery) ([]domain.FeatureSummary, domain.SourceError) {
	if query.Limit < 0 {
		return nil, domain.NewValidationError(domain.ErrValidationInvalidQuery, "limit must be greater than or equal to 0")
	}
	if query.Sort != "" && !isValidSearchSort(query.Sort) {
		return nil, domain.NewValidationError(domain.ErrValidationInvalidQuery, "sort must be one of title_asc, title_desc, status_asc, status_desc, updated_at_asc, updated_at_desc, time_asc, time_desc")
	}

	ws, err := s.db.GetWorkspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("workspace", workspaceID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}
	wsID := database.UUIDString(ws.ID)

	features, err := s.db.SearchWorkspaceFeatures(ctx, wsID, database.FeatureSearchFilters{
		Title:  query.Title,
		Status: query.Status,
		Sort:   query.Sort,
		Limit:  query.Limit,
	})
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	tasks, err := s.db.ListWorkspaceTasks(ctx, wsID)
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}
	taskCountsByFeature := make(map[string]map[string]int)
	for _, t := range tasks {
		featureID := database.UUIDString(t.FeatureID)
		if _, ok := taskCountsByFeature[featureID]; !ok {
			taskCountsByFeature[featureID] = make(map[string]int)
		}
		taskCountsByFeature[featureID]["total"]++
		if t.Status != nil {
			taskCountsByFeature[featureID][*t.Status]++
		}
	}

	out := make([]domain.FeatureSummary, 0, len(features))
	for _, f := range features {
		counts := taskCountsByFeature[database.UUIDString(f.ID)]
		out = append(out, domain.FeatureSummary{
			ID:           database.UUIDString(f.ID),
			FeatureID:    f.FeatureID,
			Title:        f.Title,
			Status:       derefStr(f.FeatureStatus),
			CurrentStage: derefStr(f.CurrentStage),
			Stages:       rawJSONOrNil(f.Stages),
			UpdatedAt:    f.UpdatedAt.Time,
			TaskCounts: domain.TaskCounts{
				Total:      counts["total"],
				Done:       counts["done"],
				InProgress: counts["in_progress"],
				Blocked:    counts["blocked"],
				Ready:      counts["ready"],
				Todo:       counts["todo"],
			},
		})
	}
	return out, domain.SourceError{}
}

// GetTask returns full task detail including activity.
func (s *WorkspaceService) GetTask(ctx context.Context, workspaceID, featureID, taskID string) (*domain.TaskDetail, domain.SourceError) {
	ws, err := s.db.GetWorkspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("workspace", workspaceID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}
	wsID := database.UUIDString(ws.ID)

	feat, err := s.db.GetWorkspaceFeature(ctx, wsID, featureID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("feature", featureID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	t, err := s.db.GetWorkspaceTask(ctx, wsID, database.UUIDString(feat.ID), taskID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("task", taskID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	activity, err := s.db.ListActivityEvents(ctx, wsID, database.UUIDString(t.FeatureID), database.UUIDString(t.ID))
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	activityEvents := make([]domain.ActivityEvent, 0, len(activity))
	for _, a := range activity {
		activityEvents = append(activityEvents, toActivityEvent(a))
	}

	var dependsOn []string
	if t.DependsOn != nil {
		_ = json.Unmarshal(t.DependsOn, &dependsOn)
	}
	if dependsOn == nil {
		dependsOn = []string{}
	}

	var execution map[string]interface{}
	if t.Execution != nil {
		_ = json.Unmarshal(t.Execution, &execution)
	}
	execCtx := domain.ExecutionContext{}
	if execution != nil {
		if v, ok := execution["actor_type"].(string); ok {
			execCtx.ActorType = v
		}
		if v, ok := execution["last_updated_by"].(string); ok {
			execCtx.LastUpdatedBy = v
		}
		if v, ok := execution["last_updated_at"].(string); ok {
			execCtx.LastUpdatedAt = v
		}
	}

	prRefs := parsePRRefs(t.Pr, t.WorkspacePr, t.FeatureName)

	return &domain.TaskDetail{
		TaskSummary: toTaskSummary(t),
		WorkspaceID: workspaceID,
		DependsOn:   dependsOn,
		Execution:   execCtx,
		PRRefs:      prRefs,
		Activity:    activityEvents,
	}, domain.SourceError{}
}

// ListActivity returns activity events for a workspace.
func (s *WorkspaceService) ListActivity(ctx context.Context, workspaceID string, scope domain.ActivityScope) ([]domain.ActivityEvent, domain.SourceError) {
	ws, err := s.db.GetWorkspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, domain.NewDatabaseNotFound("workspace", workspaceID)
		}
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}
	wsID := database.UUIDString(ws.ID)

	events, err := s.db.ListActivityEvents(ctx, wsID, scope.FeatureID, scope.TaskID)
	if err != nil {
		return nil, domain.NewDatabaseError(domain.ErrDatabaseQuery, err.Error())
	}

	out := make([]domain.ActivityEvent, 0, len(events))
	for _, a := range events {
		out = append(out, toActivityEvent(a))
	}
	return out, domain.SourceError{}
}

// githubRepoURL fetches the GitHub repo URL for a workspace from workspace_github_sources.
func (s *WorkspaceService) githubRepoURL(ctx context.Context, workspaceID string) string {
	src, err := s.db.GetGitHubSource(ctx, workspaceID)
	if err != nil {
		return ""
	}
	return src.RepoURL
}

// --- conversion helpers ---

func toSourceState(run *database.WorkspaceSyncRun, threshold time.Duration) domain.SourceState {
	if run == nil {
		return domain.DeriveSourceState(nil, threshold)
	}
	sr := &domain.SyncRun{
		ID:          database.UUIDString(run.ID),
		WorkspaceID: database.UUIDString(run.WorkspaceID),
		Trigger:     run.Trigger,
		Mode:        run.Mode,
		Status:      domain.SyncStatus(run.Status),
		StartedAt:   run.StartedAt.Time,
	}
	if run.FinishedAt.Valid {
		t := run.FinishedAt.Time
		sr.FinishedAt = &t
	}
	if run.CommitSha != nil {
		sr.CommitSHA = *run.CommitSha
	}
	if run.ErrorCode != nil {
		sr.ErrorCode = *run.ErrorCode
	}
	if run.ErrorMessage != nil {
		sr.ErrorMsg = *run.ErrorMessage
	}
	return domain.DeriveSourceState(sr, threshold)
}

func toTaskSummary(t database.WorkspaceTask) domain.TaskSummary {
	isBlocked := t.Status != nil && *t.Status == "blocked"
	return domain.TaskSummary{
		ID:            database.UUIDString(t.ID),
		TaskID:        t.TaskID,
		FeatureID:     database.UUIDString(t.FeatureID),
		FeatureName:   t.FeatureName,
		Title:         t.Title,
		Status:        derefStr(t.Status),
		Repo:          derefStr(t.Repo),
		Branch:        derefStr(t.Branch),
		IsBlocked:     isBlocked,
		BlockedReason: derefStr(t.BlockedReason),
		Pr:            rawJSONOrNil(t.Pr),
		WorkspacePr:   rawJSONOrNil(t.WorkspacePr),
	}
}

func toActivityEvent(a database.WorkspaceActivityEvent) domain.ActivityEvent {
	e := domain.ActivityEvent{
		Action:    derefStr(a.Action),
		Scope:     a.ScopeType,
		Actor:     derefStr(a.Actor),
		Note:      derefStr(a.Note),
		FeatureID: derefStr(a.FeatureID),
		TaskID:    derefStr(a.TaskID),
	}
	if a.OccurredAt != nil {
		if t, err := time.Parse(time.RFC3339, *a.OccurredAt); err == nil {
			e.OccurredAt = t
		}
	}
	return e
}

func isValidSearchSort(sort string) bool {
	switch sort {
	case "title_asc", "title_desc", "status_asc", "status_desc", "updated_at_asc", "updated_at_desc", "time_asc", "time_desc":
		return true
	default:
		return false
	}
}

func isValidTaskSearchSort(sort string) bool {
	switch sort {
	case "task_id_asc", "task_id_desc", "title_asc", "title_desc", "status_asc", "status_desc", "repo_asc", "repo_desc", "updated_at_asc", "updated_at_desc", "time_asc", "time_desc":
		return true
	default:
		return false
	}
}

func countTasks(tasks []database.WorkspaceTask) domain.TaskCounts {
	var counts domain.TaskCounts
	for _, t := range tasks {
		counts.Total++
		if t.Status != nil {
			switch *t.Status {
			case "done":
				counts.Done++
			case "in_progress":
				counts.InProgress++
			case "blocked":
				counts.Blocked++
			case "ready":
				counts.Ready++
			case "todo":
				counts.Todo++
			}
		}
	}
	return counts
}

func parsePRRefs(prData, workspacePRData []byte, featureID string) []domain.PullRequestRef {
	var refs []domain.PullRequestRef

	var pr map[string]interface{}
	if prData != nil {
		if json.Unmarshal(prData, &pr) == nil {
			if url, ok := pr["url"].(string); ok && url != "" {
				status, _ := pr["status"].(string)
				refs = append(refs, domain.PullRequestRef{
					Label:  "Implementation PR",
					URL:    url,
					Status: status,
					Repo:   featureID,
				})
			}
		}
	}

	var wsPR map[string]interface{}
	if workspacePRData != nil {
		if json.Unmarshal(workspacePRData, &wsPR) == nil {
			if url, ok := wsPR["url"].(string); ok && url != "" {
				status, _ := wsPR["status"].(string)
				refs = append(refs, domain.PullRequestRef{
					Label:  "Workspace PR",
					URL:    url,
					Status: status,
					Repo:   "management-repo",
				})
			}
		}
	}
	return refs
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func rawJSONOrNil(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" || !json.Valid(raw) {
		return nil
	}
	return raw
}
