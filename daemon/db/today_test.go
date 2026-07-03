package db

import (
	"database/sql"
	"testing"
	"time"
)

func daysAgo(n int) string {
	return time.Now().In(localLocation).AddDate(0, 0, -n).Format("2006-01-02")
}

// --- Top 3 ---

func TestGetTop3Tasks(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "second", Top3Position: sql.NullInt64{Int64: 2, Valid: true}})
	mustInsertTask(t, db, Task{ID: "t2", Title: "first", Top3Position: sql.NullInt64{Int64: 1, Valid: true}})
	mustInsertTask(t, db, Task{ID: "t3", Title: "unstarred"})

	top3, err := db.GetTop3Tasks(nil)
	if err != nil {
		t.Fatalf("GetTop3Tasks: %v", err)
	}
	if len(top3) != 2 || top3[0].ID != "t2" || top3[1].ID != "t1" {
		t.Fatalf("got %+v, want [t2 t1] in position order", top3)
	}
}

func TestSetTop3Tasks_ClearsPrevious(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "old star", Top3Position: sql.NullInt64{Int64: 1, Valid: true}})
	mustInsertTask(t, db, Task{ID: "t2", Title: "new star"})

	if err := db.SetTop3Tasks([]string{"t2"}); err != nil {
		t.Fatalf("SetTop3Tasks: %v", err)
	}

	top3, _ := db.GetTop3Tasks(nil)
	if len(top3) != 1 || top3[0].ID != "t2" || top3[0].Top3Position.Int64 != 1 {
		t.Fatalf("got %+v, want only t2 at position 1", top3)
	}
}

func TestSetTop3Tasks_AcceptsFewerThan3(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "a"})
	mustInsertTask(t, db, Task{ID: "t2", Title: "b"})

	if err := db.SetTop3Tasks([]string{"t1", "t2"}); err != nil {
		t.Fatalf("SetTop3Tasks: %v", err)
	}
	top3, _ := db.GetTop3Tasks(nil)
	if len(top3) != 2 {
		t.Fatalf("got %d starred, want 2", len(top3))
	}
}

func TestSetTop3Tasks_AcceptsEmpty(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "starred", Top3Position: sql.NullInt64{Int64: 1, Valid: true}})

	if err := db.SetTop3Tasks([]string{}); err != nil {
		t.Fatalf("SetTop3Tasks: %v", err)
	}
	top3, _ := db.GetTop3Tasks(nil)
	if len(top3) != 0 {
		t.Fatalf("got %d starred after clear, want 0", len(top3))
	}
}

func TestSetTop3Tasks_UnknownTaskRollsBack(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "starred", Top3Position: sql.NullInt64{Int64: 1, Valid: true}})

	err := db.SetTop3Tasks([]string{"missing"})
	if err == nil {
		t.Fatalf("expected error for unknown task id")
	}
	// The failed set must not have cleared the existing Top 3.
	top3, _ := db.GetTop3Tasks(nil)
	if len(top3) != 1 || top3[0].ID != "t1" {
		t.Fatalf("got %+v, want t1 still starred after rollback", top3)
	}
}

func TestSetTop3Tasks_RejectsNonPendingAndDuplicates(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "done", Status: "done"})
	mustInsertTask(t, db, Task{ID: "t2", Title: "pending"})

	// Starring a done task must fail loudly, not silently vanish from Top 3.
	if err := db.SetTop3Tasks([]string{"t1"}); err == nil {
		t.Fatalf("expected error starring a done task")
	}
	if err := db.SetTop3Tasks([]string{"t2", "t2"}); err == nil {
		t.Fatalf("expected error for duplicate task ids")
	}
	if err := db.SetTop3Tasks([]string{"a", "b", "c", "d"}); err == nil {
		t.Fatalf("expected error for more than 3 ids")
	}
}

// --- Slipping: projects ---

