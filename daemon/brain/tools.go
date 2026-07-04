package brain

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	"scaffold/db"
	googlecal "scaffold/google"
)

type ToolHandler func(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error)

func defaultToolRegistry() map[string]ToolHandler {
	return map[string]ToolHandler{
		"save_memory":         handleSaveMemory,
		"search_memories":     handleSearchMemories,
		"get_calendar_events": handleGetCalendarEvents,
		"create_task":         handleCreateTask,
		"create_note":         handleCreateNote,
		"update_task":         handleUpdateTask,
		"list_tasks":          handleListTasks,
		"query_people":        handleQueryPeople,
		"query_facts":         handleQueryFacts,
		"save_fact":           handleSaveFact,
	}
}

func ExecuteTool(ctx context.Context, name string, params json.RawMessage, database *db.DB, b *Brain, registry map[string]ToolHandler) (string, error) {
	if len(registry) == 0 {
		registry = defaultToolRegistry()
	}
	handler, ok := registry[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return handler(ctx, database, b, params)
}

func handleSaveMemory(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("save_memory: database is required")
	}

	var p struct {
		Title      string   `json:"title"`
		Content    string   `json:"content"`
		Type       string   `json:"type"`
		Importance *float64 `json:"importance"`
		Source     string   `json:"source"`
		Tags       []string `json:"tags"`
		Domain     string   `json:"domain"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("save_memory: bad params: %w", err)
	}
	p.Content = strings.TrimSpace(p.Content)
	if p.Content == "" {
		return "", fmt.Errorf("save_memory: content required")
	}

	memType := strings.TrimSpace(p.Type)
	if memType == "" {
		memType = "Memory"
	}
	source := strings.TrimSpace(p.Source)
	if source == "" {
		source = "signal"
	}
	importance := 0.5
	if p.Importance != nil {
		if *p.Importance < 0 || *p.Importance > 1 {
			return "", fmt.Errorf("save_memory: importance must be between 0 and 1")
		}
		importance = *p.Importance
	}

	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = summarizeToolText(p.Content, 80)
	}

	mem := db.Memory{
		ID:         uuid.New().String(),
		Type:       memType,
		Content:    p.Content,
		Title:      title,
		Importance: importance,
		Source:     source,
		Tags:       joinToolTags(p.Tags),
		DomainID:   resolveDomain(database, p.Domain),
	}
	if err := database.InsertMemory(mem); err != nil {
		return "", fmt.Errorf("save_memory: %w", err)
	}
	return fmt.Sprintf("Memory saved: %q (id=%s)", mem.Title, mem.ID), nil
}

func handleSearchMemories(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("search_memories: database is required")
	}

	var p struct {
		Query string `json:"query"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("search_memories: bad params: %w", err)
	}
	if p.Query == "" {
		return "", fmt.Errorf("search_memories: query required")
	}
	requestedType := strings.TrimSpace(p.Type)

	const topK = 10

	memoryResults := []db.ScoredMemory{}
	ftsResults, err := database.SearchFTS(p.Query, topK*3)
	if err == nil && len(ftsResults) > 0 {
		filtered := filterScoredMemoriesByType(ftsResults, requestedType, topK)
		if len(filtered) > 0 {
			memoryResults = filtered
		}
	}

	if len(memoryResults) == 0 {
		memoryResults, err = database.SearchMemoriesLike(p.Query, requestedType, topK)
		if err != nil {
			return "", fmt.Errorf("search_memories: %w", err)
		}
	}

	noteResults, err := searchNotesAsResults(database, p.Query, requestedType, topK-len(memoryResults))
	if err != nil {
		return "", fmt.Errorf("search_memories notes: %w", err)
	}

	results := append(memoryResults, noteResults...)
	if len(results) == 0 {
		return fmt.Sprintf("No memories found matching %q.", p.Query), nil
	}
	markSearchAccess(database, memoryResults)
	return formatSearchResults(p.Query, results), nil
}

