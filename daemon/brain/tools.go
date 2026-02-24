package brain

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"scaffold/db"
	googlecal "scaffold/google"
	"scaffold/sessionbus"
)

type ToolHandler func(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error)

func defaultToolRegistry() map[string]ToolHandler {
	return map[string]ToolHandler{
		"save_to_inbox":       handleSaveToInbox,
		"get_desk":            handleGetDesk,
		"search_memories":     handleSearchMemories,
		"update_desk_item":    handleUpdateDeskItem,
		"get_inbox":           handleGetInbox,
		"get_calendar_events": handleGetCalendarEvents,
		"send_to_session":     handleSendToSession,
		"list_sessions":       handleListSessions,
		"create_goal":         handleCreateGoal,
		"create_task":         handleCreateTask,
		"create_note":         handleCreateNote,
		"update_goal":         handleUpdateGoal,
		"update_task":         handleUpdateTask,
		"list_goals":          handleListGoals,
		"list_tasks":          handleListTasks,
		"dispatch_code_task":  handleDispatchCodeTask,
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

func handleSaveToInbox(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	var p struct {
		Title      string   `json:"title"`
		Content    string   `json:"content"`
		Type       string   `json:"type"`
		Importance float64  `json:"importance"`
		Tags       []string `json:"tags"`
		Domain     string   `json:"domain"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("save_to_inbox: bad params: %w", err)
	}
	if p.Title == "" || p.Content == "" {
		return "", fmt.Errorf("save_to_inbox: title and content required")
	}
	if database == nil {
		return "", fmt.Errorf("save_to_inbox: database is required")
	}

	var triage *TriageResult
	if b != nil {
		var triageErr error
		triage, triageErr = b.Triage(ctx, p.Content)
		if triageErr != nil {
			log.Printf("save_to_inbox triage error: %v", triageErr)
		}
	}

	typ := "Observation"
	if triage != nil && triage.Type != "" {
		typ = triage.Type
	}
	if p.Type != "" {
		typ = p.Type
	}

	importance := p.Importance
	if importance == 0 && triage != nil {
		importance = triage.Importance
	}
	if importance == 0 {
		importance = 0.5
	}

	tags := strings.Join(p.Tags, ",")
	if tags == "" && triage != nil {
		tags = strings.Join(triage.Tags, ",")
	}

	var domainID sql.NullInt64
	domainName := p.Domain
	if domainName == "" && triage != nil {
		domainName = triage.Domain
	}
	domainName = strings.TrimSpace(domainName)
	if domainName == "" {
		domainName = "Personal Development"
	}
	resolved, resolveErr := database.ResolveDomainID(domainName)
	if resolveErr != nil {
		log.Printf("save_to_inbox: resolve domain %q: %v", domainName, resolveErr)
	} else if resolved != nil {
		domainID = sql.NullInt64{Int64: int64(*resolved), Valid: true}
	} else {
		log.Printf("save_to_inbox: unknown domain %q, leaving undomained", domainName)
	}

	memoryID := uuid.New().String()
	mem := db.Memory{
		ID:         memoryID,
		Type:       typ,
		Content:    p.Content,
		Title:      p.Title,
		Importance: importance,
		Source:     "agent",
		Tags:       tags,
		DomainID:   domainID,
	}

	action := "reference"
	if triage != nil && triage.Action != "" {
		action = triage.Action
	}

	captureID, err := database.InsertProcessedCaptureWithMemory(p.Content, "agent", action, mem, "agent_tool")
	if err != nil {
		return "", fmt.Errorf("save_to_inbox: persist failed: %w", err)
	}

	return fmt.Sprintf("Saved to inbox: %q (type=%s, capture=%s, memory=%s)", p.Title, typ, captureID, memoryID), nil
}

func handleGetDesk(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("get_desk: database is required")
	}

	items, err := database.TodaysDesk()
	if err != nil {
		return "", fmt.Errorf("get_desk: %w", err)
	}
	if len(items) == 0 {
		return "Desk is empty today.", nil
	}

	var sb strings.Builder
	sb.WriteString("Today's desk:\n")
	for i, item := range items {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s (id=%s)", i+1, item.Status, item.Title, item.ID))
		if item.MicroSteps.Valid && item.MicroSteps.String != "" {
			sb.WriteString("\n   Steps: " + item.MicroSteps.String)
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
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

	if b != nil && b.embedder != nil && b.embedder.Available(ctx) {
		vec, err := b.embedder.Embed(ctx, p.Query)
		if err == nil {
			embeddingModel := strings.TrimSpace(b.embedder.ModelName())
			if embeddingModel != "" {
				results, err := database.SearchHybrid(p.Query, vec, embeddingModel, topK*3)
				if err == nil && len(results) > 0 {
					filtered := filterScoredMemoriesByType(results, requestedType, topK)
					if len(filtered) > 0 {
						markSearchAccess(database, filtered)
						return formatSearchResults(p.Query, filtered), nil
					}
				}
			}
		}
	}

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

func handleUpdateDeskItem(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("update_desk_item: database is required")
	}

	var p struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("update_desk_item: bad params: %w", err)
	}
	if p.ID == "" || p.Status == "" {
		return "", fmt.Errorf("update_desk_item: id and status required")
	}

	switch p.Status {
	case "deferred":
		if err := database.DeferDeskItem(p.ID); err != nil {
			return "", fmt.Errorf("update_desk_item: defer failed: %w", err)
		}
		return fmt.Sprintf("Desk item %s deferred to tomorrow.", p.ID), nil
	case "done", "active":
		if err := database.UpdateDeskStatus(p.ID, p.Status); err != nil {
			return "", fmt.Errorf("update_desk_item: update failed: %w", err)
		}
		return fmt.Sprintf("Desk item %s marked %s.", p.ID, p.Status), nil
	default:
		return "", fmt.Errorf("update_desk_item: invalid status %q (must be done, deferred, or active)", p.Status)
	}
}

func handleGetInbox(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("get_inbox: database is required")
	}

	captures, err := database.ListRecent(10)
	if err != nil {
		return "", fmt.Errorf("get_inbox: %w", err)
	}

	filtered := make([]db.Capture, 0, len(captures))
	for _, capture := range captures {
		if strings.EqualFold(strings.TrimSpace(capture.Source), "user:archive") {
			continue
		}
		filtered = append(filtered, capture)
	}

	if len(filtered) == 0 {
		return "Inbox is empty.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Inbox (%d items):\n", len(filtered)))
	for i, c := range filtered {
		processed := "pending"
		if c.Processed == 1 {
			processed = "processed"
		}
		action := ""
		if c.TriageAction.Valid {
			action = " [" + c.TriageAction.String + "]"
		}
		raw := c.Raw
		if len(raw) > 80 {
			raw = raw[:80] + "..."
		}
		sb.WriteString(fmt.Sprintf("%d. [%s%s] %s\n", i+1, processed, action, raw))
	}
	return sb.String(), nil
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

func handleListSessions(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if b == nil || b.sessionBus == nil {
		return "", fmt.Errorf("list_sessions: session bus is not configured")
	}

	sessions := b.sessionBus.List(ctx)
	if len(sessions) == 0 {
		return "No active sessions in session bus.", nil
	}

	var sb strings.Builder
	sb.WriteString("Active sessions:\n")
	for _, s := range sessions {
		sb.WriteString("- ")
		sb.WriteString(s.SessionID)
		sb.WriteString(" [")
		sb.WriteString(s.Provider)
		sb.WriteString("]")
		if strings.TrimSpace(s.Name) != "" {
			sb.WriteString(" ")
			sb.WriteString(s.Name)
		}
		sb.WriteString(fmt.Sprintf(" queue=%d last_seen=%s\n", s.QueueDepth, s.LastSeenAt.Format(time.RFC3339)))
	}
	return sb.String(), nil
}

func handleSendToSession(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if b == nil || b.sessionBus == nil {
		return "", fmt.Errorf("send_to_session: session bus is not configured")
	}

	var p struct {
		ToSessionID   string `json:"to_session_id"`
		Message       string `json:"message"`
		Mode          string `json:"mode"`
		FromSessionID string `json:"from_session_id"`
		FromProvider  string `json:"from_provider"`
		FromName      string `json:"from_name"`
		WaitSeconds   int    `json:"wait_seconds"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("send_to_session: bad params: %w", err)
	}

	p.ToSessionID = strings.TrimSpace(p.ToSessionID)
	p.Message = strings.TrimSpace(p.Message)
	if p.ToSessionID == "" {
		return "", fmt.Errorf("send_to_session: to_session_id is required")
	}
	if p.Message == "" {
		return "", fmt.Errorf("send_to_session: message is required")
	}

	fromID := strings.TrimSpace(p.FromSessionID)
	if fromID == "" {
		fromID = "scaffold-agent"
	}
	fromProvider := strings.TrimSpace(p.FromProvider)
	if fromProvider == "" {
		fromProvider = "scaffold"
	}
	fromName := strings.TrimSpace(p.FromName)
	if fromName == "" {
		fromName = "Scaffold Agent"
	}

	if _, err := b.sessionBus.Register(ctx, sessionbus.RegisterRequest{
		SessionID: fromID,
		Provider:  fromProvider,
		Name:      fromName,
	}); err != nil {
		return "", fmt.Errorf("send_to_session: register sender: %w", err)
	}

	delivered, err := b.sessionBus.Send(ctx, sessionbus.SendRequest{
		FromSessionID: fromID,
		ToSessionID:   p.ToSessionID,
		Mode:          p.Mode,
		Message:       p.Message,
	})
	if err != nil {
		return "", fmt.Errorf("send_to_session: %w", err)
	}

	if p.WaitSeconds <= 0 {
		return fmt.Sprintf("Message sent to %s (id=%s mode=%s)", p.ToSessionID, delivered.ID, delivered.Mode), nil
	}

	if p.WaitSeconds > 120 {
		p.WaitSeconds = 120
	}

	incoming, err := b.sessionBus.Poll(ctx, fromID, 1, time.Duration(p.WaitSeconds)*time.Second)
	if err != nil {
		return "", fmt.Errorf("send_to_session: waiting for reply: %w", err)
	}
	if len(incoming) == 0 {
		return fmt.Sprintf("Message sent to %s (id=%s). No reply within %ds.", p.ToSessionID, delivered.ID, p.WaitSeconds), nil
	}

	reply := incoming[0]
	return fmt.Sprintf("Message sent to %s (id=%s).\nReply from %s:\n%s", p.ToSessionID, delivered.ID, reply.FromSessionID, reply.Message), nil
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

func handleCreateGoal(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("create_goal: database is required")
	}

	var p struct {
		Title        string  `json:"title"`
		Domain       string  `json:"domain"`
		Context      string  `json:"context"`
		DueDate      string  `json:"due_date"`
		Type         string  `json:"type"`
		TargetValue  float64 `json:"target_value"`
		CurrentValue float64 `json:"current_value"`
		HabitType    string  `json:"habit_type"`
		ScheduleDays string  `json:"schedule_days"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("create_goal: bad params: %w", err)
	}
	if p.Title == "" {
		return "", fmt.Errorf("create_goal: title required")
	}

	g := db.Goal{
		ID:       uuid.New().String(),
		Title:    p.Title,
		DomainID: resolveDomain(database, p.Domain),
	}
	if p.Context != "" {
		g.Context = sql.NullString{String: p.Context, Valid: true}
	}
	if p.DueDate != "" {
		g.DueDate = sql.NullString{String: p.DueDate, Valid: true}
	}
	if p.Type != "" {
		g.Type = p.Type
	}
	if p.TargetValue != 0 {
		g.TargetValue = sql.NullFloat64{Float64: p.TargetValue, Valid: true}
	}
	if p.CurrentValue != 0 {
		g.CurrentValue = sql.NullFloat64{Float64: p.CurrentValue, Valid: true}
	}
	if p.HabitType != "" {
		g.HabitType = sql.NullString{String: p.HabitType, Valid: true}
	}
	if p.ScheduleDays != "" {
		g.ScheduleDays = sql.NullString{String: p.ScheduleDays, Valid: true}
	}

	if err := database.InsertGoal(g); err != nil {
		return "", fmt.Errorf("create_goal: %w", err)
	}
	return fmt.Sprintf("Goal created: %q (id=%s)", p.Title, g.ID), nil
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

func handleUpdateGoal(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("update_goal: database is required")
	}

	var p struct {
		ID           string   `json:"id"`
		Title        *string  `json:"title"`
		Domain       *string  `json:"domain"`
		Context      *string  `json:"context"`
		DueDate      *string  `json:"due_date"`
		Type         *string  `json:"type"`
		TargetValue  *float64 `json:"target_value"`
		CurrentValue *float64 `json:"current_value"`
		HabitType    *string  `json:"habit_type"`
		ScheduleDays *string  `json:"schedule_days"`
		Status       *string  `json:"status"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("update_goal: bad params: %w", err)
	}
	if p.ID == "" {
		return "", fmt.Errorf("update_goal: id required")
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
	if p.Type != nil {
		updates["type"] = *p.Type
	}
	if p.TargetValue != nil {
		updates["target_value"] = *p.TargetValue
	}
	if p.CurrentValue != nil {
		updates["current_value"] = *p.CurrentValue
	}
	if p.HabitType != nil {
		updates["habit_type"] = *p.HabitType
	}
	if p.ScheduleDays != nil {
		updates["schedule_days"] = *p.ScheduleDays
	}
	if p.Status != nil {
		updates["status"] = *p.Status
		if *p.Status == "done" {
			updates["completed_at"] = time.Now().UTC().Format(time.RFC3339)
		}
	}

	if len(updates) == 0 {
		return "No fields to update.", nil
	}

	if err := database.UpdateGoal(p.ID, updates); err != nil {
		return "", fmt.Errorf("update_goal: %w", err)
	}
	return fmt.Sprintf("Goal %s updated.", p.ID), nil
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

func handleListGoals(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if database == nil {
		return "", fmt.Errorf("list_goals: database is required")
	}

	var p struct {
		Domain string `json:"domain"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("list_goals: bad params: %w", err)
	}

	var domainID *int
	if p.Domain != "" {
		resolved, err := database.ResolveDomainID(p.Domain)
		if err == nil && resolved != nil {
			domainID = resolved
		}
	}

	goals, err := database.GoalsWithProgress(domainID)
	if err != nil {
		return "", fmt.Errorf("list_goals: %w", err)
	}

	if len(goals) == 0 {
		return "No active goals found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Goals (%d):\n", len(goals)))
	for i, g := range goals {
		sb.WriteString(fmt.Sprintf("%d. %s (id=%s, type=%s", i+1, g.Title, g.ID, g.Type))
		if g.DueDate.Valid {
			sb.WriteString(", due=" + g.DueDate.String)
		}
		if g.TotalTasks > 0 {
			sb.WriteString(fmt.Sprintf(", tasks=%d/%d %.0f%%", g.CompletedTasks, g.TotalTasks, g.Progress*100))
		}
		sb.WriteString(")\n")
	}
	return sb.String(), nil
}

func handleDispatchCodeTask(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	if b == nil || b.sessionBus == nil {
		return "", fmt.Errorf("dispatch_code_task: session bus not configured")
	}

	var p struct {
		Task  string `json:"task"`
		Chain string `json:"chain"`
		CWD   string `json:"cwd"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("dispatch_code_task: bad params: %w", err)
	}
	if strings.TrimSpace(p.Task) == "" {
		return "", fmt.Errorf("dispatch_code_task: task required")
	}
	if p.Chain == "" {
		p.Chain = "implement"
	}
	if p.CWD == "" {
		p.CWD = "/home/mikekey/Builds/scaffold"
	}

	msg := map[string]any{
		"type":     "code_task",
		"task":     p.Task,
		"chain":    p.Chain,
		"cwd":      p.CWD,
		"reply_to": "scaffold-agent",
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("dispatch_code_task: marshal: %w", err)
	}

	if _, err := b.sessionBus.Send(ctx, sessionbus.SendRequest{
		FromSessionID: "scaffold-agent",
		ToSessionID:   "scaffold-coder",
		Message:       string(data),
	}); err != nil {
		return "", fmt.Errorf("dispatch_code_task: %w", err)
	}

	return fmt.Sprintf("Code task dispatched (chain=%s). Check #/coder in the web UI for live progress. Results delivered on completion.", p.Chain), nil
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
