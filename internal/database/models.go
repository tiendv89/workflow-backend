package database

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
)

// Row types mirror the PostgreSQL core tables (read-only).

type Workspace struct {
	ID               pgtype.UUID
	Slug             string
	Name             string
	ManagementRepoID string
	BranchPattern    *string
	CreatedAt        pgtype.Timestamptz
	UpdatedAt        pgtype.Timestamptz
}

type WorkspaceRepo struct {
	ID          pgtype.UUID
	WorkspaceID pgtype.UUID
	RepoID      string
	BaseBranch  *string
	CreatedAt   pgtype.Timestamptz
	UpdatedAt   pgtype.Timestamptz
}

type WorkspaceFeature struct {
	ID            pgtype.UUID
	WorkspaceID   pgtype.UUID
	FeatureID     string
	Title         string
	FeatureStatus *string
	CurrentStage  *string
	NextAction    *string
	Stages        json.RawMessage
	SourcePath    string
	SourceHash    *string
	CreatedAt     pgtype.Timestamptz
	UpdatedAt     pgtype.Timestamptz
}

type WorkspaceFeatureDocument struct {
	ID           pgtype.UUID
	WorkspaceID  pgtype.UUID
	FeatureID    pgtype.UUID
	FeatureName  string
	DocumentType string
	SourcePath   string
	URL          *string
	CreatedAt    pgtype.Timestamptz
	UpdatedAt    pgtype.Timestamptz
}

type WorkspaceTask struct {
	ID            pgtype.UUID
	WorkspaceID   pgtype.UUID
	FeatureID     pgtype.UUID
	FeatureName   string
	TaskID        string
	Title         string
	Repo          *string
	Status        *string
	DependsOn     json.RawMessage
	BlockedReason *string
	Branch        *string
	Execution     json.RawMessage
	Pr            json.RawMessage
	WorkspacePr   json.RawMessage
	SourcePath    string
	SourceHash    *string
	CreatedAt     pgtype.Timestamptz
	UpdatedAt     pgtype.Timestamptz
}

type WorkspaceActivityEvent struct {
	ID          pgtype.UUID
	WorkspaceID pgtype.UUID
	ScopeType   string
	FeatureID   *string
	TaskID      *string
	Action      *string
	Actor       *string
	OccurredAt  *string
	Note        *string
	Sequence    int32
	RawEvent    json.RawMessage
	CreatedAt   pgtype.Timestamptz
}

type WorkspaceGitHubSource struct {
	ID            pgtype.UUID
	WorkspaceID   pgtype.UUID
	RepoURL       string
	RepoOwner     string
	RepoName      string
	DefaultBranch *string
	CreatedAt     pgtype.Timestamptz
	UpdatedAt     pgtype.Timestamptz
}

type WorkspaceSyncRun struct {
	ID           pgtype.UUID
	WorkspaceID  pgtype.UUID
	Trigger      string
	Branch       *string
	FeatureID    *string
	TaskID       *string
	Mode         string
	Status       string
	CommitSha    *string
	ChangedPaths json.RawMessage
	StartedAt    pgtype.Timestamptz
	FinishedAt   pgtype.Timestamptz
	ErrorCode    *string
	ErrorMessage *string
	Metadata     json.RawMessage
}
