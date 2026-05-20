package database

import (
	"os"
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

func TestListActivityEventsUsesIndependentActivityFilterClause(t *testing.T) {
	source, err := os.ReadFile("queries.go")
	if err != nil {
		t.Fatalf("read queries.go: %v", err)
	}

	body := string(source)
	start := strings.Index(body, "func (r *Reader) ListActivityEvents")
	if start < 0 {
		t.Fatal("ListActivityEvents not found")
	}
	end := strings.Index(body[start:], "// UUIDString converts")
	if end < 0 {
		t.Fatal("ListActivityEvents end marker not found")
	}
	listActivityEvents := body[start : start+end]

	if !strings.Contains(listActivityEvents, "activityFilterClause(featureID, taskID, 2)") {
		t.Fatalf("ListActivityEvents must build feature and task filters independently with activityFilterClause")
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

func TestQueryLayerDoesNotResolveReferencesByNames(t *testing.T) {
	// UUID validation is implicit: invalid UUIDs return ErrNotFound directly via parseUUID;
	// missing rows return ErrNotFound via pgx.ErrNoRows in the main query.
	// The query layer must never resolve by mutable name columns.
	forbidden := []string{
		"WHERE workspace_id = $1 AND feature_name = $2",
		"WHERE workspace_id = $1 AND feature_id = $2 AND task_name = $3",
		"SELECT id FROM workspace_features WHERE workspace_id = $1 AND feature_id = $2",
		"SELECT id FROM workspace_tasks WHERE workspace_id = $1 AND feature_id = $2 AND task_id = $3",
		// ensure the old existence-check round-trips are gone
		"SELECT feature_id FROM workspace_features WHERE workspace_id = $1 AND feature_id = $2",
		"SELECT task_id FROM workspace_tasks WHERE workspace_id = $1 AND feature_id = $2 AND task_id = $3",
	}

	source := queriesSourceForContractTest()
	for _, fragment := range forbidden {
		if strings.Contains(source, fragment) {
			t.Fatalf("query layer must resolve references by public UUID columns only; found %q", fragment)
		}
	}
}

func queriesSourceForContractTest() string {
	data, err := os.ReadFile("queries.go")
	if err != nil {
		panic(err)
	}
	return string(data)
}
