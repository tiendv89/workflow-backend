package database

import (
	"io/fs"
	"testing"
)

func TestMigrationFSContainsExpectedFiles(t *testing.T) {
	expected := []string{
		"00001_workspaces.sql",
		"00002_workspace_repos.sql",
		"00003_workspace_features.sql",
		"00004_workspace_feature_documents.sql",
		"00005_workspace_tasks.sql",
		"00006_workspace_activity_events.sql",
		"00007_workspace_github_sources.sql",
		"00008_workspace_sync_runs.sql",
		"00009_use_uuid_feature_ids_for_tasks_documents_and_activity_events.sql",
		"00010_feature_and_task_names.sql",
		"00011_workspace_sync_runs_uuid_refs.sql",
	}

	for _, name := range expected {
		f, err := MigrationFS.Open(name)
		if err != nil {
			t.Errorf("expected migration file %q to be embedded: %v", name, err)
			continue
		}
		stat, err := f.Stat()
		_ = f.Close()
		if err != nil {
			t.Errorf("stat %q: %v", name, err)
			continue
		}
		if stat.Size() == 0 {
			t.Errorf("migration file %q is empty", name)
		}
	}
}

func TestMigrationFSHasExactlyElevenSQLFiles(t *testing.T) {
	var count int
	err := fs.WalkDir(MigrationFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk MigrationFS: %v", err)
	}
	if count != 11 {
		t.Errorf("expected 11 migration files, got %d", count)
	}
}

func TestRunMigrationsReturnsErrorOnInvalidDatabaseURL(t *testing.T) {
	ctx := t.Context()
	err := RunMigrations(ctx, "not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for invalid database URL, got nil")
	}
}

func TestMigrateUpNReturnsErrorOnInvalidDatabaseURL(t *testing.T) {
	ctx := t.Context()
	for _, n := range []int{0, 1, 3} {
		if err := MigrateUpN(ctx, "not-a-valid-url", n); err == nil {
			t.Errorf("MigrateUpN(n=%d): expected error for invalid URL, got nil", n)
		}
	}
}

func TestMigrateDownNReturnsErrorOnInvalidDatabaseURL(t *testing.T) {
	ctx := t.Context()
	if err := MigrateDownN(ctx, "not-a-valid-url", 1); err == nil {
		t.Fatal("expected error for invalid database URL, got nil")
	}
}

func TestMigrateDownNZeroStepsIsNoOp(t *testing.T) {
	ctx := t.Context()
	// n=0: no iterations, no db call — returns nil immediately
	if err := MigrateDownN(ctx, "not-a-valid-url", 0); err != nil {
		t.Fatalf("MigrateDownN with 0 steps should be a no-op, got error: %v", err)
	}
}
