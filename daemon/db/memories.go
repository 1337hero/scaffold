package db

import "database/sql"

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
		`INSERT INTO memories (id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Type, m.Content, m.Title, m.Importance, m.Source, m.Tags,
		m.CreatedAt, m.UpdatedAt, m.AccessedAt, m.AccessCount, m.Archived, m.SuppressedAt,
	)
	return err
}

func (db *DB) ListByImportance(limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at
		 FROM memories WHERE suppressed_at IS NULL ORDER BY importance DESC LIMIT ?`, limit,
	)
}

func (db *DB) ListByType(memType string, limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at
		 FROM memories WHERE type = ? AND suppressed_at IS NULL ORDER BY importance DESC LIMIT ?`, memType, limit,
	)
}

func (db *DB) ListTodosByImportance(minImportance float64, limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at
		 FROM memories WHERE type = 'Todo' AND importance >= ? AND suppressed_at IS NULL ORDER BY importance DESC LIMIT ?`,
		minImportance, limit,
	)
}

func (db *DB) ListRecentMemories(limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at
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
		if err := rows.Scan(
			&m.ID, &m.Type, &m.Content, &m.Title, &m.Importance, &m.Source, &m.Tags,
			&m.CreatedAt, &m.UpdatedAt, &m.AccessedAt, &m.AccessCount, &m.Archived, &m.SuppressedAt,
		); err != nil {
			return nil, err
		}
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
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at
		 FROM memories WHERE id = ?`, id,
	)
	var m Memory
	err := row.Scan(
		&m.ID, &m.Type, &m.Content, &m.Title, &m.Importance, &m.Source, &m.Tags,
		&m.CreatedAt, &m.UpdatedAt, &m.AccessedAt, &m.AccessCount, &m.Archived, &m.SuppressedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}
