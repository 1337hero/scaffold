package db

import "testing"

func TestLogNotificationTrigger(t *testing.T) {
	db := openTestDB(t)

	if err := db.LogNotificationTrigger("reminder", "task1", "2026-07-04T09:00:00Z", "sent"); err != nil {
		t.Fatalf("LogNotificationTrigger: %v", err)
	}

	entries, err := db.NotificationLog("reminder", "task1")
	if err != nil {
		t.Fatalf("NotificationLog: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].TriggerDate != "2026-07-04T09:00:00Z" || entries[0].Message != "sent" {
		t.Fatalf("entry not stored correctly: %+v", entries[0])
	}

	last, err := db.LastNotification("reminder", "task1")
	if err != nil {
		t.Fatalf("LastNotification: %v", err)
	}
	if last == nil || last.TriggerDate != "2026-07-04T09:00:00Z" {
		t.Fatalf("last notification wrong: %+v", last)
	}

	if err := db.SuppressNotification(entries[0].ID); err != nil {
		t.Fatalf("SuppressNotification: %v", err)
	}
	entries, err = db.NotificationLog("reminder", "task1")
	if err != nil {
		t.Fatalf("NotificationLog after suppress: %v", err)
	}
	if len(entries) != 1 || !entries[0].SuppressedAt.Valid {
		t.Fatalf("suppressed entry not preserved: %+v", entries)
	}
}
