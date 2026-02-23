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
	if capture.Processed != 0 {
		t.Fatalf("expected processed=0 (triage no longer marks processed), got %d", capture.Processed)
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

	jobs, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue embedding jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 embedding job, got %d", len(jobs))
	}
	if jobs[0].MemoryID != "mem-persist" {
		t.Fatalf("expected embedding job for mem-persist, got %q", jobs[0].MemoryID)
	}
	if jobs[0].Reason != "triage" {
		t.Fatalf("expected embedding job reason triage, got %q", jobs[0].Reason)
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

	jobs, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue embedding jobs after rollback: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 embedding jobs after rollback, got %d", len(jobs))
	}
}

func TestInsertProcessedCaptureWithMemory(t *testing.T) {
	database := newTestDB(t)

	mem := Memory{
		ID:         "mem-tool-persist",
		Type:       "Idea",
		Content:    "Atomic save from tool",
		Title:      "Tool save",
		Importance: 0.7,
		Source:     "agent",
		Tags:       "tool,atomic",
	}

	captureID, err := database.InsertProcessedCaptureWithMemory("Atomic save from tool", "agent", "reference", mem, "agent_tool")
	if err != nil {
		t.Fatalf("insert processed capture with memory: %v", err)
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
	if !capture.MemoryID.Valid || capture.MemoryID.String != "mem-tool-persist" {
		t.Fatalf("expected memory_id mem-tool-persist, got %+v", capture.MemoryID)
	}
	if !capture.TriageAction.Valid || capture.TriageAction.String != "reference" {
		t.Fatalf("expected triage_action reference, got %+v", capture.TriageAction)
	}

	stored, err := database.GetMemory("mem-tool-persist")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if stored == nil {
		t.Fatal("expected persisted memory")
	}

	jobs, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue embedding jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 embedding job, got %d", len(jobs))
	}
	if jobs[0].MemoryID != "mem-tool-persist" {
		t.Fatalf("expected embedding job for mem-tool-persist, got %q", jobs[0].MemoryID)
	}
	if jobs[0].Reason != "agent_tool" {
		t.Fatalf("expected embedding job reason agent_tool, got %q", jobs[0].Reason)
	}
}

func TestInsertProcessedCaptureWithMemoryRollsBackOnMemoryConflict(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "mem-conflict")

	mem := Memory{
		ID:         "mem-conflict",
		Type:       "Fact",
		Content:    "conflict",
		Title:      "conflict",
		Importance: 0.5,
		Source:     "agent",
	}
	_, err := database.InsertProcessedCaptureWithMemory("conflict raw", "agent", "reference", mem, "agent_tool")
	if err == nil {
		t.Fatal("expected insert processed capture with memory to fail on duplicate memory id")
	}

	captures, err := database.ListRecent(10)
	if err != nil {
		t.Fatalf("list recent captures: %v", err)
	}
	if len(captures) != 0 {
		t.Fatalf("expected no captures after rollback, got %d", len(captures))
	}
}

