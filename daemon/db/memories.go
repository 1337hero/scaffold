package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

type Memory struct {
	ID           string
	Type         string
	Content      string
	Title        string
	Importance   float64
	Source       string
	Tags         string
	CreatedAt    string
	UpdatedAt    string
	AccessedAt   string
	AccessCount  int
	Archived     int
	SuppressedAt sql.NullString
	DomainID     sql.NullInt64
}

type EmbeddingJob struct {
	MemoryID string
	Reason   string
	Attempts int
}

type ReclassifyParams struct {
	Type       string
	Action     string
	Tags       string
	Importance float64
}

func (db *DB) InsertMemory(m Memory) error {
	if m.ID == "" {
		m.ID = newID()
	}
	ts := now()
	if m.CreatedAt == "" {
		m.CreatedAt = ts
	}
	if m.UpdatedAt == "" {
		m.UpdatedAt = ts
	}

	_, err := db.conn.Exec(
		`INSERT INTO memories (id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at, domain_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Type, m.Content, m.Title, m.Importance, m.Source, m.Tags,
		m.CreatedAt, m.UpdatedAt, m.AccessedAt, m.AccessCount, m.Archived, m.SuppressedAt, m.DomainID,
	)
	if err != nil {
		return err
	}
	if err := db.EnqueueEmbeddingJob(m.ID, "insert"); err != nil {
		return fmt.Errorf("enqueue embedding job: %w", err)
	}
	return nil
}

func (db *DB) ListByImportance(limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at, domain_id
		 FROM memories WHERE suppressed_at IS NULL ORDER BY importance DESC LIMIT ?`, limit,
	)
}

func (db *DB) ListByType(memType string, limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at, domain_id
		 FROM memories WHERE type = ? AND suppressed_at IS NULL ORDER BY importance DESC LIMIT ?`, memType, limit,
	)
}

func (db *DB) ListTodosByImportance(minImportance float64, limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at, domain_id
		 FROM memories WHERE type = 'Todo' AND importance >= ? AND suppressed_at IS NULL ORDER BY importance DESC LIMIT ?`,
		minImportance, limit,
	)
}

func (db *DB) ListRecentMemories(limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at, domain_id
		 FROM memories WHERE suppressed_at IS NULL ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
}

func (db *DB) queryMemories(query string, args ...any) ([]Memory, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Memory
	for rows.Next() {
		var m Memory
		var tags, accessedAt sql.NullString
		if err := rows.Scan(
			&m.ID, &m.Type, &m.Content, &m.Title, &m.Importance, &m.Source, &tags,
			&m.CreatedAt, &m.UpdatedAt, &accessedAt, &m.AccessCount, &m.Archived, &m.SuppressedAt, &m.DomainID,
		); err != nil {
			return nil, err
		}
		m.Tags = tags.String
		m.AccessedAt = accessedAt.String
		out = append(out, m)
	}
	return out, rows.Err()
}

func (db *DB) ReclassifyMemory(id string, p ReclassifyParams) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		`UPDATE memories SET type = ?, tags = ?, importance = ?, updated_at = ? WHERE id = ?`,
		p.Type, p.Tags, p.Importance, now(), id,
	)
	if err != nil {
		return err
	}
	if err := requireRowsAffected(result); err != nil {
		return err
	}

	if p.Action != "" {
		if _, err := tx.Exec(`UPDATE captures SET triage_action = ? WHERE memory_id = ?`, p.Action, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *DB) InsertObservation(pattern string, importance float64, evidenceIDs []string) (string, error) {
	mem := Memory{
		ID:         newID(),
		Type:       "Observation",
		Content:    pattern,
		Title:      pattern,
		Importance: importance,
		Source:     "cortex",
	}
	if err := db.InsertMemory(mem); err != nil {
		return "", fmt.Errorf("insert observation memory: %w", err)
	}

	for _, evidenceID := range evidenceIDs {
		target, err := db.GetMemory(evidenceID)
		if err != nil || target == nil {
			continue
		}
		if err := db.InsertEdge(Edge{
			FromID:   mem.ID,
			ToID:     evidenceID,
			Relation: "DerivedFrom",
			Weight:   0.7,
		}); err != nil {
			log.Printf("db: insert DerivedFrom edge %s -> %s: %v", mem.ID, evidenceID, err)
		}
	}

	return mem.ID, nil
}

func (db *DB) SuppressMemory(id string) error {
	result, err := db.conn.Exec(
		`UPDATE memories SET suppressed_at = ? WHERE id = ?`, now(), id,
	)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) UnsuppressMemory(id string) error {
	result, err := db.conn.Exec(
		`UPDATE memories SET suppressed_at = NULL WHERE id = ?`, id,
	)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) GetMemory(id string) (*Memory, error) {
	row := db.conn.QueryRow(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at, domain_id
		 FROM memories WHERE id = ?`, id,
	)
	var m Memory
	var tags, accessedAt sql.NullString
	err := row.Scan(
		&m.ID, &m.Type, &m.Content, &m.Title, &m.Importance, &m.Source, &tags,
		&m.CreatedAt, &m.UpdatedAt, &accessedAt, &m.AccessCount, &m.Archived, &m.SuppressedAt, &m.DomainID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Tags = tags.String
	m.AccessedAt = accessedAt.String
	return &m, nil
}

// MarkMemoriesAccessed updates access metadata for the provided memory IDs.
// Unknown IDs are ignored.
func (db *DB) MarkMemoriesAccessed(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	deduped := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		deduped[id] = struct{}{}
	}
	if len(deduped) == 0 {
		return nil
	}

	ts := now()
	for id := range deduped {
		if _, err := db.conn.Exec(
			`UPDATE memories SET accessed_at = ?, access_count = access_count + 1 WHERE id = ?`,
			ts, id,
		); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) EnqueueEmbeddingJob(memoryID, reason string) error {
	_, err := db.conn.Exec(
		`INSERT INTO embedding_jobs (memory_id, reason, enqueued_at, attempts)
		 VALUES (?, ?, ?, 0)
		 ON CONFLICT(memory_id) DO UPDATE SET reason = excluded.reason, enqueued_at = excluded.enqueued_at`,
		memoryID, reason, now(),
	)
	return err
}

func (db *DB) DequeueEmbeddingJobs(limit int) ([]EmbeddingJob, error) {
	rows, err := db.conn.Query(
		`SELECT ej.memory_id, ej.reason, ej.attempts
		 FROM embedding_jobs ej
		 JOIN memories m ON m.id = ej.memory_id
		 WHERE m.suppressed_at IS NULL
		 ORDER BY ej.attempts ASC, ej.enqueued_at ASC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []EmbeddingJob
	for rows.Next() {
		var j EmbeddingJob
		if err := rows.Scan(&j.MemoryID, &j.Reason, &j.Attempts); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (db *DB) IncrementEmbeddingJobAttempts(memoryID string) error {
	_, err := db.conn.Exec(`UPDATE embedding_jobs SET attempts = attempts + 1 WHERE memory_id = ?`, memoryID)
	return err
}

func (db *DB) DeleteEmbeddingJob(memoryID string) error {
	_, err := db.conn.Exec(`DELETE FROM embedding_jobs WHERE memory_id = ?`, memoryID)
	return err
}

func (db *DB) ListMemoriesWithoutEmbedding(limit int) ([]string, error) {
	rows, err := db.conn.Query(
		`SELECT m.id FROM memories m
		 LEFT JOIN memory_embeddings me ON me.memory_id = m.id
		 WHERE m.suppressed_at IS NULL AND me.memory_id IS NULL
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
