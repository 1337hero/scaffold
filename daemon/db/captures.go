package db

import (
	"database/sql"
	"fmt"
)

type Capture struct {
	ID           string
	Raw          string
	Source       string
	Processed    int
	TriageAction sql.NullString
	MemoryID     sql.NullString
	CreatedAt    string
	Confirmed    int
	DomainID     sql.NullInt64
}

func (db *DB) GetCapture(id string) (*Capture, error) {
	row := db.conn.QueryRow(
		`SELECT id, raw, source, processed, triage_action, memory_id, created_at, confirmed, domain_id
		 FROM captures WHERE id = ?`, id,
	)

	var c Capture
	err := row.Scan(&c.ID, &c.Raw, &c.Source, &c.Processed, &c.TriageAction, &c.MemoryID, &c.CreatedAt, &c.Confirmed, &c.DomainID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
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
	return db.queryCaptures(`SELECT id, raw, source, processed, triage_action, memory_id, created_at, confirmed, domain_id FROM captures WHERE processed = 0 ORDER BY created_at DESC`)
}

func (db *DB) ListRecent(limit int) ([]Capture, error) {
	return db.queryCaptures(`SELECT id, raw, source, processed, triage_action, memory_id, created_at, confirmed, domain_id FROM captures ORDER BY created_at DESC LIMIT ?`, limit)
}

func (db *DB) ListRecentBySender(sender string, limit int) ([]Capture, error) {
	return db.queryCaptures(
		`SELECT id, raw, source, processed, triage_action, memory_id, created_at, confirmed, domain_id
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
		if err := rows.Scan(&c.ID, &c.Raw, &c.Source, &c.Processed, &c.TriageAction, &c.MemoryID, &c.CreatedAt, &c.Confirmed, &c.DomainID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *DB) MarkCaptureProcessed(id, triageAction string) error {
	result, err := db.conn.Exec(
		`UPDATE captures SET processed = 1, triage_action = ? WHERE id = ?`,
		triageAction, id,
	)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) ConfirmCapture(id string) error {
	result, err := db.conn.Exec(`UPDATE captures SET confirmed = 1 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) UpdateCaptureSource(id, source string) error {
	result, err := db.conn.Exec(`UPDATE captures SET source = ? WHERE id = ?`, source, id)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

// InsertProcessedCaptureWithMemory atomically inserts a memory, enqueues its embedding job,
// and inserts a processed capture linked to that memory.
func (db *DB) InsertProcessedCaptureWithMemory(raw, source, action string, mem Memory, embeddingReason string) (string, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return "", fmt.Errorf("begin capture+memory tx: %w", err)
	}
	defer tx.Rollback()

	mem, ts, err := db.prepareAndInsertMemoryTx(tx, mem, embeddingReason)
	if err != nil {
		return "", err
	}

	captureID := newID()
	if _, err := tx.Exec(
		`INSERT INTO captures (id, raw, source, processed, triage_action, memory_id, created_at, domain_id)
		 VALUES (?, ?, ?, 1, ?, ?, ?, ?)`,
		captureID, raw, source, action, mem.ID, ts, mem.DomainID,
	); err != nil {
		return "", fmt.Errorf("insert processed capture in tx: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit capture+memory tx: %w", err)
	}
	return captureID, nil
}

// PersistTriageResult atomically inserts the memory and links capture triage fields.
func (db *DB) PersistTriageResult(captureID string, mem Memory, action string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin triage tx: %w", err)
	}
	defer tx.Rollback()

	mem, _, err = db.prepareAndInsertMemoryTx(tx, mem, "triage")
	if err != nil {
		return err
	}

	result, err := tx.Exec(
		`UPDATE captures SET triage_action = ?, memory_id = ?, domain_id = ? WHERE id = ?`,
		action, mem.ID, mem.DomainID, captureID,
	)
	if err != nil {
		return fmt.Errorf("update capture in triage tx: %w", err)
	}
	if err := requireRowsAffected(result); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit triage tx: %w", err)
	}
	return nil
}

type InboxCapture struct {
	ID           string         `json:"ID"`
	Raw          string         `json:"Raw"`
	Source       string         `json:"Source"`
	Processed    int            `json:"Processed"`
	TriageAction sql.NullString `json:"TriageAction"`
	MemoryID     sql.NullString `json:"MemoryID"`
	CreatedAt    string         `json:"CreatedAt"`
	Confirmed    int            `json:"Confirmed"`
	DomainID     sql.NullInt64  `json:"DomainID"`
	MemoryTitle  sql.NullString `json:"MemoryTitle"`
	MemoryType   sql.NullString `json:"MemoryType"`
}

func (db *DB) ListInboxCaptures(limit int) ([]InboxCapture, error) {
	rows, err := db.conn.Query(
		`SELECT c.id, c.raw, c.source, c.processed, c.triage_action, c.memory_id,
		        c.created_at, c.confirmed, c.domain_id,
		        m.title, m.type
		 FROM captures c
		 LEFT JOIN memories m ON m.id = c.memory_id
		 WHERE LOWER(TRIM(c.source)) != 'user:archive'
		 ORDER BY c.created_at DESC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []InboxCapture
	for rows.Next() {
		var c InboxCapture
		if err := rows.Scan(
			&c.ID, &c.Raw, &c.Source, &c.Processed, &c.TriageAction, &c.MemoryID,
			&c.CreatedAt, &c.Confirmed, &c.DomainID,
			&c.MemoryTitle, &c.MemoryType,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *DB) prepareAndInsertMemoryTx(tx *sql.Tx, mem Memory, embeddingReason string) (Memory, string, error) {
	if mem.ID == "" {
		mem.ID = newID()
	}
	ts := now()
	if mem.CreatedAt == "" {
		mem.CreatedAt = ts
	}
	if mem.UpdatedAt == "" {
		mem.UpdatedAt = ts
	}
	if embeddingReason == "" {
		embeddingReason = "insert"
	}

	if _, err := tx.Exec(
		`INSERT INTO memories (id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at, domain_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mem.ID, mem.Type, mem.Content, mem.Title, mem.Importance, mem.Source, mem.Tags,
		mem.CreatedAt, mem.UpdatedAt, mem.AccessedAt, mem.AccessCount, mem.Archived, mem.SuppressedAt, mem.DomainID,
	); err != nil {
		return Memory{}, "", fmt.Errorf("insert memory in tx: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO embedding_jobs (memory_id, reason, enqueued_at, attempts)
		 VALUES (?, ?, ?, 0)
		 ON CONFLICT(memory_id) DO UPDATE SET reason = excluded.reason, enqueued_at = excluded.enqueued_at`,
		mem.ID, embeddingReason, ts,
	); err != nil {
		return Memory{}, "", fmt.Errorf("enqueue embedding job in tx: %w", err)
	}

	return mem, ts, nil
}
