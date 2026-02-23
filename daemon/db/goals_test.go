package db

import (
	"database/sql"
	"testing"
)

func TestInsertAndGetGoal(t *testing.T) {
	database := newTestDB(t)

	d, err := database.CreateDomain("Goal Domain", 5, "", "")
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}

	g := Goal{
		ID:       "goal-1",
		Title:    "Ship v1",
		DomainID: sql.NullInt64{Int64: int64(d.ID), Valid: true},
		Context:  sql.NullString{String: "get it out the door", Valid: true},
		DueDate:  sql.NullString{String: "2026-03-01", Valid: true},
		Type:     "binary",
		Notify:   1,
	}
	if err := database.InsertGoal(g); err != nil {
		t.Fatalf("insert goal: %v", err)
	}

	got, err := database.GetGoal("goal-1")
	if err != nil {
		t.Fatalf("get goal: %v", err)
	}
	if got == nil {
		t.Fatal("expected goal, got nil")
	}
	if got.Title != "Ship v1" {
		t.Fatalf("expected title 'Ship v1', got %q", got.Title)
	}
	if got.Status != "active" {
		t.Fatalf("expected status 'active', got %q", got.Status)
	}
	if got.CreatedAt == "" {
		t.Fatal("expected created_at to be auto-filled")
	}
	if !got.DomainID.Valid || got.DomainID.Int64 != int64(d.ID) {
		t.Fatalf("expected domain_id %d, got %+v", d.ID, got.DomainID)
	}
	if got.Notify != 1 {
		t.Fatalf("expected notify 1, got %d", got.Notify)
	}

	missing, err := database.GetGoal("nonexistent")
	if err != nil {
		t.Fatalf("get missing goal: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for missing goal")
	}
}

func TestListGoals(t *testing.T) {
	database := newTestDB(t)

	d1, err := database.CreateDomain("Domain A", 5, "", "")
	if err != nil {
		t.Fatalf("create domain a: %v", err)
	}
	d2, err := database.CreateDomain("Domain B", 3, "", "")
	if err != nil {
		t.Fatalf("create domain b: %v", err)
	}

	if err := database.InsertGoal(Goal{
		ID:       "g-a1",
		Title:    "Goal A1",
		DomainID: sql.NullInt64{Int64: int64(d1.ID), Valid: true},
	}); err != nil {
		t.Fatalf("insert goal a1: %v", err)
	}
	if err := database.InsertGoal(Goal{
		ID:       "g-b1",
		Title:    "Goal B1",
		DomainID: sql.NullInt64{Int64: int64(d2.ID), Valid: true},
	}); err != nil {
		t.Fatalf("insert goal b1: %v", err)
	}

	all, err := database.ListGoals(nil, "active")
	if err != nil {
		t.Fatalf("list all goals: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 goals, got %d", len(all))
	}

	filtered, err := database.ListGoals(&d1.ID, "")
	if err != nil {
		t.Fatalf("list domain goals: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 goal for domain A, got %d", len(filtered))
	}
	if filtered[0].ID != "g-a1" {
		t.Fatalf("expected goal g-a1, got %q", filtered[0].ID)
	}
}

func TestUpdateGoal(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertGoal(Goal{ID: "g-upd", Title: "Original"}); err != nil {
		t.Fatalf("insert goal: %v", err)
	}

	if err := database.UpdateGoal("g-upd", map[string]any{
		"title":  "Updated",
		"status": "completed",
	}); err != nil {
		t.Fatalf("update goal: %v", err)
	}

	got, err := database.GetGoal("g-upd")
	if err != nil {
		t.Fatalf("get goal: %v", err)
	}
	if got.Title != "Updated" {
		t.Fatalf("expected title 'Updated', got %q", got.Title)
	}
	if got.Status != "completed" {
		t.Fatalf("expected status 'completed', got %q", got.Status)
	}

	err = database.UpdateGoal("g-upd", map[string]any{"bad_key": "nope"})
	if err == nil {
		t.Fatal("expected error for unsupported key")
	}

	err = database.UpdateGoal("nonexistent", map[string]any{"title": "x"})
	if err == nil {
		t.Fatal("expected error for missing goal")
	}
}

