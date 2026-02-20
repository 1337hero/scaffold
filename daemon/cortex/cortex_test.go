package cortex

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	appconfig "scaffold/config"
	"scaffold/db"
	"scaffold/embedding"
)

type stubLLM struct {
	calls                int
	lastModel            string
	lastWords            int
	lastInput            bulletinSections
	text                 string
	err                  error
	completionJSON       string
	lastCompletionModel  string
	lastCompletionSystem string
	lastCompletionUser   string
	completionErr        error
	completionCalls      int
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

func (s *stubLLM) CompletionJSON(_ context.Context, model, systemPrompt, userPrompt string, _ int64) (string, error) {
	s.completionCalls++
	s.lastCompletionModel = model
	s.lastCompletionSystem = systemPrompt
	s.lastCompletionUser = userPrompt
	if s.completionErr != nil {
		return "", s.completionErr
	}
	return s.completionJSON, nil
}

type stubEmbedder struct {
	available  bool
	embeddings [][]float32
	model      string
	err        error
	calls      int
}

func (s *stubEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if len(s.embeddings) > 0 {
		return s.embeddings[0], nil
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (s *stubEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.embeddings != nil {
		return s.embeddings, nil
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{float32(i) * 0.1, 0.2, 0.3}
	}
	return out, nil
}

func (s *stubEmbedder) Available(_ context.Context) bool {
	return s.available
}

func (s *stubEmbedder) ModelName() string {
	if s.model != "" {
		return s.model
	}
	return "test-model"
}

var _ embedding.Embedder = (*stubEmbedder)(nil)

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
			"embedding_backfill": {
				IntervalHours:  6,
				TimeoutSeconds: 300,
			},
		},
	}, nil)

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
	embBackfill := c.taskByName("embedding_backfill")
	if embBackfill == nil || embBackfill.Fn == nil {
		t.Fatal("expected embedding_backfill task handler")
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
	}, nil)

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
	}, nil)

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
	}, nil)

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
	}, nil)

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

