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

// ImportResponse is the response from adapter-service after an import has been queued.
type ImportResponse struct {
	WorkspaceID string `json:"workspace_id"`
}

// ImportWorkspace calls POST /internal/workspaces/import on adapter-service.
// Returns the workspace ID after the import sync task has been queued.
func (c *Client) ImportWorkspace(ctx context.Context, req ImportRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal import request: %w", err)
	}

	resp, err := c.post(ctx, "/internal/workspaces/import", body)
	if err != nil {
		return "", err
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
	_, err := c.post(ctx, path, nil)
	return err
}

func (c *Client) post(ctx context.Context, path string, body []byte) ([]byte, error) {
	url := c.baseURL + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
	if err != nil {
		return nil, domain.NewAdapterError(domain.ErrAdapterInternal, fmt.Sprintf("build request: %v", err))
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, domain.NewAdapterError(domain.ErrAdapterTimeout, fmt.Sprintf("call %s: %v", path, err))
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewAdapterError(domain.ErrAdapterInternal, fmt.Sprintf("read response from %s: %v", path, err))
	}

	if resp.StatusCode >= 400 {
		return nil, domain.NewAdapterError(domain.ErrAdapterInternal,
			fmt.Sprintf("%s returned HTTP %d: %s", path, resp.StatusCode, string(data)))
	}
	return data, nil
}
