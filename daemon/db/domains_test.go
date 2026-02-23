package db

import (
	"database/sql"
	"testing"
)

func TestSeedDefaultDomains(t *testing.T) {
	database := newTestDB(t)

	domains, err := database.ListDomains()
	if err != nil {
		t.Fatalf("list domains: %v", err)
	}
	if len(domains) != 7 {
		t.Fatalf("expected 7 seeded domains, got %d", len(domains))
	}

	if err := database.SeedDefaultDomains(); err != nil {
		t.Fatalf("seed again (idempotent): %v", err)
	}
	domains, err = database.ListDomains()
	if err != nil {
		t.Fatalf("list domains after re-seed: %v", err)
	}
	if len(domains) != 7 {
		t.Fatalf("expected 7 domains after idempotent seed, got %d", len(domains))
	}
}

func TestCreateDomain(t *testing.T) {
	database := newTestDB(t)

	d, err := database.CreateDomain("Test Domain", 4, "", "")
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}
	if d.Name != "Test Domain" {
		t.Fatalf("expected name 'Test Domain', got %q", d.Name)
	}
	if d.Importance != 4 {
		t.Fatalf("expected importance 4, got %d", d.Importance)
	}
	if d.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
}

func TestListDomains(t *testing.T) {
	database := newTestDB(t)

	domains, err := database.ListDomains()
	if err != nil {
		t.Fatalf("list domains: %v", err)
	}

	if len(domains) < 7 {
		t.Fatalf("expected at least 7 domains, got %d", len(domains))
	}

	if domains[0].Importance < domains[len(domains)-1].Importance {
		t.Fatal("expected domains ordered by importance DESC")
	}
}

func TestGetDomain(t *testing.T) {
	database := newTestDB(t)

	d, err := database.CreateDomain("Get Test", 3, "", "")
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}

	got, err := database.GetDomain(d.ID)
	if err != nil {
		t.Fatalf("get domain: %v", err)
	}
	if got == nil {
		t.Fatal("expected domain, got nil")
	}
	if got.Name != "Get Test" {
		t.Fatalf("expected name 'Get Test', got %q", got.Name)
	}

	missing, err := database.GetDomain(99999)
	if err != nil {
		t.Fatalf("get missing domain: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for missing domain")
	}
}

func TestUpdateDomain(t *testing.T) {
	database := newTestDB(t)

	d, err := database.CreateDomain("Update Test", 3, "", "")
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}

	sl := "active project"
	if err := database.UpdateDomain(d.ID, DomainUpdateOpts{StatusLine: &sl}); err != nil {
		t.Fatalf("update status_line: %v", err)
	}

	got, err := database.GetDomain(d.ID)
	if err != nil {
		t.Fatalf("get domain: %v", err)
	}
	if !got.StatusLine.Valid || got.StatusLine.String != "active project" {
		t.Fatalf("expected status_line 'active project', got %+v", got.StatusLine)
	}

	briefing := "current briefing"
	imp := 5
	if err := database.UpdateDomain(d.ID, DomainUpdateOpts{Briefing: &briefing, Importance: &imp}); err != nil {
		t.Fatalf("update briefing+importance: %v", err)
	}

	got, err = database.GetDomain(d.ID)
	if err != nil {
		t.Fatalf("get domain after second update: %v", err)
	}
	if !got.Briefing.Valid || got.Briefing.String != "current briefing" {
		t.Fatalf("expected briefing 'current briefing', got %+v", got.Briefing)
	}
	if got.Importance != 5 {
		t.Fatalf("expected importance 5, got %d", got.Importance)
	}
	if !got.StatusLine.Valid || got.StatusLine.String != "active project" {
		t.Fatalf("status_line should persist, got %+v", got.StatusLine)
	}

	err = database.UpdateDomain(99999, DomainUpdateOpts{StatusLine: &sl})
	if err == nil {
		t.Fatal("expected error for missing domain")
	}
}