func TestSlippingProjects_NoActivityIn7Days(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertProject(Project{ID: "p1", Name: "stale",
		LastActivityAt: sql.NullString{String: daysAgo(8), Valid: true}}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	if err := db.InsertProject(Project{ID: "p2", Name: "fresh",
		LastActivityAt: sql.NullString{String: daysAgo(1), Valid: true}}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	slipping, err := db.ProjectsSlipping(nil)
	if err != nil {
		t.Fatalf("ProjectsSlipping: %v", err)
	}
	if len(slipping) != 1 || slipping[0].ID != "p1" {
		t.Fatalf("got %+v, want only p1", slipping)
	}
}

func TestSlippingProjects_ExcludesCompleted(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertProject(Project{ID: "p1", Name: "done", Status: "completed",
		LastActivityAt: sql.NullString{String: daysAgo(30), Valid: true}}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	slipping, _ := db.ProjectsSlipping(nil)
	if len(slipping) != 0 {
		t.Fatalf("completed project surfaced as slipping: %+v", slipping)
	}
}

func TestSlippingProjects_ExcludesArchived(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertProject(Project{ID: "p1", Name: "archived", Status: "archived",
		LastActivityAt: sql.NullString{String: daysAgo(30), Valid: true}}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	slipping, _ := db.ProjectsSlipping(nil)
	if len(slipping) != 0 {
		t.Fatalf("archived project surfaced as slipping: %+v", slipping)
	}
}

func TestSlippingProjects_SurfaceFilter(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertProject(Project{ID: "p1", Name: "life stale",
		LastActivityAt: sql.NullString{String: daysAgo(10), Valid: true}}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	if err := db.InsertProject(Project{ID: "p2", Name: "biz stale", Surface: "business",
		LastActivityAt: sql.NullString{String: daysAgo(10), Valid: true}}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	slipping, err := db.ProjectsSlipping(strPtr("business"))
	if err != nil {
		t.Fatalf("ProjectsSlipping: %v", err)
	}
	if len(slipping) != 1 || slipping[0].ID != "p2" {
		t.Fatalf("got %+v, want only p2", slipping)
	}
}

// --- Slipping: tasks ---

func TestSlippingTasks_PastDueDate(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "overdue", DueDate: sql.NullString{String: daysAgo(3), Valid: true}})
	mustInsertTask(t, db, Task{ID: "t2", Title: "due today", DueDate: sql.NullString{String: daysAgo(0), Valid: true}})
	mustInsertTask(t, db, Task{ID: "t3", Title: "no due date"})

	slipping, err := db.SlippingTasks(nil)
	if err != nil {
		t.Fatalf("SlippingTasks: %v", err)
	}
	if len(slipping) != 1 || slipping[0].ID != "t1" {
		t.Fatalf("got %+v, want only t1", slipping)
	}
	if slipping[0].DaysOverdue != 3 {
		t.Fatalf("days_overdue=%d, want 3", slipping[0].DaysOverdue)
	}
}

func TestSlippingTasks_ExcludesDone(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "done late", Status: "done",
		DueDate: sql.NullString{String: daysAgo(5), Valid: true}})

	slipping, _ := db.SlippingTasks(nil)
	if len(slipping) != 0 {
		t.Fatalf("done task surfaced as slipping: %+v", slipping)
	}
}

func TestSlippingTasks_SortedOldestFirst(t *testing.T) {
	db := openTestDB(t)
	mustInsertTask(t, db, Task{ID: "t1", Title: "recent", DueDate: sql.NullString{String: daysAgo(2), Valid: true}})
	mustInsertTask(t, db, Task{ID: "t2", Title: "ancient", DueDate: sql.NullString{String: daysAgo(20), Valid: true}})

	slipping, _ := db.SlippingTasks(nil)
	if len(slipping) != 2 || slipping[0].ID != "t2" {
		t.Fatalf("got %+v, want t2 (oldest) first", slipping)
	}
}

// --- Slipping: people ---

func TestSlippingPeople_PastCadence(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertPerson(Person{ID: "per1", Name: "neglected",
		LastInteractionAt: sql.NullString{String: daysAgo(100), Valid: true}}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}
	if err := db.InsertPerson(Person{ID: "per2", Name: "recent",
		LastInteractionAt: sql.NullString{String: daysAgo(10), Valid: true}}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}

	slipping, err := db.PeopleSlipping(nil)
	if err != nil {
		t.Fatalf("PeopleSlipping: %v", err)
	}
	if len(slipping) != 1 || slipping[0].ID != "per1" {
		t.Fatalf("got %+v, want only per1", slipping)
	}
}

func TestSlippingPeople_CustomCadence(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertPerson(Person{ID: "per1", Name: "weekly friend",
		ContactCadenceDays: sql.NullInt64{Int64: 7, Valid: true},
		LastInteractionAt:  sql.NullString{String: daysAgo(10), Valid: true}}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}

	slipping, _ := db.PeopleSlipping(nil)
	if len(slipping) != 1 || slipping[0].ID != "per1" {
		t.Fatalf("custom 7-day cadence not respected: %+v", slipping)
	}
}

func TestSlippingPeople_DefaultCadence90(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertPerson(Person{ID: "per1", Name: "under 90",
		LastInteractionAt: sql.NullString{String: daysAgo(89), Valid: true}}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}
	if err := db.InsertPerson(Person{ID: "per2", Name: "over 90",
		LastInteractionAt: sql.NullString{String: daysAgo(91), Valid: true}}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}

	slipping, _ := db.PeopleSlipping(nil)
	if len(slipping) != 1 || slipping[0].ID != "per2" {
		t.Fatalf("got %+v, want only per2 (default 90-day cadence)", slipping)
	}
}

// --- Slipping: areas ---

func TestSlippingAreas_NoTasksTouched14Days(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertProject(Project{ID: "a1", Name: "dormant area", Type: "area",
		LastActivityAt: sql.NullString{String: daysAgo(15), Valid: true}}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	if err := db.InsertProject(Project{ID: "a2", Name: "active area", Type: "area"}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	// Touching a task in a2 bumps its last_activity_at.
	mustInsertTask(t, db, Task{ID: "t1", Title: "area task", ProjectID: sql.NullString{String: "a2", Valid: true}})

	slipping, err := db.AreasSlipping(nil)
	if err != nil {
		t.Fatalf("AreasSlipping: %v", err)
	}
	if len(slipping) != 1 || slipping[0].ID != "a1" {
		t.Fatalf("got %+v, want only a1", slipping)
	}
}

// --- Notifications ---

func TestTodayNotifications_IncludesReminders(t *testing.T) {
	db := openTestDB(t)
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	mustInsertTask(t, db, Task{ID: "t1", Title: "due reminder", ReminderAt: sql.NullString{String: past, Valid: true}})
	mustInsertTask(t, db, Task{ID: "t2", Title: "future reminder", ReminderAt: sql.NullString{String: future, Valid: true}})

	notifications, err := db.TodayNotifications(nil)
	if err != nil {
		t.Fatalf("TodayNotifications: %v", err)
	}
	if len(notifications) != 1 || notifications[0].Type != "reminder" || notifications[0].RefID != "t1" {
		t.Fatalf("got %+v, want one reminder for t1", notifications)
	}

	// Once a notification is logged for it, the reminder stops surfacing.
	if err := db.LogNotification("reminder", "t1", "sent"); err != nil {
		t.Fatalf("LogNotification: %v", err)
	}
	notifications, _ = db.TodayNotifications(nil)
	if len(notifications) != 0 {
		t.Fatalf("acknowledged reminder still surfacing: %+v", notifications)
	}
}

func TestTodayNotifications_IncludesBirthdays(t *testing.T) {
	db := openTestDB(t)
	inTwoDays := time.Now().In(localLocation).AddDate(0, 0, 2).Format("2006-01-02")
	if err := db.InsertPerson(Person{ID: "per1", Name: "Auggie",
		Relationship: sql.NullString{String: "family", Valid: true},
		Birthday:     sql.NullString{String: inTwoDays, Valid: true}}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}

	notifications, err := db.TodayNotifications(nil)
	if err != nil {
		t.Fatalf("TodayNotifications: %v", err)
	}
	if len(notifications) != 1 || notifications[0].Type != "birthday" {
		t.Fatalf("got %+v, want one birthday", notifications)
	}
	n := notifications[0]
	if n.Title != "Auggie" || n.DaysUntil == nil || *n.DaysUntil != 2 {
		t.Fatalf("birthday notification wrong: %+v", n)
	}
	if n.Detail != "family · self" {
		t.Fatalf("detail=%q, want relationship included", n.Detail)
	}
}

func TestTodayNotifications_IncludesFollowUps(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertPerson(Person{ID: "per1", Name: "Client"}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}
	if err := db.InsertInteraction(Interaction{ID: "i1", PersonID: "per1", Summary: "kickoff call",
		FollowUp:     sql.NullString{String: "send proposal", Valid: true},
		FollowUpDate: sql.NullString{String: daysAgo(1), Valid: true}}); err != nil {
		t.Fatalf("InsertInteraction: %v", err)
	}

	notifications, err := db.TodayNotifications(nil)
	if err != nil {
		t.Fatalf("TodayNotifications: %v", err)
	}
	if len(notifications) != 1 || notifications[0].Type != "follow_up" {
		t.Fatalf("got %+v, want one follow_up", notifications)
	}
	if notifications[0].Title != "send proposal" || notifications[0].Detail != "kickoff call" {
		t.Fatalf("follow_up content wrong: %+v", notifications[0])
	}
}

func TestTodayNotifications_IncludesNoteReviews(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertNote(Note{ID: "n1", Title: "revisit this",
		ReviewAt: sql.NullString{String: daysAgo(1), Valid: true}}); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}

	notifications, err := db.TodayNotifications(nil)
	if err != nil {
		t.Fatalf("TodayNotifications: %v", err)
	}
	if len(notifications) != 1 || notifications[0].Type != "review" || notifications[0].RefID != "n1" {
		t.Fatalf("got %+v, want one review for n1", notifications)
	}
}

func TestTodayNotifications_EmptySliceNotNull(t *testing.T) {
	db := openTestDB(t)
	notifications, err := db.TodayNotifications(nil)
	if err != nil {
		t.Fatalf("TodayNotifications: %v", err)
	}
	if notifications == nil {
		t.Fatalf("notifications is nil — must be empty slice for JSON []")
	}
	if len(notifications) != 0 {
		t.Fatalf("got %d notifications on empty db, want 0", len(notifications))
	}
}
