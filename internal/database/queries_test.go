package database

import (
	"strings"
	"testing"
)

func TestActivityFilterClauseMatchesWorkflowNamesAndUUIDs(t *testing.T) {
	clause, args, nextArg := activityFilterClause("workspace-data-backend", "T1", 2)

	if nextArg != 4 {
		t.Fatalf("expected next arg position 4, got %d", nextArg)
	}
	if len(args) != 2 || args[0] != "workspace-data-backend" || args[1] != "T1" {
		t.Fatalf("unexpected args: %#v", args)
	}

	for _, want := range []string{
		"feature_id::text = $2",
		"feature_name = $2",
		"task_id::text = $3",
		"task_name = $3",
	} {
		if !strings.Contains(clause, want) {
			t.Fatalf("expected activity filter clause to contain %q, got %q", want, clause)
		}
	}
}

func TestTaskIDOrderAscUsesNumericWorkflowOrder(t *testing.T) {
	clause := taskIDOrderAsc("t")

	for _, want := range []string{
		"regexp_replace(t.task_id, '^T([0-9]+)$', '\\1')",
		"::int ASC",
		"t.task_id ASC",
	} {
		if !strings.Contains(clause, want) {
			t.Fatalf("expected task order clause to contain %q, got %q", want, clause)
		}
	}
}
