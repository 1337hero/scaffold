package db

func (db *DB) UpsertFTS(memoryID, title, content, tags string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM memories_fts WHERE memory_id = ?`, memoryID); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO memories_fts(memory_id, title, content, tags) VALUES (?, ?, ?, ?)`,
		memoryID, title, content, tags,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) DeleteFTS(memoryID string) error {
	_, err := db.conn.Exec(`DELETE FROM memories_fts WHERE memory_id = ?`, memoryID)
	return err
}

func (db *DB) RebuildFTS() error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM memories_fts`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO memories_fts(memory_id, title, content, tags)
		SELECT id, COALESCE(title,''), content, COALESCE(tags,'')
		FROM memories
		WHERE suppressed_at IS NULL
	`); err != nil {
		return err
	}
	return tx.Commit()
}
