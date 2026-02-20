package db

import (
	"database/sql"
	"testing"
)

func TestFindConsolidationCandidatesReturnsSimilarPairs(t *testing.T) {
	database := newTestDB(t)

	for _, mem := range []Memory{
		{ID: "fc-a", Type: "Fact", Title: "Go is great", Content: "Go is a great language", Importance: 0.8, Source: "test"},
		{ID: "fc-b", Type: "Fact", Title: "Go is awesome", Content: "Go is an awesome language", Importance: 0.7, Source: "test"},
	} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert %s: %v", mem.ID, err)
		}
	}

	database.UpsertEmbedding("fc-a", []float32{0.9, 0.1, 0.0}, "m")
	database.UpsertEmbedding("fc-b", []float32{0.89, 0.12, 0.01}, "m")

	candidates, err := database.FindConsolidationCandidates(0.5, 10)
	if err != nil {
		t.Fatalf("find candidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate pair, got %d", len(candidates))
	}
	pair := candidates[0]
	if pair.Similarity <= 0 {
		t.Fatalf("expected positive similarity, got %f", pair.Similarity)
	}
	ids := pair.MemoryA.ID + "|" + pair.MemoryB.ID
	if ids != "fc-a|fc-b" && ids != "fc-b|fc-a" {
		t.Fatalf("unexpected pair IDs: %s", ids)
	}
}

func TestFindConsolidationCandidatesDeduplicatesPairs(t *testing.T) {
	database := newTestDB(t)

	for _, mem := range []Memory{
		{ID: "dd-a", Type: "Fact", Title: "A", Content: "content a", Importance: 0.5, Source: "test"},
		{ID: "dd-b", Type: "Fact", Title: "B", Content: "content b", Importance: 0.5, Source: "test"},
	} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert %s: %v", mem.ID, err)
		}
	}

	database.UpsertEmbedding("dd-a", []float32{1, 0, 0}, "m")
	database.UpsertEmbedding("dd-b", []float32{0.99, 0.01, 0}, "m")

	candidates, err := database.FindConsolidationCandidates(0.5, 10)
	if err != nil {
		t.Fatalf("find candidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected exactly 1 pair (deduped), got %d", len(candidates))
	}
}

func TestFindConsolidationCandidatesSkipsSuppressed(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertMemory(Memory{
		ID: "ss-a", Type: "Fact", Title: "A", Content: "content a", Importance: 0.5, Source: "test",
	}); err != nil {
		t.Fatalf("insert ss-a: %v", err)
	}
	if err := database.InsertMemory(Memory{
		ID: "ss-b", Type: "Fact", Title: "B", Content: "content b", Importance: 0.5, Source: "test",
		SuppressedAt: sql.NullString{String: "2026-01-01T00:00:00Z", Valid: true},
	}); err != nil {
		t.Fatalf("insert ss-b: %v", err)
	}

	database.UpsertEmbedding("ss-a", []float32{1, 0, 0}, "m")
	database.UpsertEmbedding("ss-b", []float32{0.99, 0.01, 0}, "m")

	candidates, err := database.FindConsolidationCandidates(0.5, 10)
	if err != nil {
		t.Fatalf("find candidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates (suppressed filtered), got %d", len(candidates))
	}
}

func TestFindConsolidationCandidatesSkipsIdentityType(t *testing.T) {
	database := newTestDB(t)

	for _, mem := range []Memory{
		{ID: "id-a", Type: "Identity", Title: "I am Mike", Content: "identity stuff", Importance: 1.0, Source: "test"},
		{ID: "id-b", Type: "Fact", Title: "Mike info", Content: "some info about Mike", Importance: 0.5, Source: "test"},
	} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert %s: %v", mem.ID, err)
		}
	}

	database.UpsertEmbedding("id-a", []float32{1, 0, 0}, "m")
	database.UpsertEmbedding("id-b", []float32{0.99, 0.01, 0}, "m")

	candidates, err := database.FindConsolidationCandidates(0.5, 10)
	if err != nil {
		t.Fatalf("find candidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates (identity filtered), got %d", len(candidates))
	}
}

func TestFindConsolidationCandidatesEmptyEmbeddings(t *testing.T) {
	database := newTestDB(t)

	candidates, err := database.FindConsolidationCandidates(0.85, 10)
	if err != nil {
		t.Fatalf("find candidates: %v", err)
	}
	if candidates != nil {
		t.Fatalf("expected nil for empty embeddings, got %v", candidates)
	}
}

func TestFindConsolidationCandidatesRespectsMaxPairs(t *testing.T) {
	database := newTestDB(t)

	for _, mem := range []Memory{
		{ID: "mp-a", Type: "Fact", Title: "A", Content: "a", Importance: 0.5, Source: "test"},
		{ID: "mp-b", Type: "Fact", Title: "B", Content: "b", Importance: 0.5, Source: "test"},
		{ID: "mp-c", Type: "Fact", Title: "C", Content: "c", Importance: 0.5, Source: "test"},
	} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert %s: %v", mem.ID, err)
		}
	}

	database.UpsertEmbedding("mp-a", []float32{1, 0, 0}, "m")
	database.UpsertEmbedding("mp-b", []float32{0.99, 0.01, 0}, "m")
	database.UpsertEmbedding("mp-c", []float32{0.98, 0.02, 0}, "m")

	candidates, err := database.FindConsolidationCandidates(0.5, 1)
	if err != nil {
		t.Fatalf("find candidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate (maxPairs=1), got %d", len(candidates))
	}
}

func TestEnsureUndirectedEdgeExported(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "eue-a")
	insertTestMemory(t, database, "eue-b")

	created, err := database.EnsureUndirectedEdge("eue-a", "eue-b", "RelatedTo", 0.8)
	if err != nil {
		t.Fatalf("ensure edge: %v", err)
	}
	if !created {
		t.Fatal("expected edge to be created")
	}

	created2, err := database.EnsureUndirectedEdge("eue-a", "eue-b", "RelatedTo", 0.8)
	if err != nil {
		t.Fatalf("ensure edge again: %v", err)
	}
	if created2 {
		t.Fatal("expected no duplicate edge")
	}
}

func TestCountMemoryReferencesExported(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "cmr-mem")

	refs, err := database.CountMemoryReferences("cmr-mem")
	if err != nil {
		t.Fatalf("count refs: %v", err)
	}
	if refs != 0 {
		t.Fatalf("expected 0 refs, got %d", refs)
	}
}
