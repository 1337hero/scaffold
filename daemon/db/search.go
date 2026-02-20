package db

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ftsWeight    = 0.4
	vectorWeight = 0.6
)

func (db *DB) SearchFTS(query string, topK int) ([]ScoredMemory, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query must not be empty")
	}
	escaped := escapeFTSQuery(query)
	rows, err := db.conn.Query(
		`SELECT m.id, m.type, m.content, m.title, m.importance, m.source, m.tags,
		        m.created_at, m.updated_at, m.accessed_at, m.access_count, m.archived, m.suppressed_at,
		        -fts.rank as fts_score
		 FROM memories_fts fts
		 JOIN memories m ON m.id = fts.memory_id
		 WHERE memories_fts MATCH ? AND m.suppressed_at IS NULL
		 ORDER BY fts.rank
		 LIMIT ?`,
		escaped, topK,
	)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var results []ScoredMemory
	for rows.Next() {
		var sm ScoredMemory
		if err := rows.Scan(
			&sm.ID, &sm.Type, &sm.Content, &sm.Title, &sm.Importance, &sm.Source, &sm.Tags,
			&sm.CreatedAt, &sm.UpdatedAt, &sm.AccessedAt, &sm.AccessCount, &sm.Archived, &sm.SuppressedAt,
			&sm.FTSScore,
		); err != nil {
			return nil, err
		}
		results = append(results, sm)
	}
	return results, rows.Err()
}

func (db *DB) SearchHybrid(query string, embedding []float32, topK int) ([]ScoredMemory, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query must not be empty")
	}

	ftsResults, ftsErr := db.SearchFTS(query, topK*3)

	var vecResults []ScoredMemory
	var vecErr error
	if len(embedding) > 0 {
		vecResults, vecErr = db.NearestNeighbors(embedding, topK*3, nil)
	}

	if ftsErr != nil && (len(embedding) == 0 || vecErr != nil) {
		if ftsErr != nil {
			return nil, ftsErr
		}
		return nil, vecErr
	}

	if len(vecResults) == 0 {
		if len(ftsResults) > topK {
			ftsResults = ftsResults[:topK]
		}
		return ftsResults, nil
	}

	if len(ftsResults) == 0 {
		if len(vecResults) > topK {
			vecResults = vecResults[:topK]
		}
		return vecResults, nil
	}

	maxFTS := 0.0
	for _, r := range ftsResults {
		if r.FTSScore > maxFTS {
			maxFTS = r.FTSScore
		}
	}

	type combined struct {
		mem      Memory
		ftsScore float64
		vecScore float64
	}
	byID := make(map[string]*combined)

	for _, r := range ftsResults {
		normScore := r.FTSScore
		if maxFTS > 0 {
			normScore = r.FTSScore / maxFTS
		}
		byID[r.ID] = &combined{mem: r.Memory, ftsScore: normScore}
	}

	for _, r := range vecResults {
		if c, ok := byID[r.ID]; ok {
			c.vecScore = r.VectorScore
		} else {
			byID[r.ID] = &combined{mem: r.Memory, vecScore: r.VectorScore}
		}
	}

	results := make([]ScoredMemory, 0, len(byID))
	for _, c := range byID {
		fused := ftsWeight*c.ftsScore + vectorWeight*c.vecScore
		results = append(results, ScoredMemory{
			Memory:      c.mem,
			FTSScore:    c.ftsScore,
			VectorScore: c.vecScore,
			FusedScore:  fused,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].FusedScore > results[j].FusedScore
	})

	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func escapeFTSQuery(query string) string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ReplaceAll(f, `"`, "")
		if f != "" {
			quoted = append(quoted, `"`+f+`"`)
		}
	}
	return strings.Join(quoted, " ")
}
