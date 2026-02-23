package db

import (
	"database/sql"
	"errors"
	"testing"
)

func TestInsertAndGetNote(t *testing.T) {
	database := newTestDB(t)

	err := database.InsertNote(Note{
		ID:    "note-1",
		Title: "Test Note",
		Content: sql.NullString{String: "Some content", Valid: true},
		Tags:    sql.NullString{String: "go,testing", Valid: true},
	})
	if err != nil {
		t.Fatalf("insert note: %v", err)
	}

	n, err := database.GetNote("note-1")
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if n == nil {
		t.Fatal("expected note, got nil")
	}
	if n.Title != "Test Note" {
		t.Fatalf("expected title 'Test Note', got %q", n.Title)
	}
	if n.Content.String != "Some content" {
		t.Fatalf("expected content 'Some content', got %q", n.Content.String)
	}
	if n.CreatedAt == "" {
		t.Fatal("expected created_at to be set")
	}
	if !n.UpdatedAt.Valid {
		t.Fatal("expected updated_at to be set")
	}

	missing, err := database.GetNote("nonexistent")
	if err != nil {
		t.Fatalf("get missing note: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for missing note")
	}
}

func TestListNotes(t *testing.T) {
	database := newTestDB(t)

	domainID := seedDomain(t, database)

	database.InsertNote(Note{ID: "n1", Title: "Domain Note", DomainID: sql.NullInt64{Int64: int64(domainID), Valid: true}})
	database.InsertNote(Note{ID: "n2", Title: "Other Note"})
	database.InsertNote(Note{ID: "n3", Title: "Tagged", Tags: sql.NullString{String: "work,urgent", Valid: true}})

	all, err := database.ListNotes(nil, nil, "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 notes, got %d", len(all))
	}

	filtered, err := database.ListNotes(&domainID, nil, "")
	if err != nil {
		t.Fatalf("list by domain: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 note for domain, got %d", len(filtered))
	}
	if filtered[0].ID != "n1" {
		t.Fatalf("expected n1, got %s", filtered[0].ID)
	}

	tagged, err := database.ListNotes(nil, nil, "urgent")
	if err != nil {
		t.Fatalf("list by tag: %v", err)
	}
	if len(tagged) != 1 {
		t.Fatalf("expected 1 tagged note, got %d", len(tagged))
	}
	if tagged[0].ID != "n3" {
		t.Fatalf("expected n3, got %s", tagged[0].ID)
	}
}

func TestUpdateNote(t *testing.T) {
	database := newTestDB(t)

	database.InsertNote(Note{ID: "upd-1", Title: "Original"})

	oldTS := "2020-01-01T00:00:00Z"
	database.conn.Exec(`UPDATE notes SET updated_at = ? WHERE id = ?`, oldTS, "upd-1")

	err := database.UpdateNote("upd-1", map[string]any{"title": "Updated"})
	if err != nil {
		t.Fatalf("update note: %v", err)
	}

	after, _ := database.GetNote("upd-1")
	if after.Title != "Updated" {
		t.Fatalf("expected title 'Updated', got %q", after.Title)
	}
	if after.UpdatedAt.String <= oldTS {
		t.Fatalf("expected updated_at to advance past %s, got %s", oldTS, after.UpdatedAt.String)
	}

	err = database.UpdateNote("upd-1", map[string]any{"bad_col": "x"})
	if err == nil {
		t.Fatal("expected error for unsupported column")
	}

	err = database.UpdateNote("missing", map[string]any{"title": "x"})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows for missing note, got %v", err)
	}
}

func TestDeleteNote(t *testing.T) {
	database := newTestDB(t)

	database.InsertNote(Note{ID: "del-1", Title: "Delete Me"})

	err := database.DeleteNote("del-1")
	if err != nil {
		t.Fatalf("delete note: %v", err)
	}

	n, _ := database.GetNote("del-1")
	if n != nil {
		t.Fatal("expected note to be deleted")
	}

	err = database.DeleteNote("missing")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows for missing note, got %v", err)
	}
}

func TestNotesByDomain(t *testing.T) {
	database := newTestDB(t)

	domainID := seedDomain(t, database)

	database.InsertNote(Note{ID: "d1", Title: "In Domain", DomainID: sql.NullInt64{Int64: int64(domainID), Valid: true}})
	database.InsertNote(Note{ID: "d2", Title: "In Domain 2", DomainID: sql.NullInt64{Int64: int64(domainID), Valid: true}})
	database.InsertNote(Note{ID: "d3", Title: "No Domain"})

	notes, err := database.NotesByDomain(domainID)
	if err != nil {
		t.Fatalf("notes by domain: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
}

func seedDomain(t *testing.T, database *DB) int {
	t.Helper()
	_, err := database.conn.Exec(
		`INSERT INTO domains (name, importance, last_touched_at, created_at) VALUES (?, 3, ?, ?)`,
		"test-domain", now(), now(),
	)
	if err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	var id int
	database.conn.QueryRow(`SELECT id FROM domains WHERE name = 'test-domain'`).Scan(&id)
	return id
}
