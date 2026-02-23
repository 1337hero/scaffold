package db

import (
	"database/sql"
	"testing"
	"time"
)

func TestInsertAndGetTask(t *testing.T) {
	database := newTestDB(t)

	task := Task{
		ID:    "task-1",
		Title: "Buy groceries",
	}
	if err := database.InsertTask(task); err != nil {
		t.Fatalf("insert task: %v", err)
	}

	got, err := database.GetTask("task-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got == nil {
		t.Fatal("expected task, got nil")
	}
	if got.Title != "Buy groceries" {
		t.Fatalf("expected title 'Buy groceries', got %q", got.Title)
	}
	if got.Status != "pending" {
		t.Fatalf("expected default status 'pending', got %q", got.Status)
	}
	if got.Priority != "normal" {
		t.Fatalf("expected default priority 'normal', got %q", got.Priority)
	}
	if got.CreatedAt == "" {
		t.Fatal("expected auto-filled CreatedAt")
	}

	missing, err := database.GetTask("nonexistent")
	if err != nil {
		t.Fatalf("get missing task: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for missing task")
	}
}

func TestInsertTaskAutoID(t *testing.T) {
	database := newTestDB(t)

	task := Task{Title: "Auto ID task"}
	if err := database.InsertTask(task); err != nil {
		t.Fatalf("insert task: %v", err)
	}
}

func TestListTasks(t *testing.T) {
	database := newTestDB(t)

	d, err := database.CreateDomain("Test Domain", 3, "", "")
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}
	domainID := sql.NullInt64{Int64: int64(d.ID), Valid: true}

	if err := database.InsertTask(Task{
		ID:       "task-list-1",
		Title:    "Domain task",
		DomainID: domainID,
		Status:   "pending",
		DueDate:  sql.NullString{String: today(), Valid: true},
	}); err != nil {
		t.Fatalf("insert task 1: %v", err)
	}

	if err := database.InsertTask(Task{
		ID:     "task-list-2",
		Title:  "No domain task",
		Status: "pending",
	}); err != nil {
		t.Fatalf("insert task 2: %v", err)
	}

	if err := database.InsertTask(Task{
		ID:       "task-list-3",
		Title:    "Done task",
		DomainID: domainID,
		Status:   "done",
	}); err != nil {
		t.Fatalf("insert task 3: %v", err)
	}

	all, err := database.ListTasks(nil, nil, "pending", "")
	if err != nil {
		t.Fatalf("list all pending: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 pending tasks, got %d", len(all))
	}

	did := d.ID
	byDomain, err := database.ListTasks(&did, nil, "pending", "")
	if err != nil {
		t.Fatalf("list by domain: %v", err)
	}
	if len(byDomain) != 1 {
		t.Fatalf("expected 1 domain task, got %d", len(byDomain))
	}

	done, err := database.ListTasks(nil, nil, "done", "")
	if err != nil {
		t.Fatalf("list done: %v", err)
	}
	if len(done) != 1 {
		t.Fatalf("expected 1 done task, got %d", len(done))
	}
}