func TestTouchDomain(t *testing.T) {
	database := newTestDB(t)

	d, err := database.CreateDomain("Touch Test", 3, "", "")
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}

	_, err = database.conn.Exec(
		`UPDATE domains SET last_touched_at = ? WHERE id = ?`,
		"2020-01-01T00:00:00Z", d.ID,
	)
	if err != nil {
		t.Fatalf("backdate domain: %v", err)
	}

	if err := database.TouchDomain(d.ID); err != nil {
		t.Fatalf("touch domain: %v", err)
	}

	got, err := database.GetDomain(d.ID)
	if err != nil {
		t.Fatalf("get domain: %v", err)
	}

	if got.LastTouchedAt == "2020-01-01T00:00:00Z" {
		t.Fatal("expected last_touched_at to be updated")
	}
}

func TestDomainDetail(t *testing.T) {
	database := newTestDB(t)

	d, err := database.CreateDomain("Detail Test", 4, "", "")
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}
	domainID := sql.NullInt64{Int64: int64(d.ID), Valid: true}

	err = database.InsertMemory(Memory{
		ID:         "mem-domain-detail",
		Type:       "Fact",
		Content:    "domain content",
		Title:      "Domain Memory",
		Importance: 0.8,
		Source:     "test",
		DomainID:   domainID,
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	err = database.InsertDeskItem(DeskItem{
		ID:       "desk-domain-detail",
		Title:    "Domain Task",
		Position: 1,
		Status:   "active",
		Date:     today(),
		DomainID: domainID,
	})
	if err != nil {
		t.Fatalf("insert desk item: %v", err)
	}

	detail, err := database.DomainDetailByID(d.ID)
	if err != nil {
		t.Fatalf("domain detail: %v", err)
	}
	if detail == nil {
		t.Fatal("expected domain detail, got nil")
	}
	if detail.Name != "Detail Test" {
		t.Fatalf("expected name 'Detail Test', got %q", detail.Name)
	}
	if len(detail.DeskItems) != 1 {
		t.Fatalf("expected 1 desk item, got %d", len(detail.DeskItems))
	}
	if len(detail.RecentMemories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(detail.RecentMemories))
	}

	missing, err := database.DomainDetailByID(99999)
	if err != nil {
		t.Fatalf("domain detail missing: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for missing domain detail")
	}
}

func TestComputeDriftStates(t *testing.T) {
	database := newTestDB(t)

	domains, err := database.ListDomains()
	if err != nil {
		t.Fatalf("list domains: %v", err)
	}
	if len(domains) == 0 {
		t.Fatal("expected seeded domains")
	}

	drifts, err := database.ComputeDriftStates()
	if err != nil {
		t.Fatalf("compute drift states: %v", err)
	}
	if len(drifts) != len(domains) {
		t.Fatalf("expected %d drift states, got %d", len(domains), len(drifts))
	}

	for _, drift := range drifts {
		if drift.State == "" {
			t.Fatalf("domain %q has empty state", drift.Name)
		}
		if drift.Label == "" {
			t.Fatalf("domain %q has empty label", drift.Name)
		}
	}

	d, err := database.CreateDomain("Fresh", 3, "", "")
	if err != nil {
		t.Fatalf("create fresh domain: %v", err)
	}
	if err := database.TouchDomain(d.ID); err != nil {
		t.Fatalf("touch fresh domain: %v", err)
	}

	drifts, err = database.ComputeDriftStates()
	if err != nil {
		t.Fatalf("compute drift states after touch: %v", err)
	}

	var fresh *DomainDrift
	for i := range drifts {
		if drifts[i].Name == "Fresh" {
			fresh = &drifts[i]
			break
		}
	}
	if fresh == nil {
		t.Fatal("expected to find 'Fresh' domain in drift states")
	}
	if fresh.State != "active" {
		t.Fatalf("expected fresh domain state 'active', got %q", fresh.State)
	}
}

