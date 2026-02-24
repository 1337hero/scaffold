package db

import (
	"database/sql"
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
		        m.created_at, m.updated_at, m.accessed_at, m.access_count, m.archived, m.suppressed_at, m.domain_id,
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
		var tags, accessedAt sql.NullString
		if err := rows.Scan(
			&sm.ID, &sm.Type, &sm.Content, &sm.Title, &sm.Importance, &sm.Source, &tags,
			&sm.CreatedAt, &sm.UpdatedAt, &accessedAt, &sm.AccessCount, &sm.Archived, &sm.SuppressedAt, &sm.DomainID,
			&sm.FTSScore,
		); err != nil {
			return nil, err
		}
		sm.Tags = tags.String
		sm.AccessedAt = accessedAt.String
		results = append(results, sm)
	}
	return results, rows.Err()
}

func (db *DB) SearchHybrid(query string, embedding []float32, embeddingModel string, topK int) ([]ScoredMemory, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query must not be empty")
	}
	embeddingModel = strings.TrimSpace(embeddingModel)

	ftsResults, ftsErr := db.SearchFTS(query, topK*3)

	vecEnabled := len(embedding) > 0 && embeddingModel != ""
	var vecResults []ScoredMemory
	var vecErr error
	if vecEnabled {
		vecResults, vecErr = db.NearestNeighbors(embedding, topK*3, nil, embeddingModel)
	}

	if ftsErr != nil && (!vecEnabled || vecErr != nil) {
		if ftsErr != nil {
			return nil, ftsErr
		}
		return nil, vecErr
	}
	if ftsErr != nil && len(vecResults) == 0 {
		return nil, ftsErr
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

func (db *DB) SearchMemoriesLike(query string, memoryType string, limit int) ([]ScoredMemory, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query must not be empty")
	}
	if limit <= 0 {
		limit = 10
	}

	like := "%" + strings.ToLower(query) + "%"
	args := []any{like, like, like}
	sqlText := `SELECT id, type, content, title, importance, source, tags,
	                created_at, updated_at, accessed_at, access_count, archived, suppressed_at, domain_id
	            FROM memories
	            WHERE suppressed_at IS NULL
	              AND (LOWER(title) LIKE ? OR LOWER(content) LIKE ? OR LOWER(tags) LIKE ?)`
	if mt := strings.TrimSpace(memoryType); mt != "" {
		sqlText += ` AND type = ? COLLATE NOCASE`
		args = append(args, mt)
	}
	sqlText += ` ORDER BY importance DESC, created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.conn.Query(sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("memory like search: %w", err)
	}
	defer rows.Close()

	results := make([]ScoredMemory, 0, limit)
	for rows.Next() {
		var sm ScoredMemory
		var tags, accessedAt sql.NullString
		if err := rows.Scan(
			&sm.ID, &sm.Type, &sm.Content, &sm.Title, &sm.Importance, &sm.Source, &tags,
			&sm.CreatedAt, &sm.UpdatedAt, &accessedAt, &sm.AccessCount, &sm.Archived, &sm.SuppressedAt, &sm.DomainID,
		); err != nil {
			return nil, err
		}
		sm.Tags = tags.String
		sm.AccessedAt = accessedAt.String
		results = append(results, sm)
	}
	return results, rows.Err()
}

type SearchResult struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Title    string        `json:"title"`
	Snippet  string        `json:"snippet"`
	DomainID sql.NullInt64 `json:"domain_id"`
	Status   string        `json:"status"`
}

func (db *DB) SearchAll(query string, domainID *int, entityType string, status string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query must not be empty")
	}

	like := "%" + query + "%"

	var parts []string
	var args []any

	if entityType == "" || entityType == "goal" {
		q := `SELECT id, 'goal' AS type, title, COALESCE(context, '') AS snippet, domain_id, status
		      FROM goals WHERE (title LIKE ? OR context LIKE ?)`
		a := []any{like, like}
		if domainID != nil {
			q += ` AND domain_id = ?`
			a = append(a, *domainID)
		}
		if status != "" {
			q += ` AND status = ?`
			a = append(a, status)
		}
		parts = append(parts, q)
		args = append(args, a...)
	}

	if entityType == "" || entityType == "task" {
		q := `SELECT id, 'task' AS type, title, COALESCE(context, '') AS snippet, domain_id, status
		      FROM tasks WHERE (title LIKE ? OR context LIKE ?)`
		a := []any{like, like}
		if domainID != nil {
			q += ` AND domain_id = ?`
			a = append(a, *domainID)
		}
		if status != "" {
			q += ` AND status = ?`
			a = append(a, status)
		}
		parts = append(parts, q)
		args = append(args, a...)
	}

	if entityType == "" || entityType == "note" {
		q := `SELECT id, 'note' AS type, title, COALESCE(content, '') AS snippet, domain_id, '' AS status
		      FROM notes WHERE (title LIKE ? OR content LIKE ?)`
		a := []any{like, like}
		if domainID != nil {
			q += ` AND domain_id = ?`
			a = append(a, *domainID)
		}
		parts = append(parts, q)
		args = append(args, a...)
	}

	if len(parts) == 0 {
		return []SearchResult{}, nil
	}

	fullQuery := strings.Join(parts, " UNION ALL ") + " LIMIT 50"

	rows, err := db.conn.Query(fullQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search all: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Type, &r.Title, &r.Snippet, &r.DomainID, &r.Status); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if results == nil {
		results = []SearchResult{}
	}
	return results, rows.Err()
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
