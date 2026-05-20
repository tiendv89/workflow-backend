package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tiendv89/workflow-backend/internal/domain"
)

func TestImportWorkspace_DecodesSourceErrorResponse(t *testing.T) {
	client := New("https://adapter.local")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/internal/workspaces/import" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonResponse(http.StatusUnauthorized, domain.SourceError{
			Code:      domain.ErrGitHubUnauthorized,
			Message:   "GitHub token is missing or invalid",
			Source:    domain.ErrorSourceGitHub,
			Retryable: false,
		}), nil
	})
	_, err := client.ImportWorkspace(context.Background(), ImportRequest{RepoURL: "https://github.com/org/repo"})
	if err == nil {
		t.Fatal("expected error")
	}

	se, ok := err.(domain.SourceError)
	if !ok {
		t.Fatalf("expected SourceError, got %T", err)
	}
	if se.Code != domain.ErrGitHubUnauthorized {
		t.Errorf("expected %s, got %s", domain.ErrGitHubUnauthorized, se.Code)
	}
	if se.Source != domain.ErrorSourceGitHub {
		t.Errorf("expected source %s, got %s", domain.ErrorSourceGitHub, se.Source)
	}
	if se.Retryable {
		t.Error("expected retryable=false")
	}
}

func TestImportWorkspace_DecodesAPIErrorResponse(t *testing.T) {
	client := New("https://adapter.local")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusTooManyRequests, domain.APIError{
			Code:      domain.ErrGitHubRateLimit,
			Message:   "GitHub API rate limit exceeded",
			Source:    domain.ErrorSourceGitHub,
			Retryable: true,
		}), nil
	})
	_, err := client.ImportWorkspace(context.Background(), ImportRequest{RepoURL: "https://github.com/org/repo"})
	if err == nil {
		t.Fatal("expected error")
	}

	se, ok := err.(domain.SourceError)
	if !ok {
		t.Fatalf("expected SourceError, got %T", err)
	}
	if se.Code != domain.ErrGitHubRateLimit {
		t.Errorf("expected %s, got %s", domain.ErrGitHubRateLimit, se.Code)
	}
	if se.Source != domain.ErrorSourceGitHub {
		t.Errorf("expected source %s, got %s", domain.ErrorSourceGitHub, se.Source)
	}
	if !se.Retryable {
		t.Error("expected retryable=true")
	}
}

func TestImportWorkspace_MapsStatusWhenErrorBodyIsNotStructured(t *testing.T) {
	client := New("https://adapter.local")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return textResponse(http.StatusUnauthorized, "unauthorized"), nil
	})
	_, err := client.ImportWorkspace(context.Background(), ImportRequest{RepoURL: "https://github.com/org/repo"})
	if err == nil {
		t.Fatal("expected error")
	}

	se, ok := err.(domain.SourceError)
	if !ok {
		t.Fatalf("expected SourceError, got %T", err)
	}
	if se.Code != domain.ErrGitHubUnauthorized {
		t.Errorf("expected %s, got %s", domain.ErrGitHubUnauthorized, se.Code)
	}
	if se.Source != domain.ErrorSourceGitHub {
		t.Errorf("expected source %s, got %s", domain.ErrorSourceGitHub, se.Source)
	}
	if se.Retryable {
		t.Error("expected retryable=false")
	}
}

func TestImportWorkspace_RejectsAcceptedOnlyResponse(t *testing.T) {
	client := New("https://adapter.local")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/internal/workspaces/import" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonResponse(http.StatusAccepted, map[string]string{
			"workspace_id": "11111111-1111-1111-1111-111111111111",
			"status":       "accepted",
		}), nil
	})

	_, err := client.ImportWorkspace(context.Background(), ImportRequest{RepoURL: "https://github.com/org/repo"})
	if err == nil {
		t.Fatal("expected error for async accepted-only import")
	}

	se, ok := err.(domain.SourceError)
	if !ok {
		t.Fatalf("expected SourceError, got %T", err)
	}
	if se.Code != domain.ErrAdapterInternal {
		t.Errorf("expected %s, got %s", domain.ErrAdapterInternal, se.Code)
	}
}

func TestSyncWorkspace_RejectsAcceptedOnlyResponse(t *testing.T) {
	client := New("https://adapter.local")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/internal/workspaces/11111111-1111-1111-1111-111111111111/sync" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonResponse(http.StatusAccepted, map[string]string{"status": "accepted"}), nil
	})
	err := client.SyncWorkspace(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err == nil {
		t.Fatal("expected error for async accepted-only sync")
	}

	se, ok := err.(domain.SourceError)
	if !ok {
		t.Fatalf("expected SourceError, got %T", err)
	}
	if se.Code != domain.ErrAdapterInternal {
		t.Errorf("expected %s, got %s", domain.ErrAdapterInternal, se.Code)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse[T any](status int, body T) *http.Response {
	data, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

func textResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
