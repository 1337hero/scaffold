package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

type DecayReport struct {
	Updated int
}

type ConsolidationReport struct {
	GroupsFound        int
	DuplicatesFound    int
	EdgesCreated       int
	MemoriesSuppressed int
	SkippedReferenced  int
	PairsEvaluated     int
	LLMDecisions       int
}

type ConsolidationCandidate struct {
	MemoryA    Memory
	MemoryB    Memory
	Similarity float64
}

func (db *DB) DecayMemories(factor float64, exemptTypes []string, importanceFloor float64, olderThanDays int) (DecayReport, error) {
	if factor <= 0 || factor >= 1 {
		return DecayReport{}, fmt.Errorf("factor must be between 0 and 1 (exclusive)")
	}
	if olderThanDays <= 0 {
		olderThanDays = 30
	}
	if importanceFloor < 0 {
		importanceFloor = 0
	}
	if importanceFloor > 1 {
		importanceFloor = 1
	}

	placeholders := ""
	args := []any{factor, factor, factor, now(), time.Now().UTC().AddDate(0, 0, -olderThanDays).Format(time.RFC3339), importanceFloor}

	cleanExempt := make([]string, 0, len(exemptTypes))
	for _, typ := range exemptTypes {
		typ = strings.TrimSpace(typ)
		if typ != "" {
			cleanExempt = append(cleanExempt, typ)
		}
	}
	if len(cleanExempt) > 0 {
		parts := make([]string, 0, len(cleanExempt))
		for _, typ := range cleanExempt {
			parts = append(parts, "?")
			args = append(args, typ)
		}
		placeholders = " AND type NOT IN (" + strings.Join(parts, ",") + ")"
	}

	query := `
		UPDATE memories
		SET importance = CASE
			WHEN importance * ? < 0 THEN 0
			WHEN importance * ? > 1 THEN 1
			ELSE importance * ?
		END,
		    updated_at = ?
		WHERE suppressed_at IS NULL
		  AND (accessed_at IS NULL OR accessed_at <= ?)
		  AND importance > ?` + placeholders

	result, err := db.conn.Exec(query, args...)
	if err != nil {
		return DecayReport{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return DecayReport{}, err
	}
	return DecayReport{Updated: int(rows)}, nil
}

func (db *DB) ConsolidateMemories() (ConsolidationReport, error) {
	rows, err := db.conn.Query(`
		SELECT id, type, title, content, importance, created_at
		FROM memories
		WHERE suppressed_at IS NULL
		ORDER BY created_at ASC`,
	)
	if err != nil {
		return ConsolidationReport{}, err
	}
	defer rows.Close()

	type candidate struct {
		ID         string
		Type       string
		Title      string
		Content    string
		Importance float64
		CreatedAt  string
	}

	groups := make(map[string][]candidate)
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.ID, &c.Type, &c.Title, &c.Content, &c.Importance, &c.CreatedAt); err != nil {
			return ConsolidationReport{}, err
		}

		key := consolidationKey(c.Type, c.Title, c.Content)
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], c)
	}
	if err := rows.Err(); err != nil {
		return ConsolidationReport{}, err
	}

	report := ConsolidationReport{}
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		report.GroupsFound++
		report.DuplicatesFound += len(group) - 1

		sort.SliceStable(group, func(i, j int) bool {
			if group[i].Importance == group[j].Importance {
				return group[i].CreatedAt < group[j].CreatedAt
			}
			return group[i].Importance > group[j].Importance
		})
		canonical := group[0]

		for _, dup := range group[1:] {
			if canonical.ID == dup.ID {
				continue
			}

			edgeCreated, err := db.ensureUndirectedEdge(dup.ID, canonical.ID, "RelatedTo", 0.9)
			if err != nil {
				return report, err
			}
			if edgeCreated {
				report.EdgesCreated++
			}

			refs, err := db.countPruneReferences(dup.ID)
			if err != nil {
				return report, err
			}
			if refs > 0 || strings.EqualFold(dup.Type, "Identity") {
				report.SkippedReferenced++
				continue
			}

			if err := db.SuppressMemory(dup.ID); err != nil {
				if err == sql.ErrNoRows {
					continue
				}
				return report, err
			}
			report.MemoriesSuppressed++
		}
	}

	return report, nil
}

func (db *DB) ensureUndirectedEdge(idA, idB, relation string, weight float64) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*)
		 FROM edges
		 WHERE relation = ?
		   AND ((from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?))`,
		relation, idA, idB, idB, idA,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if err := db.InsertEdge(Edge{
		FromID:   idA,
		ToID:     idB,
		Relation: relation,
		Weight:   weight,
	}); err != nil {
		return false, err
	}
	return true, nil
}

func consolidationKey(memoryType, title, content string) string {
	memoryType = strings.ToLower(strings.TrimSpace(memoryType))
	title = normalizeConsolidationText(title)
	content = normalizeConsolidationText(content)
	if memoryType == "" && title == "" && content == "" {
		return ""
	}
	return memoryType + "|" + title + "|" + content
}

func normalizeConsolidationText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func (db *DB) EnsureUndirectedEdge(idA, idB, relation string, weight float64) (bool, error) {
	return db.ensureUndirectedEdge(idA, idB, relation, weight)
}
