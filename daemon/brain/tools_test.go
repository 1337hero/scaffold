package brain

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"scaffold/db"
)

func today() string {
	return time.Now().Format("2006-01-02")
}

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestGetDeskEmpty(t *testing.T) {
	database := openTestDB(t)
	result, err := handleGetDesk(context.Background(), database, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Desk is empty today." {
		t.Fatalf("expected empty desk message, got %q", result)
	}
}

func TestGetDeskWithItems(t *testing.T) {
	database := openTestDB(t)

	err := database.InsertDeskItem(db.DeskItem{
		ID:       "desk-1",
		Title:    "Ship feature",
		Position: 1,
		Status:   "active",
		Date:     today(),
	})
	if err != nil {
		t.Fatalf("insert desk item: %v", err)
	}

	result, err := handleGetDesk(context.Background(), database, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Today's desk:") {
		t.Fatalf("expected desk header, got %q", result)
	}
	if !strings.Contains(result, "Ship feature") {
		t.Fatalf("expected item title in result, got %q", result)
	}
	if !strings.Contains(result, "desk-1") {
		t.Fatalf("expected item id in result, got %q", result)
	}
}

func TestSearchMemoriesNoResults(t *testing.T) {
	database := openTestDB(t)
	params, _ := json.Marshal(map[string]string{"query": "nonexistent"})
	result, err := handleSearchMemories(context.Background(), database, nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "No memories found") {
		t.Fatalf("expected no results message, got %q", result)
	}
}

func TestSearchMemoriesWithResults(t *testing.T) {
	database := openTestDB(t)

	err := database.InsertMemory(db.Memory{
		ID:         "mem-1",
		Type:       "Fact",
		Content:    "Go is a compiled language",
		Title:      "Go language fact",
		Importance: 0.7,
		Source:     "test",
		Tags:       "golang,programming",
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	params, _ := json.Marshal(map[string]string{"query": "golang"})
	result, err := handleSearchMemories(context.Background(), database, nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Found 1") {
		t.Fatalf("expected 1 result, got %q", result)
	}
	if !strings.Contains(result, "Go language fact") {
		t.Fatalf("expected memory title in result, got %q", result)
	}
}

func TestSearchMemoriesMissingQuery(t *testing.T) {
	database := openTestDB(t)
	params, _ := json.Marshal(map[string]string{})
	_, err := handleSearchMemories(context.Background(), database, nil, params)
	if err == nil {
		t.Fatal("expected error for missing query")
	}
	if !strings.Contains(err.Error(), "query required") {
		t.Fatalf("expected query required error, got %v", err)
	}
}

func TestGetInboxEmpty(t *testing.T) {
	database := openTestDB(t)
	result, err := handleGetInbox(context.Background(), database, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Inbox is empty." {
		t.Fatalf("expected empty inbox message, got %q", result)
	}
}

func TestGetInboxWithItems(t *testing.T) {
	database := openTestDB(t)

	_, err := database.InsertCapture("Buy groceries", "signal")
	if err != nil {
		t.Fatalf("insert capture: %v", err)
	}

	result, err := handleGetInbox(context.Background(), database, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Inbox (1 items)") {
		t.Fatalf("expected 1 item, got %q", result)
	}
	if !strings.Contains(result, "Buy groceries") {
		t.Fatalf("expected capture text, got %q", result)
	}
}

func TestUpdateDeskItemInvalidStatus(t *testing.T) {
	database := openTestDB(t)
	params, _ := json.Marshal(map[string]string{"id": "x", "status": "bogus"})
	_, err := handleUpdateDeskItem(context.Background(), database, nil, params)
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	if !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("expected invalid status error, got %v", err)
	}
}

func TestUpdateDeskItemMissingFields(t *testing.T) {
	database := openTestDB(t)
	params, _ := json.Marshal(map[string]string{"id": "x"})
	_, err := handleUpdateDeskItem(context.Background(), database, nil, params)
	if err == nil {
		t.Fatal("expected error for missing status")
	}
	if !strings.Contains(err.Error(), "id and status required") {
		t.Fatalf("expected missing fields error, got %v", err)
	}
}

func TestAddToNotebookStub(t *testing.T) {
	result, err := handleAddToNotebook(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "not yet available") {
		t.Fatalf("expected stub message, got %q", result)
	}
}

func TestSaveToInboxMissingTitle(t *testing.T) {
	params, _ := json.Marshal(map[string]string{"content": "stuff"})
	_, err := handleSaveToInbox(context.Background(), nil, nil, params)
	if err == nil {
		t.Fatal("expected error for missing title")
	}
	if !strings.Contains(err.Error(), "title and content required") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestSaveToInboxMissingContent(t *testing.T) {
	params, _ := json.Marshal(map[string]string{"title": "stuff"})
	_, err := handleSaveToInbox(context.Background(), nil, nil, params)
	if err == nil {
		t.Fatal("expected error for missing content")
	}
	if !strings.Contains(err.Error(), "title and content required") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestSaveToInboxNilBrain(t *testing.T) {
	database := openTestDB(t)
	params, _ := json.Marshal(map[string]interface{}{
		"title":   "Test idea",
		"content": "This is a test capture",
		"type":    "Idea",
	})
	result, err := handleSaveToInbox(context.Background(), database, nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Saved to inbox") {
		t.Fatalf("expected saved confirmation, got %q", result)
	}
	if !strings.Contains(result, "type=Idea") {
		t.Fatalf("expected type=Idea in result, got %q", result)
	}
}

func TestExecuteToolUnknown(t *testing.T) {
	_, err := ExecuteTool(context.Background(), "nonexistent", nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("expected unknown tool error, got %v", err)
	}
}
