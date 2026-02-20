package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	"scaffold/db"
)

type ToolHandler func(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error)

func defaultToolRegistry() map[string]ToolHandler {
	return map[string]ToolHandler{
		"save_to_inbox":    handleSaveToInbox,
		"get_desk":         handleGetDesk,
		"search_memories":  handleSearchMemories,
		"update_desk_item": handleUpdateDeskItem,
		"get_inbox":        handleGetInbox,
		"add_to_notebook":  handleAddToNotebook,
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

	memoryID := uuid.New().String()
	mem := db.Memory{
		ID:         memoryID,
		Type:       typ,
		Content:    p.Content,
		Title:      p.Title,
		Importance: importance,
		Source:     "agent",
		Tags:       tags,
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

	memories, err := database.ListByImportance(200)
	if err != nil {
		return "", fmt.Errorf("search_memories: %w", err)
	}

	query := strings.ToLower(p.Query)
	var results []db.ScoredMemory
	for _, m := range memories {
		if requestedType != "" && !strings.EqualFold(strings.TrimSpace(m.Type), requestedType) {
			continue
		}
		if strings.Contains(strings.ToLower(m.Title), query) ||
			strings.Contains(strings.ToLower(m.Content), query) ||
			strings.Contains(strings.ToLower(m.Tags), query) {
			results = append(results, db.ScoredMemory{Memory: m})
			if len(results) >= topK {
				break
			}
		}
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

	ids := make([]string, 0, len(results))
	for _, result := range results {
		if id := strings.TrimSpace(result.ID); id != "" {
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

func handleAddToNotebook(ctx context.Context, database *db.DB, b *Brain, params json.RawMessage) (string, error) {
	return "Notebooks are not yet available. This feature is coming in a future update.", nil
}
