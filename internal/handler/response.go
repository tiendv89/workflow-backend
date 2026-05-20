package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tiendv89/workflow-backend/internal/domain"
)

type apiEnvelope struct {
	Status string         `json:"status"`
	Data   any            `json:"data,omitempty"`
	Error  *apiErrorShape `json:"error,omitempty"`
}

type apiErrorShape struct {
	Code      domain.ErrorCode `json:"code"`
	Message   string           `json:"message"`
	Retryable bool             `json:"retryable"`
}

func respondSuccess(c *gin.Context, statusCode int, data any) {
	c.JSON(statusCode, apiEnvelope{
		Status: "success",
		Data:   data,
	})
}

func respondError(c *gin.Context, err error) {
	if se, ok := err.(domain.SourceError); ok {
		respondSourceError(c, se, nil)
		return
	}
	respondSourceError(c, domain.NewAdapterError(domain.ErrAdapterInternal, err.Error()), nil)
}

func respondSourceError(c *gin.Context, se domain.SourceError, _ interface{}) {
	c.JSON(sourceErrorHTTPStatus(se), apiEnvelope{
		Status: "error",
		Error: &apiErrorShape{
			Code:      se.Code,
			Message:   se.Message,
			Retryable: se.Retryable,
		},
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
