package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tiendv89/workflow-backend/internal/domain"
)

func TestImportWorkspace_DecodesSourceErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/workspaces/import" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(domain.SourceError{
			Code:      domain.ErrGitHubUnauthorized,
			Message:   "GitHub token is missing or invalid",
			Source:    domain.ErrorSourceGitHub,
			Retryable: false,
		})
	}))
	defer srv.Close()

	client := New(srv.URL)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(domain.APIError{
			Code:      domain.ErrGitHubRateLimit,
			Message:   "GitHub API rate limit exceeded",
			Source:    domain.ErrorSourceGitHub,
			Retryable: true,
		})
	}))
	defer srv.Close()

	client := New(srv.URL)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer srv.Close()

	client := New(srv.URL)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/workspaces/import" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"workspace_id": "11111111-1111-1111-1111-111111111111",
			"status":       "accepted",
		})
	}))
	defer srv.Close()

	client := New(srv.URL)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/workspaces/11111111-1111-1111-1111-111111111111/sync" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	}))
	defer srv.Close()

	client := New(srv.URL)
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