func TestRunEmbeddingBackfillProcessesJobs(t *testing.T) {
	database := openTestDB(t)
	if err := database.InsertMemory(db.Memory{
		ID:         "emb-job-1",
		Type:       "Fact",
		Content:    "some content",
		Title:      "Some Title",
		Importance: 0.7,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	jobs, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 enqueued job from InsertMemory, got %d", len(jobs))
	}

	emb := &stubEmbedder{available: true}
	c := &Cortex{
		db:       database,
		embedder: emb,
		cfg:      appconfig.CortexConfig{},
	}

	if err := c.runEmbeddingBackfill(context.Background()); err != nil {
		t.Fatalf("run embedding backfill: %v", err)
	}

	if emb.calls != 1 {
		t.Fatalf("expected 1 embedder call, got %d", emb.calls)
	}

	remaining, err := database.DequeueEmbeddingJobs(10)
	if err != nil {
		t.Fatalf("dequeue after backfill: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected 0 remaining jobs, got %d", len(remaining))
	}

	vec, err := database.GetEmbedding("emb-job-1")
	if err != nil {
		t.Fatalf("get embedding: %v", err)
	}
	if vec == nil {
		t.Fatal("expected embedding to be stored")
	}
}

func TestRunEmbeddingBackfillSkipsWhenUnavailable(t *testing.T) {
	database := openTestDB(t)
	emb := &stubEmbedder{available: false}
	c := &Cortex{
		db:       database,
		embedder: emb,
		cfg:      appconfig.CortexConfig{},
	}

	if err := c.runEmbeddingBackfill(context.Background()); err != nil {
		t.Fatalf("expected nil error when unavailable, got %v", err)
	}
	if emb.calls != 0 {
		t.Fatalf("expected no embedder calls when unavailable, got %d", emb.calls)
	}
}

func TestRunEmbeddingBackfillBackfillsMemoriesWithoutEmbedding(t *testing.T) {
	database := openTestDB(t)
	if err := database.InsertMemory(db.Memory{
		ID:         "emb-backfill-1",
		Type:       "Fact",
		Content:    "needs embedding",
		Title:      "Backfill Me",
		Importance: 0.5,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	_ = database.DeleteEmbeddingJob("emb-backfill-1")

	emb := &stubEmbedder{available: true}
	c := &Cortex{
		db:       database,
		embedder: emb,
		cfg:      appconfig.CortexConfig{},
	}

	if err := c.runEmbeddingBackfill(context.Background()); err != nil {
		t.Fatalf("run embedding backfill: %v", err)
	}

	vec, err := database.GetEmbedding("emb-backfill-1")
	if err != nil {
		t.Fatalf("get embedding: %v", err)
	}
	if vec == nil {
		t.Fatal("expected embedding to be stored via backfill")
	}
}

func TestConsolidationDecisionParsesMerge(t *testing.T) {
	stub := &stubLLM{
		completionJSON: `{"decision": "merge", "keep_id": "cd-a", "reason": "duplicates"}`,
	}
	c := &Cortex{llm: stub}

	candidate := db.ConsolidationCandidate{
		MemoryA:    db.Memory{ID: "cd-a", Title: "Test A", Content: "content A"},
		MemoryB:    db.Memory{ID: "cd-b", Title: "Test B", Content: "content B"},
		Similarity: 0.9,
	}

	decision, err := c.consolidationDecision(context.Background(), candidate)
	if err != nil {
		t.Fatalf("consolidation decision: %v", err)
	}
	if decision.Action != "merge" {
		t.Fatalf("expected merge, got %s", decision.Action)
	}
	if decision.KeepID != "cd-a" {
		t.Fatalf("expected keep_id cd-a, got %s", decision.KeepID)
	}
	if decision.Reason != "duplicates" {
		t.Fatalf("expected reason duplicates, got %s", decision.Reason)
	}
	if stub.completionCalls != 1 {
		t.Fatalf("expected 1 completion call, got %d", stub.completionCalls)
	}
}

func TestConsolidationDecisionParsesRelate(t *testing.T) {
	stub := &stubLLM{
		completionJSON: `{"decision": "relate", "keep_id": "", "reason": "related topics"}`,
	}
	c := &Cortex{llm: stub}

	candidate := db.ConsolidationCandidate{
		MemoryA:    db.Memory{ID: "cr-a", Title: "Test A", Content: "content A"},
		MemoryB:    db.Memory{ID: "cr-b", Title: "Test B", Content: "content B"},
		Similarity: 0.87,
	}

	decision, err := c.consolidationDecision(context.Background(), candidate)
	if err != nil {
		t.Fatalf("consolidation decision: %v", err)
	}
	if decision.Action != "relate" {
		t.Fatalf("expected relate, got %s", decision.Action)
	}
}

func TestConsolidationDecisionParsesKeepSeparate(t *testing.T) {
	stub := &stubLLM{
		completionJSON: `{"decision": "keep_separate", "keep_id": "", "reason": "different topics"}`,
	}
	c := &Cortex{llm: stub}

	candidate := db.ConsolidationCandidate{
		MemoryA:    db.Memory{ID: "ks-a", Title: "Test A", Content: "content A"},
		MemoryB:    db.Memory{ID: "ks-b", Title: "Test B", Content: "content B"},
		Similarity: 0.86,
	}

	decision, err := c.consolidationDecision(context.Background(), candidate)
	if err != nil {
		t.Fatalf("consolidation decision: %v", err)
	}
	if decision.Action != "keep_separate" {
		t.Fatalf("expected keep_separate, got %s", decision.Action)
	}
}

func TestConsolidationDecisionHandlesLLMError(t *testing.T) {
	stub := &stubLLM{
		completionErr: fmt.Errorf("api error"),
	}
	c := &Cortex{llm: stub}

	candidate := db.ConsolidationCandidate{
		MemoryA:    db.Memory{ID: "err-a", Title: "A", Content: "A"},
		MemoryB:    db.Memory{ID: "err-b", Title: "B", Content: "B"},
		Similarity: 0.9,
	}

	_, err := c.consolidationDecision(context.Background(), candidate)
	if err == nil {
		t.Fatal("expected error from LLM failure")
	}
}

func TestConsolidationDecisionHandlesInvalidJSON(t *testing.T) {
	stub := &stubLLM{
		completionJSON: `not json at all`,
	}
	c := &Cortex{llm: stub}

	candidate := db.ConsolidationCandidate{
		MemoryA:    db.Memory{ID: "ij-a", Title: "A", Content: "A"},
		MemoryB:    db.Memory{ID: "ij-b", Title: "B", Content: "B"},
		Similarity: 0.9,
	}

	_, err := c.consolidationDecision(context.Background(), candidate)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRunConsolidationSemanticPathMerge(t *testing.T) {
	database := openTestDB(t)

	for _, mem := range []db.Memory{
		{ID: "sem-a", Type: "Fact", Title: "Go channels", Content: "Go channels are great for concurrency", Importance: 0.8, Source: "test"},
		{ID: "sem-b", Type: "Fact", Title: "Go concurrency", Content: "Go concurrency with channels is powerful", Importance: 0.6, Source: "test"},
	} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert %s: %v", mem.ID, err)
		}
	}

	database.UpsertEmbedding("sem-a", []float32{0.9, 0.1, 0.0}, "m")
	database.UpsertEmbedding("sem-b", []float32{0.89, 0.11, 0.01}, "m")

	stub := &stubLLM{
		completionJSON: `{"decision": "merge", "keep_id": "sem-a", "reason": "duplicate"}`,
	}
	emb := &stubEmbedder{available: true}

	c := &Cortex{
		db:       database,
		llm:      stub,
		embedder: emb,
		cfg:      appconfig.CortexConfig{},
	}

	if err := c.runConsolidation(context.Background()); err != nil {
		t.Fatalf("run consolidation: %v", err)
	}

	mem, err := database.GetMemory("sem-b")
	if err != nil {
		t.Fatalf("get sem-b: %v", err)
	}
	if mem == nil || !mem.SuppressedAt.Valid {
		t.Fatalf("expected sem-b to be suppressed via semantic merge, got %+v", mem)
	}

	if stub.completionCalls == 0 {
		t.Fatal("expected at least one LLM completion call")
	}
}

func TestRunConsolidationSemanticPathSkipsWhenEmbedderUnavailable(t *testing.T) {
	database := openTestDB(t)

	if err := database.InsertMemory(db.Memory{
		ID: "skip-a", Type: "Fact", Title: "A", Content: "content a", Importance: 0.5, Source: "test",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	stub := &stubLLM{}
	emb := &stubEmbedder{available: false}

	c := &Cortex{
		db:       database,
		llm:      stub,
		embedder: emb,
		cfg:      appconfig.CortexConfig{},
	}

	if err := c.runConsolidation(context.Background()); err != nil {
		t.Fatalf("run consolidation: %v", err)
	}

	if stub.completionCalls != 0 {
		t.Fatalf("expected 0 completion calls when embedder unavailable, got %d", stub.completionCalls)
	}
}

func TestRunConsolidationSemanticPathRelate(t *testing.T) {
	database := openTestDB(t)

	for _, mem := range []db.Memory{
		{ID: "rel-a", Type: "Fact", Title: "React hooks", Content: "React hooks simplify state management", Importance: 0.7, Source: "test"},
		{ID: "rel-b", Type: "Fact", Title: "Vue composition", Content: "Vue composition API is similar to hooks", Importance: 0.7, Source: "test"},
	} {
		if err := database.InsertMemory(mem); err != nil {
			t.Fatalf("insert %s: %v", mem.ID, err)
		}
	}

	database.UpsertEmbedding("rel-a", []float32{0.9, 0.1, 0.0}, "m")
	database.UpsertEmbedding("rel-b", []float32{0.88, 0.12, 0.01}, "m")

	stub := &stubLLM{
		completionJSON: `{"decision": "relate", "keep_id": "", "reason": "related frameworks"}`,
	}
	emb := &stubEmbedder{available: true}

	c := &Cortex{
		db:       database,
		llm:      stub,
		embedder: emb,
		cfg:      appconfig.CortexConfig{},
	}

	if err := c.runConsolidation(context.Background()); err != nil {
		t.Fatalf("run consolidation: %v", err)
	}

	memA, _ := database.GetMemory("rel-a")
	memB, _ := database.GetMemory("rel-b")
	if memA == nil || memA.SuppressedAt.Valid {
		t.Fatal("expected rel-a to remain unsuppressed")
	}
	if memB == nil || memB.SuppressedAt.Valid {
		t.Fatal("expected rel-b to remain unsuppressed")
	}
}