func searchNotesAsResults(database *db.DB, query, requestedType string, limit int) ([]db.ScoredMemory, error) {
	if limit <= 0 {
		return nil, nil
	}
	kind, include := noteKindFilter(requestedType)
	if !include {
		return nil, nil
	}

	notes, err := database.ListNotes(db.NoteFilters{Query: query, Kind: kind})
	if err != nil {
		return nil, err
	}
	if len(notes) > limit {
		notes = notes[:limit]
	}

	results := make([]db.ScoredMemory, 0, len(notes))
	for _, note := range notes {
		results = append(results, db.ScoredMemory{
			Memory: db.Memory{
				ID:         note.ID,
				Type:       noteResultType(note.Kind),
				Content:    note.Content.String,
				Title:      note.Title,
				Importance: 0.5,
				Source:     note.Source.String,
				Tags:       note.Tags.String,
				CreatedAt:  note.CreatedAt,
				UpdatedAt:  note.UpdatedAt.String,
			},
		})
	}
	return results, nil
}

func noteKindFilter(requestedType string) (*string, bool) {
	switch strings.ToLower(strings.TrimSpace(requestedType)) {
	case "":
		return nil, true
	case "note":
		return nil, true
	case "journal":
		kind := "journal"
		return &kind, true
	case "quote":
		kind := "quote"
		return &kind, true
	default:
		return nil, false
	}
}

func noteResultType(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "journal":
		return "journal"
	case "quote":
		return "quote"
	default:
		return "Note"
	}
}

func filterScoredMemoriesByType(results []db.ScoredMemory, requestedType string, limit int) []db.ScoredMemory {
	if limit <= 0 {
		limit = len(results)
	}
	requestedType = strings.TrimSpace(requestedType)
	if requestedType == "" {
		if len(results) > limit {
			return results[:limit]
		}
		return results
	}

	filtered := make([]db.ScoredMemory, 0, len(results))
	for _, result := range results {
		if strings.EqualFold(strings.TrimSpace(result.Type), requestedType) {
			filtered = append(filtered, result)
			if len(filtered) >= limit {
				break
			}
		}
	}
	return filtered
}

func formatSearchResults(query string, results []db.ScoredMemory) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d results matching %q:\n", len(results), query))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. [%s/%.1f] %s\n", i+1, r.Type, r.Importance, r.Title))
		if r.Content != r.Title {
			content := r.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %s\n", content))
		}
		if r.FTSScore > 0 {
			sb.WriteString(fmt.Sprintf("   score: %.3f\n", r.FTSScore))
		}
	}
	return sb.String()
}

