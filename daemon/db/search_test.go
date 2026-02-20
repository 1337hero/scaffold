package db

import (
	"database/sql"
	"testing"
	"time"
)

func TestSearchFTS_BasicMatch(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertMemory(Memory{
		ID:         "fts-basic",
		Type:       "fact",
		Content:    "the quick brown fox jumps over the lazy dog",
		Title:      "fox story",
		Importance: 0.7,
		Source:     "test",
		Tags:       "animals,proverbs",
	}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	results, err := database.SearchFTS("fox", 10)
	if err != nil {
		t.Fatalf("search fts: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "fts-basic" {
		t.Fatalf("expected fts-basic, got %s", results[0].ID)
	}
	if results[0].FTSScore <= 0 {
		t.Fatalf("expected positive FTS score, got %f", results[0].FTSScore)
	}
}

func TestSearchFTS_EmptyQuery(t *testing.T) {
	database := newTestDB(t)

	_, err := database.SearchFTS("", 10)
	if err == nil {
		t.Fatal("expected error for empty query")
	}

	_, err = database.SearchFTS("   ", 10)
	if err == nil {
		t.Fatal("expected error for whitespace-only query")
	}
}

func TestSearchFTS_SuppressedExcluded(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertMemory(Memory{
		ID:         "fts-visible",
		Type:       "fact",
		Content:    "golang concurrency patterns",
		Title:      "go patterns",
		Importance: 0.7,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert visible: %v", err)
	}

	if err := database.InsertMemory(Memory{
		ID:         "fts-suppressed",
		Type:       "fact",
		Content:    "golang error handling patterns",
		Title:      "go errors",
		Importance: 0.6,
		Source:     "test",
		SuppressedAt: sql.NullString{
			String: time.Now().Format(time.RFC3339),
			Valid:  true,
		},
	}); err != nil {
		t.Fatalf("insert suppressed: %v", err)
	}

	results, err := database.SearchFTS("golang", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (suppressed excluded), got %d", len(results))
	}
	if results[0].ID != "fts-visible" {
		t.Fatalf("expected fts-visible, got %s", results[0].ID)
	}
}

func TestSearchFTS_NoResults(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertMemory(Memory{
		ID:         "fts-nomatch",
		Type:       "fact",
		Content:    "something completely different",
		Title:      "different",
		Importance: 0.5,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	results, err := database.SearchFTS("zyxwvutsrqp", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestSearchHybrid_FTSOnly(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertMemory(Memory{
		ID:         "hybrid-fts",
		Type:       "fact",
		Content:    "machine learning transformers attention mechanism",
		Title:      "ml notes",
		Importance: 0.8,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	results, err := database.SearchHybrid("transformers", nil, "", 10)
	if err != nil {
		t.Fatalf("hybrid search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "hybrid-fts" {
		t.Fatalf("expected hybrid-fts, got %s", results[0].ID)
	}
	if results[0].VectorScore != 0 {
		t.Fatalf("expected vector score 0 with nil embedding, got %f", results[0].VectorScore)
	}
}

func TestSearchHybrid_VectorOnly(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertMemory(Memory{
		ID:         "vec-only",
		Type:       "fact",
		Content:    "alpha beta gamma",
		Title:      "greek",
		Importance: 0.7,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := database.UpsertEmbedding("vec-only", []float32{1, 0, 0}, "test-model"); err != nil {
		t.Fatalf("upsert embedding: %v", err)
	}

	results, err := database.SearchHybrid("zyxwvutsrqp", []float32{1, 0, 0}, "test-model", 10)
	if err != nil {
		t.Fatalf("hybrid search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result from vector, got %d", len(results))
	}
	if results[0].ID != "vec-only" {
		t.Fatalf("expected vec-only, got %s", results[0].ID)
	}
	if results[0].VectorScore <= 0 {
		t.Fatalf("expected positive vector score, got %f", results[0].VectorScore)
	}
}

func TestSearchHybrid_Fusion(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertMemory(Memory{
		ID:         "fusion-both",
		Type:       "fact",
		Content:    "rust ownership borrow checker",
		Title:      "rust notes",
		Importance: 0.8,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert fusion-both: %v", err)
	}
	if err := database.UpsertEmbedding("fusion-both", []float32{0.9, 0.1, 0}, "test-model"); err != nil {
		t.Fatalf("upsert embedding fusion-both: %v", err)
	}

	if err := database.InsertMemory(Memory{
		ID:         "fusion-fts-only",
		Type:       "fact",
		Content:    "rust cargo package manager",
		Title:      "cargo notes",
		Importance: 0.6,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert fusion-fts-only: %v", err)
	}

	if err := database.InsertMemory(Memory{
		ID:         "fusion-vec-only",
		Type:       "fact",
		Content:    "completely unrelated content here",
		Title:      "unrelated",
		Importance: 0.5,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert fusion-vec-only: %v", err)
	}
	if err := database.UpsertEmbedding("fusion-vec-only", []float32{0.8, 0.2, 0}, "test-model"); err != nil {
		t.Fatalf("upsert embedding fusion-vec-only: %v", err)
	}

	results, err := database.SearchHybrid("rust", []float32{1, 0, 0}, "test-model", 10)
	if err != nil {
		t.Fatalf("hybrid search: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// fusion-both should rank highest: it has both FTS and vector scores
	if results[0].ID != "fusion-both" {
		t.Fatalf("expected fusion-both ranked first, got %s", results[0].ID)
	}
	if results[0].FTSScore <= 0 {
		t.Fatalf("expected positive FTS score for fusion-both, got %f", results[0].FTSScore)
	}
	if results[0].VectorScore <= 0 {
		t.Fatalf("expected positive vector score for fusion-both, got %f", results[0].VectorScore)
	}
	if results[0].FusedScore <= 0 {
		t.Fatalf("expected positive fused score, got %f", results[0].FusedScore)
	}

	// Verify fused score is the weighted combination
	expected := ftsWeight*results[0].FTSScore + vectorWeight*results[0].VectorScore
	diff := results[0].FusedScore - expected
	if diff > 0.001 || diff < -0.001 {
		t.Fatalf("expected fused score %f, got %f", expected, results[0].FusedScore)
	}
}

func TestSearchHybrid_EmptyQuery(t *testing.T) {
	database := newTestDB(t)

	_, err := database.SearchHybrid("", nil, "", 10)
	if err == nil {
		t.Fatal("expected error for empty query")
	}

	_, err = database.SearchHybrid("  ", []float32{1, 0}, "test-model", 10)
	if err == nil {
		t.Fatal("expected error for whitespace-only query")
	}
}

func TestEscapeFTSQuery(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", `"hello" "world"`},
		{"single", `"single"`},
		{"", ""},
		{"   ", ""},
		{`with "quotes" inside`, `"with" "quotes" "inside"`},
		{"special AND OR NOT", `"special" "AND" "OR" "NOT"`},
		{`"already quoted"`, `"already" "quoted"`},
	}

	for _, tt := range tests {
		got := escapeFTSQuery(tt.input)
		if got != tt.want {
			t.Errorf("escapeFTSQuery(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSearchFTS_MultipleResults_Ranked(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertMemory(Memory{
		ID:         "rank-a",
		Type:       "fact",
		Content:    "database indexing strategies for postgresql",
		Title:      "database indexing",
		Importance: 0.8,
		Source:     "test",
		Tags:       "database",
	}); err != nil {
		t.Fatalf("insert rank-a: %v", err)
	}

	if err := database.InsertMemory(Memory{
		ID:         "rank-b",
		Type:       "fact",
		Content:    "cooking recipe for pasta with tomato sauce",
		Title:      "pasta recipe",
		Importance: 0.5,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert rank-b: %v", err)
	}

	results, err := database.SearchFTS("database", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'database', got %d", len(results))
	}
	if results[0].ID != "rank-a" {
		t.Fatalf("expected rank-a, got %s", results[0].ID)
	}
}

func TestSearchHybrid_FiltersVectorByEmbeddingModel(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertMemory(Memory{
		ID:         "hybrid-model-a",
		Type:       "fact",
		Content:    "model A memory content",
		Title:      "model a",
		Importance: 0.8,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert model-a memory: %v", err)
	}
	if err := database.InsertMemory(Memory{
		ID:         "hybrid-model-b",
		Type:       "fact",
		Content:    "model B memory content",
		Title:      "model b",
		Importance: 0.8,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert model-b memory: %v", err)
	}

	if err := database.UpsertEmbedding("hybrid-model-a", []float32{1, 0, 0}, "model-a"); err != nil {
		t.Fatalf("upsert model-a embedding: %v", err)
	}
	if err := database.UpsertEmbedding("hybrid-model-b", []float32{0.99, 0.01, 0}, "model-b"); err != nil {
		t.Fatalf("upsert model-b embedding: %v", err)
	}

	results, err := database.SearchHybrid("noftsquerytoken", []float32{1, 0, 0}, "model-a", 10)
	if err != nil {
		t.Fatalf("hybrid search with model filter: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for model-a, got %d", len(results))
	}
	if results[0].ID != "hybrid-model-a" {
		t.Fatalf("expected hybrid-model-a, got %s", results[0].ID)
	}
}