func TestMarkMemoriesAccessed(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "mem-access")

	if err := database.MarkMemoriesAccessed([]string{"mem-access", "mem-access", " ", "missing"}); err != nil {
		t.Fatalf("mark memories accessed: %v", err)
	}

	mem, err := database.GetMemory("mem-access")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if mem == nil {
		t.Fatal("expected memory")
	}
	if mem.AccessCount != 1 {
		t.Fatalf("expected access_count 1, got %d", mem.AccessCount)
	}
	if mem.AccessedAt == "" {
		t.Fatal("expected accessed_at to be set")
	}

	if err := database.MarkMemoriesAccessed([]string{"mem-access"}); err != nil {
		t.Fatalf("mark memories accessed second time: %v", err)
	}
	mem, err = database.GetMemory("mem-access")
	if err != nil {
		t.Fatalf("get memory after second access: %v", err)
	}
	if mem.AccessCount != 2 {
		t.Fatalf("expected access_count 2, got %d", mem.AccessCount)
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

func TestPruneSuppressedMemoriesAppliesGuards(t *testing.T) {
	database := newTestDB(t)
	oldSuppressedAt := time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339)

	if err := database.InsertMemory(Memory{
		ID:         "mem-active",
		Type:       "Fact",
		Content:    "active memory",
		Title:      "Active",
		Importance: 0.6,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert active memory: %v", err)
	}

	if err := database.InsertMemory(Memory{
		ID:         "mem-edge-guard",
		Type:       "Fact",
		Content:    "has active edge",
		Title:      "Edge Guard",
		Importance: 0.5,
		Source:     "test",
		SuppressedAt: sql.NullString{
			String: oldSuppressedAt,
			Valid:  true,
		},
	}); err != nil {
		t.Fatalf("insert edge-guard memory: %v", err)
	}
	if err := database.InsertEdge(Edge{
		FromID:   "mem-edge-guard",
		ToID:     "mem-active",
		Relation: "RelatedTo",
	}); err != nil {
		t.Fatalf("insert active edge: %v", err)
	}

	if err := database.InsertMemory(Memory{
		ID:         "mem-desk-ref",
		Type:       "Todo",
		Content:    "used by desk",
		Title:      "Desk Ref",
		Importance: 0.5,
		Source:     "test",
		SuppressedAt: sql.NullString{
			String: oldSuppressedAt,
			Valid:  true,
		},
	}); err != nil {
		t.Fatalf("insert desk-ref memory: %v", err)
	}
	if err := database.InsertDeskItem(DeskItem{
		ID: "desk-ref-item",
		MemoryID: sql.NullString{
			String: "mem-desk-ref",
			Valid:  true,
		},
		Title:    "Desk item",
		Position: 1,
		Status:   "active",
		Date:     today(),
	}); err != nil {
		t.Fatalf("insert desk item ref: %v", err)
	}

	if err := database.InsertMemory(Memory{
		ID:         "mem-capture-ref",
		Type:       "Fact",
		Content:    "used by capture",
		Title:      "Capture Ref",
		Importance: 0.5,
		Source:     "test",
		SuppressedAt: sql.NullString{
			String: oldSuppressedAt,
			Valid:  true,
		},
	}); err != nil {
		t.Fatalf("insert capture-ref memory: %v", err)
	}
	captureID, err := database.InsertCapture("capture for ref", "test")
	if err != nil {
		t.Fatalf("insert capture: %v", err)
	}
	if err := database.UpdateTriage(captureID, "reference", "mem-capture-ref"); err != nil {
		t.Fatalf("link capture ref: %v", err)
	}

	for _, id := range []string{"mem-prune-a", "mem-prune-b"} {
		if err := database.InsertMemory(Memory{
			ID:         id,
			Type:       "Observation",
			Content:    "old suppressed",
			Title:      id,
			Importance: 0.2,
			Source:     "test",
			SuppressedAt: sql.NullString{
				String: oldSuppressedAt,
				Valid:  true,
			},
		}); err != nil {
			t.Fatalf("insert prune candidate %s: %v", id, err)
		}
	}
	if err := database.InsertEdge(Edge{
		FromID:   "mem-prune-a",
		ToID:     "mem-prune-b",
		Relation: "RelatedTo",
	}); err != nil {
		t.Fatalf("insert inactive edge: %v", err)
	}

	report, err := database.PruneSuppressedMemories(30)
	if err != nil {
		t.Fatalf("prune suppressed memories: %v", err)
	}

	if report.Candidates != 5 {
		t.Fatalf("expected 5 candidates, got %d", report.Candidates)
	}
	if report.Deleted != 2 {
		t.Fatalf("expected 2 deleted, got %d", report.Deleted)
	}
	if report.SkippedActiveEdges != 1 {
		t.Fatalf("expected 1 skipped active edge, got %d", report.SkippedActiveEdges)
	}
	if report.SkippedReferences != 2 {
		t.Fatalf("expected 2 skipped references, got %d", report.SkippedReferences)
	}
	if report.EdgeRowsDeleted < 1 {
		t.Fatalf("expected at least 1 edge row deleted, got %d", report.EdgeRowsDeleted)
	}

	for _, keepID := range []string{"mem-edge-guard", "mem-desk-ref", "mem-capture-ref"} {
		mem, err := database.GetMemory(keepID)
		if err != nil {
			t.Fatalf("get memory %s: %v", keepID, err)
		}
		if mem == nil {
			t.Fatalf("expected memory %s to remain after prune", keepID)
		}
	}

	for _, deletedID := range []string{"mem-prune-a", "mem-prune-b"} {
		mem, err := database.GetMemory(deletedID)
		if err != nil {
			t.Fatalf("get memory %s: %v", deletedID, err)
		}
		if mem != nil {
			t.Fatalf("expected memory %s to be pruned", deletedID)
		}
	}
}

func TestPruneSuppressedMemoriesHonorsSuppressedAge(t *testing.T) {
	database := newTestDB(t)

	if err := database.InsertMemory(Memory{
		ID:         "mem-fresh-suppressed",
		Type:       "Observation",
		Content:    "recently suppressed",
		Title:      "Fresh Suppressed",
		Importance: 0.2,
		Source:     "test",
		SuppressedAt: sql.NullString{
			String: time.Now().UTC().AddDate(0, 0, -5).Format(time.RFC3339),
			Valid:  true,
		},
	}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	report, err := database.PruneSuppressedMemories(30)
	if err != nil {
		t.Fatalf("prune suppressed memories: %v", err)
	}
	if report.Candidates != 0 || report.Deleted != 0 {
		t.Fatalf("expected no candidates/deletes, got %+v", report)
	}

	mem, err := database.GetMemory("mem-fresh-suppressed")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if mem == nil {
		t.Fatal("expected fresh suppressed memory to remain")
	}
}

func TestDecayMemoriesRespectsExemptTypesAndAge(t *testing.T) {
	database := newTestDB(t)
	oldAccessed := time.Now().UTC().AddDate(0, 0, -45).Format(time.RFC3339)
	recentAccessed := time.Now().UTC().AddDate(0, 0, -2).Format(time.RFC3339)

	if err := database.InsertMemory(Memory{
		ID:         "decay-old",
		Type:       "Fact",
		Content:    "old accessed",
		Title:      "Old",
		Importance: 0.8,
		Source:     "test",
		AccessedAt: oldAccessed,
	}); err != nil {
		t.Fatalf("insert decay-old: %v", err)
	}
	if err := database.InsertMemory(Memory{
		ID:         "decay-recent",
		Type:       "Fact",
		Content:    "recent accessed",
		Title:      "Recent",
		Importance: 0.8,
		Source:     "test",
		AccessedAt: recentAccessed,
	}); err != nil {
		t.Fatalf("insert decay-recent: %v", err)
	}
	if err := database.InsertMemory(Memory{
		ID:         "decay-exempt",
		Type:       "Identity",
		Content:    "identity",
		Title:      "Identity",
		Importance: 0.8,
		Source:     "test",
		AccessedAt: oldAccessed,
	}); err != nil {
		t.Fatalf("insert decay-exempt: %v", err)
	}

	report, err := database.DecayMemories(0.5, []string{"Identity"}, 0.1, 30)
	if err != nil {
		t.Fatalf("decay memories: %v", err)
	}
	if report.Updated != 1 {
		t.Fatalf("expected 1 updated memory, got %d", report.Updated)
	}

	oldMem, err := database.GetMemory("decay-old")
	if err != nil {
		t.Fatalf("get decay-old: %v", err)
	}
	if oldMem == nil || oldMem.Importance != 0.4 {
		t.Fatalf("expected decay-old importance 0.4, got %+v", oldMem)
	}

	recentMem, err := database.GetMemory("decay-recent")
	if err != nil {
		t.Fatalf("get decay-recent: %v", err)
	}
	if recentMem == nil || recentMem.Importance != 0.8 {
		t.Fatalf("expected decay-recent unchanged at 0.8, got %+v", recentMem)
	}

	exemptMem, err := database.GetMemory("decay-exempt")
	if err != nil {
		t.Fatalf("get decay-exempt: %v", err)
	}
	if exemptMem == nil || exemptMem.Importance != 0.8 {
		t.Fatalf("expected decay-exempt unchanged at 0.8, got %+v", exemptMem)
	}
}

func TestConsolidateMemoriesCreatesEdgeAndSuppressesUnreferencedDuplicate(t *testing.T) {
	database := newTestDB(t)

	canonical := Memory{
		ID:         "consolidate-canonical",
		Type:       "Fact",
		Title:      "Same thought",
		Content:    "Same thought content",
		Importance: 0.9,
		Source:     "test",
		CreatedAt:  "2026-01-01T00:00:00Z",
		UpdatedAt:  "2026-01-01T00:00:00Z",
	}
	duplicate := Memory{
		ID:         "consolidate-dup",
		Type:       "Fact",
		Title:      "Same thought",
		Content:    "Same thought content",
		Importance: 0.5,
		Source:     "test",
		CreatedAt:  "2026-01-02T00:00:00Z",
		UpdatedAt:  "2026-01-02T00:00:00Z",
	}
	referencedDup := Memory{
		ID:         "consolidate-ref",
		Type:       "Fact",
		Title:      "Same thought",
		Content:    "Same thought content",
		Importance: 0.4,
		Source:     "test",
		CreatedAt:  "2026-01-03T00:00:00Z",
		UpdatedAt:  "2026-01-03T00:00:00Z",
	}
	for _, mem := range []Memory{canonical, duplicate, referencedDup} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert memory %s: %v", mem.ID, err)
		}
	}

	captureID, err := database.InsertCapture("capture", "test")
	if err != nil {
		t.Fatalf("insert capture: %v", err)
	}
	if err := database.UpdateTriage(captureID, "reference", "consolidate-ref"); err != nil {
		t.Fatalf("update capture triage ref: %v", err)
	}

	report, err := database.ConsolidateMemories()
	if err != nil {
		t.Fatalf("consolidate memories: %v", err)
	}
	if report.GroupsFound != 1 {
		t.Fatalf("expected 1 group, got %d", report.GroupsFound)
	}
	if report.DuplicatesFound != 2 {
		t.Fatalf("expected 2 duplicates, got %d", report.DuplicatesFound)
	}
	if report.EdgesCreated < 2 {
		t.Fatalf("expected at least 2 edges created, got %d", report.EdgesCreated)
	}
	if report.MemoriesSuppressed != 1 {
		t.Fatalf("expected 1 suppressed duplicate, got %d", report.MemoriesSuppressed)
	}
	if report.SkippedReferenced != 1 {
		t.Fatalf("expected 1 referenced duplicate skipped, got %d", report.SkippedReferenced)
	}

	dupMem, err := database.GetMemory("consolidate-dup")
	if err != nil {
		t.Fatalf("get consolidate-dup: %v", err)
	}
	if dupMem == nil || !dupMem.SuppressedAt.Valid {
		t.Fatalf("expected consolidate-dup suppressed, got %+v", dupMem)
	}

	refMem, err := database.GetMemory("consolidate-ref")
	if err != nil {
		t.Fatalf("get consolidate-ref: %v", err)
	}
	if refMem == nil || refMem.SuppressedAt.Valid {
		t.Fatalf("expected consolidate-ref to remain unsuppressed, got %+v", refMem)
	}
}

