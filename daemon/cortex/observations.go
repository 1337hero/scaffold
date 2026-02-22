package cortex

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	observationsMinEntries   = 10
	observationsMaxNew       = 5
	observationsLookbackDays = 7
)

type observationPattern struct {
	Pattern           string   `json:"pattern"`
	EvidenceMemoryIDs []string `json:"evidence_memory_ids"`
	Importance        float64  `json:"importance"`
}

func (c *Cortex) runObservations(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}

	since := time.Now().UTC().AddDate(0, 0, -observationsLookbackDays).Format(time.RFC3339)
	entries, err := c.db.ListConversationSince(since, 100)
	if err != nil {
		return fmt.Errorf("list conversation: %w", err)
	}

	if len(entries) < observationsMinEntries {
		log.Printf("cortex: observations skipped (only %d entries, need %d)", len(entries), observationsMinEntries)
		return nil
	}

	recentMemories, err := c.db.ListRecentMemories(50)
	if err != nil {
		return fmt.Errorf("list recent memories: %w", err)
	}

	existingObs, err := c.db.ListByType("Observation", 50)
	if err != nil {
		return fmt.Errorf("list observations: %w", err)
	}
	existingTitles := make(map[string]bool, len(existingObs))
	for _, o := range existingObs {
		existingTitles[strings.ToLower(strings.TrimSpace(o.Title))] = true
	}

	var sb strings.Builder
	sb.WriteString("Recent conversations (last 7 days):\n")
	for _, e := range entries {
		ts := e.CreatedAt
		if len(ts) >= 10 {
			ts = ts[:10]
		}
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", ts, e.Role, truncateWords(e.Content, 30)))
	}
	sb.WriteString("\nRecent memories:\n")
	for _, m := range recentMemories {
		sb.WriteString(fmt.Sprintf("[id=%s] [%s] %s: %s\n", m.ID, m.Type, m.Title, truncateWords(m.Content, 20)))
	}

	existingList := make([]string, 0, len(existingObs))
	for _, o := range existingObs {
		existingList = append(existingList, o.Title)
	}

	systemPrompt := `You are an observation pattern detector for a personal AI assistant.
Identify recurring patterns, repeated topics, or behavioral trends from the provided data.
Only report patterns that appear 3 or more times. Avoid duplicating existing observations.
Respond with a JSON array only — no prose, no markdown fences.`

	userPrompt := fmt.Sprintf(`%s

Existing observation memories (do not duplicate):
%s

Return JSON array of new patterns found (max 5):
[{"pattern": "description of the recurring pattern", "evidence_memory_ids": ["id1", "id2"], "importance": 0.3}]

Use only memory IDs that appear in the "Recent memories" section above for evidence_memory_ids. Do not invent IDs.

If no new patterns found, return: []`,
		sb.String(),
		strings.Join(existingList, "\n"),
	)

	raw, err := c.observationsClient().CompletionJSON(ctx, c.observationsModel(), systemPrompt, userPrompt, 512)
	if err != nil {
		return fmt.Errorf("observations LLM call: %w", err)
	}

	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) > 2 {
			raw = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var patterns []observationPattern
	if err := json.Unmarshal([]byte(raw), &patterns); err != nil {
		log.Printf("cortex: observations parse error: %v (raw: %s)", err, raw)
		return nil
	}

	created := 0
	for _, pattern := range patterns {
		if created >= observationsMaxNew {
			break
		}
		if strings.TrimSpace(pattern.Pattern) == "" {
			continue
		}
		if existingTitles[strings.ToLower(strings.TrimSpace(pattern.Pattern))] {
			continue
		}

		importance := pattern.Importance
		if importance <= 0 || importance > 1 {
			importance = 0.3
		}

		memID, err := c.db.InsertObservation(pattern.Pattern, importance, pattern.EvidenceMemoryIDs)
		if err != nil {
			log.Printf("cortex: insert observation: %v", err)
			continue
		}

		created++
		log.Printf("cortex: observation created: %q (importance=%.2f, evidence=%d)", pattern.Pattern, importance, len(pattern.EvidenceMemoryIDs))
		_ = memID
	}

	log.Printf("cortex: observations created=%d", created)
	return nil
}
