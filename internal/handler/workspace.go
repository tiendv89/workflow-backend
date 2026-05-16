// Package handler implements gin HTTP handlers for the workspace API routes.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tiendv89/workflow-backend/internal/domain"
	"github.com/tiendv89/workflow-backend/internal/service"
)

// WorkspaceHandler holds a reference to the source service.
type WorkspaceHandler struct {
	svc *service.WorkspaceService
}

// New creates a new WorkspaceHandler.
func New(svc *service.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{svc: svc}
}

// RegisterRoutes registers all workspace API routes on the given gin router group.
func (h *WorkspaceHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/workspaces", h.ListWorkspaces)
	rg.POST("/workspaces/import", h.ImportWorkspace)
	rg.GET("/workspaces/:workspaceId", h.GetWorkspace)
	rg.POST("/workspaces/:workspaceId/sync", h.SyncWorkspace)
	rg.GET("/workspaces/:workspaceId/features/:featureId", h.GetFeature)
	rg.GET("/workspaces/:workspaceId/features/:featureId/tasks", h.ListFeatureTasks)
	rg.GET("/workspaces/:workspaceId/tasks/:taskId", h.GetTask)
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
	c.JSON(http.StatusOK, result)
}

// ImportWorkspace godoc
// POST /api/workspaces/import
func (h *WorkspaceHandler) ImportWorkspace(c *gin.Context) {
	var input domain.ImportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.APIError{
			Code:      domain.ErrValidationMissingInput,
			Message:   err.Error(),
			Source:    domain.ErrorSourceValidation,
			Retryable: false,
		})
		return
	}

	detail, se := h.svc.ImportWorkspace(c.Request.Context(), input)
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	c.JSON(http.StatusCreated, detail)
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
	c.JSON(http.StatusOK, detail)
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
	// On sync success or stale-cache fallback, always return 200 with cached_data.
	c.JSON(http.StatusOK, detail)
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
	c.JSON(http.StatusOK, detail)
}

// ListFeatureTasks godoc
// GET /api/workspaces/:workspaceId/features/:featureId/tasks
func (h *WorkspaceHandler) ListFeatureTasks(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	featureID := c.Param("featureId")
	tasks, se := h.svc.ListFeatureTasks(c.Request.Context(), workspaceID, featureID)
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// GetTask godoc
// GET /api/workspaces/:workspaceId/tasks/:taskId
func (h *WorkspaceHandler) GetTask(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	taskID := c.Param("taskId")
	// The route uses :taskId only; featureId must be provided as a query param
	// or embedded in the task record itself. For determinism, we require featureId.
	featureID := c.Query("featureId")
	detail, se := h.svc.GetTask(c.Request.Context(), workspaceID, featureID, taskID)
	if se != (domain.SourceError{}) {
		respondSourceError(c, se, nil)
		return
	}
	c.JSON(http.StatusOK, detail)
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
	c.JSON(http.StatusOK, events)
}

// --- error response helpers ---

func respondError(c *gin.Context, err error) {
	if se, ok := err.(domain.SourceError); ok {
		respondSourceError(c, se, nil)
		return
	}
	c.JSON(http.StatusInternalServerError, domain.APIError{
		Code:      domain.ErrAdapterInternal,
		Message:   err.Error(),
		Source:    domain.ErrorSourceAdapter,
		Retryable: true,
	})
}

func respondSourceError(c *gin.Context, se domain.SourceError, cachedData interface{}) {
	statusCode := sourceErrorHTTPStatus(se)
	c.JSON(statusCode, domain.FromSourceError(se, cachedData))
}

func sourceErrorHTTPStatus(se domain.SourceError) int {
	switch se.Code {
	case domain.ErrDatabaseNotFound, domain.ErrGitHubNotFound:
		return http.StatusNotFound
	case domain.ErrValidationInvalidURL, domain.ErrValidationMissingInput:
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
