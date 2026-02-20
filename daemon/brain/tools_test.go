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

type testEmbedder struct {
	available bool
	vector    []float32
}

func (e *testEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	if len(e.vector) > 0 {
		return e.vector, nil
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (e *testEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2, 0.3}
	}
	return out, nil
}

func (e *testEmbedder) Available(_ context.Context) bool {
	return e.available
}

func (e *testEmbedder) ModelName() string {
	return "test-model"
}

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

func TestSearchMemoriesTypeFilterAppliesToFTSPath(t *testing.T) {
	database := openTestDB(t)

	for _, mem := range []db.Memory{
		{
			ID:         "fts-type-idea",
			Type:       "Idea",
			Content:    "golang workflows for agent systems",
			Title:      "Idea memory",
			Importance: 0.7,
			Source:     "test",
		},
		{
			ID:         "fts-type-fact",
			Type:       "Fact",
			Content:    "golang workflows for daemon systems",
			Title:      "Fact memory",
			Importance: 0.7,
			Source:     "test",
		},
	} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert memory %s: %v", mem.ID, err)
		}
	}

	params, _ := json.Marshal(map[string]string{
		"query": "golang",
		"type":  "Idea",
	})
	result, err := handleSearchMemories(context.Background(), database, nil, params)
	if err != nil {
		t.Fatalf("search memories: %v", err)
	}
	if !strings.Contains(result, "Idea memory") {
		t.Fatalf("expected Idea memory in result, got %q", result)
	}
	if strings.Contains(result, "Fact memory") {
		t.Fatalf("expected Fact memory to be filtered out, got %q", result)
	}
}

func TestSearchMemoriesTypeFilterAppliesToHybridPath(t *testing.T) {
	database := openTestDB(t)

	for _, mem := range []db.Memory{
		{
			ID:         "hybrid-type-idea",
			Type:       "Idea",
			Content:    "semantic embedding test idea",
			Title:      "Hybrid Idea",
			Importance: 0.7,
			Source:     "test",
		},
		{
			ID:         "hybrid-type-fact",
			Type:       "Fact",
			Content:    "semantic embedding test fact",
			Title:      "Hybrid Fact",
			Importance: 0.7,
			Source:     "test",
		},
	} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert memory %s: %v", mem.ID, err)
		}
	}
	if err := database.UpsertEmbedding("hybrid-type-idea", []float32{1, 0, 0}, "test-model"); err != nil {
		t.Fatalf("upsert idea embedding: %v", err)
	}
	if err := database.UpsertEmbedding("hybrid-type-fact", []float32{0.9, 0.1, 0}, "test-model"); err != nil {
		t.Fatalf("upsert fact embedding: %v", err)
	}

	b := &Brain{
		embedder: &testEmbedder{
			available: true,
			vector:    []float32{1, 0, 0},
		},
	}

	params, _ := json.Marshal(map[string]string{
		"query": "nonexistentftsquery",
		"type":  "Idea",
	})
	result, err := handleSearchMemories(context.Background(), database, b, params)
	if err != nil {
		t.Fatalf("search memories: %v", err)
	}
	if !strings.Contains(result, "Hybrid Idea") {
		t.Fatalf("expected Hybrid Idea in result, got %q", result)
	}
	if strings.Contains(result, "Hybrid Fact") {
		t.Fatalf("expected Hybrid Fact to be filtered out, got %q", result)
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

func TestSearchMemoriesNilEmbedderFallsBackToSubstring(t *testing.T) {
	database := openTestDB(t)

	err := database.InsertMemory(db.Memory{
		ID:         "mem-fallback",
		Type:       "Fact",
		Content:    "Rust is a systems programming language",
		Title:      "Rust fact",
		Importance: 0.8,
		Source:     "test",
		Tags:       "rust,systems",
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	b := &Brain{}
	params, _ := json.Marshal(map[string]string{"query": "rust"})
	result, err := handleSearchMemories(context.Background(), database, b, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Rust fact") {
		t.Fatalf("expected substring fallback to find memory, got %q", result)
	}
}

func TestSearchMemoriesNilBrainFallsBack(t *testing.T) {
	database := openTestDB(t)

	err := database.InsertMemory(db.Memory{
		ID:         "mem-nilbrain",
		Type:       "Todo",
		Content:    "Buy coffee beans",
		Title:      "Coffee task",
		Importance: 0.6,
		Source:     "test",
		Tags:       "shopping",
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	params, _ := json.Marshal(map[string]string{"query": "coffee"})
	result, err := handleSearchMemories(context.Background(), database, nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Coffee task") {
		t.Fatalf("expected nil brain fallback to find memory, got %q", result)
	}
}

func TestFormatSearchResultsWithScores(t *testing.T) {
	results := []db.ScoredMemory{
		{
			Memory: db.Memory{
				Type:       "Fact",
				Title:      "Go concurrency",
				Content:    "Go uses goroutines for concurrent execution",
				Importance: 0.9,
			},
			FTSScore:    0.75,
			VectorScore: 0.88,
			FusedScore:  0.83,
		},
		{
			Memory: db.Memory{
				Type:       "Todo",
				Title:      "same title",
				Content:    "same title",
				Importance: 0.5,
			},
		},
	}

	output := formatSearchResults("concurrency", results)
	if !strings.Contains(output, `Found 2 memories matching "concurrency"`) {
		t.Fatalf("expected header, got %q", output)
	}
	if !strings.Contains(output, "score: 0.830 (fts=0.750 vec=0.880)") {
		t.Fatalf("expected score line for fused result, got %q", output)
	}
	if strings.Count(output, "score:") != 1 {
		t.Fatalf("expected exactly one score line (zero FusedScore should be omitted), got %q", output)
	}
	if !strings.Contains(output, "Go uses goroutines") {
		t.Fatalf("expected content snippet, got %q", output)
	}
	if strings.Contains(output, "   same title\n") {
		t.Fatalf("should not show content when it matches title, got %q", output)
	}
}

func TestSearchMemoriesFTSPath(t *testing.T) {
	database := openTestDB(t)

	for _, m := range []db.Memory{
		{ID: "mem-fts1", Type: "Todo", Content: "Deploy the application to production", Title: "Deploy app", Importance: 0.8, Source: "test", Tags: "infra"},
		{ID: "mem-fts2", Type: "Fact", Content: "Production server runs on port 8080", Title: "Server port", Importance: 0.6, Source: "test", Tags: "infra"},
	} {
		if err := database.InsertMemory(m); err != nil {
			t.Fatalf("insert memory: %v", err)
		}
	}

	params, _ := json.Marshal(map[string]string{"query": "production"})
	result, err := handleSearchMemories(context.Background(), database, nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Found 2") {
		t.Fatalf("expected FTS to find both memories, got %q", result)
	}
	if !strings.Contains(result, "Deploy app") || !strings.Contains(result, "Server port") {
		t.Fatalf("expected both memories in FTS results, got %q", result)
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
