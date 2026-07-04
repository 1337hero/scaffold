package db

import (
	"database/sql"
	"math"
	"testing"
)

func insertFactT(t *testing.T, db *DB, f Fact, related ...string) Fact {
	t.Helper()
	if f.ID == "" {
		f.ID = newID()
	}
	if err := db.InsertFact(f, related); err != nil {
		t.Fatalf("InsertFact: %v", err)
	}
	return f
}

func trustOf(t *testing.T, db *DB, id string) float64 {
	t.Helper()
	f, err := db.GetFact(id)
	if err != nil {
		t.Fatalf("GetFact: %v", err)
	}
	if f == nil {
		t.Fatalf("fact %s not found", id)
	}
	return f.Trust
}

// --- CRUD ---

func TestInsertAndGetFact(t *testing.T) {
	db := openTestDB(t)

	f := insertFactT(t, db, Fact{
		Entity:   "Auggie",
		Content:  "Auggie is Mike's dog",
		Category: sql.NullString{String: "general", Valid: true},
		Tags:     sql.NullString{String: "pets,family", Valid: true},
	})

	got, err := db.GetFact(f.ID)
	if err != nil {
		t.Fatalf("GetFact: %v", err)
	}
	if got == nil || got.Entity != "Auggie" || got.Content != "Auggie is Mike's dog" {
		t.Fatalf("GetFact returned %+v", got)
	}
	if got.Trust != 0.5 {
		t.Fatalf("trust=%v, want default 0.5", got.Trust)
	}
	if got.CreatedAt == "" || got.UpdatedAt == "" {
		t.Fatalf("timestamps not set: %+v", got)
	}
}

