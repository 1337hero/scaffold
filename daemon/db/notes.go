package db

import (
	"database/sql"
	"fmt"
	"strings"
)

type Note struct {
	ID            string
	Title         string
	DomainID      sql.NullInt64
	GoalID        sql.NullString
	TaskID        sql.NullString
	Content       sql.NullString
	Tags          sql.NullString
	Kind          string // note|journal|quote
	Source        sql.NullString
	FlagForReview int // 0|1
	ReviewAt      sql.NullString
	PersonID      sql.NullString
	ProjectID     sql.NullString
	SuppressedAt  sql.NullString
	CreatedAt     string
	UpdatedAt     sql.NullString
}

const noteCols = `id, title, domain_id, goal_id, task_id, content, tags,
	kind, source, flag_for_review, review_at, person_id, project_id, suppressed_at, created_at, updated_at`

func validateNoteKind(kind string) error {
	switch kind {
	case "note", "journal", "quote":
		return nil
	default:
		return fmt.Errorf("%w: note kind %q (must be note|journal|quote)", ErrInvalidEnum, kind)
	}
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
	if n.Kind == "" {
		n.Kind = "note"
	}
	if err := validateNoteKind(n.Kind); err != nil {
		return err
	}

	_, err := db.conn.Exec(
		`INSERT INTO notes (id, title, domain_id, goal_id, task_id, content, tags,
		    kind, source, flag_for_review, review_at, person_id, project_id, suppressed_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Title, n.DomainID, n.GoalID, n.TaskID, n.Content, n.Tags,
		n.Kind, n.Source, n.FlagForReview, n.ReviewAt, n.PersonID, n.ProjectID, n.SuppressedAt, n.CreatedAt, n.UpdatedAt,
	)
	return err
}

func scanNote(s interface {
	Scan(...any) error
}) (Note, error) {
	var n Note
	err := s.Scan(&n.ID, &n.Title, &n.DomainID, &n.GoalID, &n.TaskID, &n.Content, &n.Tags,
		&n.Kind, &n.Source, &n.FlagForReview, &n.ReviewAt, &n.PersonID, &n.ProjectID, &n.SuppressedAt, &n.CreatedAt, &n.UpdatedAt)
	return n, err
}

func (db *DB) GetNote(id string) (*Note, error) {
	row := db.conn.QueryRow(`SELECT `+noteCols+` FROM notes WHERE id = ? AND suppressed_at IS NULL`, id)
	n, err := scanNote(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// NoteFilters are the optional AND'd filters for ListNotes; nil skips a filter.
type NoteFilters struct {
	DomainID      *int
	GoalID        *string
	TaskID        *string
	Tags          string // substring match
	Kind          *string
	PersonID      *string
	ProjectID     *string
	Source        *string
	FlagForReview *bool
	Surface       *string
	Query         string
}

func (db *DB) ListNotes(f NoteFilters) ([]Note, error) {
	q := `SELECT ` + noteCols + ` FROM notes WHERE suppressed_at IS NULL`
	var args []any

	if f.DomainID != nil {
		q += ` AND domain_id = ?`
		args = append(args, *f.DomainID)
	}
	if f.GoalID != nil {
		q += ` AND goal_id = ?`
		args = append(args, *f.GoalID)
	}
	if f.TaskID != nil {
		q += ` AND task_id = ?`
		args = append(args, *f.TaskID)
	}
	if f.Tags != "" {
		q += ` AND tags LIKE ?`
		args = append(args, "%"+f.Tags+"%")
	}
	if f.Kind != nil {
		q += ` AND kind = ?`
		args = append(args, *f.Kind)
	}
	if f.PersonID != nil {
		q += ` AND person_id = ?`
		args = append(args, *f.PersonID)
	}
	if f.ProjectID != nil {
		q += ` AND project_id = ?`
		args = append(args, *f.ProjectID)
	}
	if f.Source != nil {
		q += ` AND source = ?`
		args = append(args, *f.Source)
	}
	if f.FlagForReview != nil {
		q += ` AND flag_for_review = ?`
		if *f.FlagForReview {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if f.Surface != nil {
		q += ` AND (domain_id IS NULL OR domain_id IN (SELECT id FROM domains WHERE surface = ?))`
		args = append(args, *f.Surface)
	}
	if strings.TrimSpace(f.Query) != "" {
		like := "%" + strings.ToLower(strings.TrimSpace(f.Query)) + "%"
		q += ` AND (LOWER(title) LIKE ? OR LOWER(COALESCE(content, '')) LIKE ? OR LOWER(COALESCE(tags, '')) LIKE ? OR LOWER(COALESCE(source, '')) LIKE ?)`
		args = append(args, like, like, like, like)
	}

	q += ` ORDER BY COALESCE(updated_at, created_at) DESC`
	return db.queryNotes(q, args...)
}

var noteUpdateCols = map[string]bool{
	"title":           true,
	"domain_id":       true,
	"goal_id":         true,
	"task_id":         true,
	"content":         true,
	"tags":            true,
	"kind":            true,
	"source":          true,
	"flag_for_review": true,
	"review_at":       true,
	"person_id":       true,
	"project_id":      true,
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
		switch col {
		case "kind":
			s, ok := val.(string)
			if !ok {
				return fmt.Errorf("%w: note kind must be a string", ErrInvalidEnum)
			}
			if err := validateNoteKind(s); err != nil {
				return err
			}
		case "flag_for_review":
			// Accept JSON booleans; the column is INTEGER 0|1.
			if b, ok := val.(bool); ok {
				if b {
					val = 1
				} else {
					val = 0
				}
			}
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
	ts := now()
	result, err := db.conn.Exec(`UPDATE notes SET suppressed_at = ?, updated_at = ? WHERE id = ? AND suppressed_at IS NULL`, ts, ts, id)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) NotesByDomain(domainID int) ([]Note, error) {
	return db.queryNotes(
		`SELECT `+noteCols+` FROM notes WHERE suppressed_at IS NULL AND domain_id = ? ORDER BY COALESCE(updated_at, created_at) DESC`,
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
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (db *DB) NotesByTask(taskID string) ([]Note, error) {
	return db.queryNotes(
		`SELECT `+noteCols+` FROM notes WHERE suppressed_at IS NULL AND task_id = ? ORDER BY COALESCE(updated_at, created_at) DESC`,
		taskID,
	)
}
