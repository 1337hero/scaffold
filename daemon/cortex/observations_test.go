package cortex

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	appconfig "scaffold/config"
	"scaffold/db"
)

func newObservationsCortex(t *testing.T, llm *stubLLM) (*Cortex, *db.DB) {
	t.Helper()
	database := openTestDB(t)
	return &Cortex{
		db:  database,
		llm: llm,
		cfg: appconfig.CortexConfig{
			Bulletin: appconfig.BulletinConfig{
				IntervalMinutes:    60,
				MaxWords:           500,
				MaxStaleMultiplier: 3,
				Model:              "claude-haiku-4-5",
			},
		},
		bulletin: newBulletinCache(3 * time.Hour),
	}, database
}

func TestRunObservations_PromptIncludesMemoryIDs(t *testing.T) {
	stub := &stubLLM{
		completionJSON: `[]`,
	}
	c, database := newObservationsCortex(t, stub)

	if err := database.InsertMemory(db.Memory{
		ID:         "obs-evidence-1",
		Type:       "Fact",
		Content:    "Repeated mention evidence",
		Title:      "Evidence Memory",
		Importance: 0.5,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert evidence memory: %v", err)
	}
	seedConversationEntries(t, database, 15)

	if err := c.runObservations(context.Background()); err != nil {
		t.Fatalf("runObservations: %v", err)
	}

	if stub.completionCalls != 1 {
		t.Fatalf("expected 1 LLM call, got %d", stub.completionCalls)
	}
	if !strings.Contains(stub.lastCompletionUser, "[id=obs-evidence-1]") {
		t.Fatalf("expected prompt to include memory ID, got: %s", stub.lastCompletionUser)
	}
}

func TestRunObservations_UsesConfiguredModel(t *testing.T) {
	stub := &stubLLM{
		completionJSON: `[]`,
	}
	c, database := newObservationsCortex(t, stub)
	c.cfg.Bulletin.Model = "custom-observations-model"

	seedConversationEntries(t, database, 15)

	if err := c.runObservations(context.Background()); err != nil {
		t.Fatalf("runObservations: %v", err)
	}
	if stub.lastCompletionModel != "custom-observations-model" {
		t.Fatalf("expected custom model, got %q", stub.lastCompletionModel)
	}
}

func seedConversationEntries(t *testing.T, database *db.DB, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		if _, err := database.InsertConversationEntry(
			"test-sender",
			"user",
			fmt.Sprintf("conversation message %d about recurring topic", i),
		); err != nil {
			t.Fatalf("insert conversation entry %d: %v", i, err)
		}
	}
}

func TestRunObservations_SkipsWhenFewEntries(t *testing.T) {
	stub := &stubLLM{}
	c, database := newObservationsCortex(t, stub)

	seedConversationEntries(t, database, 5)

	if err := c.runObservations(context.Background()); err != nil {
		t.Fatalf("runObservations: %v", err)
	}
	if stub.completionCalls != 0 {
		t.Fatalf("expected 0 LLM calls with few entries, got %d", stub.completionCalls)
	}
}

func TestRunObservations_CreatesPattern(t *testing.T) {
	stub := &stubLLM{
		completionJSON: `[{"pattern": "User frequently discusses Go concurrency", "evidence_memory_ids": [], "importance": 0.5}]`,
	}
	c, database := newObservationsCortex(t, stub)

	seedConversationEntries(t, database, 15)

	if err := c.runObservations(context.Background()); err != nil {
		t.Fatalf("runObservations: %v", err)
	}
	if stub.completionCalls != 1 {
		t.Fatalf("expected 1 LLM call, got %d", stub.completionCalls)
	}

	obs, err := database.ListByType("Observation", 10)
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	if obs[0].Title != "User frequently discusses Go concurrency" {
		t.Fatalf("unexpected title: %q", obs[0].Title)
	}
	if obs[0].Importance != 0.5 {
		t.Fatalf("expected importance 0.5, got %f", obs[0].Importance)
	}
}

