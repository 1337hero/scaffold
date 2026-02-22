package db

import (
	"testing"
)

func TestUpsertIngestionChunkAndCompletionCheck(t *testing.T) {
	database := newTestDB(t)

	done, err := database.IsIngestionChunkCompleted("missing")
	if err != nil {
		t.Fatalf("check missing chunk completion: %v", err)
	}
	if done {
		t.Fatal("expected missing chunk to be incomplete")
	}

	if err := database.UpsertIngestionChunk(IngestionChunk{
		ChunkHash:  "chunk-1",
		FilePath:   "/tmp/doc.md",
		FileHash:   "file-hash-1",
		ChunkIndex: 0,
		Status:     "processing",
	}); err != nil {
		t.Fatalf("insert chunk progress: %v", err)
	}

	done, err = database.IsIngestionChunkCompleted("chunk-1")
	if err != nil {
		t.Fatalf("check processing chunk completion: %v", err)
	}
	if done {
		t.Fatal("expected processing chunk to be incomplete")
	}

	if err := database.UpsertIngestionChunk(IngestionChunk{
		ChunkHash:  "chunk-1",
		FilePath:   "/tmp/doc.md",
		FileHash:   "file-hash-1",
		ChunkIndex: 0,
		Status:     "completed",
	}); err != nil {
		t.Fatalf("update chunk progress: %v", err)
	}

	done, err = database.IsIngestionChunkCompleted("chunk-1")
	if err != nil {
		t.Fatalf("check completed chunk completion: %v", err)
	}
	if !done {
		t.Fatal("expected completed chunk to be complete")
	}
}

func TestUpsertIngestionFileAndList(t *testing.T) {
	database := newTestDB(t)

	if err := database.UpsertIngestionFile(IngestionFile{
		Path:            "/tmp/doc.md",
		FileHash:        "hash-1",
		Status:          "processing",
		TotalChunks:     4,
		ProcessedChunks: 1,
	}); err != nil {
		t.Fatalf("insert ingestion file: %v", err)
	}

	if err := database.UpsertIngestionFile(IngestionFile{
		Path:            "/tmp/doc.md",
		FileHash:        "hash-1",
		Status:          "completed",
		TotalChunks:     4,
		ProcessedChunks: 4,
	}); err != nil {
		t.Fatalf("update ingestion file: %v", err)
	}

	files, err := database.ListIngestionFiles(10)
	if err != nil {
		t.Fatalf("list ingestion files: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 ingestion file, got %d", len(files))
	}
	if files[0].Status != "completed" {
		t.Fatalf("expected status completed, got %q", files[0].Status)
	}
	if !files[0].CompletedAt.Valid {
		t.Fatal("expected completed_at to be set")
	}
}
