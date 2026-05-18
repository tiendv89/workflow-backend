package integration_test

import (
	"testing"
	"time"

	"github.com/tiendv89/workflow-backend/internal/domain"
)

// TestDeriveSourceState covers all staleness derivation paths.
func TestDeriveSourceState_NilRun_IsStale(t *testing.T) {
	ss := domain.DeriveSourceState(nil, 30*time.Minute)
	if !ss.Stale {
		t.Error("expected stale=true when no sync run exists")
	}
}

func TestDeriveSourceState_SuccessRecentRun_NotStale(t *testing.T) {
	finished := time.Now().Add(-5 * time.Minute)
	run := &domain.SyncRun{
		Status:     domain.SyncStatusSuccess,
		FinishedAt: &finished,
	}
	ss := domain.DeriveSourceState(run, 30*time.Minute)
	if ss.Stale {
		t.Error("expected stale=false for a recent successful sync")
	}
	if ss.LastSyncedAt == nil {
		t.Error("expected last_synced_at to be populated")
	}
}

func TestDeriveSourceState_SuccessOldRun_IsStale(t *testing.T) {
	finished := time.Now().Add(-60 * time.Minute)
	run := &domain.SyncRun{
		Status:     domain.SyncStatusSuccess,
		FinishedAt: &finished,
	}
	ss := domain.DeriveSourceState(run, 30*time.Minute)
	if !ss.Stale {
		t.Error("expected stale=true for a sync that finished 60 minutes ago with a 30-minute threshold")
	}
}

func TestDeriveSourceState_FailedRun_IsStale(t *testing.T) {
	finished := time.Now().Add(-1 * time.Minute)
	run := &domain.SyncRun{
		Status:     domain.SyncStatusFailed,
		FinishedAt: &finished,
		ErrorCode:  "GITHUB_RATE_LIMIT",
		ErrorMsg:   "rate limit exceeded",
	}
	ss := domain.DeriveSourceState(run, 30*time.Minute)
	if !ss.Stale {
		t.Error("expected stale=true for a failed sync")
	}
	if ss.ErrorCode != "GITHUB_RATE_LIMIT" {
		t.Errorf("expected error_code 'GITHUB_RATE_LIMIT', got %q", ss.ErrorCode)
	}
	if ss.ErrorMessage != "" {
		t.Error("expected empty error_message for failed sync")
	}
}

func TestDeriveSourceState_RunningRun_NotStale(t *testing.T) {
	started := time.Now().Add(-30 * time.Second)
	run := &domain.SyncRun{
		Status:    domain.SyncStatusRunning,
		StartedAt: started,
	}
	ss := domain.DeriveSourceState(run, 30*time.Minute)
	if ss.Stale {
		t.Error("expected stale=false while sync is running")
	}
}

func TestDeriveSourceState_PartialRun_NotStale_WhenRecent(t *testing.T) {
	finished := time.Now().Add(-1 * time.Minute)
	run := &domain.SyncRun{
		Status:     domain.SyncStatusPartial,
		FinishedAt: &finished,
	}
	ss := domain.DeriveSourceState(run, 30*time.Minute)
	if ss.Stale {
		t.Error("expected stale=false for a recent partial sync")
	}
}

func TestDeriveSourceState_UnknownStatus_IsStale(t *testing.T) {
	run := &domain.SyncRun{
		Status: domain.SyncStatus("unknown_status"),
	}
	ss := domain.DeriveSourceState(run, 30*time.Minute)
	if !ss.Stale {
		t.Error("expected stale=true for unknown sync status")
	}
}