func TestInsertFact_TrimsEntity(t *testing.T) {
	db := openTestDB(t)
	insertFactT(t, db, Fact{Entity: "  Auggie ", Content: "pasted with whitespace"})

	facts, err := db.ProbeFacts("Auggie")
	if err != nil {
		t.Fatalf("ProbeFacts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("got %d facts, want 1 — untrimmed entity is unreachable by probe", len(facts))
	}
}

func TestGetFact_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := db.GetFact("missing")
	if err != nil {
		t.Fatalf("GetFact err: %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

func TestInsertFact_InvalidCategory(t *testing.T) {
	db := openTestDB(t)
	err := db.InsertFact(Fact{
		Entity:   "Auggie",
		Content:  "x",
		Category: sql.NullString{String: "bogus", Valid: true},
	}, nil)
	if err == nil {
		t.Fatalf("expected invalid category error")
	}
}

func TestInsertFact_WithRelatedEntities_CreatesEdges(t *testing.T) {
	db := openTestDB(t)

	f := insertFactT(t, db, Fact{
		Entity:  "fitness",
		Content: "Morning walks with Auggie double as exercise",
	}, "Auggie", "Mike", "  ", "")

	edges, err := db.ListFactEdges(f.ID)
	if err != nil {
		t.Fatalf("ListFactEdges: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("got %d edges, want 2 (blank entities skipped)", len(edges))
	}
	if edges[0].TargetEntity != "Auggie" || edges[1].TargetEntity != "Mike" {
		t.Fatalf("edges = %+v", edges)
	}
	for _, e := range edges {
		if e.Relation.String != "about" {
			t.Fatalf("relation=%q, want about", e.Relation.String)
		}
	}
}

func TestUpdateFact(t *testing.T) {
	db := openTestDB(t)
	f := insertFactT(t, db, Fact{Entity: "Mike", Content: "wrong claim"})

	if err := db.UpdateFact(f.ID, map[string]any{
		"content":  "corrected claim",
		"category": "user_pref",
	}); err != nil {
		t.Fatalf("UpdateFact: %v", err)
	}

	got, _ := db.GetFact(f.ID)
	if got.Content != "corrected claim" || got.Category.String != "user_pref" {
		t.Fatalf("got %+v", got)
	}

	if err := db.UpdateFact(f.ID, map[string]any{"trust": 1.0}); err == nil {
		t.Fatalf("expected error updating non-whitelisted column")
	}
	if err := db.UpdateFact(f.ID, map[string]any{"category": "bogus"}); err == nil {
		t.Fatalf("expected invalid category error")
	}
	if err := db.UpdateFact("missing", map[string]any{"content": "x"}); err != sql.ErrNoRows {
		t.Fatalf("err=%v, want sql.ErrNoRows", err)
	}
}

func TestListFacts_Filters(t *testing.T) {
	db := openTestDB(t)
	insertFactT(t, db, Fact{Entity: "Mike", Content: "a",
		Category: sql.NullString{String: "user_pref", Valid: true},
		Tags:     sql.NullString{String: "golang,adhd", Valid: true}})
	insertFactT(t, db, Fact{Entity: "Auggie", Content: "b",
		Category: sql.NullString{String: "general", Valid: true},
		Tags:     sql.NullString{String: "go", Valid: true}})

	entity := "Mike"
	facts, err := db.ListFacts(&entity, nil, nil, 0)
	if err != nil {
		t.Fatalf("ListFacts: %v", err)
	}
	if len(facts) != 1 || facts[0].Entity != "Mike" {
		t.Fatalf("entity filter: got %+v", facts)
	}

	category := "general"
	facts, _ = db.ListFacts(nil, &category, nil, 0)
	if len(facts) != 1 || facts[0].Entity != "Auggie" {
		t.Fatalf("category filter: got %+v", facts)
	}

	// Tag "go" must not match "golang".
	tag := "go"
	facts, _ = db.ListFacts(nil, nil, &tag, 0)
	if len(facts) != 1 || facts[0].Entity != "Auggie" {
		t.Fatalf("tag filter: got %+v", facts)
	}

	facts, _ = db.ListFacts(nil, nil, nil, 1)
	if len(facts) != 1 {
		t.Fatalf("limit: got %d facts, want 1", len(facts))
	}
}

func TestListFacts_TagFilterEscapesWildcards(t *testing.T) {
	db := openTestDB(t)
	insertFactT(t, db, Fact{Entity: "Mike", Content: "a",
		Tags: sql.NullString{String: "a_b", Valid: true}})
	insertFactT(t, db, Fact{Entity: "Mike", Content: "b",
		Tags: sql.NullString{String: "axb", Valid: true}})

	tag := "a_b"
	facts, err := db.ListFacts(nil, nil, &tag, 0)
	if err != nil {
		t.Fatalf("ListFacts: %v", err)
	}
	if len(facts) != 1 || facts[0].Tags.String != "a_b" {
		t.Fatalf("got %+v, want only the literal a_b tag ('_' must not act as wildcard)", facts)
	}
}

func TestPromptFacts_RanksRelevanceAndBumpsRetrieval(t *testing.T) {
	db := openTestDB(t)
	insertFactT(t, db, Fact{Entity: "Unrelated", Content: "High trust but irrelevant", Trust: 0.95})
	relevant := insertFactT(t, db, Fact{Entity: "Mike", Content: "Mike wants fence updates kept short.", Trust: 0.6})

	facts, err := db.PromptFacts([]string{"Mike"}, "fence project", 1)
	if err != nil {
		t.Fatalf("PromptFacts: %v", err)
	}
	if len(facts) != 1 || facts[0].ID != relevant.ID {
		t.Fatalf("got %+v, want relevant Mike fence fact first", facts)
	}

	got, _ := db.GetFact(relevant.ID)
	if got.RetrievalCount != 1 {
		t.Fatalf("retrieval_count=%d, want 1", got.RetrievalCount)
	}
}

func TestPromptFacts_AppliesTrustFloorAndLimit(t *testing.T) {
	db := openTestDB(t)
	insertFactT(t, db, Fact{Entity: "Mike", Content: "low", Trust: 0.29})
	insertFactT(t, db, Fact{Entity: "Mike", Content: "one", Trust: 0.7})
	insertFactT(t, db, Fact{Entity: "Mike", Content: "two", Trust: 0.6})

	facts, err := db.PromptFacts([]string{"Mike"}, "", 1)
	if err != nil {
		t.Fatalf("PromptFacts: %v", err)
	}
	if len(facts) != 1 || facts[0].Content != "one" {
		t.Fatalf("got %+v, want highest trust fact only", facts)
	}
}

// --- Trust scoring ---

func TestTrustAdjustment_Helpful_Increases(t *testing.T) {
	db := openTestDB(t)
	f := insertFactT(t, db, Fact{Entity: "Mike", Content: "x"})

	if err := db.FeedbackFact(f.ID, true); err != nil {
		t.Fatalf("FeedbackFact: %v", err)
	}
	if got := trustOf(t, db, f.ID); math.Abs(got-0.55) > 1e-9 {
		t.Fatalf("trust=%v, want 0.55", got)
	}
}

func TestTrustAdjustment_Unhelpful_Decreases(t *testing.T) {
	db := openTestDB(t)
	f := insertFactT(t, db, Fact{Entity: "Mike", Content: "x"})

	if err := db.FeedbackFact(f.ID, false); err != nil {
		t.Fatalf("FeedbackFact: %v", err)
	}
	if got := trustOf(t, db, f.ID); math.Abs(got-0.40) > 1e-9 {
		t.Fatalf("trust=%v, want 0.40", got)
	}
}

func TestTrustAdjustment_ClampAtOne(t *testing.T) {
	db := openTestDB(t)
	f := insertFactT(t, db, Fact{Entity: "Mike", Content: "x", Trust: 0.98})

	if err := db.FeedbackFact(f.ID, true); err != nil {
		t.Fatalf("FeedbackFact: %v", err)
	}
	if got := trustOf(t, db, f.ID); got != 1.0 {
		t.Fatalf("trust=%v, want clamp at 1.0", got)
	}
}

func TestTrustAdjustment_ClampAtZero(t *testing.T) {
	db := openTestDB(t)
	f := insertFactT(t, db, Fact{Entity: "Mike", Content: "x", Trust: 0.05})

	if err := db.FeedbackFact(f.ID, false); err != nil {
		t.Fatalf("FeedbackFact: %v", err)
	}
	if got := trustOf(t, db, f.ID); got != 0.0 {
		t.Fatalf("trust=%v, want clamp at 0.0", got)
	}
}

func TestFeedbackFact_BumpsHelpfulCount(t *testing.T) {
	db := openTestDB(t)
	f := insertFactT(t, db, Fact{Entity: "Mike", Content: "x"})

	if err := db.FeedbackFact(f.ID, true); err != nil {
		t.Fatalf("FeedbackFact helpful: %v", err)
	}
	if err := db.FeedbackFact(f.ID, false); err != nil {
		t.Fatalf("FeedbackFact unhelpful: %v", err)
	}

	got, _ := db.GetFact(f.ID)
	if got.HelpfulCount != 1 {
		t.Fatalf("helpful_count=%d, want 1 (unhelpful must not bump)", got.HelpfulCount)
	}

	if err := db.FeedbackFact("missing", true); err != sql.ErrNoRows {
		t.Fatalf("err=%v, want sql.ErrNoRows", err)
	}
}

// --- Retrieval floor ---

func TestRetrievalFloor_ExcludesLowTrustFacts(t *testing.T) {
	db := openTestDB(t)
	insertFactT(t, db, Fact{Entity: "Mike", Content: "low", Trust: 0.29})
	insertFactT(t, db, Fact{Entity: "Mike", Content: "high", Trust: 0.8})

	facts, err := db.ProbeFacts("Mike")
	if err != nil {
		t.Fatalf("ProbeFacts: %v", err)
	}
	if len(facts) != 1 || facts[0].Content != "high" {
		t.Fatalf("got %+v, want only the high-trust fact", facts)
	}
}

func TestRetrievalFloor_IncludesExactlyPointThree(t *testing.T) {
	db := openTestDB(t)
	insertFactT(t, db, Fact{Entity: "Mike", Content: "boundary", Trust: 0.3})

	facts, err := db.ProbeFacts("Mike")
	if err != nil {
		t.Fatalf("ProbeFacts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("got %d facts, want 1 (trust == 0.3 is included)", len(facts))
	}
}

// --- Probe / suppress ---

func TestProbeFacts_BumpsRetrievalCount(t *testing.T) {
	db := openTestDB(t)
	f := insertFactT(t, db, Fact{Entity: "Auggie", Content: "x"})

	if _, err := db.ProbeFacts("Auggie"); err != nil {
		t.Fatalf("ProbeFacts: %v", err)
	}
	facts, err := db.ProbeFacts("Auggie")
	if err != nil {
		t.Fatalf("ProbeFacts: %v", err)
	}
	if facts[0].RetrievalCount != 2 {
		t.Fatalf("retrieval_count=%d, want 2", facts[0].RetrievalCount)
	}

	got, _ := db.GetFact(f.ID)
	if got.RetrievalCount != 2 {
		t.Fatalf("stored retrieval_count=%d, want 2", got.RetrievalCount)
	}
}

func TestProbeFacts_SortedByTrustDesc(t *testing.T) {
	db := openTestDB(t)
	insertFactT(t, db, Fact{Entity: "Mike", Content: "mid", Trust: 0.5})
	insertFactT(t, db, Fact{Entity: "Mike", Content: "top", Trust: 0.9})

	facts, err := db.ProbeFacts("Mike")
	if err != nil {
		t.Fatalf("ProbeFacts: %v", err)
	}
	if len(facts) != 2 || facts[0].Content != "top" {
		t.Fatalf("got %+v, want trust DESC order", facts)
	}
}

func TestSuppressFact_ExcludedFromProbe(t *testing.T) {
	db := openTestDB(t)
	f := insertFactT(t, db, Fact{Entity: "Mike", Content: "bad fact"})

	if err := db.SuppressFact(f.ID); err != nil {
		t.Fatalf("SuppressFact: %v", err)
	}

	facts, err := db.ProbeFacts("Mike")
	if err != nil {
		t.Fatalf("ProbeFacts: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("got %d facts, want 0", len(facts))
	}

	listed, _ := db.ListFacts(nil, nil, nil, 0)
	if len(listed) != 0 {
		t.Fatalf("ListFacts returned suppressed fact: %+v", listed)
	}

	// Still retrievable by id for review.
	got, _ := db.GetFact(f.ID)
	if got == nil || !got.SuppressedAt.Valid {
		t.Fatalf("suppressed fact should remain gettable: %+v", got)
	}

	if err := db.SuppressFact("missing"); err != sql.ErrNoRows {
		t.Fatalf("err=%v, want sql.ErrNoRows", err)
	}
}

// --- Relational ---

func TestRelatedEntities_ReturnsLinkedEntities(t *testing.T) {
	db := openTestDB(t)
	insertFactT(t, db, Fact{Entity: "Auggie", Content: "walks with Mike"}, "Mike", "fitness")
	insertFactT(t, db, Fact{Entity: "ADHD", Content: "exercise helps focus"}, "fitness")
	insertFactT(t, db, Fact{Entity: "unrelated", Content: "nothing shared"})

	related, err := db.RelatedEntities("Auggie")
	if err != nil {
		t.Fatalf("RelatedEntities: %v", err)
	}
	if len(related) != 2 || related[0] != "Mike" || related[1] != "fitness" {
		t.Fatalf("got %v, want [Mike fitness]", related)
	}

	// From the target side: fitness is adjacent to Auggie, Mike, and ADHD.
	related, err = db.RelatedEntities("fitness")
	if err != nil {
		t.Fatalf("RelatedEntities: %v", err)
	}
	if len(related) != 3 {
		t.Fatalf("got %v, want [ADHD Auggie Mike]", related)
	}
}

func TestRelatedEntities_RespectsFloorAndSuppression(t *testing.T) {
	db := openTestDB(t)
	insertFactT(t, db, Fact{Entity: "Auggie", Content: "low trust link", Trust: 0.1}, "Mike")
	f := insertFactT(t, db, Fact{Entity: "Auggie", Content: "suppressed link"}, "fitness")
	if err := db.SuppressFact(f.ID); err != nil {
		t.Fatalf("SuppressFact: %v", err)
	}

	related, err := db.RelatedEntities("Auggie")
	if err != nil {
		t.Fatalf("RelatedEntities: %v", err)
	}
	if len(related) != 0 {
		t.Fatalf("got %v, want none (floor + suppression)", related)
	}
}

func TestReasonFacts_FindsBridgingFacts(t *testing.T) {
	db := openTestDB(t)
	bridge := insertFactT(t, db,
		Fact{Entity: "fitness", Content: "morning walks calm ADHD symptoms"}, "ADHD")
	insertFactT(t, db, Fact{Entity: "fitness", Content: "only about fitness"})
	insertFactT(t, db, Fact{Entity: "ADHD", Content: "only about adhd"})

	facts, err := db.ReasonFacts([]string{"fitness", "ADHD"})
	if err != nil {
		t.Fatalf("ReasonFacts: %v", err)
	}
	if len(facts) != 1 || facts[0].ID != bridge.ID {
		t.Fatalf("got %+v, want only the bridging fact", facts)
	}
}

// --- Contradict ---

func TestContradictingFacts_SameEntity(t *testing.T) {
	db := openTestDB(t)
	insertFactT(t, db, Fact{Entity: "Mike", Content: "prefers tabs"})
	insertFactT(t, db, Fact{Entity: "Mike", Content: "prefers spaces"})
	insertFactT(t, db, Fact{Entity: "Auggie", Content: "other entity"})

	facts, err := db.ContradictingFacts("Mike")
	if err != nil {
		t.Fatalf("ContradictingFacts: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("got %d facts, want 2", len(facts))
	}
}

func TestConflictingFacts_NegationAndExclusiveValue(t *testing.T) {
	db := openTestDB(t)
	negation := insertFactT(t, db, Fact{Entity: "Mike", Content: "Mike likes late calls"})
	preference := insertFactT(t, db, Fact{Entity: "Mike", Content: "Mike prefers tabs"})
	insertFactT(t, db, Fact{Entity: "Mike", Content: "Mike likes coffee"})
	insertFactT(t, db, Fact{Entity: "Mike", Content: "Mike dislikes stale claim", Trust: 0.2})

	conflicts, err := db.ConflictingFacts("Mike", "Mike does not like late calls")
	if err != nil {
		t.Fatalf("ConflictingFacts negation: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0].ID != negation.ID {
		t.Fatalf("got %+v, want negation conflict only", conflicts)
	}

	conflicts, err = db.ConflictingFacts("Mike", "Mike prefers spaces")
	if err != nil {
		t.Fatalf("ConflictingFacts preference: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0].ID != preference.ID {
		t.Fatalf("got %+v, want exclusive preference conflict only", conflicts)
	}

	conflicts, err = db.ConflictingFacts("Mike", "Mike likes tea")
	if err != nil {
		t.Fatalf("ConflictingFacts adjacent: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("adjacent positive preferences should not conflict: %+v", conflicts)
	}
}
