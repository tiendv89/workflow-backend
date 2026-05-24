// Package response provides gin HTTP response helpers for the workspace API.
package response

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tiendv89/workflow-backend/internal/domain"
)

type apiSuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

type apiErrorResponse struct {
	Success bool          `json:"success"`
	Error   *apiErrorBody `json:"error"`
}

type apiErrorBody struct {
	Code      domain.ErrorCode   `json:"code"`
	Message   string             `json:"message"`
	Source    domain.ErrorSource `json:"source"`
	Retryable bool               `json:"retryable"`
	Path      string             `json:"path,omitempty"`
}

// RespondOK writes a 200 JSON response with a success envelope.
func RespondOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, apiSuccessResponse{
		Success: true,
		Data:    data,
	})
}

// RespondError writes an error JSON response derived from a SourceError.
func RespondError(c *gin.Context, se domain.SourceError) {
	statusCode := sourceErrorHTTPStatus(se)
	c.JSON(statusCode, apiErrorResponse{
		Success: false,
		Error: &apiErrorBody{
			Code:      se.Code,
			Message:   se.Message,
			Source:    se.Source,
			Retryable: se.Retryable,
			Path:      se.Path,
		},
	})
}

// RespondValidationError writes a 400 error response for input validation failures.
func RespondValidationError(c *gin.Context, code domain.ErrorCode, msg string) {
	RespondError(c, domain.NewValidationError(code, msg))
}

// ParsePagination parses page and limit query parameters.
// On invalid input it writes an error response and returns ok=false.
func ParsePagination(c *gin.Context) (page, limit int, ok bool) {
	page = 1
	if rawPage := c.Query("page"); rawPage != "" {
		parsed, err := strconv.Atoi(rawPage)
		if err != nil {
			RespondValidationError(c, domain.ErrValidationInvalidQuery, "page must be an integer")
			return 0, 0, false
		}
		if parsed < 1 {
			RespondValidationError(c, domain.ErrValidationInvalidQuery, "page must be greater than or equal to 1")
			return 0, 0, false
		}
		page = parsed
	}

	limit = 0
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			RespondValidationError(c, domain.ErrValidationInvalidQuery, "limit must be an integer")
			return 0, 0, false
		}
		limit = parsed
	}
	return page, limit, true
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
