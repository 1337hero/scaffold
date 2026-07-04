package cortex

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appconfig "scaffold/config"
	"scaffold/db"
)

func openCortexTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestRunPushCheckBatchesAndDedups(t *testing.T) {
	database := openCortexTestDB(t)
	loc := briefLocation()
	now := time.Now().In(loc)
	reminderAt := now.Add(-time.Minute).UTC().Format(time.RFC3339)
	today := now.Format("2006-01-02")

	if err := database.InsertTask(db.Task{
		ID:         "task1",
		Title:      "Pay tax estimate",
		ReminderAt: sql.NullString{String: reminderAt, Valid: true},
		DueDate:    sql.NullString{String: today, Valid: true},
	}); err != nil {
		t.Fatalf("InsertTask: %v", err)
	}
	if err := database.InsertPerson(db.Person{ID: "person1", Name: "Taylor"}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}
	if err := database.InsertInteraction(db.Interaction{
		ID:           "interaction1",
		PersonID:     "person1",
		Date:         today,
		Summary:      "Kickoff call",
		FollowUp:     sql.NullString{String: "send proposal", Valid: true},
		FollowUpDate: sql.NullString{String: today, Valid: true},
	}); err != nil {
		t.Fatalf("InsertInteraction: %v", err)
	}

	var sent []string
	c := NewWithLLM(database, nil, appconfig.CortexConfig{}, LLMRoutes{})
	c.SetNotificationsConfig(appconfig.NotificationsConfig{Enabled: true})
	c.SetSignalSender(func(ctx context.Context, message string) error {
		sent = append(sent, message)
		return nil
	})

	if err := c.runPushCheck(context.Background()); err != nil {
		t.Fatalf("runPushCheck: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("got %d sends, want 1", len(sent))
	}
	if !strings.Contains(sent[0], "Notifications") ||
		!strings.Contains(sent[0], "you set this reminder") ||
		!strings.Contains(sent[0], "Follow up with Taylor") {
		t.Fatalf("unexpected batch message: %s", sent[0])
	}
	todayNotifications, err := database.TodayNotifications(nil)
	if err != nil {
		t.Fatalf("TodayNotifications: %v", err)
	}
	for _, notification := range todayNotifications {
		if notification.Type == "reminder" && notification.RefID == "task1" {
			t.Fatalf("pushed reminder still surfaced in Today notifications: %+v", todayNotifications)
		}
	}

	if err := c.runPushCheck(context.Background()); err != nil {
		t.Fatalf("runPushCheck second: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("dedup failed; sends=%d messages=%+v", len(sent), sent)
	}
}

func TestRunBirthdayCheckSendsThreeDayAndTodayOnly(t *testing.T) {
	database := openCortexTestDB(t)
	loc := briefLocation()
	now := time.Now().In(loc)

	people := []db.Person{
		{ID: "p1", Name: "Auggie", Birthday: sql.NullString{String: "2010-" + now.Format("01-02"), Valid: true}},
		{ID: "p2", Name: "Marcus", Birthday: sql.NullString{String: "2014-" + now.AddDate(0, 0, 3).Format("01-02"), Valid: true}},
		{ID: "p3", Name: "Skip", Birthday: sql.NullString{String: "2014-" + now.AddDate(0, 0, 1).Format("01-02"), Valid: true}},
	}
	for _, person := range people {
		if err := database.InsertPerson(person); err != nil {
			t.Fatalf("InsertPerson %s: %v", person.ID, err)
		}
	}

	var sent []string
	c := NewWithLLM(database, nil, appconfig.CortexConfig{}, LLMRoutes{})
	c.SetNotificationsConfig(appconfig.NotificationsConfig{Enabled: true})
	c.SetSignalSender(func(ctx context.Context, message string) error {
		sent = append(sent, message)
		return nil
	})

	if err := c.runBirthdayCheck(context.Background()); err != nil {
		t.Fatalf("runBirthdayCheck: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("got %d sends, want 1", len(sent))
	}
	if !strings.Contains(sent[0], "Auggie's birthday is today") ||
		!strings.Contains(sent[0], "Marcus's birthday is in 3 days") ||
		strings.Contains(sent[0], "Skip") {
		t.Fatalf("unexpected birthday batch: %s", sent[0])
	}

	if err := c.runBirthdayCheck(context.Background()); err != nil {
		t.Fatalf("runBirthdayCheck second: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("birthday dedup failed; sends=%d messages=%+v", len(sent), sent)
	}
}
