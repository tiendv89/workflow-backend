package domain

import "fmt"

// ErrorSource identifies the system that produced a SourceError.
type ErrorSource string

const (
	ErrorSourceGitHub     ErrorSource = "github"
	ErrorSourceDatabase   ErrorSource = "database"
	ErrorSourceParser     ErrorSource = "parser"
	ErrorSourceAdapter    ErrorSource = "adapter"
	ErrorSourceValidation ErrorSource = "validation"
)

// ErrorCode is a machine-readable error identifier.
type ErrorCode string

const (
	ErrGitHubRateLimit    ErrorCode = "GITHUB_RATE_LIMIT"
	ErrGitHubNotFound     ErrorCode = "GITHUB_NOT_FOUND"
	ErrGitHubUnauthorized ErrorCode = "GITHUB_UNAUTHORIZED"
	ErrGitHubNetworkError ErrorCode = "GITHUB_NETWORK_ERROR"
	ErrGitHubServerError  ErrorCode = "GITHUB_SERVER_ERROR"

	ErrDatabaseConnection  ErrorCode = "DATABASE_CONNECTION"
	ErrDatabaseQuery       ErrorCode = "DATABASE_QUERY"
	ErrDatabaseTransaction ErrorCode = "DATABASE_TRANSACTION"
	ErrDatabaseNotFound    ErrorCode = "DATABASE_NOT_FOUND"

	ErrAdapterInternal ErrorCode = "ADAPTER_INTERNAL"
	ErrAdapterTimeout  ErrorCode = "ADAPTER_TIMEOUT"

	ErrValidationInvalidURL   ErrorCode = "VALIDATION_INVALID_URL"
	ErrValidationMissingInput ErrorCode = "VALIDATION_MISSING_INPUT"
)

// SourceError is the normalized error shape for all backend errors.
type SourceError struct {
	Code      ErrorCode   `json:"code"`
	Message   string      `json:"message"`
	Source    ErrorSource `json:"source"`
	Retryable bool        `json:"retryable"`
	Path      string      `json:"path,omitempty"`
}

func (e SourceError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("[%s/%s] %s (path: %s)", e.Source, e.Code, e.Message, e.Path)
	}
	return fmt.Sprintf("[%s/%s] %s", e.Source, e.Code, e.Message)
}

// APIError is the HTTP response body for error responses.
type APIError struct {
	Code       ErrorCode   `json:"code"`
	Message    string      `json:"message"`
	Source     ErrorSource `json:"source"`
	Retryable  bool        `json:"retryable"`
	CachedData interface{} `json:"cached_data,omitempty"`
}

// NewDatabaseNotFound returns a SourceError for missing rows.
func NewDatabaseNotFound(resource, id string) SourceError {
	return SourceError{
		Code:      ErrDatabaseNotFound,
		Message:   fmt.Sprintf("%s not found: %s", resource, id),
		Source:    ErrorSourceDatabase,
		Retryable: false,
	}
}

// NewDatabaseError returns a SourceError for database failures.
func NewDatabaseError(code ErrorCode, detail string) SourceError {
	return SourceError{
		Code:      code,
		Message:   fmt.Sprintf("Database error: %s", detail),
		Source:    ErrorSourceDatabase,
		Retryable: true,
	}
}

// NewAdapterError returns a SourceError for adapter-service failures.
func NewAdapterError(code ErrorCode, detail string) SourceError {
	return SourceError{
		Code:      code,
		Message:   fmt.Sprintf("Adapter service error: %s", detail),
		Source:    ErrorSourceAdapter,
		Retryable: true,
	}
}

// NewValidationError returns a SourceError for input validation failures.
func NewValidationError(code ErrorCode, message string) SourceError {
	return SourceError{
		Code:      code,
		Message:   message,
		Source:    ErrorSourceValidation,
		Retryable: false,
	}
}

// FromSourceError converts a SourceError to an APIError with optional cached data.
func FromSourceError(e SourceError, cachedData interface{}) APIError {
	return APIError{
		Code:       e.Code,
		Message:    e.Message,
		Source:     e.Source,
		Retryable:  e.Retryable,
		CachedData: cachedData,
	}
}