func markSearchAccess(database *db.DB, results []db.ScoredMemory) {
	if database == nil || len(results) == 0 {
		return
	}

	seen := make(map[string]struct{}, len(results))
	ids := make([]string, 0, len(results))
	for _, result := range results {
		if id := strings.TrimSpace(result.ID); id != "" {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}

	if err := database.MarkMemoriesAccessed(ids); err != nil {
		log.Printf("search_memories: mark accessed failed: %v", err)
	}
}

func handleGetCalendarEvents(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if b == nil || b.calendarClient == nil {
		return "Google Calendar is not configured. Ask Mike to run: scaffold-daemon auth google", nil
	}

	var p struct {
		Scope string `json:"scope"`
		Hours int    `json:"hours"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("get_calendar_events: bad params: %w", err)
	}

	hours := p.Hours
	if hours <= 0 {
		hours = 4
	}
	if hours > 24 {
		hours = 24
	}

	calendarID := b.calendarClient.CalendarID

	var events []googlecal.Event
	var err error

	switch p.Scope {
	case "upcoming":
		events, err = b.calendarClient.UpcomingEvents(ctx, calendarID, hours)
	default:
		events, err = b.calendarClient.TodayEvents(ctx, calendarID)
	}

	if err != nil {
		return "", fmt.Errorf("get_calendar_events: %w", err)
	}

	return googlecal.FormatEvents(events), nil
}

func resolveDomain(database *db.DB, name string) sql.NullInt64 {
	name = strings.TrimSpace(name)
	if name == "" {
		return sql.NullInt64{}
	}
	resolved, err := database.ResolveDomainID(name)
	if err != nil || resolved == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*resolved), Valid: true}
}

func handleCreateTask(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("create_task: database is required")
	}

	var p struct {
		Title      string `json:"title"`
		Domain     string `json:"domain"`
		Context    string `json:"context"`
		DueDate    string `json:"due_date"`
		Recurring  string `json:"recurring"`
		Priority   string `json:"priority"`
		ProjectID  string `json:"project_id"`
		ReminderAt string `json:"reminder_at"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("create_task: bad params: %w", err)
	}
	if p.Title == "" {
		return "", fmt.Errorf("create_task: title required")
	}

	t := db.Task{
		ID:       uuid.New().String(),
		Title:    p.Title,
		DomainID: resolveDomain(database, p.Domain),
	}
	if p.Context != "" {
		t.Context = sql.NullString{String: p.Context, Valid: true}
	}
	if p.DueDate != "" {
		t.DueDate = sql.NullString{String: p.DueDate, Valid: true}
	}
	if p.Recurring != "" {
		t.Recurring = sql.NullString{String: p.Recurring, Valid: true}
	}
	if p.Priority != "" {
		t.Priority = p.Priority
	}
	if strings.TrimSpace(p.ProjectID) != "" {
		t.ProjectID = sql.NullString{String: strings.TrimSpace(p.ProjectID), Valid: true}
	}
	if strings.TrimSpace(p.ReminderAt) != "" {
		t.ReminderAt = sql.NullString{String: strings.TrimSpace(p.ReminderAt), Valid: true}
	}

	if err := database.InsertTask(t); err != nil {
		return "", fmt.Errorf("create_task: %w", err)
	}
	return fmt.Sprintf("Task created: %q (id=%s)", p.Title, t.ID), nil
}

func handleCreateNote(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("create_note: database is required")
	}

	var p struct {
		Title     string   `json:"title"`
		Domain    string   `json:"domain"`
		TaskID    string   `json:"task_id"`
		PersonID  string   `json:"person_id"`
		ProjectID string   `json:"project_id"`
		Kind      string   `json:"kind"`
		Content   string   `json:"content"`
		Tags      []string `json:"tags"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("create_note: bad params: %w", err)
	}
	if p.Title == "" {
		return "", fmt.Errorf("create_note: title required")
	}

	n := db.Note{
		ID:       uuid.New().String(),
		Title:    p.Title,
		DomainID: resolveDomain(database, p.Domain),
	}
	if p.TaskID != "" {
		n.TaskID = sql.NullString{String: p.TaskID, Valid: true}
	}
	if strings.TrimSpace(p.PersonID) != "" {
		n.PersonID = sql.NullString{String: strings.TrimSpace(p.PersonID), Valid: true}
	}
	if strings.TrimSpace(p.ProjectID) != "" {
		n.ProjectID = sql.NullString{String: strings.TrimSpace(p.ProjectID), Valid: true}
	}
	if strings.TrimSpace(p.Kind) != "" {
		n.Kind = strings.TrimSpace(p.Kind)
	}
	if p.Content != "" {
		n.Content = sql.NullString{String: p.Content, Valid: true}
	}
	if len(p.Tags) > 0 {
		n.Tags = sql.NullString{String: joinToolTags(p.Tags), Valid: true}
	}

	if err := database.InsertNote(n); err != nil {
		return "", fmt.Errorf("create_note: %w", err)
	}
	return fmt.Sprintf("Note created: %q (id=%s)", p.Title, n.ID), nil
}

func handleUpdateTask(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("update_task: database is required")
	}

	var p struct {
		ID         string  `json:"id"`
		Title      *string `json:"title"`
		Domain     *string `json:"domain"`
		Context    *string `json:"context"`
		DueDate    *string `json:"due_date"`
		Recurring  *string `json:"recurring"`
		Priority   *string `json:"priority"`
		Status     *string `json:"status"`
		ProjectID  *string `json:"project_id"`
		ReminderAt *string `json:"reminder_at"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("update_task: bad params: %w", err)
	}
	if p.ID == "" {
		return "", fmt.Errorf("update_task: id required")
	}

	if p.Status != nil && *p.Status == "done" {
		if err := database.CompleteTask(p.ID); err != nil {
			return "", fmt.Errorf("update_task: %w", err)
		}
		return fmt.Sprintf("Task %s completed.", p.ID), nil
	}

	updates := map[string]any{}
	if p.Title != nil {
		updates["title"] = *p.Title
	}
	if p.Domain != nil {
		domainID := resolveDomain(database, *p.Domain)
		if domainID.Valid {
			updates["domain_id"] = domainID.Int64
		}
	}
	if p.Context != nil {
		updates["context"] = *p.Context
	}
	if p.DueDate != nil {
		updates["due_date"] = *p.DueDate
	}
	if p.Recurring != nil {
		updates["recurring"] = *p.Recurring
	}
	if p.Priority != nil {
		updates["priority"] = *p.Priority
	}
	if p.Status != nil {
		updates["status"] = *p.Status
	}
	if p.ProjectID != nil {
		updates["project_id"] = nullableUpdateString(*p.ProjectID)
	}
	if p.ReminderAt != nil {
		updates["reminder_at"] = nullableUpdateString(*p.ReminderAt)
	}

	if len(updates) == 0 {
		return "No fields to update.", nil
	}

	if err := database.UpdateTask(p.ID, updates); err != nil {
		return "", fmt.Errorf("update_task: %w", err)
	}
	return fmt.Sprintf("Task %s updated.", p.ID), nil
}