func TestRunObservations_SkipsDuplicates(t *testing.T) {
	stub := &stubLLM{
		completionJSON: `[{"pattern": "Existing pattern", "evidence_memory_ids": [], "importance": 0.4}]`,
	}
	c, database := newObservationsCortex(t, stub)

	seedConversationEntries(t, database, 15)

	if err := database.InsertMemory(db.Memory{
		ID:         "existing-obs",
		Type:       "Observation",
		Content:    "Existing pattern",
		Title:      "Existing pattern",
		Importance: 0.4,
		Source:     "cortex",
	}); err != nil {
		t.Fatalf("insert existing observation: %v", err)
	}

	if err := c.runObservations(context.Background()); err != nil {
		t.Fatalf("runObservations: %v", err)
	}

	obs, err := database.ListByType("Observation", 10)
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation (no duplicate), got %d", len(obs))
	}
}

func TestRunObservations_MaxFive(t *testing.T) {
	stub := &stubLLM{
		completionJSON: `[
			{"pattern": "Pattern 1", "evidence_memory_ids": [], "importance": 0.3},
			{"pattern": "Pattern 2", "evidence_memory_ids": [], "importance": 0.3},
			{"pattern": "Pattern 3", "evidence_memory_ids": [], "importance": 0.3},
			{"pattern": "Pattern 4", "evidence_memory_ids": [], "importance": 0.3},
			{"pattern": "Pattern 5", "evidence_memory_ids": [], "importance": 0.3},
			{"pattern": "Pattern 6", "evidence_memory_ids": [], "importance": 0.3}
		]`,
	}
	c, database := newObservationsCortex(t, stub)

	seedConversationEntries(t, database, 15)

	if err := c.runObservations(context.Background()); err != nil {
		t.Fatalf("runObservations: %v", err)
	}

	obs, err := database.ListByType("Observation", 10)
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	if len(obs) != 5 {
		t.Fatalf("expected max 5 observations, got %d", len(obs))
	}
}

func TestRunObservations_HandlesEmptyLLMResponse(t *testing.T) {
	stub := &stubLLM{
		completionJSON: `[]`,
	}
	c, database := newObservationsCortex(t, stub)

	seedConversationEntries(t, database, 15)

	if err := c.runObservations(context.Background()); err != nil {
		t.Fatalf("runObservations: %v", err)
	}

	obs, err := database.ListByType("Observation", 10)
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	if len(obs) != 0 {
		t.Fatalf("expected 0 observations for empty response, got %d", len(obs))
	}
}

func TestRunObservations_HandlesBadJSON(t *testing.T) {
	stub := &stubLLM{
		completionJSON: `this is not valid json at all`,
	}
	c, database := newObservationsCortex(t, stub)

	seedConversationEntries(t, database, 15)

	err := c.runObservations(context.Background())
	if err != nil {
		t.Fatalf("expected nil error for bad JSON (non-fatal), got %v", err)
	}

	obs, err := database.ListByType("Observation", 10)
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	if len(obs) != 0 {
		t.Fatalf("expected 0 observations for bad JSON, got %d", len(obs))
	}
}

func TestRunObservations_CreatesEvidenceEdges(t *testing.T) {
	c, database := newObservationsCortex(t, &stubLLM{})

	if err := database.InsertMemory(db.Memory{
		ID:         "evidence-1",
		Type:       "Fact",
		Content:    "Some evidence",
		Title:      "Evidence",
		Importance: 0.5,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert evidence memory: %v", err)
	}

	c.llm = &stubLLM{
		completionJSON: `[{"pattern": "Pattern with evidence", "evidence_memory_ids": ["evidence-1", "nonexistent-id"], "importance": 0.4}]`,
	}

	seedConversationEntries(t, database, 15)

	if err := c.runObservations(context.Background()); err != nil {
		t.Fatalf("runObservations: %v", err)
	}

	obs, err := database.ListByType("Observation", 10)
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}

	edges, err := database.EdgesFrom(obs[0].ID)
	if err != nil {
		t.Fatalf("edges from observation: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge (nonexistent evidence skipped), got %d", len(edges))
	}
	if edges[0].Relation != "DerivedFrom" {
		t.Fatalf("expected DerivedFrom relation, got %q", edges[0].Relation)
	}
	if edges[0].ToID != "evidence-1" {
		t.Fatalf("expected edge to evidence-1, got %q", edges[0].ToID)
	}
}
