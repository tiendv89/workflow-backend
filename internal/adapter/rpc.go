// Package adapter provides an HTTP client for calling adapter-service RPC endpoints.
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tiendv89/workflow-backend/internal/domain"
)

// Client calls adapter-service internal RPC endpoints for import and sync operations.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new RPC Client targeting the given adapter-service base URL.
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ImportRequest is the payload sent to adapter-service for workspace import.
type ImportRequest struct {
	RepoURL       string `json:"repo_url"`
	DefaultBranch string `json:"default_branch,omitempty"`
	Name          string `json:"name,omitempty"`
}

// ImportResponse is the response from adapter-service after an import is accepted or found.
type ImportResponse struct {
	WorkspaceID string `json:"workspace_id"`
}

// ImportWorkspace calls POST /internal/workspaces/import on adapter-service.
// Returns the workspace ID once adapter-service accepts or finds the import.
func (c *Client) ImportWorkspace(ctx context.Context, req ImportRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal import request: %w", err)
	}

	const path = "/internal/workspaces/import"
	resp, statusCode, err := c.post(ctx, path, body)
	if err != nil {
		return "", err
	}
	if statusCode != http.StatusOK && statusCode != http.StatusAccepted {
		return "", sourceErrorFromHTTPStatus(path, statusCode, resp) // TODO: Test
	}

	var out ImportResponse
	if err := json.Unmarshal(resp, &out); err != nil {
		return "", fmt.Errorf("decode import response: %w", err)
	}
	if out.WorkspaceID == "" {
		return "", fmt.Errorf("decode import response: missing workspace_id")
	}
	return out.WorkspaceID, nil
}

// SyncWorkspace calls POST /internal/workspaces/{workspaceId}/sync on adapter-service.
func (c *Client) SyncWorkspace(ctx context.Context, workspaceID string) error {
	path := fmt.Sprintf("/internal/workspaces/%s/sync", workspaceID)
	data, statusCode, err := c.post(ctx, path, nil)
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK {
		return sourceErrorFromHTTPStatus(path, statusCode, data)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, body []byte) ([]byte, int, error) {
	url := c.baseURL + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
	if err != nil {
		return nil, 0, domain.NewAdapterError(domain.ErrAdapterInternal, fmt.Sprintf("build request: %v", err))
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, domain.NewAdapterError(domain.ErrAdapterTimeout, fmt.Sprintf("call %s: %v", path, err))
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, domain.NewAdapterError(domain.ErrAdapterInternal, fmt.Sprintf("read response from %s: %v", path, err))
	}

	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, decodeSourceErrorResponse(path, resp.StatusCode, data)
	}
	if resp.StatusCode >= 300 {
		return nil, resp.StatusCode, sourceErrorFromHTTPStatus(path, resp.StatusCode, data)
	}
	return data, resp.StatusCode, nil
}

func decodeSourceErrorResponse(path string, statusCode int, data []byte) domain.SourceError {
	var se domain.SourceError
	if err := json.Unmarshal(data, &se); err == nil && se.Code != "" && se.Source != "" {
		if se.Message == "" {
			se.Message = fmt.Sprintf("adapter-service returned HTTP %d", statusCode)
		}
		return se
	}

	var apiErr domain.APIError
	if err := json.Unmarshal(data, &apiErr); err == nil && apiErr.Code != "" && apiErr.Source != "" {
		msg := apiErr.Message
		if msg == "" {
			msg = fmt.Sprintf("adapter-service returned HTTP %d", statusCode)
		}
		return domain.SourceError{
			Code:      apiErr.Code,
			Message:   msg,
			Source:    apiErr.Source,
			Retryable: apiErr.Retryable,
		}
	}

	return sourceErrorFromHTTPStatus(path, statusCode, data)
}

func sourceErrorFromHTTPStatus(path string, statusCode int, data []byte) domain.SourceError {
	message := fmt.Sprintf("adapter-service %s returned HTTP %d", path, statusCode)
	if len(data) > 0 {
		message = fmt.Sprintf("%s: %s", message, string(data))
	}

	switch statusCode {
	case http.StatusUnauthorized:
		return domain.SourceError{Code: domain.ErrGitHubUnauthorized, Message: message, Source: domain.ErrorSourceGitHub, Retryable: false}
	case http.StatusTooManyRequests:
		return domain.SourceError{Code: domain.ErrGitHubRateLimit, Message: message, Source: domain.ErrorSourceGitHub, Retryable: true}
	case http.StatusNotFound:
		return domain.SourceError{Code: domain.ErrGitHubNotFound, Message: message, Source: domain.ErrorSourceGitHub, Retryable: false}
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return domain.SourceError{Code: domain.ErrAdapterTimeout, Message: message, Source: domain.ErrorSourceAdapter, Retryable: true}
	default:
		return domain.SourceError{Code: domain.ErrAdapterInternal, Message: message, Source: domain.ErrorSourceAdapter, Retryable: statusCode >= 500}
	}
}
