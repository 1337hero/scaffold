package ingestion

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"scaffold/db"
)

func newTestService(t *testing.T) (*Service, *db.DB, string) {
	t.Helper()

	tempDir := t.TempDir()
	database, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ingestDir := filepath.Join(tempDir, "ingest")
	svc, err := New(database, nil, ingestDir, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("new ingestion service: %v", err)
	}
	return svc, database, ingestDir
}

func TestChunkByLinesHonorsLineBoundaries(t *testing.T) {
	text := strings.Repeat("line one\n", 500)
	chunks := chunkByLines(text, 40)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	for _, chunk := range chunks {
		if len(chunk) > 40 {
			t.Fatalf("expected chunk length <= 40, got %d", len(chunk))
		}
		if !strings.HasSuffix(chunk, "\n") {
			t.Fatalf("expected chunk to end with newline boundary, got %q", chunk)
		}
	}
}

func TestChunkDocumentMarkdownSplitsBySections(t *testing.T) {
	text := `# Profile

## Who I Am
Details here.

## Active Goals
Goal one.

## Preferences
Direct and concise.
`

	chunks := chunkDocument("profile.md", text, 4000)
	if len(chunks) < 3 {
		t.Fatalf("expected markdown section chunking, got %d chunks", len(chunks))
	}
}

func TestChunkDocumentNonMarkdownFallsBackToLineChunking(t *testing.T) {
	text := strings.Repeat("line one\n", 50)
	chunks := chunkDocument("notes.txt", text, 40)
	if len(chunks) == 0 {
		t.Fatal("expected chunks for txt fallback")
	}
}

func TestUploadAndIngestNowCreatesMemoriesAndDeletesFile(t *testing.T) {
	svc, database, ingestDir := newTestService(t)

	content := "Business priority: reduce churn in Q2.\nCurrent blocker: onboarding friction.\n"
	uploaded, err := svc.Upload(context.Background(), "business-notes.md", strings.NewReader(content))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if !strings.HasPrefix(uploaded, ingestDir) {
		t.Fatalf("expected uploaded path inside ingest dir, got %s", uploaded)
	}

	if err := svc.IngestNow(context.Background()); err != nil {
		t.Fatalf("ingest now: %v", err)
	}

	if _, err := os.Stat(uploaded); !os.IsNotExist(err) {
		t.Fatalf("expected uploaded file to be deleted after success, got err=%v", err)
	}

	memories, err := database.ListRecentMemories(10)
	if err != nil {
		t.Fatalf("list memories: %v", err)
	}
	if len(memories) == 0 {
		t.Fatal("expected at least one ingested memory")
	}
	if !strings.HasPrefix(memories[0].Source, "ingest:") {
		t.Fatalf("expected ingest source, got %q", memories[0].Source)
	}
}

func TestIngestNowSkipsAlreadyCompletedChunk(t *testing.T) {
	svc, database, ingestDir := newTestService(t)

	path := filepath.Join(ingestDir, "dedupe.md")
	content := "Same text every run.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := svc.IngestNow(context.Background()); err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	memories, err := database.ListRecentMemories(50)
	if err != nil {
		t.Fatalf("list memories after first ingest: %v", err)
	}
	firstCount := len(memories)
	if firstCount == 0 {
		t.Fatal("expected at least one memory after first ingest")
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	if err := svc.IngestNow(context.Background()); err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	memories, err = database.ListRecentMemories(50)
	if err != nil {
		t.Fatalf("list memories after second ingest: %v", err)
	}
	if len(memories) != firstCount {
		t.Fatalf("expected memory count to remain %d, got %d", firstCount, len(memories))
	}
}
