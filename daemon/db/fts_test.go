package db

import "testing"

func TestUpsertFTS(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "fts-1")

	if err := database.UpsertFTS("fts-1", "title", "content", "tags"); err != nil {
		t.Fatalf("upsert fts: %v", err)
	}

	var count int
	if err := database.conn.QueryRow(`SELECT COUNT(*) FROM memories_fts WHERE memory_id = ?`, "fts-1").Scan(&count); err != nil {
		t.Fatalf("count fts: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 fts entry after upsert, got %d", count)
	}
}

func TestUpsertFTSReplacesExisting(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "fts-replace")

	if err := database.UpsertFTS("fts-replace", "original", "content", "tags"); err != nil {
		t.Fatalf("upsert fts first: %v", err)
	}
	if err := database.UpsertFTS("fts-replace", "updated", "new content", "new,tags"); err != nil {
		t.Fatalf("upsert fts second: %v", err)
	}

	var count int
	if err := database.conn.QueryRow(`SELECT COUNT(*) FROM memories_fts WHERE memory_id = ?`, "fts-replace").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 fts entry after replace, got %d", count)
	}
}

func TestDeleteFTS(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "fts-del")

	if err := database.DeleteFTS("fts-del"); err != nil {
		t.Fatalf("delete fts: %v", err)
	}

	var count int
	if err := database.conn.QueryRow(`SELECT COUNT(*) FROM memories_fts WHERE memory_id = ?`, "fts-del").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 fts entries after delete, got %d", count)
	}
}

func TestRebuildFTS(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "fts-rb-1")
	insertTestMemory(t, database, "fts-rb-2")

	if err := database.DeleteFTS("fts-rb-1"); err != nil {
		t.Fatalf("delete fts: %v", err)
	}
	if err := database.DeleteFTS("fts-rb-2"); err != nil {
		t.Fatalf("delete fts: %v", err)
	}

	var countBefore int
	database.conn.QueryRow(`SELECT COUNT(*) FROM memories_fts`).Scan(&countBefore)
	if countBefore != 0 {
		t.Fatalf("expected 0 before rebuild, got %d", countBefore)
	}

	if err := database.RebuildFTS(); err != nil {
		t.Fatalf("rebuild fts: %v", err)
	}

	var countAfter int
	database.conn.QueryRow(`SELECT COUNT(*) FROM memories_fts`).Scan(&countAfter)
	if countAfter != 2 {
		t.Fatalf("expected 2 after rebuild, got %d", countAfter)
	}
}

func TestFTSTriggerInsert(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertMemory(Memory{
		ID:         "trigger-ins",
		Type:       "fact",
		Content:    "trigger test content",
		Title:      "trigger title",
		Importance: 0.5,
		Source:     "test",
		Tags:       "alpha,beta",
	}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	var count int
	if err := database.conn.QueryRow(`SELECT COUNT(*) FROM memories_fts WHERE memory_id = ?`, "trigger-ins").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected trigger to insert 1 fts row, got %d", count)
	}
}

func TestFTSTriggerDelete(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "trigger-del")

	var countBefore int
	database.conn.QueryRow(`SELECT COUNT(*) FROM memories_fts WHERE memory_id = ?`, "trigger-del").Scan(&countBefore)
	if countBefore != 1 {
		t.Fatalf("expected 1 fts row from trigger, got %d", countBefore)
	}

	if _, err := database.conn.Exec(`DELETE FROM memories WHERE id = ?`, "trigger-del"); err != nil {
		t.Fatalf("delete memory: %v", err)
	}

	var countAfter int
	database.conn.QueryRow(`SELECT COUNT(*) FROM memories_fts WHERE memory_id = ?`, "trigger-del").Scan(&countAfter)
	if countAfter != 0 {
		t.Fatalf("expected 0 fts rows after delete trigger, got %d", countAfter)
	}
}

func TestFTSTriggerUpdate(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "trigger-upd")

	if _, err := database.conn.Exec(
		`UPDATE memories SET title = ?, content = ?, tags = ? WHERE id = ?`,
		"new title", "new content", "new,tags", "trigger-upd",
	); err != nil {
		t.Fatalf("update memory: %v", err)
	}

	var count int
	database.conn.QueryRow(`SELECT COUNT(*) FROM memories_fts WHERE memory_id = ?`, "trigger-upd").Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 fts row after update trigger, got %d", count)
	}

	rows, err := database.conn.Query(`SELECT memory_id, title, content, tags FROM memories_fts WHERE memory_id = ?`, "trigger-upd")
	if err != nil {
		t.Fatalf("query fts: %v", err)
	}
	defer rows.Close()

	if rows.Next() {
		var mid, title, content, tags string
		if err := rows.Scan(&mid, &title, &content, &tags); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if title != "new title" {
			t.Fatalf("expected updated title, got %s", title)
		}
		if content != "new content" {
			t.Fatalf("expected updated content, got %s", content)
		}
		if tags != "new,tags" {
			t.Fatalf("expected updated tags, got %s", tags)
		}
	}
}
