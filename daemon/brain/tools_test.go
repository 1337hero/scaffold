package brain

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"scaffold/agentprompt"
	"scaffold/db"
)

func openBrainTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func rawJSON(s string) json.RawMessage {
	return json.RawMessage(s)
}

func TestDefaultToolRegistryPRD15(t *testing.T) {
	registry := defaultToolRegistry()
	expected := []string{
		"save_memory",
		"search_memories",
		"create_task",
		"update_task",
		"list_tasks",
		"create_note",
		"get_calendar_events",
		"query_people",
		"query_facts",
		"save_fact",
	}

	if len(registry) != len(expected) {
		t.Fatalf("registry has %d tools, want %d: %#v", len(registry), len(expected), registry)
	}
	for _, name := range expected {
		if registry[name] == nil {
			t.Fatalf("missing tool handler %q", name)
		}
	}
	for _, removed := range []string{"create_calendar_event", "update_calendar_event", "save_to_inbox", "get_inbox", "create_goal", "list_goals", "search_email", "get_email"} {
		if registry[removed] != nil {
			t.Fatalf("removed tool %q is still registered", removed)
		}
	}
}

func TestSaveMemoryTool(t *testing.T) {
	database := openBrainTestDB(t)

	out, err := ExecuteTool(context.Background(), "save_memory", rawJSON(`{
		"content": "Mike prefers terse PR closeouts with verification notes.",
		"type": "Preference",
		"tags": ["prd", "closeout", "prd"],
		"importance": 0.8
	}`), database, nil, nil)
	if err != nil {
		t.Fatalf("save_memory: %v", err)
	}
	if !strings.Contains(out, "Memory saved") {
		t.Fatalf("unexpected output: %s", out)
	}

	memories, err := database.SearchMemoriesLike("verification notes", "Preference", 10)
	if err != nil {
		t.Fatalf("SearchMemoriesLike: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("got %d memories, want 1", len(memories))
	}
	if memories[0].Source != "signal" || memories[0].Tags != "prd,closeout" || memories[0].Importance != 0.8 {
		t.Fatalf("memory not normalized: %+v", memories[0])
	}

	if _, err := ExecuteTool(context.Background(), "save_memory", rawJSON(`{"title":"empty"}`), database, nil, nil); err == nil {
		t.Fatal("expected missing content to fail")
	}
}

func TestTaskToolsProjectAndReminder(t *testing.T) {
	database := openBrainTestDB(t)
	if err := database.InsertProject(db.Project{ID: "proj1", Name: "PRD 15"}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	_, err := ExecuteTool(context.Background(), "create_task", rawJSON(`{
		"title": "Run PRD 15 harness",
		"project_id": "proj1",
		"reminder_at": "2026-07-04T09:00:00Z",
		"priority": "high"
	}`), database, nil, nil)
	if err != nil {
		t.Fatalf("create_task: %v", err)
	}

	tasks, err := database.ListTasks(db.TaskFilters{ProjectID: strPtr("proj1")})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
	task := tasks[0]
	if task.ReminderAt.String != "2026-07-04T09:00:00Z" || task.Priority != "high" {
		t.Fatalf("task fields not stored: %+v", task)
	}

	_, err = ExecuteTool(context.Background(), "update_task", rawJSON(fmt.Sprintf(`{
		"id": %q,
		"reminder_at": "2026-07-05T10:30:00Z",
		"project_id": ""
	}`, task.ID)), database, nil, nil)
	if err != nil {
		t.Fatalf("update_task: %v", err)
	}

	updated, err := database.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if updated == nil || updated.ProjectID.Valid || updated.ReminderAt.String != "2026-07-05T10:30:00Z" {
		t.Fatalf("task update failed: %+v", updated)
	}

	list, err := ExecuteTool(context.Background(), "list_tasks", rawJSON(`{"reminder_at":"2026-07-05T10:30:00Z"}`), database, nil, nil)
	if err != nil {
		t.Fatalf("list_tasks: %v", err)
	}
	if !strings.Contains(list, "reminder=2026-07-05T10:30:00Z") {
		t.Fatalf("list_tasks omitted reminder: %s", list)
	}
}

func TestCreateNoteToolPersonProjectAndKind(t *testing.T) {
	database := openBrainTestDB(t)
	if err := database.InsertPerson(db.Person{ID: "person1", Name: "Jason"}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}
	if err := database.InsertProject(db.Project{ID: "proj1", Name: "PRD 15"}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	_, err := ExecuteTool(context.Background(), "create_note", rawJSON(`{
		"title": "Jason quote",
		"content": "Keep the tool surface small.",
		"person_id": "person1",
		"project_id": "proj1",
		"kind": "quote",
		"tags": ["agent", "tools"]
	}`), database, nil, nil)
	if err != nil {
		t.Fatalf("create_note: %v", err)
	}

	notes, err := database.ListNotes(db.NoteFilters{PersonID: strPtr("person1"), ProjectID: strPtr("proj1"), Kind: strPtr("quote")})
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1", len(notes))
	}
	if notes[0].Tags.String != "agent,tools" || notes[0].Content.String != "Keep the tool surface small." {
		t.Fatalf("note fields not stored: %+v", notes[0])
	}

	search, err := ExecuteTool(context.Background(), "search_memories", rawJSON(`{"query":"tool surface","type":"quote"}`), database, nil, nil)
	if err != nil {
		t.Fatalf("search_memories note search: %v", err)
	}
	if !strings.Contains(search, "Jason quote") || !strings.Contains(search, "[quote/0.5]") {
		t.Fatalf("search_memories did not include note result: %s", search)
	}

	if _, err := ExecuteTool(context.Background(), "create_note", rawJSON(`{"title":"bad","person_id":"missing"}`), database, nil, nil); err == nil {
		t.Fatal("expected FK failure for missing person")
	}
}

func TestQueryPeopleToolModes(t *testing.T) {
	database := openBrainTestDB(t)
	loc, err := time.LoadLocation("America/Boise")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	kidBirthday := time.Now().In(loc).AddDate(0, 0, 2).Format("01-02")
	kids, err := db.MarshalKids([]db.Kid{{Name: "Marcus", Birthday: "2014-" + kidBirthday}})
	if err != nil {
		t.Fatalf("MarshalKids: %v", err)
	}
	if err := database.InsertPerson(db.Person{
		ID:           "person1",
		Name:         "Jason",
		Surface:      "life",
		Relationship: sql.NullString{String: "friend", Valid: true},
		Kids:         kids,
	}); err != nil {
		t.Fatalf("InsertPerson: %v", err)
	}

	search, err := ExecuteTool(context.Background(), "query_people", rawJSON(`{"mode":"search","query":"jason"}`), database, nil, nil)
	if err != nil {
		t.Fatalf("query_people search: %v", err)
	}
	if !strings.Contains(search, "Jason") || !strings.Contains(search, "relationship=friend") {
		t.Fatalf("unexpected search output: %s", search)
	}

	_, err = ExecuteTool(context.Background(), "query_people", rawJSON(`{
		"mode": "log_interaction",
		"person_id": "person1",
		"summary": "Talked about PRD 15",
		"date": "2026-07-04",
		"follow_up": "Send summary",
		"follow_up_date": "2026-07-05"
	}`), database, nil, nil)
	if err != nil {
		t.Fatalf("query_people log_interaction: %v", err)
	}
	interactions, err := database.ListInteractions("person1")
	if err != nil {
		t.Fatalf("ListInteractions: %v", err)
	}
	if len(interactions) != 1 || interactions[0].FollowUpDate.String != "2026-07-05" {
		t.Fatalf("interaction not stored: %+v", interactions)
	}

	birthdays, err := ExecuteTool(context.Background(), "query_people", rawJSON(`{"mode":"birthdays","days":7}`), database, nil, nil)
	if err != nil {
		t.Fatalf("query_people birthdays: %v", err)
	}
	if !strings.Contains(birthdays, "Marcus") || !strings.Contains(birthdays, "urgency=") {
		t.Fatalf("unexpected birthday output: %s", birthdays)
	}

	if _, err := ExecuteTool(context.Background(), "query_people", rawJSON(`{"mode":"log_interaction","summary":"missing person"}`), database, nil, nil); err == nil {
		t.Fatal("expected missing person_id to fail")
	}
}

func TestFactTools(t *testing.T) {
	database := openBrainTestDB(t)

	_, err := ExecuteTool(context.Background(), "save_fact", rawJSON(`{
		"entity": "PRD 15",
		"content": "The agent tool set has ten witness tools.",
		"category": "project",
		"tags": ["agent", "tools"],
		"related_entities": ["Scaffold", "Signal"]
	}`), database, nil, nil)
	if err != nil {
		t.Fatalf("save_fact: %v", err)
	}

	facts, err := database.ListFacts(strPtr("PRD 15"), nil, nil, 10)
	if err != nil {
		t.Fatalf("ListFacts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("got %d facts, want 1", len(facts))
	}
	if facts[0].Trust != 0.5 || facts[0].Category.String != "project" || facts[0].Tags.String != "agent,tools" {
		t.Fatalf("fact not normalized: %+v", facts[0])
	}
	edges, err := database.ListFactEdges(facts[0].ID)
	if err != nil {
		t.Fatalf("ListFactEdges: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("got %d fact edges, want 2: %+v", len(edges), edges)
	}

	out, err := ExecuteTool(context.Background(), "query_facts", rawJSON(`{"entity":"PRD 15"}`), database, nil, nil)
	if err != nil {
		t.Fatalf("query_facts: %v", err)
	}
	if !strings.Contains(out, "ten witness tools") || !strings.Contains(out, "trust=0.50") {
		t.Fatalf("unexpected facts output: %s", out)
	}

	if _, err := ExecuteTool(context.Background(), "save_fact", rawJSON(`{"entity":"PRD 15","content":"bad","category":"bogus"}`), database, nil, nil); err == nil {
		t.Fatal("expected invalid category to fail")
	}
}

func TestSaveFactSurfacesConflictsWithoutSaving(t *testing.T) {
	database := openBrainTestDB(t)
	if _, err := ExecuteTool(context.Background(), "save_fact", rawJSON(`{
		"entity": "Mike",
		"content": "Mike prefers tabs."
	}`), database, nil, nil); err != nil {
		t.Fatalf("initial save_fact: %v", err)
	}

	out, err := ExecuteTool(context.Background(), "save_fact", rawJSON(`{
		"entity": "Mike",
		"content": "Mike prefers spaces."
	}`), database, nil, nil)
	if err != nil {
		t.Fatalf("conflicting save_fact: %v", err)
	}
	if !strings.Contains(out, "Potential fact conflict") || !strings.Contains(out, "not saved") {
		t.Fatalf("conflict was not surfaced: %s", out)
	}

	facts, err := database.ListFacts(strPtr("Mike"), nil, nil, 10)
	if err != nil {
		t.Fatalf("ListFacts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("got %d facts, want only original fact: %+v", len(facts), facts)
	}
}

type promptCaptureResponder struct {
	prompt string
}

func (r *promptCaptureResponder) Respond(_ context.Context, req ToolUseRequest) (*ToolUseResponse, error) {
	r.prompt = req.SystemPrompt
	return &ToolUseResponse{Text: "ok"}, nil
}

func TestRespondAssemblesIdentityPromptWithBulletinSurfaceAndFacts(t *testing.T) {
	database := openBrainTestDB(t)
	fact := insertBrainFact(t, database, db.Fact{
		Entity:  "Mike",
		Content: "Mike prefers concise PRD closeouts.",
		Trust:   0.8,
	})
	insertBrainFact(t, database, db.Fact{Entity: "Mike", Content: "low trust", Trust: 0.2})

	responder := &promptCaptureResponder{}
	b := NewWithDependencies(database, Config{
		UserName:        "Mike",
		PromptFactLimit: 1,
		Identity: agentprompt.Identity{
			Name:     "Scaffold",
			UserName: "Mike",
			Voice:    []string{"Direct, warm, concise."},
			CannotDo: []string{"Access email.", "Run code."},
		},
	}, responder)
	b.SetBulletinProvider(func() (string, bool) {
		return "Fence project keeps slipping.", true
	})

	if _, err := b.Respond(context.Background(), "switch to life mode; what do you remember about PRD closeouts?", nil); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	for _, want := range []string{
		"Direct, warm, concise.",
		"LifeOS",
		"Fence project keeps slipping.",
		"Mike prefers concise PRD closeouts.",
		"Access email.",
	} {
		if !strings.Contains(responder.prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, responder.prompt)
		}
	}
	if strings.Contains(responder.prompt, "low trust") {
		t.Fatalf("low-trust fact should not be injected:\n%s", responder.prompt)
	}

	got, _ := database.GetFact(fact.ID)
	if got.RetrievalCount != 1 {
		t.Fatalf("retrieval_count=%d, want prompt injection to bump once", got.RetrievalCount)
	}
}

type loopingResponder struct {
	calls  int
	models []string
}

func (r *loopingResponder) Respond(_ context.Context, req ToolUseRequest) (*ToolUseResponse, error) {
	r.calls++
	r.models = append(r.models, req.Model)
	return &ToolUseResponse{
		Text: "still using tools",
		ToolCalls: []ToolCall{{
			ID:    fmt.Sprintf("tool-%d", r.calls),
			Name:  "unknown_tool",
			Input: rawJSON(`{}`),
		}},
	}, nil
}

func TestRespondToolLoopCapAndDefaultModel(t *testing.T) {
	responder := &loopingResponder{}
	b := NewWithDependencies(nil, Config{}, responder)

	_, err := b.Respond(context.Background(), "keep going", nil)
	if err == nil {
		t.Fatal("expected tool loop cap error")
	}
	if !strings.Contains(err.Error(), "tool loop exceeded 5 rounds") {
		t.Fatalf("unexpected error: %v", err)
	}
	if responder.calls != 5 {
		t.Fatalf("model called %d times, want 5", responder.calls)
	}
	for _, model := range responder.models {
		if model != "claude-sonnet-4-6" {
			t.Fatalf("default respond model = %q, want claude-sonnet-4-6", model)
		}
	}
}

func strPtr(s string) *string {
	return &s
}

func insertBrainFact(t *testing.T, database *db.DB, fact db.Fact) db.Fact {
	t.Helper()
	if fact.ID == "" {
		fact.ID = "fact-" + strings.ReplaceAll(fact.Content, " ", "-")
	}
	if err := database.InsertFact(fact, nil); err != nil {
		t.Fatalf("InsertFact: %v", err)
	}
	return fact
}
