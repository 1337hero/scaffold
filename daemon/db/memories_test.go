package db

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

func insertTestMemory(t *testing.T, database *DB, id string) {
	t.Helper()
	err := database.InsertMemory(Memory{
		ID:         id,
		Type:       "fact",
		Content:    "test content",
		Title:      "test title",
		Importance: 0.8,
		Source:     "test",
		Tags:       "test,memory",
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}
}

func TestSuppressMemory(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "mem-suppress")

	list, err := database.ListByImportance(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(list))
	}

	if err := database.SuppressMemory("mem-suppress"); err != nil {
		t.Fatalf("suppress: %v", err)
	}

	list, err = database.ListByImportance(10)
	if err != nil {
		t.Fatalf("list after suppress: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 memories after suppress, got %d", len(list))
	}
}

func TestUnsuppressMemory(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "mem-unsuppress")

	if err := database.SuppressMemory("mem-unsuppress"); err != nil {
		t.Fatalf("suppress: %v", err)
	}

	list, err := database.ListByImportance(10)
	if err != nil {
		t.Fatalf("list after suppress: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 after suppress, got %d", len(list))
	}

	if err := database.UnsuppressMemory("mem-unsuppress"); err != nil {
		t.Fatalf("unsuppress: %v", err)
	}

	list, err = database.ListByImportance(10)
	if err != nil {
		t.Fatalf("list after unsuppress: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 after unsuppress, got %d", len(list))
	}
}

func TestReclassifyMemory(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "mem-reclassify")

	captureID, err := database.InsertCapture("capture for reclassify", "web")
	if err != nil {
		t.Fatalf("insert capture: %v", err)
	}
	if err := database.UpdateTriage(captureID, "explore", "mem-reclassify"); err != nil {
		t.Fatalf("link capture to memory: %v", err)
	}

	err = database.ReclassifyMemory("mem-reclassify", ReclassifyParams{
		Type:       "preference",
		Action:     "do",
		Tags:       "updated,tags",
		Importance: 0.95,
	})
	if err != nil {
		t.Fatalf("reclassify: %v", err)
	}

	m, err := database.GetMemory("mem-reclassify")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if m == nil {
		t.Fatal("expected memory, got nil")
	}
	if m.Type != "preference" {
		t.Fatalf("expected type preference, got %s", m.Type)
	}
	if m.Tags != "updated,tags" {
		t.Fatalf("expected tags updated,tags, got %s", m.Tags)
	}
	if m.Importance != 0.95 {
		t.Fatalf("expected importance 0.95, got %f", m.Importance)
	}

	caps, err := database.ListRecent(10)
	if err != nil {
		t.Fatalf("list captures: %v", err)
	}
	if len(caps) != 1 {
		t.Fatalf("expected 1 capture, got %d", len(caps))
	}
	if !caps[0].TriageAction.Valid || caps[0].TriageAction.String != "do" {
		t.Fatalf("expected capture triage_action do, got %+v", caps[0].TriageAction)
	}
}

func TestGetMemory(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "mem-get")

	m, err := database.GetMemory("mem-get")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if m == nil {
		t.Fatal("expected memory, got nil")
	}
	if m.ID != "mem-get" {
		t.Fatalf("expected id mem-get, got %s", m.ID)
	}
	if m.Type != "fact" {
		t.Fatalf("expected type fact, got %s", m.Type)
	}
	if m.Content != "test content" {
		t.Fatalf("expected content 'test content', got %s", m.Content)
	}
	if m.Title != "test title" {
		t.Fatalf("expected title 'test title', got %s", m.Title)
	}
	if m.Importance != 0.8 {
		t.Fatalf("expected importance 0.8, got %f", m.Importance)
	}
	if m.Source != "test" {
		t.Fatalf("expected source test, got %s", m.Source)
	}
	if m.Tags != "test,memory" {
		t.Fatalf("expected tags test,memory, got %s", m.Tags)
	}

	nilMem, err := database.GetMemory("nonexistent")
	if err != nil {
		t.Fatalf("get nonexistent: %v", err)
	}
	if nilMem != nil {
		t.Fatal("expected nil for nonexistent memory")
	}
}

func TestConversationLog(t *testing.T) {
	database := newTestDB(t)

	_, err := database.InsertConversationEntry("mike", "user", "hello")
	if err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	_, err = database.InsertConversationEntry("mike", "assistant", "hi there")
	if err != nil {
		t.Fatalf("insert 2: %v", err)
	}
	_, err = database.InsertConversationEntry("mike", "user", "how are you")
	if err != nil {
		t.Fatalf("insert 3: %v", err)
	}

	entries, err := database.ListRecentConversation("mike", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Content != "hello" {
		t.Fatalf("expected first entry 'hello', got %s", entries[0].Content)
	}
	if entries[1].Content != "hi there" {
		t.Fatalf("expected second entry 'hi there', got %s", entries[1].Content)
	}
	if entries[2].Content != "how are you" {
		t.Fatalf("expected third entry 'how are you', got %s", entries[2].Content)
	}

	count, err := database.ConversationCount("mike")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count 3, got %d", count)
	}
}

