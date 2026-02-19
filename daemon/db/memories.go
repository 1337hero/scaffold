package db

type Memory struct {
	ID          string
	Type        string
	Content     string
	Title       string
	Importance  float64
	Source      string
	Tags        string
	CreatedAt   string
	UpdatedAt   string
	AccessedAt  string
	AccessCount int
	Archived    int
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
		`INSERT INTO memories (id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Type, m.Content, m.Title, m.Importance, m.Source, m.Tags,
		m.CreatedAt, m.UpdatedAt, m.AccessedAt, m.AccessCount, m.Archived,
	)
	return err
}

func (db *DB) ListByImportance(limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived
		 FROM memories WHERE archived = 0 ORDER BY importance DESC LIMIT ?`, limit,
	)
}

func (db *DB) ListByType(memType string, limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived
		 FROM memories WHERE type = ? AND archived = 0 ORDER BY importance DESC LIMIT ?`, memType, limit,
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
			&m.CreatedAt, &m.UpdatedAt, &m.AccessedAt, &m.AccessCount, &m.Archived,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
