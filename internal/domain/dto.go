package domain

import (
	"encoding/json"
	"time"
)

// SourceState describes the freshness and error state of a workspace's data.
type SourceState struct {
	Stale        bool       `json:"stale"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	ErrorCode    string     `json:"error_code,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

// PullRequestRef is a reference to a pull request associated with a task.
type PullRequestRef struct {
	Label  string `json:"label"`
	URL    string `json:"url"`
	Status string `json:"status"`
	Repo   string `json:"repo"`
}

// ActivityEvent is a single normalized activity record.
type ActivityEvent struct {
	Action     string    `json:"action"`
	Scope      string    `json:"scope"`
	Actor      string    `json:"actor"`
	OccurredAt time.Time `json:"occurred_at"`
	Note       string    `json:"note,omitempty"`
	FeatureID  string    `json:"feature_id,omitempty"`
	TaskID     string    `json:"task_id,omitempty"`
}

// ActivityScope filters activity queries.
type ActivityScope struct {
	FeatureID string
	TaskID    string
}

// FeatureSearchQuery filters and controls workspace feature search results.
type FeatureSearchQuery struct {
	Title  string
	Status string
	Sort   string
	Page   int
	Limit  int
}

// TaskSearchQuery filters and controls task search results.
type TaskSearchQuery struct {
	TaskID string
	Title  string
	Status string
	Repo   string
	Sort   string
	Page   int
	Limit  int
}

// WorkspaceSummary is the list-view representation of a workspace.
type WorkspaceSummary struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	RepoURL     string      `json:"repo_url"`
	SourceState SourceState `json:"source_state"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// WorkspaceDetail is the full workspace view.
type WorkspaceDetail struct {
	WorkspaceSummary
	Features []FeatureSummary `json:"features"`
	Tasks    []TaskSummary    `json:"tasks"`
}

// FeatureSummary is the list-view representation of a feature.
type FeatureSummary struct {
	ID           string          `json:"id"`
	FeatureID    string          `json:"feature_id"`
	FeatureName  string          `json:"feature_name"`
	Title        string          `json:"title"`
	Status       string          `json:"status"`
	CurrentStage string          `json:"current_stage,omitempty"`
	Stages       json.RawMessage `json:"stages,omitempty"`
	UpdatedAt    time.Time       `json:"updated_at"`
	TaskCounts   TaskCounts      `json:"task_counts"`
}

// TaskCounts summarises task status distribution within a feature.
type TaskCounts struct {
	Total      int `json:"total"`
	Done       int `json:"done"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Ready      int `json:"ready"`
	Todo       int `json:"todo"`
}

// DocumentLink is a GitHub web URL for a feature document.
type DocumentLink struct {
	DocumentType string `json:"document_type"`
	SourcePath   string `json:"source_path"`
	URL          string `json:"url"`
}

// FeatureDetail is the full feature view.
type FeatureDetail struct {
	FeatureSummary
	WorkspaceID string          `json:"workspace_id"`
	Documents   []DocumentLink  `json:"documents"`
	Tasks       []TaskSummary   `json:"tasks"`
	Activity    []ActivityEvent `json:"activity"`
	SourceState SourceState     `json:"source_state"`
}

// TaskSummary is the list-view representation of a task.
type TaskSummary struct {
	ID            string          `json:"id"`
	TaskID        string          `json:"task_id"`
	TaskName      string          `json:"task_name"`
	FeatureID     string          `json:"feature_id"`
	FeatureName   string          `json:"feature_name"`
	Title         string          `json:"title"`
	Status        string          `json:"status"`
	Repo          string          `json:"repo,omitempty"`
	Branch        string          `json:"branch,omitempty"`
	NextAction    string          `json:"next_action,omitempty"`
	IsBlocked     bool            `json:"is_blocked"`
	BlockedReason string          `json:"blocked_reason,omitempty"`
	Pr            json.RawMessage `json:"pr"`
	WorkspacePr   json.RawMessage `json:"workspace_pr"`
}

// ExecutionContext holds execution metadata for a task.
type ExecutionContext struct {
	ActorType     string `json:"actor_type"`
	LastUpdatedBy string `json:"last_updated_by,omitempty"`
	LastUpdatedAt string `json:"last_updated_at,omitempty"`
}

// TaskDetail is the full task view.
type TaskDetail struct {
	ID            string           `json:"id"`
	TaskID        string           `json:"task_id"`
	TaskName      string           `json:"task_name"`
	FeatureID     string           `json:"feature_id"`
	FeatureName   string           `json:"feature_name"`
	Title         string           `json:"title"`
	Status        string           `json:"status"`
	Repo          string           `json:"repo,omitempty"`
	Branch        string           `json:"branch,omitempty"`
	NextAction    string           `json:"next_action,omitempty"`
	IsBlocked     bool             `json:"is_blocked"`
	BlockedReason string           `json:"blocked_reason,omitempty"`
	WorkspaceID   string           `json:"workspace_id"`
	DependsOn     []string         `json:"depends_on"`
	Execution     ExecutionContext `json:"execution"`
	PRRefs        []PullRequestRef `json:"pr_refs,omitempty"`
	Activity      []ActivityEvent  `json:"activity"`
}

// ImportInput is the request body for workspace import.
type ImportInput struct {
	RepoURL       string `json:"repo_url" binding:"required"`
	DefaultBranch string `json:"default_branch"`
	Name          string `json:"name"`
}