func TestSoftDeleteGoal(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertGoal(Goal{ID: "g-del", Title: "To Delete"}); err != nil {
		t.Fatalf("insert goal: %v", err)
	}

	if err := database.SoftDeleteGoal("g-del"); err != nil {
		t.Fatalf("soft delete goal: %v", err)
	}

	got, err := database.GetGoal("g-del")
	if err != nil {
		t.Fatalf("get goal: %v", err)
	}
	if got.Status != "abandoned" {
		t.Fatalf("expected status 'abandoned', got %q", got.Status)
	}

	err = database.SoftDeleteGoal("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing goal")
	}
}

func TestGoalsByDomain(t *testing.T) {
	database := newTestDB(t)

	d1, err := database.CreateDomain("Goals D1", 5, "", "")
	if err != nil {
		t.Fatalf("create domain 1: %v", err)
	}
	d2, err := database.CreateDomain("Goals D2", 3, "", "")
	if err != nil {
		t.Fatalf("create domain 2: %v", err)
	}

	if err := database.InsertGoal(Goal{
		ID:       "g-dom1a",
		Title:    "D1 Goal A",
		DomainID: sql.NullInt64{Int64: int64(d1.ID), Valid: true},
	}); err != nil {
		t.Fatalf("insert goal: %v", err)
	}
	if err := database.InsertGoal(Goal{
		ID:       "g-dom1b",
		Title:    "D1 Goal B",
		DomainID: sql.NullInt64{Int64: int64(d1.ID), Valid: true},
	}); err != nil {
		t.Fatalf("insert goal: %v", err)
	}
	if err := database.InsertGoal(Goal{
		ID:       "g-dom2a",
		Title:    "D2 Goal A",
		DomainID: sql.NullInt64{Int64: int64(d2.ID), Valid: true},
	}); err != nil {
		t.Fatalf("insert goal: %v", err)
	}

	goals, err := database.GoalsByDomain(d1.ID)
	if err != nil {
		t.Fatalf("goals by domain: %v", err)
	}
	if len(goals) != 2 {
		t.Fatalf("expected 2 goals for domain 1, got %d", len(goals))
	}

	goals, err = database.GoalsByDomain(d2.ID)
	if err != nil {
		t.Fatalf("goals by domain 2: %v", err)
	}
	if len(goals) != 1 {
		t.Fatalf("expected 1 goal for domain 2, got %d", len(goals))
	}
}

func TestGoalsWithProgress(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertGoal(Goal{ID: "g-prog", Title: "Progress Goal"}); err != nil {
		t.Fatalf("insert goal: %v", err)
	}

	ts := now()
	for _, task := range []struct {
		id     string
		status string
	}{
		{"t-1", "done"},
		{"t-2", "done"},
		{"t-3", "pending"},
	} {
		_, err := database.conn.Exec(
			`INSERT INTO tasks (id, title, goal_id, status, created_at) VALUES (?, ?, ?, ?, ?)`,
			task.id, "Task "+task.id, "g-prog", task.status, ts,
		)
		if err != nil {
			t.Fatalf("insert task %s: %v", task.id, err)
		}
	}

	gps, err := database.GoalsWithProgress(nil)
	if err != nil {
		t.Fatalf("goals with progress: %v", err)
	}
	if len(gps) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(gps))
	}

	gp := gps[0]
	if gp.TotalTasks != 3 {
		t.Fatalf("expected 3 total tasks, got %d", gp.TotalTasks)
	}
	if gp.CompletedTasks != 2 {
		t.Fatalf("expected 2 completed tasks, got %d", gp.CompletedTasks)
	}
	want := 2.0 / 3.0
	if gp.Progress < want-0.01 || gp.Progress > want+0.01 {
		t.Fatalf("expected progress ~%.4f, got %.4f", want, gp.Progress)
	}
}
