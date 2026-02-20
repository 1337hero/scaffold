package cortex

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	appconfig "scaffold/config"
	"scaffold/db"
)

type stubLLM struct {
	calls     int
	lastModel string
	lastWords int
	lastInput bulletinSections
	text      string
	err       error
}

func (s *stubLLM) SynthesizeBulletin(_ context.Context, model string, maxWords int, sections bulletinSections) (string, error) {
	s.calls++
	s.lastModel = model
	s.lastWords = maxWords
	s.lastInput = sections
	if s.err != nil {
		return "", s.err
	}
	return s.text, nil
}

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "cortex.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestBulletinCacheFreshness(t *testing.T) {
	cache := newBulletinCache(time.Second)

	content, fresh := cache.Get()
	if content != "" || !fresh {
		t.Fatalf("expected empty fresh cache, got content=%q fresh=%v", content, fresh)
	}

	cache.Set("hello")
	content, fresh = cache.Get()
	if content != "hello" || !fresh {
		t.Fatalf("expected fresh stored content, got content=%q fresh=%v", content, fresh)
	}

	cache.generatedAt.Store(time.Now().Add(-2 * time.Second).Unix())
	_, fresh = cache.Get()
	if fresh {
		t.Fatal("expected stale cache")
	}
}

func TestCortexTaskShouldRun(t *testing.T) {
	task := &CortexTask{
		Name:     "bulletin",
		Interval: time.Minute,
		Fn:       func(context.Context) error { return nil },
	}

	if !task.ShouldRun(time.Now()) {
		t.Fatal("expected task to run when never run before")
	}

	task.lastRun.Store(time.Now().Unix())
	if task.ShouldRun(time.Now()) {
		t.Fatal("expected task not to run before interval")
	}

	past := time.Now().Add(2 * time.Minute)
	if !task.ShouldRun(past) {
		t.Fatal("expected task to run after interval")
	}
}

func TestGenerateBulletinSkipsLLMWhenNoMemories(t *testing.T) {
	database := openTestDB(t)
	stub := &stubLLM{text: "should not be used"}

	c := &Cortex{
		db:  database,
		llm: stub,
		cfg: appconfig.CortexConfig{
			Bulletin: appconfig.BulletinConfig{
				Model:              "claude-haiku-4-5",
				MaxWords:           500,
				IntervalMinutes:    60,
				MaxStaleMultiplier: 3,
			},
		},
		bulletin: newBulletinCache(3 * time.Hour),
	}

	if err := c.generateBulletin(context.Background()); err != nil {
		t.Fatalf("generate bulletin: %v", err)
	}
	if stub.calls != 0 {
		t.Fatalf("expected no llm calls for empty graph, got %d", stub.calls)
	}
}

