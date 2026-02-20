package cortex

import (
	"context"
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