func TestResolveDomainID(t *testing.T) {
	database := newTestDB(t)

	id, err := database.ResolveDomainID("Finances")
	if err != nil {
		t.Fatalf("resolve domain: %v", err)
	}
	if id == nil {
		t.Fatal("expected non-nil ID for 'Finances'")
	}

	id, err = database.ResolveDomainID("finanaces")
	if err != nil {
		t.Fatalf("resolve domain case-insensitive: %v", err)
	}
	if id == nil {
		t.Fatal("expected non-nil ID for 'finanaces' alias")
	}

	id, err = database.ResolveDomainID("Health")
	if err != nil {
		t.Fatalf("resolve legacy health alias: %v", err)
	}
	if id == nil {
		t.Fatal("expected non-nil ID for legacy alias 'Health'")
	}

	id, err = database.ResolveDomainID("Nonexistent Domain")
	if err != nil {
		t.Fatalf("resolve missing domain: %v", err)
	}
	if id != nil {
		t.Fatal("expected nil for nonexistent domain")
	}

	id, err = database.ResolveDomainID("")
	if err != nil {
		t.Fatalf("resolve empty: %v", err)
	}
	if id != nil {
		t.Fatal("expected nil for empty name")
	}
}

func TestDumpItems(t *testing.T) {
	database := newTestDB(t)

	_, err := database.InsertCapture("undomained capture", "test")
	if err != nil {
		t.Fatalf("insert capture: %v", err)
	}

	items, err := database.DumpItems(10)
	if err != nil {
		t.Fatalf("dump items: %v", err)
	}
	if len(items) < 1 {
		t.Fatal("expected at least 1 dump item")
	}
}

func TestDumpMemories(t *testing.T) {
	database := newTestDB(t)

	err := database.InsertMemory(Memory{
		ID:         "mem-dump",
		Type:       "Fact",
		Content:    "undomained memory",
		Title:      "Dump Memory",
		Importance: 0.5,
		Source:     "test",
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	memories, err := database.DumpMemories(10)
	if err != nil {
		t.Fatalf("dump memories: %v", err)
	}
	if len(memories) < 1 {
		t.Fatal("expected at least 1 dump memory")
	}
}

func TestCountDumpItems(t *testing.T) {
	database := newTestDB(t)

	if _, err := database.InsertCapture("undomained capture", "test"); err != nil {
		t.Fatalf("insert capture: %v", err)
	}
	if err := database.InsertMemory(Memory{
		ID:         "mem-dump-count",
		Type:       "Fact",
		Content:    "undomained memory",
		Title:      "Dump Count Memory",
		Importance: 0.5,
		Source:     "test",
	}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	count, err := database.CountDumpItems()
	if err != nil {
		t.Fatalf("count dump items: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected dump count 2, got %d", count)
	}
}

func TestTouchDomainByMemory(t *testing.T) {
	database := newTestDB(t)

	d, err := database.CreateDomain("Touch By Memory", 3, "", "")
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}

	_, err = database.conn.Exec(
		`UPDATE domains SET last_touched_at = ? WHERE id = ?`,
		"2020-01-01T00:00:00Z", d.ID,
	)
	if err != nil {
		t.Fatalf("backdate domain: %v", err)
	}

	err = database.InsertMemory(Memory{
		ID:         "mem-touch-domain",
		Type:       "Fact",
		Content:    "test",
		Title:      "test",
		Importance: 0.5,
		Source:     "test",
		DomainID:   sql.NullInt64{Int64: int64(d.ID), Valid: true},
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	database.TouchDomainByMemory("mem-touch-domain")

	got, err := database.GetDomain(d.ID)
	if err != nil {
		t.Fatalf("get domain: %v", err)
	}
	if got.LastTouchedAt == "2020-01-01T00:00:00Z" {
		t.Fatal("expected last_touched_at to change after touch by memory")
	}

	database.TouchDomainByMemory("nonexistent")
}

func TestDriftClassification(t *testing.T) {
	tests := []struct {
		name       string
		importance int
		days       int
		openTasks  int
		wantState  string
	}{
		{"overactive", 2, 0, 4, "overactive"},
		{"active", 5, 1, 0, "active"},
		{"neglected", 5, 6, 0, "neglected"},
		{"drifting", 3, 4, 0, "drifting"},
		{"cold", 1, 10, 0, "cold"},
		{"default fallback", 3, 3, 0, "drifting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driftScore := float64(tt.importance) * float64(tt.days)
			state, _ := classifyDrift(tt.importance, tt.days, driftScore, tt.openTasks)
			if state != tt.wantState {
				t.Fatalf("classifyDrift(%d, %d, %.0f, %d) state = %q, want %q",
					tt.importance, tt.days, driftScore, tt.openTasks, state, tt.wantState)
			}
		})
	}
}