func handleListTasks(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("list_tasks: database is required")
	}

	var p struct {
		Domain     string `json:"domain"`
		ProjectID  string `json:"project_id"`
		Surface    string `json:"surface"`
		Status     string `json:"status"`
		Due        string `json:"due"`
		ReminderAt string `json:"reminder_at"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("list_tasks: bad params: %w", err)
	}

	var domainID *int
	if p.Domain != "" {
		resolved, err := database.ResolveDomainID(p.Domain)
		if err == nil && resolved != nil {
			domainID = resolved
		}
	}

	var projectID *string
	if strings.TrimSpace(p.ProjectID) != "" {
		projectID = &p.ProjectID
	}
	var surface *string
	if strings.TrimSpace(p.Surface) != "" {
		surface = &p.Surface
	}
	var reminderAt *string
	if strings.TrimSpace(p.ReminderAt) != "" {
		reminderAt = &p.ReminderAt
	}

	tasks, err := database.ListTasks(db.TaskFilters{
		DomainID:   domainID,
		ProjectID:  projectID,
		Surface:    surface,
		Status:     p.Status,
		Due:        p.Due,
		ReminderAt: reminderAt,
	})
	if err != nil {
		return "", fmt.Errorf("list_tasks: %w", err)
	}

	if len(tasks) == 0 {
		return "No tasks found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tasks (%d):\n", len(tasks)))
	for i, t := range tasks {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s (id=%s, priority=%s", i+1, t.Status, t.Title, t.ID, t.Priority))
		if t.DueDate.Valid {
			sb.WriteString(", due=" + t.DueDate.String)
		}
		if t.ProjectID.Valid {
			sb.WriteString(", project=" + t.ProjectID.String)
		}
		if t.ReminderAt.Valid {
			sb.WriteString(", reminder=" + t.ReminderAt.String)
		}
		sb.WriteString(")\n")
	}
	return sb.String(), nil
}

func handleQueryPeople(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("query_people: database is required")
	}

	var p struct {
		Mode         string `json:"mode"`
		Query        string `json:"query"`
		Surface      string `json:"surface"`
		Relationship string `json:"relationship"`
		Limit        int    `json:"limit"`
		PersonID     string `json:"person_id"`
		Summary      string `json:"summary"`
		Date         string `json:"date"`
		FollowUp     string `json:"follow_up"`
		FollowUpDate string `json:"follow_up_date"`
		Days         int    `json:"days"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("query_people: bad params: %w", err)
	}

	mode := strings.TrimSpace(p.Mode)
	if mode == "" {
		mode = "search"
	}
	switch mode {
	case "search":
		return queryPeopleSearch(database, p.Query, p.Surface, p.Relationship, p.Limit)
	case "log_interaction":
		return queryPeopleLogInteraction(database, p.PersonID, p.Summary, p.Date, p.FollowUp, p.FollowUpDate)
	case "birthdays":
		return queryPeopleBirthdays(database, p.Days)
	default:
		return "", fmt.Errorf("query_people: unsupported mode %q", mode)
	}
}

