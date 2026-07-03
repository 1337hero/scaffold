package db

import (
	"database/sql"
	"testing"
)

// PRD 07: new task/note/domain columns and filters.

func strPtr(s string) *string { return &s }

func TestInsertTask_WithProjectID(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertProject(Project{ID: "p1", Name: "Fence"}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	task := Task{ID: "t1", Title: "buy posts", ProjectID: sql.NullString{String: "p1", Valid: true}}
	if err := db.InsertTask(task); err != nil {
		t.Fatalf("InsertTask: %v", err)
	}

	got, err := db.GetTask("t1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.ProjectID.String != "p1" {
		t.Fatalf("project_id=%q, want p1", got.ProjectID.String)
	}

	// Insert with project bumps the project's last_activity_at.
	proj, _ := db.GetProject("p1")
	if !proj.LastActivityAt.Valid {
		t.Fatalf("last_activity_at not bumped on task insert")
	}
}

func TestInsertTask_WithReminderAt(t *testing.T) {
	db := openTestDB(t)
	task := Task{ID: "t1", Title: "call bank", ReminderAt: sql.NullString{String: "2026-07-04T09:00:00Z", Valid: true}}
	if err := db.InsertTask(task); err != nil {
		t.Fatalf("InsertTask: %v", err)
	}

	got, _ := db.GetTask("t1")
	if got.ReminderAt.String != "2026-07-04T09:00:00Z" {
		t.Fatalf("reminder_at=%q", got.ReminderAt.String)
	}

	// The reminder_at filter returns tasks due by the given timestamp.
	due, err := db.ListTasks(TaskFilters{ReminderAt: strPtr("2026-07-04T09:00:00Z")})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("got %d due reminders, want 1", len(due))
	}
	notYet, _ := db.ListTasks(TaskFilters{ReminderAt: strPtr("2026-07-04T08:59:59Z")})
	if len(notYet) != 0 {
		t.Fatalf("got %d due reminders before the time, want 0", len(notYet))
	}
}

func TestInsertTask_WithTop3Position(t *testing.T) {
	db := openTestDB(t)
	task := Task{ID: "t1", Title: "big rock", Top3Position: sql.NullInt64{Int64: 1, Valid: true}}
	if err := db.InsertTask(task); err != nil {
		t.Fatalf("InsertTask: %v", err)
	}

	got, _ := db.GetTask("t1")
	if got.Top3Position.Int64 != 1 {
		t.Fatalf("top3_position=%v, want 1", got.Top3Position)
	}

	// Invalid position rejected.
	bad := Task{Title: "x", Top3Position: sql.NullInt64{Int64: 4, Valid: true}}
	if err := db.InsertTask(bad); err == nil {
		t.Fatalf("expected error for top3_position 4")
	}
}

func TestListTasks_FilterByProjectID(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertProject(Project{ID: "p1", Name: "Fence"}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	mustInsertTask(t, db, Task{ID: "t1", Title: "in project", ProjectID: sql.NullString{String: "p1", Valid: true}})
	mustInsertTask(t, db, Task{ID: "t2", Title: "loose task"})

	tasks, err := db.ListTasks(TaskFilters{ProjectID: strPtr("p1")})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "t1" {
		t.Fatalf("got %+v, want only t1", tasks)
	}
}

func TestListTasks_FilterBySurface(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "life task"})
	mustInsertTask(t, db, Task{ID: "t2", Title: "work task", Surface: "business"})

	tasks, err := db.ListTasks(TaskFilters{Surface: strPtr("business")})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "t2" {
		t.Fatalf("got %+v, want only t2", tasks)
	}

	// Invalid surface rejected on insert.
	if err := db.InsertTask(Task{Title: "x", Surface: "bogus"}); err == nil {
		t.Fatalf("expected error for invalid surface")
	}
}

func TestListTasks_FilterByTop3(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "starred", Top3Position: sql.NullInt64{Int64: 2, Valid: true}})
	mustInsertTask(t, db, Task{ID: "t2", Title: "not starred"})

	yes := true
	tasks, err := db.ListTasks(TaskFilters{Top3: &yes})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "t1" {
		t.Fatalf("got %+v, want only t1", tasks)
	}
}

func TestUpdateTask_ClearTop3Position(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "starred", Top3Position: sql.NullInt64{Int64: 3, Valid: true}})

	// JSON null arrives as nil in the updates map.
	if err := db.UpdateTask("t1", map[string]any{"top3_position": nil}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	got, _ := db.GetTask("t1")
	if got.Top3Position.Valid {
		t.Fatalf("top3_position=%v, want null", got.Top3Position)
	}

	// Invalid values rejected (JSON numbers arrive as float64).
	if err := db.UpdateTask("t1", map[string]any{"top3_position": float64(5)}); err == nil {
		t.Fatalf("expected error for top3_position 5")
	}
	if err := db.UpdateTask("t1", map[string]any{"surface": "bogus"}); err == nil {
		t.Fatalf("expected error for invalid surface")
	}
}

func TestListTasks_StatusAll_ExcludesDeleted(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "pending"})
	mustInsertTask(t, db, Task{ID: "t2", Title: "done", Status: "done"})
	mustInsertTask(t, db, Task{ID: "t3", Title: "gone"})
	if err := db.SoftDeleteTask("t3"); err != nil {
		t.Fatalf("SoftDeleteTask: %v", err)
	}

	tasks, err := db.ListTasks(TaskFilters{Status: "all"})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2 (deleted excluded)", len(tasks))
	}
}

