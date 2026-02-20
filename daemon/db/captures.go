package db

import "database/sql"

type Capture struct {
	ID           string
	Raw          string
	Source       string
	Processed    int
	TriageAction sql.NullString
	MemoryID     sql.NullString
	CreatedAt    string
	Confirmed    int
}

func (db *DB) InsertCapture(raw, source string) (string, error) {
	id := newID()
	_, err := db.conn.Exec(
		`INSERT INTO captures (id, raw, source, created_at) VALUES (?, ?, ?, ?)`,
		id, raw, source, now(),
	)
	return id, err
}

func (db *DB) InsertProcessedCapture(raw, source, triageAction string) (string, error) {
	id := newID()
	_, err := db.conn.Exec(
		`INSERT INTO captures (id, raw, source, processed, triage_action, created_at) VALUES (?, ?, ?, 1, ?, ?)`,
		id, raw, source, triageAction, now(),
	)
	return id, err
}

func (db *DB) ListUnprocessed() ([]Capture, error) {
	return db.queryCaptures(`SELECT id, raw, source, processed, triage_action, memory_id, created_at, confirmed FROM captures WHERE processed = 0 ORDER BY created_at DESC`)
}

func (db *DB) ListRecent(limit int) ([]Capture, error) {
	return db.queryCaptures(`SELECT id, raw, source, processed, triage_action, memory_id, created_at, confirmed FROM captures ORDER BY created_at DESC LIMIT ?`, limit)
}

func (db *DB) ListRecentBySender(sender string, limit int) ([]Capture, error) {
	return db.queryCaptures(
		`SELECT id, raw, source, processed, triage_action, memory_id, created_at, confirmed
		 FROM captures
		 WHERE source IN (?, ?, ?, 'signal')
		 ORDER BY created_at DESC
		 LIMIT ?`,
		"signal:user:"+sender, "signal:assistant:"+sender, "signal:"+sender, limit,
	)
}

func (db *DB) UpdateTriage(id, action, memoryID string) error {
	_, err := db.conn.Exec(
		`UPDATE captures SET processed = 1, triage_action = ?, memory_id = ? WHERE id = ?`,
		action, memoryID, id,
	)
	return err
}

func (db *DB) queryCaptures(query string, args ...any) ([]Capture, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Capture
	for rows.Next() {
		var c Capture
		if err := rows.Scan(&c.ID, &c.Raw, &c.Source, &c.Processed, &c.TriageAction, &c.MemoryID, &c.CreatedAt, &c.Confirmed); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *DB) ConfirmCapture(id string) error {
	result, err := db.conn.Exec(`UPDATE captures SET confirmed = 1 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}
