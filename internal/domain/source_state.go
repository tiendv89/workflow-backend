package domain

import "time"

// SyncStatus reflects the outcome of a workspace sync run.
type SyncStatus string

const (
	SyncStatusRunning SyncStatus = "running"
	SyncStatusSuccess SyncStatus = "success"
	SyncStatusPartial SyncStatus = "partial"
	SyncStatusFailed  SyncStatus = "failed"
	SyncStatusSkipped SyncStatus = "skipped"
)

// SyncRun is a record of a single sync attempt.
type SyncRun struct {
	ID          string
	WorkspaceID string
	Trigger     string
	Mode        string
	Status      SyncStatus
	CommitSHA   string
	StartedAt   time.Time
	FinishedAt  *time.Time
	ErrorCode   string
	ErrorMsg    string
}

// DefaultStaleThreshold is the default duration after which a successful sync is stale.
const DefaultStaleThreshold = 30 * time.Minute

// DeriveSourceState computes a SourceState from the latest SyncRun.
// When latestRun is nil, the workspace has never been synced — data is stale.
func DeriveSourceState(latestRun *SyncRun, staleThreshold time.Duration) SourceState {
	if latestRun == nil {
		return SourceState{Stale: true}
	}

	switch latestRun.Status {
	case SyncStatusFailed:
		return SourceState{
			Stale:        true,
			LastSyncedAt: latestRun.FinishedAt,
			ErrorCode:    latestRun.ErrorCode,
		}
	case SyncStatusSuccess, SyncStatusPartial:
		if latestRun.FinishedAt == nil {
			return SourceState{Stale: true}
		}
		age := time.Since(*latestRun.FinishedAt)
		return SourceState{
			Stale:        age > staleThreshold,
			LastSyncedAt: latestRun.FinishedAt,
		}
	case SyncStatusRunning:
		return SourceState{
			Stale:        false,
			LastSyncedAt: &latestRun.StartedAt,
		}
	default:
		return SourceState{Stale: true}
	}
}
