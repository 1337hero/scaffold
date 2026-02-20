package db

type ConversationEntry struct {
	ID        string
	Sender    string
	Role      string
	Content   string
	CreatedAt string
}

func (db *DB) InsertConversationEntry(sender, role, content string) (string, error) {
	id := newID()
	_, err := db.conn.Exec(
		`INSERT INTO conversation_log (id, sender, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, sender, role, content, now(),
	)
	return id, err
}

func (db *DB) ListRecentConversation(sender string, limit int) ([]ConversationEntry, error) {
	return db.queryConversation(
		`SELECT id, sender, role, content, created_at FROM (
			SELECT id, sender, role, content, created_at
			FROM conversation_log WHERE sender = ?
			ORDER BY created_at DESC LIMIT ?
		) ORDER BY created_at ASC`, sender, limit,
	)
}

func (db *DB) ListConversationSince(since string, limit int) ([]ConversationEntry, error) {
	return db.queryConversation(
		`SELECT id, sender, role, content, created_at
		 FROM conversation_log
		 WHERE created_at >= ?
		 ORDER BY created_at ASC
		 LIMIT ?`, since, limit,
	)
}

func (db *DB) queryConversation(query string, args ...any) ([]ConversationEntry, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ConversationEntry
	for rows.Next() {
		var e ConversationEntry
		if err := rows.Scan(&e.ID, &e.Sender, &e.Role, &e.Content, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (db *DB) ConversationCount(sender string) (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM conversation_log WHERE sender = ?`, sender).Scan(&count)
	return count, err
}