func TestListNotes_FilterByTaskID(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "task"})
	if err := db.InsertNote(Note{ID: "n1", Title: "linked", TaskID: sql.NullString{String: "t1", Valid: true}}); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}
	if err := db.InsertNote(Note{ID: "n2", Title: "loose"}); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}

	notes, err := db.ListNotes(NoteFilters{TaskID: strPtr("t1")})
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 || notes[0].ID != "n1" {
		t.Fatalf("got %+v, want only n1", notes)
	}
}

func TestUpdateTask_NotFound(t *testing.T) {
	db := openTestDB(t)
	// requireRowsAffected must surface sql.ErrNoRows so handlers return 404.
	if err := db.UpdateTask("missing", map[string]any{"title": "x"}); err != sql.ErrNoRows {
		t.Fatalf("err=%v, want sql.ErrNoRows", err)
	}
	if err := db.SoftDeleteTask("missing"); err != sql.ErrNoRows {
		t.Fatalf("err=%v, want sql.ErrNoRows", err)
	}
}

func TestArchiveDomain_NotFound(t *testing.T) {
	db := openTestDB(t)
	if err := db.ArchiveDomain(99999); err != sql.ErrNoRows {
		t.Fatalf("err=%v, want sql.ErrNoRows", err)
	}
}

// The v2 domain seed must fire on a single migrate() pass — a first boot has
// no second chance.
func TestMigrate_SeedsV2DomainsFirstBoot(t *testing.T) {
	db := openTestDB(t)
	domains, err := db.ListDomains(strPtr("business"))
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(domains) == 0 {
		t.Fatalf("no business domains after first migrate — v2 seed did not fire")
	}
}

func mustInsertTask(t *testing.T, db *DB, task Task) {
	t.Helper()
	if err := db.InsertTask(task); err != nil {
		t.Fatalf("InsertTask %s: %v", task.ID, err)
	}
}

// --- Notes ---

func TestInsertNote_WithKind(t *testing.T) {
	db := openTestDB(t)
	n := Note{ID: "n1", Title: "morning pages", Kind: "journal"}
	if err := db.InsertNote(n); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}

	got, err := db.GetNote("n1")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if got.Kind != "journal" {
		t.Fatalf("kind=%q, want journal", got.Kind)
	}

	// Default kind is note; invalid kind rejected.
	if err := db.InsertNote(Note{ID: "n2", Title: "plain"}); err != nil {
		t.Fatalf("InsertNote default kind: %v", err)
	}
	plain, _ := db.GetNote("n2")
	if plain.Kind != "note" {
		t.Fatalf("default kind=%q, want note", plain.Kind)
	}
	if err := db.InsertNote(Note{Title: "x", Kind: "bogus"}); err == nil {
		t.Fatalf("expected error for invalid kind")
	}
}

func TestInsertNote_WithPersonID(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertPerson(Person{ID: "per1", Name: "Auggie"}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}

	n := Note{ID: "n1", Title: "on Auggie", PersonID: sql.NullString{String: "per1", Valid: true}}
	if err := db.InsertNote(n); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}

	notes, err := db.ListNotes(NoteFilters{PersonID: strPtr("per1")})
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 || notes[0].ID != "n1" {
		t.Fatalf("got %+v, want only n1", notes)
	}
}

func TestListNotes_FilterByKind(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertNote(Note{ID: "n1", Title: "entry", Kind: "journal"}); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}
	if err := db.InsertNote(Note{ID: "n2", Title: "plain"}); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}

	notes, err := db.ListNotes(NoteFilters{Kind: strPtr("journal")})
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 || notes[0].ID != "n1" {
		t.Fatalf("got %+v, want only n1", notes)
	}
}

func TestListNotes_FilterByFlagForReview(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertNote(Note{ID: "n1", Title: "flagged", FlagForReview: 1,
		ReviewAt: sql.NullString{String: "2026-07-10", Valid: true}}); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}
	if err := db.InsertNote(Note{ID: "n2", Title: "plain"}); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}

	flagged := true
	notes, err := db.ListNotes(NoteFilters{FlagForReview: &flagged})
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 || notes[0].ID != "n1" {
		t.Fatalf("got %+v, want only n1", notes)
	}

	// Update accepts JSON bool for flag_for_review.
	if err := db.UpdateNote("n2", map[string]any{"flag_for_review": true}); err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}
	notes, _ = db.ListNotes(NoteFilters{FlagForReview: &flagged})
	if len(notes) != 2 {
		t.Fatalf("got %d flagged notes after update, want 2", len(notes))
	}
}

// --- Domains ---

func TestListDomains_FilterBySurface(t *testing.T) {
	db := openTestDB(t)
	// openTestDB seeds the v2 domains; add one of each surface on top.
	if _, err := db.CreateDomain("Test Life", 5, "", "", "life"); err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}
	if _, err := db.CreateDomain("Test Biz", 5, "", "", "business"); err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}

	domains, err := db.ListDomains(strPtr("business"))
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	foundBiz := false
	for _, d := range domains {
		if d.Surface != "business" {
			t.Fatalf("domain %q has surface %q in business filter", d.Name, d.Surface)
		}
		if d.Name == "Test Biz" {
			foundBiz = true
		}
		if d.Name == "Test Life" {
			t.Fatalf("life domain leaked into business filter")
		}
	}
	if !foundBiz {
		t.Fatalf("Test Biz missing from business filter: %+v", domains)
	}

	// Default surface is life; invalid surface rejected.
	d, err := db.CreateDomain("Test Default", 3, "", "", "")
	if err != nil {
		t.Fatalf("CreateDomain default: %v", err)
	}
	if d.Surface != "life" {
		t.Fatalf("default surface=%q, want life", d.Surface)
	}
	if _, err := db.CreateDomain("Test Bad", 3, "", "", "bogus"); err == nil {
		t.Fatalf("expected error for invalid surface")
	}
}