func queryPeopleSearch(database *db.DB, query, surface, relationship string, limit int) (string, error) {
	var surfaceFilter *string
	if strings.TrimSpace(surface) != "" {
		surface = strings.TrimSpace(surface)
		surfaceFilter = &surface
	}
	var relationshipFilter *string
	if strings.TrimSpace(relationship) != "" {
		relationship = strings.TrimSpace(relationship)
		relationshipFilter = &relationship
	}
	if limit <= 0 {
		limit = 10
	}

	people, err := database.ListPeople(surfaceFilter, relationshipFilter)
	if err != nil {
		return "", fmt.Errorf("query_people search: %w", err)
	}

	needle := strings.ToLower(strings.TrimSpace(query))
	matches := make([]db.Person, 0, len(people))
	for _, person := range people {
		if needle == "" || personMatchesQuery(person, needle) {
			matches = append(matches, person)
			if len(matches) >= limit {
				break
			}
		}
	}
	if len(matches) == 0 {
		return "No people found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("People (%d):\n", len(matches)))
	for i, person := range matches {
		sb.WriteString(fmt.Sprintf("%d. %s (id=%s", i+1, person.Name, person.ID))
		if person.Relationship.Valid {
			sb.WriteString(", relationship=" + person.Relationship.String)
		}
		if person.Surface != "" {
			sb.WriteString(", surface=" + person.Surface)
		}
		if person.LastInteractionAt.Valid {
			sb.WriteString(", last_interaction=" + person.LastInteractionAt.String)
		}
		sb.WriteString(")\n")
	}
	return sb.String(), nil
}

func personMatchesQuery(person db.Person, needle string) bool {
	haystacks := []string{
		person.Name,
		person.Surface,
		person.Relationship.String,
		person.Spouse.String,
		person.Notes.String,
	}
	for _, haystack := range haystacks {
		if strings.Contains(strings.ToLower(haystack), needle) {
			return true
		}
	}
	kids, err := person.KidList()
	if err == nil {
		for _, kid := range kids {
			if strings.Contains(strings.ToLower(kid.Name), needle) {
				return true
			}
		}
	}
	return false
}

func queryPeopleLogInteraction(database *db.DB, personID, summary, date, followUp, followUpDate string) (string, error) {
	personID = strings.TrimSpace(personID)
	summary = strings.TrimSpace(summary)
	if personID == "" {
		return "", fmt.Errorf("query_people log_interaction: person_id required")
	}
	if summary == "" {
		return "", fmt.Errorf("query_people log_interaction: summary required")
	}

	interaction := db.Interaction{
		ID:       uuid.New().String(),
		PersonID: personID,
		Summary:  summary,
	}
	if strings.TrimSpace(date) != "" {
		interaction.Date = strings.TrimSpace(date)
	}
	if strings.TrimSpace(followUp) != "" {
		interaction.FollowUp = sql.NullString{String: strings.TrimSpace(followUp), Valid: true}
	}
	if strings.TrimSpace(followUpDate) != "" {
		interaction.FollowUpDate = sql.NullString{String: strings.TrimSpace(followUpDate), Valid: true}
	}

	if err := database.InsertInteraction(interaction); err != nil {
		return "", fmt.Errorf("query_people log_interaction: %w", err)
	}
	return fmt.Sprintf("Interaction logged for person %s (id=%s)", personID, interaction.ID), nil
}

func queryPeopleBirthdays(database *db.DB, days int) (string, error) {
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}
	hits, err := database.UpcomingBirthdays(days)
	if err != nil {
		return "", fmt.Errorf("query_people birthdays: %w", err)
	}
	if len(hits) == 0 {
		return "No birthdays found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Birthdays (%d):\n", len(hits)))
	for i, hit := range hits {
		sb.WriteString(fmt.Sprintf("%d. %s (%s, person_id=%s, date=%s, days_until=%d, urgency=%s",
			i+1, hit.Name, hit.Kind, hit.PersonID, hit.Date, hit.DaysUntil, hit.Urgency))
		if hit.Relationship != "" {
			sb.WriteString(", relationship=" + hit.Relationship)
		}
		sb.WriteString(")\n")
	}
	return sb.String(), nil
}

