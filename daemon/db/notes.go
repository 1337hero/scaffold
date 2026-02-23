package db

import (
	"database/sql"
	"fmt"
	"strings"
)

type Note struct {
	ID        string
	Title     string
	DomainID  sql.NullInt64
	GoalID    sql.NullString
	Content   sql.NullString
	Tags      sql.NullString
	CreatedAt string
	UpdatedAt sql.NullString
}

func (db *DB) InsertNote(n Note) error {
	if n.ID == "" {
		n.ID = newID()
	}
	if n.CreatedAt == "" {
		n.CreatedAt = now()
	}
	if !n.UpdatedAt.Valid {
		n.UpdatedAt = sql.NullString{String: now(), Valid: true}
	}

	_, err := db.conn.Exec(
		`INSERT INTO notes (id, title, domain_id, goal_id, content, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Title, n.DomainID, n.GoalID, n.Content, n.Tags, n.CreatedAt, n.UpdatedAt,
	)
	return err
}

func (db *DB) GetNote(id string) (*Note, error) {
	row := db.conn.QueryRow(
		`SELECT id, title, domain_id, goal_id, content, tags, created_at, updated_at
		 FROM notes WHERE id = ?`, id,
	)

	var n Note
	err := row.Scan(&n.ID, &n.Title, &n.DomainID, &n.GoalID, &n.Content, &n.Tags, &n.CreatedAt, &n.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (db *DB) ListNotes(domainID *int, goalID *string, tags string) ([]Note, error) {
	q := `SELECT id, title, domain_id, goal_id, content, tags, created_at, updated_at FROM notes WHERE 1=1`
	var args []any

	if domainID != nil {
		q += ` AND domain_id = ?`
		args = append(args, *domainID)
	}
	if goalID != nil {
		q += ` AND goal_id = ?`
		args = append(args, *goalID)
	}
	if tags != "" {
		q += ` AND tags LIKE ?`
		args = append(args, "%"+tags+"%")
	}

	q += ` ORDER BY COALESCE(updated_at, created_at) DESC`
	return db.queryNotes(q, args...)
}

var noteUpdateCols = map[string]bool{
	"title":     true,
	"domain_id": true,
	"goal_id":   true,
	"content":   true,
	"tags":      true,
}

func (db *DB) UpdateNote(id string, updates map[string]any) error {
	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	var setClauses []string
	var args []any
	for col, val := range updates {
		if !noteUpdateCols[col] {
			return fmt.Errorf("unsupported update column: %s", col)
		}
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}

	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now())
	args = append(args, id)

	q := fmt.Sprintf("UPDATE notes SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	result, err := db.conn.Exec(q, args...)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) DeleteNote(id string) error {
	result, err := db.conn.Exec(`DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) NotesByDomain(domainID int) ([]Note, error) {
	return db.queryNotes(
		`SELECT id, title, domain_id, goal_id, content, tags, created_at, updated_at
		 FROM notes WHERE domain_id = ? ORDER BY COALESCE(updated_at, created_at) DESC`,
		domainID,
	)
}

func (db *DB) queryNotes(query string, args ...any) ([]Note, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Note, 0)
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.DomainID, &n.GoalID, &n.Content, &n.Tags, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