func TestListTasksDueFilters(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertTask(Task{
		ID:      "task-due-today",
		Title:   "Due today",
		DueDate: sql.NullString{String: today(), Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if err := database.InsertTask(Task{
		ID:      "task-overdue",
		Title:   "Overdue",
		DueDate: sql.NullString{String: yesterday, Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := database.InsertTask(Task{
		ID:      "task-due-tomorrow",
		Title:   "Due tomorrow",
		DueDate: sql.NullString{String: tomorrow(), Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	todayTasks, err := database.ListTasks(nil, nil, "pending", "today")
	if err != nil {
		t.Fatalf("list today: %v", err)
	}
	if len(todayTasks) != 2 {
		t.Fatalf("expected 2 tasks due today (incl overdue), got %d", len(todayTasks))
	}

	tomorrowTasks, err := database.ListTasks(nil, nil, "pending", "tomorrow")
	if err != nil {
		t.Fatalf("list tomorrow: %v", err)
	}
	if len(tomorrowTasks) != 1 {
		t.Fatalf("expected 1 task due tomorrow, got %d", len(tomorrowTasks))
	}
}

func TestCompleteTask(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertTask(Task{
		ID:    "task-complete",
		Title: "Complete me",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := database.CompleteTask("task-complete"); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	got, err := database.GetTask("task-complete")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "done" {
		t.Fatalf("expected status 'done', got %q", got.Status)
	}
	if !got.CompletedAt.Valid {
		t.Fatal("expected completed_at to be set")
	}

	var count int
	if err := database.conn.QueryRow(
		`SELECT COUNT(*) FROM task_completions WHERE task_id = ?`, "task-complete",
	).Scan(&count); err != nil {
		t.Fatalf("count completions: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 completion logged, got %d", count)
	}
}

func TestCompleteTaskWithGoal(t *testing.T) {
	database := newTestDB(t)

	goalID := "goal-for-complete"
	_, err := database.conn.Exec(
		`INSERT INTO goals (id, title, created_at) VALUES (?, ?, ?)`,
		goalID, "Test Goal", now(),
	)
	if err != nil {
		t.Fatalf("insert goal: %v", err)
	}

	if err := database.InsertTask(Task{
		ID:     "task-with-goal",
		Title:  "Goal task",
		GoalID: sql.NullString{String: goalID, Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := database.CompleteTask("task-with-goal"); err != nil {
		t.Fatalf("complete: %v", err)
	}

	count, err := database.CompletionCount(goalID)
	if err != nil {
		t.Fatalf("completion count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 completion for goal, got %d", count)
	}
}

func TestCompleteRecurringTask(t *testing.T) {
	database := newTestDB(t)

	dueDate := today()
	if err := database.InsertTask(Task{
		ID:        "task-recurring",
		Title:     "Daily standup",
		Recurring: sql.NullString{String: "daily", Valid: true},
		DueDate:   sql.NullString{String: dueDate, Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := database.CompleteTask("task-recurring"); err != nil {
		t.Fatalf("complete recurring: %v", err)
	}

	got, err := database.GetTask("task-recurring")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "pending" {
		t.Fatalf("expected recurring task reset to 'pending', got %q", got.Status)
	}
	if got.CompletedAt.Valid {
		t.Fatal("expected completed_at to be NULL after recurring reset")
	}
	if !got.DueDate.Valid {
		t.Fatal("expected due_date to be set after recurring reset")
	}

	expectedNext := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	parsed, err := time.Parse("2006-01-02", dueDate)
	if err == nil {
		expectedNext = parsed.AddDate(0, 0, 1).Format("2006-01-02")
	}
	if got.DueDate.String != expectedNext {
		t.Fatalf("expected due_date bumped to %s, got %s", expectedNext, got.DueDate.String)
	}
}

func TestCompleteRecurringWeekly(t *testing.T) {
	database := newTestDB(t)

	dueDate := today()
	if err := database.InsertTask(Task{
		ID:        "task-weekly",
		Title:     "Weekly review",
		Recurring: sql.NullString{String: "weekly", Valid: true},
		DueDate:   sql.NullString{String: dueDate, Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := database.CompleteTask("task-weekly"); err != nil {
		t.Fatalf("complete: %v", err)
	}

	got, err := database.GetTask("task-weekly")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	parsed, _ := time.Parse("2006-01-02", dueDate)
	expectedNext := parsed.AddDate(0, 0, 7).Format("2006-01-02")
	if got.DueDate.String != expectedNext {
		t.Fatalf("expected weekly bump to %s, got %s", expectedNext, got.DueDate.String)
	}
}

func TestTodaysTasks(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertTask(Task{
		ID:      "task-today-1",
		Title:   "Due today",
		DueDate: sql.NullString{String: today(), Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if err := database.InsertTask(Task{
		ID:      "task-overdue-today",
		Title:   "Overdue",
		DueDate: sql.NullString{String: yesterday, Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := database.InsertTask(Task{
		ID:        "task-recurring-today",
		Title:     "Recurring no due",
		Recurring: sql.NullString{String: "daily", Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	nextWeek := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	if err := database.InsertTask(Task{
		ID:      "task-future",
		Title:   "Future task",
		DueDate: sql.NullString{String: nextWeek, Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := database.InsertTask(Task{
		ID:     "task-done-today",
		Title:  "Already done",
		Status: "done",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	tasks, err := database.TodaysTasks()
	if err != nil {
		t.Fatalf("todays tasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 todays tasks, got %d", len(tasks))
	}

	ids := make(map[string]bool)
	for _, task := range tasks {
		ids[task.ID] = true
	}
	if !ids["task-today-1"] {
		t.Fatal("expected task-today-1 in today's tasks")
	}
	if !ids["task-overdue-today"] {
		t.Fatal("expected task-overdue-today in today's tasks")
	}
	if !ids["task-recurring-today"] {
		t.Fatal("expected task-recurring-today in today's tasks")
	}
}

func TestTodaysTasksPriorityOrder(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertTask(Task{
		ID:       "task-low",
		Title:    "Low",
		Priority: "low",
		DueDate:  sql.NullString{String: today(), Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := database.InsertTask(Task{
		ID:       "task-high",
		Title:    "High",
		Priority: "high",
		DueDate:  sql.NullString{String: today(), Valid: true},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	tasks, err := database.TodaysTasks()
	if err != nil {
		t.Fatalf("todays tasks: %v", err)
	}
	if len(tasks) < 2 {
		t.Fatalf("expected at least 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "task-high" {
		t.Fatalf("expected high priority first, got %q", tasks[0].ID)
	}
}

func TestTasksByGoal(t *testing.T) {
	database := newTestDB(t)

	goalID := "goal-tasks-by"
	_, err := database.conn.Exec(
		`INSERT INTO goals (id, title, created_at) VALUES (?, ?, ?)`,
		goalID, "Test Goal", now(),
	)
	if err != nil {
		t.Fatalf("insert goal: %v", err)
	}

	if err := database.InsertTask(Task{
		ID:       "task-goal-1",
		Title:    "Goal task 1",
		GoalID:   sql.NullString{String: goalID, Valid: true},
		Position: 1,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := database.InsertTask(Task{
		ID:       "task-goal-2",
		Title:    "Goal task 2",
		GoalID:   sql.NullString{String: goalID, Valid: true},
		Position: 0,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := database.InsertTask(Task{
		ID:    "task-no-goal",
		Title: "No goal task",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	tasks, err := database.TasksByGoal(goalID)
	if err != nil {
		t.Fatalf("tasks by goal: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 goal tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "task-goal-2" {
		t.Fatalf("expected position 0 first, got %q", tasks[0].ID)
	}
}

func TestUpdateTask(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertTask(Task{
		ID:    "task-update",
		Title: "Original",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := database.UpdateTask("task-update", map[string]any{
		"title":    "Updated",
		"priority": "high",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := database.GetTask("task-update")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "Updated" {
		t.Fatalf("expected title 'Updated', got %q", got.Title)
	}
	if got.Priority != "high" {
		t.Fatalf("expected priority 'high', got %q", got.Priority)
	}

	err = database.UpdateTask("task-update", map[string]any{"bogus": "field"})
	if err == nil {
		t.Fatal("expected error for unsupported field")
	}
}

func TestReorderTask(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertTask(Task{
		ID:    "task-reorder",
		Title: "Reorder me",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := database.ReorderTask("task-reorder", 5); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	got, err := database.GetTask("task-reorder")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Position != 5 {
		t.Fatalf("expected position 5, got %d", got.Position)
	}
}

func TestSoftDeleteTask(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertTask(Task{
		ID:    "task-delete",
		Title: "Delete me",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := database.SoftDeleteTask("task-delete"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	got, err := database.GetTask("task-delete")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "deleted" {
		t.Fatalf("expected status 'deleted', got %q", got.Status)
	}

	pending, err := database.ListTasks(nil, nil, "pending", "")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	for _, task := range pending {
		if task.ID == "task-delete" {
			t.Fatal("soft-deleted task should not appear in pending list")
		}
	}
}

func TestCompletionsSince(t *testing.T) {
	database := newTestDB(t)

	goalID := "goal-completions-since"
	_, err := database.conn.Exec(
		`INSERT INTO goals (id, title, created_at) VALUES (?, ?, ?)`,
		goalID, "Test Goal", now(),
	)
	if err != nil {
		t.Fatalf("insert goal: %v", err)
	}

	if err := database.InsertTask(Task{
		ID:     "task-for-completion",
		Title:  "Completable task",
		GoalID: sql.NullString{String: goalID, Valid: true},
	}); err != nil {
		t.Fatalf("insert task: %v", err)
	}

	if err := database.LogCompletion("task-for-completion", goalID); err != nil {
		t.Fatalf("log completion: %v", err)
	}

	since := time.Now().Add(-1 * time.Hour)
	completions, err := database.CompletionsSince(goalID, since)
	if err != nil {
		t.Fatalf("completions since: %v", err)
	}
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}

	old := time.Now().Add(1 * time.Hour)
	completions, err = database.CompletionsSince(goalID, old)
	if err != nil {
		t.Fatalf("completions since future: %v", err)
	}
	if len(completions) != 0 {
		t.Fatalf("expected 0 completions, got %d", len(completions))
	}
}
