// Package handler implements gin HTTP handlers for the workspace API routes.
package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tiendv89/workflow-backend/internal/domain"
)

// Service is the interface the handler depends on.
type Service interface {
	ListWorkspaces(ctx context.Context) ([]domain.WorkspaceSummary, error)
	ImportWorkspace(ctx context.Context, input domain.ImportInput) (*domain.WorkspaceDetail, domain.SourceError)
	GetWorkspace(ctx context.Context, workspaceID string) (*domain.WorkspaceDetail, domain.SourceError)
	SearchFeatures(ctx context.Context, workspaceID string, query domain.FeatureSearchQuery) (*domain.PagedFeatures, domain.SourceError)
	SearchTasks(ctx context.Context, workspaceID, featureID string, query domain.TaskSearchQuery) (*domain.PagedTasks, domain.SourceError)
	SearchWorkspaceTasks(ctx context.Context, workspaceID string, query domain.TaskSearchQuery) (*domain.PagedTasks, domain.SourceError)
	GetWorkspaceTask(ctx context.Context, workspaceID, taskID string) (*domain.TaskDetail, domain.SourceError)
	SyncWorkspace(ctx context.Context, workspaceID string) (*domain.WorkspaceDetail, domain.SourceError)
	GetFeature(ctx context.Context, workspaceID, featureID string) (*domain.FeatureDetail, domain.SourceError)
	GetTask(ctx context.Context, workspaceID, featureID, taskID string) (*domain.TaskDetail, domain.SourceError)
	ListActivity(ctx context.Context, workspaceID string, scope domain.ActivityScope) ([]domain.ActivityEvent, domain.SourceError)
}

// WorkspaceHandler holds a reference to the source service.
type WorkspaceHandler struct {
	svc Service
}

// New creates a new WorkspaceHandler.
func New(svc Service) *WorkspaceHandler {
	return &WorkspaceHandler{svc: svc}
}

// RegisterRoutes registers all workspace API routes on the given gin router group.
func (h *WorkspaceHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/workspaces", h.ListWorkspaces)
	rg.POST("/workspaces/import", h.ImportWorkspace)
	rg.GET("/workspaces/:workspaceId", h.GetWorkspace)
	rg.GET("/workspaces/:workspaceId/features", h.SearchFeatures)
	rg.GET("/workspaces/:workspaceId/tasks", h.SearchWorkspaceTasks)
	rg.GET("/workspaces/:workspaceId/tasks/:taskId", h.GetWorkspaceTask)
	rg.POST("/workspaces/:workspaceId/sync", h.SyncWorkspace)
	rg.GET("/workspaces/:workspaceId/features/:featureId", h.GetFeature)
	rg.GET("/workspaces/:workspaceId/features/:featureId/tasks", h.SearchTasks)
	rg.GET("/workspaces/:workspaceId/features/:featureId/tasks/:taskId", h.GetTask)
	rg.GET("/workspaces/:workspaceId/activity", h.ListActivity)
}

// ListWorkspaces godoc
// GET /api/workspaces
func (h *WorkspaceHandler) ListWorkspaces(c *gin.Context) {
	result, err := h.svc.ListWorkspaces(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, result)
}

// ImportWorkspace godoc
// POST /api/workspaces/import
func (h *WorkspaceHandler) ImportWorkspace(c *gin.Context) {
	var input domain.ImportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		respondSourceError(c, domain.NewValidationError(domain.ErrValidationMissingInput, err.Error()), nil)
		return
	}

	detail, se := h.svc.ImportWorkspace(c.Request.Context(), input)
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	respondOK(c, detail)
}

// GetWorkspace godoc
// GET /api/workspaces/:workspaceId
func (h *WorkspaceHandler) GetWorkspace(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	detail, se := h.svc.GetWorkspace(c.Request.Context(), workspaceID)
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	respondOK(c, detail)
}

// SearchFeatures godoc
// GET /api/workspaces/:workspaceId/features?title=&status=&sort=&page=&limit=
func (h *WorkspaceHandler) SearchFeatures(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	page, limit, ok := parsePagination(c)
	if !ok {
		return
	}

	paged, se := h.svc.SearchFeatures(c.Request.Context(), workspaceID, domain.FeatureSearchQuery{
		Title:  c.Query("title"),
		Status: c.Query("status"),
		Sort:   c.Query("sort"),
		Page:   page,
		Limit:  limit,
	})
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	respondOK(c, paged)
}

// SearchTasks godoc
// GET /api/workspaces/:workspaceId/features/:featureId/tasks?task_id=&title=&status=&repo=&sort=&page=&limit=
func (h *WorkspaceHandler) SearchTasks(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	featureID := c.Param("featureId")
	page, limit, ok := parsePagination(c)
	if !ok {
		return
	}

	paged, se := h.svc.SearchTasks(c.Request.Context(), workspaceID, featureID, domain.TaskSearchQuery{
		TaskID: c.Query("task_id"),
		Title:  c.Query("title"),
		Status: c.Query("status"),
		Repo:   c.Query("repo"),
		Sort:   c.Query("sort"),
		Page:   page,
		Limit:  limit,
	})
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	respondOK(c, paged)
}

