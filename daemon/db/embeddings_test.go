package db

import (
	"math"
	"testing"
)

func TestUpsertEmbeddingRoundTrip(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "emb-rt")

	vec := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	if err := database.UpsertEmbedding("emb-rt", vec, "test-model"); err != nil {
		t.Fatalf("upsert embedding: %v", err)
	}

	got, err := database.GetEmbedding("emb-rt")
	if err != nil {
		t.Fatalf("get embedding: %v", err)
	}
	if len(got) != len(vec) {
		t.Fatalf("expected %d floats, got %d", len(vec), len(got))
	}
	for i := range vec {
		if got[i] != vec[i] {
			t.Fatalf("index %d: expected %f, got %f", i, vec[i], got[i])
		}
	}
}

func TestUpsertEmbeddingOverwrites(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "emb-ow")

	if err := database.UpsertEmbedding("emb-ow", []float32{1, 2, 3}, "v1"); err != nil {
		t.Fatalf("upsert first: %v", err)
	}
	if err := database.UpsertEmbedding("emb-ow", []float32{4, 5, 6}, "v2"); err != nil {
		t.Fatalf("upsert second: %v", err)
	}

	got, err := database.GetEmbedding("emb-ow")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got[0] != 4 || got[1] != 5 || got[2] != 6 {
		t.Fatalf("expected [4,5,6], got %v", got)
	}
}

func TestGetEmbeddingNotFound(t *testing.T) {
	database := newTestDB(t)

	got, err := database.GetEmbedding("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing embedding, got %v", got)
	}
}

func TestListEmbeddings(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "list-a")
	insertTestMemory(t, database, "list-b")

	if err := database.UpsertEmbedding("list-a", []float32{1, 0, 0}, "m"); err != nil {
		t.Fatalf("upsert a: %v", err)
	}
	if err := database.UpsertEmbedding("list-b", []float32{0, 1, 0}, "m"); err != nil {
		t.Fatalf("upsert b: %v", err)
	}

	all, err := database.ListEmbeddings()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(all))
	}
	if all["list-a"][0] != 1 {
		t.Fatalf("expected list-a[0]=1, got %f", all["list-a"][0])
	}
	if all["list-b"][1] != 1 {
		t.Fatalf("expected list-b[1]=1, got %f", all["list-b"][1])
	}
}

func TestListEmbeddingsEmpty(t *testing.T) {
	database := newTestDB(t)

	all, err := database.ListEmbeddings()
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected 0 embeddings, got %d", len(all))
	}
}

func TestNearestNeighbors(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "nn-a")
	insertTestMemory(t, database, "nn-b")
	insertTestMemory(t, database, "nn-c")

	database.UpsertEmbedding("nn-a", []float32{1, 0, 0}, "m")
	database.UpsertEmbedding("nn-b", []float32{0.9, 0.1, 0}, "m")
	database.UpsertEmbedding("nn-c", []float32{0, 0, 1}, "m")

	results, err := database.NearestNeighbors([]float32{1, 0, 0}, 2, nil)
	if err != nil {
		t.Fatalf("nearest neighbors: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "nn-a" {
		t.Fatalf("expected nn-a first, got %s", results[0].ID)
	}
	if results[1].ID != "nn-b" {
		t.Fatalf("expected nn-b second, got %s", results[1].ID)
	}
}

func TestNearestNeighborsExclude(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "ex-a")
	insertTestMemory(t, database, "ex-b")

	database.UpsertEmbedding("ex-a", []float32{1, 0}, "m")
	database.UpsertEmbedding("ex-b", []float32{0.9, 0.1}, "m")

	results, err := database.NearestNeighbors([]float32{1, 0}, 10, []string{"ex-a"})
	if err != nil {
		t.Fatalf("nn with exclude: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "ex-b" {
		t.Fatalf("expected ex-b, got %s", results[0].ID)
	}
}

func TestNearestNeighborsEmpty(t *testing.T) {
	database := newTestDB(t)

	results, err := database.NearestNeighbors([]float32{1, 0, 0}, 5, nil)
	if err != nil {
		t.Fatalf("nn empty: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestCosineSimilarityParallel(t *testing.T) {
	a := []float32{1, 2, 3}
	sim := cosineSimilarity(a, a)
	if math.Abs(sim-1.0) > 1e-6 {
		t.Fatalf("expected 1.0 for parallel vectors, got %f", sim)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim) > 1e-6 {
		t.Fatalf("expected 0.0 for orthogonal vectors, got %f", sim)
	}
}

func TestCosineSimilarityOpposite(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{-1, 0}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim+1.0) > 1e-6 {
		t.Fatalf("expected -1.0 for opposite vectors, got %f", sim)
	}
}

func TestCosineSimilarityEmpty(t *testing.T) {
	sim := cosineSimilarity(nil, nil)
	if sim != 0 {
		t.Fatalf("expected 0 for empty vectors, got %f", sim)
	}
}

func TestCosineSimilarityMismatchedLength(t *testing.T) {
	sim := cosineSimilarity([]float32{1, 2}, []float32{1})
	if sim != 0 {
		t.Fatalf("expected 0 for mismatched lengths, got %f", sim)
	}
}

func TestFloat32BytesRoundTrip(t *testing.T) {
	original := []float32{3.14, -2.71, 0, 1e10, -1e-10}
	encoded := float32ToBytes(original)
	decoded := bytesToFloat32(encoded)

	if len(decoded) != len(original) {
		t.Fatalf("expected %d floats, got %d", len(original), len(decoded))
	}
	for i := range original {
		if decoded[i] != original[i] {
			t.Fatalf("index %d: expected %f, got %f", i, original[i], decoded[i])
		}
	}
}
