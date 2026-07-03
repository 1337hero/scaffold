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
		"search_memories":        handleSearchMemories,
		"get_calendar_events":    handleGetCalendarEvents,
		"create_calendar_event":  handleCreateCalendarEvent,
		"update_calendar_event":  handleUpdateCalendarEvent,
		"create_task":            handleCreateTask,
		"create_note":            handleCreateNote,
		"update_task":            handleUpdateTask,
		"list_tasks":             handleListTasks,
		"search_email":           handleSearchEmail,
		"get_email":              handleGetEmail,
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

	ftsResults, err := database.SearchFTS(p.Query, topK*3)
	if err == nil && len(ftsResults) > 0 {
		filtered := filterScoredMemoriesByType(ftsResults, requestedType, topK)
		if len(filtered) > 0 {
			markSearchAccess(database, filtered)
			return formatSearchResults(p.Query, filtered), nil
		}
	}

	results, err := database.SearchMemoriesLike(p.Query, requestedType, topK)
	if err != nil {
		return "", fmt.Errorf("search_memories: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No memories found matching %q.", p.Query), nil
	}
	markSearchAccess(database, results)
	return formatSearchResults(p.Query, results), nil
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
	sb.WriteString(fmt.Sprintf("Found %d memories matching %q:\n", len(results), query))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. [%s/%.1f] %s\n", i+1, r.Type, r.Importance, r.Title))
		if r.Content != r.Title {
			content := r.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %s\n", content))
		}
		if r.FusedScore > 0 {
			sb.WriteString(fmt.Sprintf("   score: %.3f (fts=%.3f vec=%.3f)\n", r.FusedScore, r.FTSScore, r.VectorScore))
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
		Title     string `json:"title"`
		Domain    string `json:"domain"`
		GoalID    string `json:"goal_id"`
		Context   string `json:"context"`
		DueDate   string `json:"due_date"`
		Recurring string `json:"recurring"`
		Priority  string `json:"priority"`
		Source    string `json:"source"`
		SourceRef string `json:"source_ref"`
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
	if p.GoalID != "" {
		t.GoalID = sql.NullString{String: p.GoalID, Valid: true}
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
	if p.Source != "" {
		t.Source = sql.NullString{String: p.Source, Valid: true}
	}
	if p.SourceRef != "" {
		t.SourceRef = sql.NullString{String: p.SourceRef, Valid: true}
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
		Title   string   `json:"title"`
		Domain  string   `json:"domain"`
		GoalID  string   `json:"goal_id"`
		TaskID  string   `json:"task_id"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
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
	if p.GoalID != "" {
		n.GoalID = sql.NullString{String: p.GoalID, Valid: true}
	}
	if p.TaskID != "" {
		n.TaskID = sql.NullString{String: p.TaskID, Valid: true}
	}
	if p.Content != "" {
		n.Content = sql.NullString{String: p.Content, Valid: true}
	}
	if len(p.Tags) > 0 {
		n.Tags = sql.NullString{String: strings.Join(p.Tags, ","), Valid: true}
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
		ID        string  `json:"id"`
		Title     *string `json:"title"`
		Domain    *string `json:"domain"`
		GoalID    *string `json:"goal_id"`
		Context   *string `json:"context"`
		DueDate   *string `json:"due_date"`
		Recurring *string `json:"recurring"`
		Priority  *string `json:"priority"`
		Status    *string `json:"status"`
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
	if p.GoalID != nil {
		updates["goal_id"] = *p.GoalID
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
		Domain string `json:"domain"`
		GoalID string `json:"goal_id"`
		Status string `json:"status"`
		Due    string `json:"due"`
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

	var goalID *string
	if p.GoalID != "" {
		goalID = &p.GoalID
	}

	tasks, err := database.ListTasks(domainID, goalID, p.Status, p.Due)
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
		if t.GoalID.Valid {
			sb.WriteString(", goal=" + t.GoalID.String)
		}
		sb.WriteString(")\n")
	}
	return sb.String(), nil
}

func handleCreateCalendarEvent(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if b == nil || b.calendarClient == nil {
		return "Google Calendar is not configured. Run: scaffold-daemon auth google", nil
	}

	var p struct {
		Title       string `json:"title"`
		Start       string `json:"start"`
		End         string `json:"end"`
		Location    string `json:"location"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("create_calendar_event: bad params: %w", err)
	}
	if p.Title == "" || p.Start == "" || p.End == "" {
		return "", fmt.Errorf("create_calendar_event: title, start, and end are required")
	}

	event := googlecal.Event{
		Title:       p.Title,
		Start:       p.Start,
		End:         p.End,
		Location:    p.Location,
		Description: p.Description,
	}
	created, err := b.calendarClient.CreateEvent(ctx, b.calendarClient.CalendarID, event)
	if err != nil {
		return "", fmt.Errorf("create_calendar_event: %w", err)
	}
	return fmt.Sprintf("Event created: %q on %s (id=%s)", created.Title, created.Start, created.ID), nil
}

func handleUpdateCalendarEvent(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if b == nil || b.calendarClient == nil {
		return "Google Calendar is not configured. Run: scaffold-daemon auth google", nil
	}

	var p struct {
		EventID     string `json:"event_id"`
		Title       string `json:"title"`
		Start       string `json:"start"`
		End         string `json:"end"`
		Location    string `json:"location"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("update_calendar_event: bad params: %w", err)
	}
	if p.EventID == "" {
		return "", fmt.Errorf("update_calendar_event: event_id is required")
	}

	event := googlecal.Event{
		Title:       p.Title,
		Start:       p.Start,
		End:         p.End,
		Location:    p.Location,
		Description: p.Description,
	}
	updated, err := b.calendarClient.UpdateEvent(ctx, b.calendarClient.CalendarID, p.EventID, event)
	if err != nil {
		return "", fmt.Errorf("update_calendar_event: %w", err)
	}
	return fmt.Sprintf("Event updated: %q (id=%s)", updated.Title, updated.ID), nil
}

func handleSearchEmail(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	return "Gmail search is not available in v2. Email triage has been removed.", nil
}

func handleGetEmail(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	return "Gmail access is not available in v2. Email triage has been removed.", nil
}