// SearchWorkspaceTasks godoc
// GET /api/workspaces/:workspaceId/tasks?task_id=&title=&status=&repo=&sort=&page=&limit=
func (h *WorkspaceHandler) SearchWorkspaceTasks(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	page, limit, ok := parsePagination(c)
	if !ok {
		return
	}

	paged, se := h.svc.SearchWorkspaceTasks(c.Request.Context(), workspaceID, domain.TaskSearchQuery{
		TaskID: c.Query("task_id"),
		Title:  c.Query("title"),
		Status: c.Query("status"),
		Repo:   c.Query("repo"),
		Sort:   c.Query("sort"),
		Page:   page,
		Limit:  limit,
	})
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	respondOK(c, paged)
}

// GetWorkspaceTask godoc
// GET /api/workspaces/:workspaceId/tasks/:taskId
func (h *WorkspaceHandler) GetWorkspaceTask(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	taskID := c.Param("taskId")
	detail, se := h.svc.GetWorkspaceTask(c.Request.Context(), workspaceID, taskID)
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	respondOK(c, detail)
}

// SyncWorkspace godoc
// POST /api/workspaces/:workspaceId/sync
func (h *WorkspaceHandler) SyncWorkspace(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	detail, se := h.svc.SyncWorkspace(c.Request.Context(), workspaceID)
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	respondOK(c, detail)
}

// GetFeature godoc
// GET /api/workspaces/:workspaceId/features/:featureId
func (h *WorkspaceHandler) GetFeature(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	featureID := c.Param("featureId")
	detail, se := h.svc.GetFeature(c.Request.Context(), workspaceID, featureID)
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	respondOK(c, detail)
}

// GetTask godoc
// GET /api/workspaces/:workspaceId/features/:featureId/tasks/:taskId
func (h *WorkspaceHandler) GetTask(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	featureID := c.Param("featureId")
	taskID := c.Param("taskId")
	detail, se := h.svc.GetTask(c.Request.Context(), workspaceID, featureID, taskID)
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	respondOK(c, detail)
}

// ListActivity godoc
// GET /api/workspaces/:workspaceId/activity
func (h *WorkspaceHandler) ListActivity(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	scope := domain.ActivityScope{
		FeatureID: c.Query("featureId"),
		TaskID:    c.Query("taskId"),
	}
	events, se := h.svc.ListActivity(c.Request.Context(), workspaceID, scope)
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	respondOK(c, events)
}

// --- error response helpers ---

type apiSuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

type apiErrorResponse struct {
	Success bool          `json:"success"`
	Error   *apiErrorBody `json:"error"`
}

type apiErrorBody struct {
	Code       domain.ErrorCode   `json:"code"`
	Message    string             `json:"message"`
	Source     domain.ErrorSource `json:"source"`
	Retryable  bool               `json:"retryable"`
	Path       string             `json:"path,omitempty"`
	CachedData interface{}        `json:"cached_data,omitempty"`
}

func respondOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, apiSuccessResponse{
		Success: true,
		Data:    data,
	})
}

func respondError(c *gin.Context, err error) {
	if se, ok := err.(domain.SourceError); ok {
		respondSourceError(c, se, nil)
		return
	}
	respondSourceError(c, domain.SourceError{
		Code:      domain.ErrAdapterInternal,
		Message:   err.Error(),
		Source:    domain.ErrorSourceAdapter,
		Retryable: true,
	}, nil)
}

func parsePagination(c *gin.Context) (int, int, bool) {
	page := 1
	if rawPage := c.Query("page"); rawPage != "" {
		parsed, err := strconv.Atoi(rawPage)
		if err != nil {
			respondSourceError(c, domain.NewValidationError(domain.ErrValidationInvalidQuery, "page must be an integer"), nil)
			return 0, 0, false
		}
		if parsed < 1 {
			respondSourceError(c, domain.NewValidationError(domain.ErrValidationInvalidQuery, "page must be greater than or equal to 1"), nil)
			return 0, 0, false
		}
		page = parsed
	}

	limit := 0
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			respondSourceError(c, domain.NewValidationError(domain.ErrValidationInvalidQuery, "limit must be an integer"), nil)
			return 0, 0, false
		}
		limit = parsed
	}
	return page, limit, true
}

func respondSourceError(c *gin.Context, se domain.SourceError, cachedData interface{}) {
	statusCode := sourceErrorHTTPStatus(se)
	errBody := &apiErrorBody{
		Code:      se.Code,
		Message:   se.Message,
		Source:    se.Source,
		Retryable: se.Retryable,
		Path:      se.Path,
	}
	if cachedData != nil {
		errBody.CachedData = cachedData
	}
	c.JSON(statusCode, apiErrorResponse{
		Success: false,
		Error:   errBody,
	})
}

func sourceErrorHTTPStatus(se domain.SourceError) int {
	switch se.Code {
	case domain.ErrDatabaseNotFound, domain.ErrGitHubNotFound:
		return http.StatusNotFound
	case domain.ErrValidationInvalidURL, domain.ErrValidationMissingInput, domain.ErrValidationInvalidQuery:
		return http.StatusBadRequest
	case domain.ErrGitHubUnauthorized:
		return http.StatusUnauthorized
	case domain.ErrGitHubRateLimit:
		return http.StatusTooManyRequests
	case domain.ErrAdapterTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