func TestGenerateBulletinStoresSynthesis(t *testing.T) {
	database := openTestDB(t)
	if err := database.InsertMemory(db.Memory{
		ID:         "mem-1",
		Type:       "Decision",
		Content:    "Use model-agnostic tool boundary",
		Title:      "Provider boundary decision",
		Importance: 0.9,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	stub := &stubLLM{text: "Prioritize provider-agnostic architecture."}

	c := &Cortex{
		db:  database,
		llm: stub,
		cfg: appconfig.CortexConfig{
			Bulletin: appconfig.BulletinConfig{
				Model:              "claude-haiku-4-5",
				MaxWords:           500,
				IntervalMinutes:    60,
				MaxStaleMultiplier: 3,
			},
		},
		bulletin: newBulletinCache(3 * time.Hour),
	}

	if err := c.generateBulletin(context.Background()); err != nil {
		t.Fatalf("generate bulletin: %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("expected one llm call, got %d", stub.calls)
	}

	content, fresh := c.CurrentBulletin()
	if !fresh {
		t.Fatal("expected fresh bulletin")
	}
	if content != "Prioritize provider-agnostic architecture." {
		t.Fatalf("unexpected bulletin content: %q", content)
	}
}

func TestBuildTasksIncludesRuntimeHandlers(t *testing.T) {
	database := openTestDB(t)
	c := New(database, nil, "test-key", appconfig.CortexConfig{
		Bulletin: appconfig.BulletinConfig{
			IntervalMinutes:    60,
			MaxWords:           500,
			MaxStaleMultiplier: 3,
			Model:              "claude-haiku-4-5",
		},
		Tasks: map[string]appconfig.TaskConfig{
			"consolidation": {
				IntervalHours:  6,
				TimeoutSeconds: 60,
			},
			"decay": {
				IntervalHours:  24,
				TimeoutSeconds: 15,
				Factor:         0.95,
				ExemptTypes:    []string{"Identity", "Permanent"},
			},
			"prioritize": {
				IntervalHours:  24,
				TimeoutSeconds: 120,
			},
			"prune": {
				IntervalHours:  24,
				TimeoutSeconds: 15,
			},
			"reindex": {
				IntervalHours:  12,
				TimeoutSeconds: 30,
			},
			"session_cleanup": {
				IntervalHours:  24,
				TimeoutSeconds: 15,
			},
		},
	})

	consolidation := c.taskByName("consolidation")
	if consolidation == nil || consolidation.Fn == nil {
		t.Fatal("expected consolidation task handler")
	}
	decay := c.taskByName("decay")
	if decay == nil || decay.Fn == nil {
		t.Fatal("expected decay task handler")
	}
	prioritize := c.taskByName("prioritize")
	if prioritize == nil || prioritize.Fn == nil {
		t.Fatal("expected prioritize task handler")
	}
	cleanup := c.taskByName("session_cleanup")
	if cleanup == nil || cleanup.Fn == nil {
		t.Fatal("expected session_cleanup task handler")
	}
	prune := c.taskByName("prune")
	if prune == nil || prune.Fn == nil {
		t.Fatal("expected prune task handler")
	}
	reindex := c.taskByName("reindex")
	if reindex == nil || reindex.Fn == nil {
		t.Fatal("expected reindex task handler")
	}
}

func TestRunPruneDeletesEligibleSuppressedMemory(t *testing.T) {
	database := openTestDB(t)
	oldSuppressedAt := time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339)
	if err := database.InsertMemory(db.Memory{
		ID:         "mem-prune-me",
		Type:       "Observation",
		Content:    "old suppressed memory",
		Title:      "Prune Me",
		Importance: 0.2,
		Source:     "test",
		SuppressedAt: sql.NullString{
			String: oldSuppressedAt,
			Valid:  true,
		},
	}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	c := New(database, nil, "test-key", appconfig.CortexConfig{
		Bulletin: appconfig.BulletinConfig{
			IntervalMinutes:    60,
			MaxWords:           500,
			MaxStaleMultiplier: 3,
			Model:              "claude-haiku-4-5",
		},
		Tasks: map[string]appconfig.TaskConfig{
			"prune": {
				IntervalHours:  24,
				TimeoutSeconds: 15,
				SuppressedDays: 30,
			},
		},
	})

	if err := c.runPrune(context.Background()); err != nil {
		t.Fatalf("run prune: %v", err)
	}

	mem, err := database.GetMemory("mem-prune-me")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if mem != nil {
		t.Fatal("expected suppressed memory to be pruned")
	}
}

func TestRunDecayUpdatesEligibleMemory(t *testing.T) {
	database := openTestDB(t)
	if err := database.InsertMemory(db.Memory{
		ID:         "decay-target",
		Type:       "Fact",
		Content:    "old",
		Title:      "Old",
		Importance: 0.8,
		Source:     "test",
		AccessedAt: time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	c := New(database, nil, "test-key", appconfig.CortexConfig{
		Bulletin: appconfig.BulletinConfig{
			IntervalMinutes:    60,
			MaxWords:           500,
			MaxStaleMultiplier: 3,
			Model:              "claude-haiku-4-5",
		},
		Tasks: map[string]appconfig.TaskConfig{
			"decay": {
				IntervalHours:   24,
				TimeoutSeconds:  15,
				Factor:          0.5,
				ExemptTypes:     []string{"Identity"},
				ImportanceFloor: 0.1,
			},
		},
	})

	if err := c.runDecay(context.Background()); err != nil {
		t.Fatalf("run decay: %v", err)
	}

	mem, err := database.GetMemory("decay-target")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if mem == nil || mem.Importance != 0.4 {
		t.Fatalf("expected decayed importance 0.4, got %+v", mem)
	}
}

func TestRunReindexWritesCentralityRows(t *testing.T) {
	database := openTestDB(t)
	for _, mem := range []db.Memory{
		{ID: "reindex-a", Type: "Fact", Content: "A", Title: "A", Importance: 0.6, Source: "test"},
		{ID: "reindex-b", Type: "Fact", Content: "B", Title: "B", Importance: 0.6, Source: "test"},
	} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert memory %s: %v", mem.ID, err)
		}
	}
	if err := database.InsertEdge(db.Edge{FromID: "reindex-a", ToID: "reindex-b", Relation: "RelatedTo"}); err != nil {
		t.Fatalf("insert edge: %v", err)
	}

	c := New(database, nil, "test-key", appconfig.CortexConfig{
		Bulletin: appconfig.BulletinConfig{
			IntervalMinutes:    60,
			MaxWords:           500,
			MaxStaleMultiplier: 3,
			Model:              "claude-haiku-4-5",
		},
		Tasks: map[string]appconfig.TaskConfig{
			"reindex": {
				IntervalHours:  12,
				TimeoutSeconds: 30,
			},
		},
	})

	if err := c.runReindex(context.Background()); err != nil {
		t.Fatalf("run reindex: %v", err)
	}

	count, err := database.MemoryCentralityCount()
	if err != nil {
		t.Fatalf("count centrality rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 centrality rows, got %d", count)
	}
}

func TestRunConsolidationSuppressesDuplicate(t *testing.T) {
	database := openTestDB(t)
	for _, mem := range []db.Memory{
		{
			ID:         "cons-a",
			Type:       "Fact",
			Content:    "duplicate idea",
			Title:      "Duplicate Idea",
			Importance: 0.9,
			Source:     "test",
			CreatedAt:  "2026-01-01T00:00:00Z",
			UpdatedAt:  "2026-01-01T00:00:00Z",
		},
		{
			ID:         "cons-b",
			Type:       "Fact",
			Content:    "duplicate idea",
			Title:      "Duplicate Idea",
			Importance: 0.3,
			Source:     "test",
			CreatedAt:  "2026-01-02T00:00:00Z",
			UpdatedAt:  "2026-01-02T00:00:00Z",
		},
	} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert memory %s: %v", mem.ID, err)
		}
	}

	c := New(database, nil, "test-key", appconfig.CortexConfig{
		Bulletin: appconfig.BulletinConfig{
			IntervalMinutes:    60,
			MaxWords:           500,
			MaxStaleMultiplier: 3,
			Model:              "claude-haiku-4-5",
		},
		Tasks: map[string]appconfig.TaskConfig{
			"consolidation": {
				IntervalHours:  6,
				TimeoutSeconds: 60,
			},
		},
	})

	if err := c.runConsolidation(context.Background()); err != nil {
		t.Fatalf("run consolidation: %v", err)
	}

	mem, err := database.GetMemory("cons-b")
	if err != nil {
		t.Fatalf("get consolidated memory: %v", err)
	}
	if mem == nil || !mem.SuppressedAt.Valid {
		t.Fatalf("expected duplicate memory to be suppressed, got %+v", mem)
	}
}
