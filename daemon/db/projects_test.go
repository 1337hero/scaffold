package db

import (
	"database/sql"
	"encoding/json"
	"testing"
)

// --- Project CRUD ---

func TestInsertAndGetProject(t *testing.T) {
	db := openTestDB(t)

	p := Project{
		Name:    "Website Redesign",
		Type:    "project",
		Surface: "business",
		Status:  "active",
	}
	if err := db.InsertProject(p); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	projects, err := db.ListProjects(nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("got %d projects, want 1", len(projects))
	}

	got, err := db.GetProject(projects[0].ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got == nil || got.Name != "Website Redesign" || got.Type != "project" {
		t.Fatalf("GetProject returned %+v", got)
	}
	if got.CreatedAt == "" || got.UpdatedAt == "" {
		t.Fatalf("timestamps not set: %+v", got)
	}
}

func TestGetProject_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := db.GetProject("missing")
	if err != nil {
		t.Fatalf("GetProject err: %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

func TestUpdateProject(t *testing.T) {
	db := openTestDB(t)
	p := Project{ID: "p1", Name: "Fence", Type: "project", Surface: "life"}
	if err := db.InsertProject(p); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	if err := db.UpdateProject("p1", map[string]any{
		"description": "replace the back fence",
		"status":      "on_hold",
	}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	got, _ := db.GetProject("p1")
	if got.Description.String != "replace the back fence" {
		t.Fatalf("description=%q, want 'replace the back fence'", got.Description.String)
	}
	if got.Status != "on_hold" {
		t.Fatalf("status=%q, want on_hold", got.Status)
	}

	if err := db.UpdateProject("p1", map[string]any{"bogus": 1}); err == nil {
		t.Fatalf("expected error updating non-whitelisted column")
	}
	if err := db.UpdateProject("missing", map[string]any{"status": "active"}); err != sql.ErrNoRows {
		t.Fatalf("update missing: got %v, want sql.ErrNoRows", err)
	}
}

func TestSuppressProject(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertProject(Project{ID: "p1", Name: "Ghost", Type: "area", Surface: "life"}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	if err := db.SuppressProject("p1"); err != nil {
		t.Fatalf("SuppressProject: %v", err)
	}

	// Archived projects excluded from default list.
	projects, _ := db.ListProjects(nil, nil, nil, nil)
	if len(projects) != 0 {
		t.Fatalf("archived project still listed: %d", len(projects))
	}

	// But visible with explicit status filter.
	archived := "archived"
	projects, _ = db.ListProjects(nil, nil, &archived, nil)
	if len(projects) != 1 {
		t.Fatalf("archived filter returned %d, want 1", len(projects))
	}

	if err := db.SuppressProject("missing"); err != sql.ErrNoRows {
		t.Fatalf("suppress missing: got %v, want sql.ErrNoRows", err)
	}
}

func TestInsertProjectDefaults(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertProject(Project{Name: "Health", Surface: "life"})

	projects, _ := db.ListProjects(nil, nil, nil, nil)
	if len(projects) != 1 {
		t.Fatalf("got %d, want 1", len(projects))
	}
	if projects[0].Type != "project" {
		t.Fatalf("type=%q, want 'project'", projects[0].Type)
	}
	if projects[0].Status != "active" {
		t.Fatalf("status=%q, want 'active'", projects[0].Status)
	}
}

func TestInsertProject_EnumValidation(t *testing.T) {
	db := openTestDB(t)

	tests := []struct {
		name string
		p    Project
	}{
		{"bad type", Project{Name: "x", Type: "retianer"}},
		{"bad surface", Project{Name: "x", Surface: "bluelife"}},
		{"bad status", Project{Name: "x", Status: "archvied"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.InsertProject(tt.p)
			if err == nil {
				t.Fatal("expected error for invalid enum value, got nil")
			}
		})
	}
}

func TestListProjectsFilter(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertProject(Project{Name: "Client Work", Type: "project", Surface: "business"})
	_ = db.InsertProject(Project{Name: "Fitness", Type: "area", Surface: "life"})
	_ = db.InsertProject(Project{Name: "Monthly Support", Type: "retainer", Surface: "business"})

	project := "project"
	got, err := db.ListProjects(&project, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Client Work" {
		t.Fatalf("type=project returned %+v", got)
	}

	area := "area"
	got, _ = db.ListProjects(&area, nil, nil, nil)
	if len(got) != 1 || got[0].Name != "Fitness" {
		t.Fatalf("type=area returned %+v", got)
	}

	life := "life"
	got, _ = db.ListProjects(nil, &life, nil, nil)
	if len(got) != 1 || got[0].Name != "Fitness" {
		t.Fatalf("surface=life returned %+v", got)
	}

	biz := "business"
	got, _ = db.ListProjects(nil, &biz, nil, nil)
	if len(got) != 2 {
		t.Fatalf("surface=business returned %d, want 2", len(got))
	}
}

// --- Milestones ---

func TestInsertAndListMilestones(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertProject(Project{ID: "proj", Name: "Build Deck", Type: "project", Surface: "life"})

	if err := db.InsertMilestone(Milestone{ProjectID: "proj", Title: "Design", Position: 1}); err != nil {
		t.Fatalf("InsertMilestone: %v", err)
	}
	if err := db.InsertMilestone(Milestone{ProjectID: "proj", Title: "Pour footings", Position: 2}); err != nil {
		t.Fatalf("InsertMilestone: %v", err)
	}

	milestones, err := db.ListMilestones("proj")
	if err != nil {
		t.Fatalf("ListMilestones: %v", err)
	}
	if len(milestones) != 2 {
		t.Fatalf("got %d milestones, want 2", len(milestones))
	}
	// Ordered by position.
	if milestones[0].Title != "Design" || milestones[1].Title != "Pour footings" {
		t.Fatalf("milestones out of order: %+v", milestones)
	}

	// Completion: 0 of 2.
	completed, total, err := db.MilestoneCompletion("proj")
	if err != nil {
		t.Fatalf("MilestoneCompletion: %v", err)
	}
	if completed != 0 || total != 2 {
		t.Fatalf("got completed=%d total=%d, want 0/2", completed, total)
	}

	// Complete one.
	if err := db.UpdateMilestone(milestones[0].ID, map[string]any{"completed": 1}); err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}

	completed, total, _ = db.MilestoneCompletion("proj")
	if completed != 1 || total != 2 {
		t.Fatalf("after completing one: got %d/%d, want 1/2", completed, total)
	}
}

func TestDeleteMilestone(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertProject(Project{ID: "proj", Name: "Shed", Type: "project", Surface: "life"})
	_ = db.InsertMilestone(Milestone{ID: "m1", ProjectID: "proj", Title: "Frame", Position: 1})

	if err := db.DeleteMilestone("m1"); err != nil {
		t.Fatalf("DeleteMilestone: %v", err)
	}
	if err := db.DeleteMilestone("m1"); err != sql.ErrNoRows {
		t.Fatalf("delete again: got %v, want sql.ErrNoRows", err)
	}

	milestones, _ := db.ListMilestones("proj")
	if len(milestones) != 0 {
		t.Fatalf("got %d after delete, want 0", len(milestones))
	}
}

// --- Checklists ---

func TestInsertAndListChecklists(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertProject(Project{ID: "proj", Name: "Site Launch", Type: "project", Surface: "business"})

	c := Checklist{
		ProjectID: sql.NullString{String: "proj", Valid: true},
		Title:     "Go-live",
		Items:     `[{"text":"DNS setup","completed":false},{"text":"SSL cert","completed":true}]`,
	}
	if err := db.InsertChecklist(c); err != nil {
		t.Fatalf("InsertChecklist: %v", err)
	}

	checklists, err := db.ListChecklists("proj")
	if err != nil {
		t.Fatalf("ListChecklists: %v", err)
	}
	if len(checklists) != 1 || checklists[0].Title != "Go-live" {
		t.Fatalf("ListChecklists returned %+v", checklists)
	}
}

func TestChecklistTemplates(t *testing.T) {
	db := openTestDB(t)
	// Insert a template (project_id null + is_template=1)
	_ = db.InsertChecklist(Checklist{
		ID:         "tpl1",
		Title:      "New Website",
		Items:      `[{"text":"Buy domain","completed":false}]`,
		IsTemplate: 1,
	})
	// Insert a normal checklist on a project
	_ = db.InsertProject(Project{ID: "proj", Name: "Acme", Type: "project", Surface: "business"})
	_ = db.InsertChecklist(Checklist{
		ProjectID: sql.NullString{String: "proj", Valid: true},
		Title:     "Launch",
		Items:     `[]`,
	})

	templates, err := db.ListChecklistTemplates()
	if err != nil {
		t.Fatalf("ListChecklistTemplates: %v", err)
	}
	if len(templates) != 1 || templates[0].Title != "New Website" {
		t.Fatalf("got %+v, want 1 template named 'New Website'", templates)
	}
}

func TestCloneChecklist(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertChecklist(Checklist{
		ID:         "tpl1",
		Title:      "New Website",
		Items:      `[{"text":"Buy domain","completed":true},{"text":"Set up hosting","completed":false}]`,
		IsTemplate: 1,
	})
	_ = db.InsertProject(Project{ID: "proj", Name: "Client Site", Type: "project", Surface: "business"})

	cloned, err := db.CloneChecklist("tpl1", "proj")
	if err != nil {
		t.Fatalf("CloneChecklist: %v", err)
	}
	if cloned == nil {
		t.Fatalf("CloneChecklist returned nil")
	}
	if cloned.Title != "New Website" {
		t.Fatalf("title=%q, want 'New Website'", cloned.Title)
	}
	if !cloned.ProjectID.Valid || cloned.ProjectID.String != "proj" {
		t.Fatalf("project_id=%+v, want 'proj'", cloned.ProjectID)
	}

	// Verify items were reset — both should be false.
	var items []map[string]any
	if err := json.Unmarshal([]byte(cloned.Items), &items); err != nil {
		t.Fatalf("parse cloned items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	for i, item := range items {
		if item["completed"].(bool) {
			t.Fatalf("item %d is still completed after clone", i)
		}
	}

	// Unknown template returns nil, not error.
	missing, err := db.CloneChecklist("nope", "proj")
	if err != nil {
		t.Fatalf("CloneChecklist missing template: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil for missing template, got %+v", missing)
	}
}

// --- Activity ---

func TestInsertAndListActivity(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertProject(Project{ID: "proj", Name: "Redesign", Type: "project", Surface: "business"})

	if err := db.InsertActivity(Activity{
		ProjectID:   "proj",
		Description: "Wireframed homepage",
		Hours:       sql.NullFloat64{Float64: 2.5, Valid: true},
	}); err != nil {
		t.Fatalf("InsertActivity: %v", err)
	}

	activity, err := db.ListActivity("proj")
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	if len(activity) != 1 || activity[0].Description != "Wireframed homepage" {
		t.Fatalf("ListActivity returned %+v", activity)
	}

	// Verify last_activity_at was bumped.
	proj, _ := db.GetProject("proj")
	if !proj.LastActivityAt.Valid || proj.LastActivityAt.String == "" {
		t.Fatalf("last_activity_at not set after InsertActivity")
	}
}

func TestInsertActivity_BumpsLastActivityForwardOnly(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertProject(Project{ID: "proj", Name: "Site", Type: "project", Surface: "business"})

	// First activity stamps today.
	_ = db.InsertActivity(Activity{ProjectID: "proj", Description: "first"})
	proj, _ := db.GetProject("proj")
	firstActivityDate := proj.LastActivityAt.String

	// Second activity (same day, should stay same or advance).
	_ = db.InsertActivity(Activity{ProjectID: "proj", Description: "second"})
	proj, _ = db.GetProject("proj")
	if proj.LastActivityAt.String < firstActivityDate {
		t.Fatalf("last_activity_at regressed from %s to %s", firstActivityDate, proj.LastActivityAt.String)
	}
}

// --- Slipping ---

func TestProjectsSlipping(t *testing.T) {
	db := openTestDB(t)

	// Active project with no activity — should slip (7+ days threshold, NULL means slipping).
	_ = db.InsertProject(Project{ID: "slip", Name: "Neglected", Type: "project", Surface: "life"})
	// Active project with recent activity — should not slip.
	_ = db.InsertProject(Project{ID: "fresh", Name: "Active", Type: "project", Surface: "life"})
	_ = db.InsertActivity(Activity{ProjectID: "fresh", Description: "just did"})
	// Area with no activity — excluded from ProjectsSlipping.
	_ = db.InsertProject(Project{ID: "area", Name: "Health", Type: "area", Surface: "life"})

	slipping, err := db.ProjectsSlipping()
	if err != nil {
		t.Fatalf("ProjectsSlipping: %v", err)
	}
	if len(slipping) != 1 || slipping[0].ID != "slip" {
		t.Fatalf("ProjectsSlipping returned %+v", slipping)
	}
}

func TestAreasSlipping(t *testing.T) {
	db := openTestDB(t)

	// Active area with no activity — should slip.
	_ = db.InsertProject(Project{ID: "slip", Name: "Fitness", Type: "area", Surface: "life"})
	// Active area with activity — should not slip.
	_ = db.InsertProject(Project{ID: "fresh", Name: "Reading", Type: "area", Surface: "life"})
	_ = db.InsertActivity(Activity{ProjectID: "fresh", Description: "read a book"})
	// Project — excluded from AreasSlipping.
	_ = db.InsertProject(Project{ID: "proj", Name: "Shed", Type: "project", Surface: "life"})

	slipping, err := db.AreasSlipping()
	if err != nil {
		t.Fatalf("AreasSlipping: %v", err)
	}
	if len(slipping) != 1 || slipping[0].ID != "slip" {
		t.Fatalf("AreasSlipping returned %+v", slipping)
	}
}

func TestResetRetainerChecklists(t *testing.T) {
	db := openTestDB(t)

	// Create a retainer project with ACTUAL checklists (not templates).
	_ = db.InsertProject(Project{ID: "retainer", Name: "Monthly Support", Type: "retainer", Surface: "business"})

	// Insert two checklists with some completed items.
	_ = db.InsertChecklist(Checklist{
		ID:        "cl1",
		ProjectID: sql.NullString{String: "retainer", Valid: true},
		Title:     "Monthly Tasks",
		Items:     `[{"text":"Review logs","completed":true},{"text":"Send report","completed":false}]`,
	})
	_ = db.InsertChecklist(Checklist{
		ID:        "cl2",
		ProjectID: sql.NullString{String: "retainer", Valid: true},
		Title:     "Weekly Review",
		Items:     `[{"text":"Check uptime","completed":true},{"text":"Backup DB","completed":true}]`,
	})

	// Also log an activity — should NOT prevent the reset.
	_ = db.InsertActivity(Activity{ProjectID: "retainer", Description: "Monthly check-in"})

	n, err := db.ResetRetainerChecklists()
	if err != nil {
		t.Fatalf("ResetRetainerChecklists: %v", err)
	}
	if n != 1 {
		t.Fatalf("reset count=%d, want 1", n)
	}

	// Verify items were reset in-place.
	checklists, err := db.ListChecklists("retainer")
	if err != nil {
		t.Fatalf("ListChecklists: %v", err)
	}
	if len(checklists) != 2 {
		t.Fatalf("got %d checklists after reset, want 2 (no cloning)", len(checklists))
	}

	// Both checklists should have all items reset to completed=false.
	for _, cl := range checklists {
		var items []map[string]any
		if err := json.Unmarshal([]byte(cl.Items), &items); err != nil {
			t.Fatalf("parse items: %v", err)
		}
		for i, item := range items {
			if item["completed"].(bool) {
				t.Fatalf("checklist %s item %d still completed after reset", cl.ID, i)
			}
		}
	}

	// Verify last_reset_at was stamped but last_activity_at is unchanged (from InsertActivity).
	proj, _ := db.GetProject("retainer")
	if !proj.LastResetAt.Valid || proj.LastResetAt.String == "" {
		t.Fatalf("last_reset_at not set")
	}
	if !proj.LastActivityAt.Valid || proj.LastActivityAt.String == "" {
		t.Fatalf("last_activity_at wiped by reset")
	}

	// Second call in same month — should be a no-op.
	n, err = db.ResetRetainerChecklists()
	if err != nil {
		t.Fatalf("second ResetRetainerChecklists: %v", err)
	}
	if n != 0 {
		t.Fatalf("second reset returned %d, want 0 (already reset this month)", n)
	}
}