func handleQueryFacts(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("query_facts: database is required")
	}

	var p struct {
		Entity string `json:"entity"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("query_facts: bad params: %w", err)
	}
	entity := strings.TrimSpace(p.Entity)
	if entity == "" {
		return "", fmt.Errorf("query_facts: entity required")
	}

	facts, err := database.ProbeFacts(entity)
	if err != nil {
		return "", fmt.Errorf("query_facts: %w", err)
	}
	if p.Limit > 0 && len(facts) > p.Limit {
		facts = facts[:p.Limit]
	}
	if len(facts) == 0 {
		return fmt.Sprintf("No facts found for %q.", entity), nil
	}
	return formatFacts(entity, facts), nil
}

func handleSaveFact(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("save_fact: database is required")
	}

	var p struct {
		Entity          string   `json:"entity"`
		Content         string   `json:"content"`
		Category        string   `json:"category"`
		Tags            []string `json:"tags"`
		RelatedEntities []string `json:"related_entities"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("save_fact: bad params: %w", err)
	}
	entity := strings.TrimSpace(p.Entity)
	content := strings.TrimSpace(p.Content)
	if entity == "" {
		return "", fmt.Errorf("save_fact: entity required")
	}
	if content == "" {
		return "", fmt.Errorf("save_fact: content required")
	}

	fact := db.Fact{
		ID:      uuid.New().String(),
		Entity:  entity,
		Content: content,
	}
	if strings.TrimSpace(p.Category) != "" {
		fact.Category = sql.NullString{String: strings.TrimSpace(p.Category), Valid: true}
	}
	if tags := joinToolTags(p.Tags); tags != "" {
		fact.Tags = sql.NullString{String: tags, Valid: true}
	}
	if err := database.InsertFact(fact, p.RelatedEntities); err != nil {
		return "", fmt.Errorf("save_fact: %w", err)
	}
	return fmt.Sprintf("Fact saved for %q (id=%s)", entity, fact.ID), nil
}

func formatFacts(entity string, facts []db.Fact) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Facts for %q (%d):\n", entity, len(facts)))
	for i, fact := range facts {
		sb.WriteString(fmt.Sprintf("%d. %s (id=%s, trust=%.2f", i+1, fact.Content, fact.ID, fact.Trust))
		if fact.Category.Valid {
			sb.WriteString(", category=" + fact.Category.String)
		}
		if fact.Tags.Valid {
			sb.WriteString(", tags=" + fact.Tags.String)
		}
		sb.WriteString(")\n")
	}
	return sb.String()
}

func nullableUpdateString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func joinToolTags(tags []string) string {
	clean := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		clean = append(clean, tag)
	}
	return strings.Join(clean, ",")
}

func summarizeToolText(text string, max int) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if max <= 0 || len(runes) <= max {
		return text
	}
	return strings.TrimSpace(string(runes[:max])) + "..."
}
