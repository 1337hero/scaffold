package db

import (
	"database/sql"
	"testing"
)

func TestSearchAllDoesNotRequireGoalsTable(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{
		ID:      "task-search",
		Title:   "Clean frontend routes",
		Context: sql.NullString{String: "remove stale domains UI", Valid: true},
	})
	if err := db.InsertNote(Note{
		ID:      "note-search",
		Title:   "Frontend cleanup note",
		Content: sql.NullString{String: "search should cover notes", Valid: true},
	}); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}

	results, err := db.SearchAll("frontend", nil, "", "")
	if err != nil {
		t.Fatalf("SearchAll: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want task and note: %+v", len(results), results)
	}
	for _, result := range results {
		if result.Type == "goal" {
			t.Fatalf("goal results should not be returned: %+v", results)
		}
	}

	results, err = db.SearchAll("frontend", nil, "goal", "")
	if err != nil {
		t.Fatalf("SearchAll goal filter: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("goal filter should return empty results, got %+v", results)
	}
}
