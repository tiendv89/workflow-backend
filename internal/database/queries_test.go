package database

import (
	"strings"
	"testing"
)

func TestActivityFilterClauseMatchesWorkflowNamesAndUUIDs(t *testing.T) {
	featureID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	taskID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	clause, args, nextArg, err := activityFilterClause(featureID, taskID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if nextArg != 4 {
		t.Fatalf("expected next arg position 4, got %d", nextArg)
	}
	if len(args) != 2 {
		t.Fatalf("unexpected args: %#v", args)
	}

	for _, want := range []string{
		"feature_id = $2",
		"task_id = $3",
	} {
		if !strings.Contains(clause, want) {
			t.Fatalf("expected activity filter clause to contain %q, got %q", want, clause)
		}
	}
	for _, forbidden := range []string{"feature_id::text", "task_id::text", "feature_name", "task_name"} {
		if strings.Contains(clause, forbidden) {
			t.Fatalf("activity filter must not match %q in ID filters, got %q", forbidden, clause)
		}
	}
}

func TestTaskIDOrderAscUsesNumericWorkflowOrder(t *testing.T) {
	clause := taskIDOrderAsc("t")

	for _, want := range []string{
		"regexp_replace(t.task_name, '^T([0-9]+)$', '\\1')",
		"::int ASC",
		"t.task_name ASC",
	} {
		if !strings.Contains(clause, want) {
			t.Fatalf("expected task order clause to contain %q, got %q", want, clause)
		}
	}
	if strings.Contains(clause, "regexp_replace(t.task_id") {
		t.Fatalf("task order must not call regexp_replace on UUID task_id, got %q", clause)
	}
}