func TestReindexMemoryCentralityCreatesScores(t *testing.T) {
	database := newTestDB(t)

	for _, mem := range []Memory{
		{ID: "central-a", Type: "Fact", Title: "A", Content: "A", Importance: 0.5, Source: "test"},
		{ID: "central-b", Type: "Fact", Title: "B", Content: "B", Importance: 0.5, Source: "test"},
		{ID: "central-c", Type: "Fact", Title: "C", Content: "C", Importance: 0.5, Source: "test"},
	} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert %s: %v", mem.ID, err)
		}
	}
	if err := database.InsertEdge(Edge{FromID: "central-a", ToID: "central-b", Relation: "RelatedTo"}); err != nil {
		t.Fatalf("insert edge ab: %v", err)
	}
	if err := database.InsertEdge(Edge{FromID: "central-b", ToID: "central-c", Relation: "RelatedTo"}); err != nil {
		t.Fatalf("insert edge bc: %v", err)
	}

	report, err := database.ReindexMemoryCentrality()
	if err != nil {
		t.Fatalf("reindex memory centrality: %v", err)
	}
	if report.MemoriesIndexed != 3 {
		t.Fatalf("expected 3 indexed memories, got %d", report.MemoriesIndexed)
	}
	if report.MaxDegree != 2 {
		t.Fatalf("expected max degree 2, got %d", report.MaxDegree)
	}

	row := database.conn.QueryRow(`SELECT score FROM memory_centrality WHERE memory_id = ?`, "central-b")
	var score float64
	if err := row.Scan(&score); err != nil {
		t.Fatalf("scan centrality score: %v", err)
	}
	if score != 1 {
		t.Fatalf("expected central-b score 1, got %.4f", score)
	}
}

