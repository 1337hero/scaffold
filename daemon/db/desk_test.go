package db

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestUpdateDeskStatusNotFound(t *testing.T) {
	database := newTestDB(t)

	err := database.UpdateDeskStatus("missing-id", "done")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestUpdateDeskStatusLifecycle(t *testing.T) {
	database := newTestDB(t)
	itemID := "desk-lifecycle"
	insertDeskItemForDate(t, database, itemID, today())

	if err := database.UpdateDeskStatus(itemID, "done"); err != nil {
		t.Fatalf("mark done: %v", err)
	}

	items, err := database.TodaysDesk()
	if err != nil {
		t.Fatalf("query today's desk: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Status != "done" {
		t.Fatalf("expected status done, got %s", items[0].Status)
	}
	if !items[0].CompletedAt.Valid {
		t.Fatal("expected completed_at to be set")
	}

	if err := database.UpdateDeskStatus(itemID, "active"); err != nil {
		t.Fatalf("mark active: %v", err)
	}

	items, err = database.TodaysDesk()
	if err != nil {
		t.Fatalf("query today's desk after reopen: %v", err)
	}
	if items[0].Status != "active" {
		t.Fatalf("expected status active, got %s", items[0].Status)
	}
	if items[0].CompletedAt.Valid {
		t.Fatal("expected completed_at to be cleared")
	}
}

func TestDeferDeskItemMovesDateToTomorrow(t *testing.T) {
	database := newTestDB(t)
	itemID := "desk-defer"
	insertDeskItemForDate(t, database, itemID, today())

	if err := database.DeferDeskItem(itemID); err != nil {
		t.Fatalf("defer desk item: %v", err)
	}

	items, err := database.TodaysDesk()
	if err != nil {
		t.Fatalf("query today's desk: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items for today after defer, got %d", len(items))
	}

	var status string
	var date string
	var completedAt sql.NullString
	err = database.conn.QueryRow(
		`SELECT status, date, completed_at FROM desk WHERE id = ?`,
		itemID,
	).Scan(&status, &date, &completedAt)
	if err != nil {
		t.Fatalf("query deferred item: %v", err)
	}
	if status != "deferred" {
		t.Fatalf("expected deferred status, got %s", status)
	}
	if date != tomorrow() {
		t.Fatalf("expected date %s, got %s", tomorrow(), date)
	}
	if completedAt.Valid {
		t.Fatal("expected completed_at to be NULL after defer")
	}
}

func newTestDB(t *testing.T) *DB {
	t.Helper()

	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	return database
}

func insertDeskItemForDate(t *testing.T, database *DB, id, date string) {
	t.Helper()

	err := database.InsertDeskItem(DeskItem{
		ID:       id,
		Title:    "Task",
		Position: 1,
		Status:   "active",
		Date:     date,
	})
	if err != nil {
		t.Fatalf("insert desk item: %v", err)
	}
}
