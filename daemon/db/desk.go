package db

import "database/sql"

type DeskItem struct {
	ID          string
	MemoryID    sql.NullString
	Title       string
	Position    int
	Status      string
	MicroSteps  sql.NullString
	Date        string
	CreatedAt   string
	CompletedAt sql.NullString
}

func (db *DB) InsertDeskItem(d DeskItem) error {
	if d.ID == "" {
		d.ID = newID()
	}
	if d.CreatedAt == "" {
		d.CreatedAt = now()
	}
	if d.Status == "" {
		d.Status = "active"
	}

	_, err := db.conn.Exec(
		`INSERT INTO desk (id, memory_id, title, position, status, micro_steps, date, created_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.MemoryID, d.Title, d.Position, d.Status, d.MicroSteps, d.Date, d.CreatedAt, d.CompletedAt,
	)
	return err
}

func (db *DB) TodaysDesk() ([]DeskItem, error) {
	return db.queryDesk(
		`SELECT id, memory_id, title, position, status, micro_steps, date, created_at, completed_at
		 FROM desk WHERE date = ? ORDER BY position ASC`, today(),
	)
}

func (db *DB) YesterdaysDesk() ([]DeskItem, error) {
	return db.queryDesk(
		`SELECT id, memory_id, title, position, status, micro_steps, date, created_at, completed_at
		 FROM desk WHERE date = date('now', '-1 day') ORDER BY position ASC`,
	)
}

func (db *DB) UpdateDeskStatus(id, status string) error {
	q := `UPDATE desk SET status = ?`
	args := []any{status}

	if status == "done" {
		q += `, completed_at = ?`
		args = append(args, now())
	} else {
		q += `, completed_at = NULL`
	}
	q += ` WHERE id = ?`
	args = append(args, id)

	result, err := db.conn.Exec(q, args...)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) DeferDeskItem(id string) error {
	result, err := db.conn.Exec(
		`UPDATE desk SET status = 'deferred', date = ?, completed_at = NULL WHERE id = ?`,
		tomorrow(), id,
	)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) queryDesk(query string, args ...any) ([]DeskItem, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DeskItem, 0)
	for rows.Next() {
		var d DeskItem
		if err := rows.Scan(
			&d.ID, &d.MemoryID, &d.Title, &d.Position, &d.Status, &d.MicroSteps,
			&d.Date, &d.CreatedAt, &d.CompletedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func requireRowsAffected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