func TestEnqueueEmbeddingJobCreatesJob(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "emb-enqueue")

	if err := database.EnqueueEmbeddingJob("emb-enqueue", "insert"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	jobs, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].MemoryID != "emb-enqueue" {
		t.Fatalf("expected memory_id emb-enqueue, got %s", jobs[0].MemoryID)
	}
	if jobs[0].Reason != "insert" {
		t.Fatalf("expected reason insert, got %s", jobs[0].Reason)
	}
	if jobs[0].Attempts != 0 {
		t.Fatalf("expected 0 attempts, got %d", jobs[0].Attempts)
	}
}

func TestEnqueueEmbeddingJobUpserts(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "emb-upsert")

	if err := database.EnqueueEmbeddingJob("emb-upsert", "insert"); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	if err := database.EnqueueEmbeddingJob("emb-upsert", "update"); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}

	jobs, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job after upsert, got %d", len(jobs))
	}
	if jobs[0].Reason != "update" {
		t.Fatalf("expected reason update after upsert, got %s", jobs[0].Reason)
	}
}

func TestDequeueEmbeddingJobsOrdersByAttemptsAndTime(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "emb-order-a")
	insertTestMemory(t, database, "emb-order-b")

	if err := database.EnqueueEmbeddingJob("emb-order-a", "insert"); err != nil {
		t.Fatalf("enqueue a: %v", err)
	}
	if err := database.EnqueueEmbeddingJob("emb-order-b", "insert"); err != nil {
		t.Fatalf("enqueue b: %v", err)
	}
	if err := database.IncrementEmbeddingJobAttempts("emb-order-a"); err != nil {
		t.Fatalf("increment a: %v", err)
	}

	jobs, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].MemoryID != "emb-order-b" {
		t.Fatalf("expected emb-order-b first (fewer attempts), got %s", jobs[0].MemoryID)
	}
	if jobs[1].MemoryID != "emb-order-a" {
		t.Fatalf("expected emb-order-a second, got %s", jobs[1].MemoryID)
	}
}