func TestConfirmCapture(t *testing.T) {
	database := newTestDB(t)

	id, err := database.InsertCapture("test raw", "test-source")
	if err != nil {
		t.Fatalf("insert capture: %v", err)
	}

	caps, err := database.ListRecent(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(caps) != 1 {
		t.Fatalf("expected 1 capture, got %d", len(caps))
	}
	if caps[0].Confirmed != 0 {
		t.Fatalf("expected confirmed 0, got %d", caps[0].Confirmed)
	}

	if err := database.ConfirmCapture(id); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	caps, err = database.ListRecent(10)
	if err != nil {
		t.Fatalf("list after confirm: %v", err)
	}
	if caps[0].Confirmed != 1 {
		t.Fatalf("expected confirmed 1, got %d", caps[0].Confirmed)
	}
}

func TestPersistTriageResult(t *testing.T) {
	database := newTestDB(t)
	captureID, err := database.InsertCapture("test raw", "test-source")
	if err != nil {
		t.Fatalf("insert capture: %v", err)
	}

	mem := Memory{
		ID:         "mem-persist",
		Type:       "Todo",
		Content:    "atomic content",
		Title:      "Atomic title",
		Importance: 0.8,
		Source:     "signal",
		Tags:       "atomic",
	}
	if err := database.PersistTriageResult(captureID, mem, "do"); err != nil {
		t.Fatalf("persist triage result: %v", err)
	}

	capture, err := database.GetCapture(captureID)
	if err != nil {
		t.Fatalf("get capture: %v", err)
	}
	if capture == nil {
		t.Fatal("expected capture")
	}
	if capture.Processed != 1 {
		t.Fatalf("expected processed=1, got %d", capture.Processed)
	}
	if !capture.MemoryID.Valid || capture.MemoryID.String != "mem-persist" {
		t.Fatalf("expected memory_id mem-persist, got %+v", capture.MemoryID)
	}
	if !capture.TriageAction.Valid || capture.TriageAction.String != "do" {
		t.Fatalf("expected triage action do, got %+v", capture.TriageAction)
	}

	stored, err := database.GetMemory("mem-persist")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if stored == nil {
		t.Fatal("expected stored memory")
	}
}

func TestPersistTriageResultRollsBackWhenCaptureMissing(t *testing.T) {
	database := newTestDB(t)

	mem := Memory{
		ID:         "mem-rollback",
		Type:       "Todo",
		Content:    "atomic content",
		Title:      "Atomic title",
		Importance: 0.8,
		Source:     "signal",
		Tags:       "atomic",
	}
	err := database.PersistTriageResult("missing-capture", mem, "do")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}

	stored, err := database.GetMemory("mem-rollback")
	if err != nil {
		t.Fatalf("get memory after rollback: %v", err)
	}
	if stored != nil {
		t.Fatal("expected memory insert rollback")
	}
}

func TestListRecentMemoriesFiltersSuppressedAndOrdersNewestFirst(t *testing.T) {
	database := newTestDB(t)

	err := database.InsertMemory(Memory{
		ID:         "mem-old",
		Type:       "Fact",
		Content:    "older",
		Title:      "Older",
		Importance: 0.5,
		Source:     "test",
		CreatedAt:  "2026-02-18T10:00:00Z",
		UpdatedAt:  "2026-02-18T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("insert old memory: %v", err)
	}

	err = database.InsertMemory(Memory{
		ID:         "mem-new",
		Type:       "Fact",
		Content:    "newer",
		Title:      "Newer",
		Importance: 0.6,
		Source:     "test",
		CreatedAt:  "2026-02-19T10:00:00Z",
		UpdatedAt:  "2026-02-19T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("insert new memory: %v", err)
	}

	err = database.InsertMemory(Memory{
		ID:         "mem-suppressed",
		Type:       "Fact",
		Content:    "hidden",
		Title:      "Suppressed",
		Importance: 0.9,
		Source:     "test",
		CreatedAt:  "2026-02-20T10:00:00Z",
		UpdatedAt:  "2026-02-20T10:00:00Z",
		SuppressedAt: sql.NullString{
			String: time.Now().Format(time.RFC3339),
			Valid:  true,
		},
	})
	if err != nil {
		t.Fatalf("insert suppressed memory: %v", err)
	}

	memories, err := database.ListRecentMemories(10)
	if err != nil {
		t.Fatalf("list recent memories: %v", err)
	}

	if len(memories) != 2 {
		t.Fatalf("expected 2 visible memories, got %d", len(memories))
	}
	if memories[0].ID != "mem-new" {
		t.Fatalf("expected newest memory first, got %s", memories[0].ID)
	}
	if memories[1].ID != "mem-old" {
		t.Fatalf("expected older memory second, got %s", memories[1].ID)
	}
}