func TestDequeueEmbeddingJobsSkipsSuppressed(t *testing.T) {
	database := newTestDB(t)
	if err := database.InsertMemory(Memory{
		ID:         "emb-suppressed",
		Type:       "Fact",
		Content:    "suppressed",
		Title:      "Suppressed",
		Importance: 0.5,
		Source:     "test",
		SuppressedAt: sql.NullString{
			String: time.Now().Format(time.RFC3339),
			Valid:  true,
		},
	}); err != nil {
		t.Fatalf("insert suppressed: %v", err)
	}
	if err := database.EnqueueEmbeddingJob("emb-suppressed", "insert"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	jobs, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs for suppressed memory, got %d", len(jobs))
	}
}

func TestIncrementEmbeddingJobAttempts(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "emb-inc")

	if err := database.EnqueueEmbeddingJob("emb-inc", "insert"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := database.IncrementEmbeddingJobAttempts("emb-inc"); err != nil {
		t.Fatalf("increment: %v", err)
	}
	if err := database.IncrementEmbeddingJobAttempts("emb-inc"); err != nil {
		t.Fatalf("increment 2: %v", err)
	}

	jobs, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", jobs[0].Attempts)
	}
}

func TestDeleteEmbeddingJob(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "emb-del")

	if err := database.EnqueueEmbeddingJob("emb-del", "insert"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := database.DeleteEmbeddingJob("emb-del"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	jobs, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs after delete, got %d", len(jobs))
	}
}

func TestListMemoriesWithoutEmbedding(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "emb-has")
	insertTestMemory(t, database, "emb-missing")

	if err := database.UpsertEmbedding("emb-has", []float32{0.1, 0.2, 0.3}, "test"); err != nil {
		t.Fatalf("upsert embedding: %v", err)
	}

	ids, err := database.ListMemoriesWithoutEmbedding(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 memory without embedding, got %d", len(ids))
	}
	if ids[0] != "emb-missing" {
		t.Fatalf("expected emb-missing, got %s", ids[0])
	}
}

func TestListMemoriesWithoutEmbeddingExcludesSuppressed(t *testing.T) {
	database := newTestDB(t)
	if err := database.InsertMemory(Memory{
		ID:         "emb-supp-no-emb",
		Type:       "Fact",
		Content:    "suppressed no emb",
		Title:      "Suppressed",
		Importance: 0.5,
		Source:     "test",
		SuppressedAt: sql.NullString{
			String: time.Now().Format(time.RFC3339),
			Valid:  true,
		},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	ids, err := database.ListMemoriesWithoutEmbedding(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected 0 memories (suppressed excluded), got %d", len(ids))
	}
}

func TestInsertMemoryEnqueuesEmbeddingJob(t *testing.T) {
	database := newTestDB(t)
	insertTestMemory(t, database, "emb-auto")

	jobs, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 auto-enqueued job, got %d", len(jobs))
	}
	if jobs[0].MemoryID != "emb-auto" {
		t.Fatalf("expected memory_id emb-auto, got %s", jobs[0].MemoryID)
	}
	if jobs[0].Reason != "insert" {
		t.Fatalf("expected reason insert, got %s", jobs[0].Reason)
	}
}
