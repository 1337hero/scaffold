package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Task struct {
	ID          string
	Title       string
	DomainID    sql.NullInt64
	DomainName  string
	GoalID      sql.NullString
	Context     sql.NullString
	DueDate     sql.NullString
	Recurring   sql.NullString
	Priority    string
	Status      string
	MicroSteps  sql.NullString
	Notify      int
	Position    int
	IsFocus     int
	Source      sql.NullString
	SourceRef   sql.NullString
	CreatedAt   string
	CompletedAt sql.NullString
}

func (db *DB) InsertTask(t Task) error {
	if t.ID == "" {
		t.ID = newID()
	}
	if t.CreatedAt == "" {
		t.CreatedAt = now()
	}
	if t.Status == "" {
		t.Status = "pending"
	}
	if t.Priority == "" {
		t.Priority = "normal"
	}

	_, err := db.conn.Exec(
		`INSERT INTO tasks (id, title, domain_id, goal_id, context, due_date, recurring, priority, status, micro_steps, notify, position, source, source_ref, created_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.DomainID, t.GoalID, t.Context, t.DueDate, t.Recurring,
		t.Priority, t.Status, t.MicroSteps, t.Notify, t.Position, t.Source, t.SourceRef,
		t.CreatedAt, t.CompletedAt,
	)
	return err
}

func (db *DB) GetTask(id string) (*Task, error) {
	row := db.conn.QueryRow(
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring, t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus, t.source, t.source_ref, t.created_at, t.completed_at, d.name
		 FROM tasks t LEFT JOIN domains d ON t.domain_id = d.id
		 WHERE t.id = ?`, id,
	)

	var t Task
	var domainName sql.NullString
	err := row.Scan(&t.ID, &t.Title, &t.DomainID, &t.GoalID, &t.Context, &t.DueDate,
		&t.Recurring, &t.Priority, &t.Status, &t.MicroSteps, &t.Notify, &t.Position, &t.IsFocus,
		&t.Source, &t.SourceRef, &t.CreatedAt, &t.CompletedAt, &domainName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if domainName.Valid {
		t.DomainName = domainName.String
	}
	return &t, nil
}

func (db *DB) ListTasks(domainID *int, goalID *string, status string, due string) ([]Task, error) {
	if status == "" {
		status = "pending"
	}

	clauses := []string{"t.status = ?"}
	args := []any{status}

	if domainID != nil {
		clauses = append(clauses, "t.domain_id = ?")
		args = append(args, *domainID)
	}
	if goalID != nil {
		clauses = append(clauses, "t.goal_id = ?")
		args = append(args, *goalID)
	}

	switch due {
	case "today":
		clauses = append(clauses, "t.due_date <= ?")
		args = append(args, today())
	case "tomorrow":
		clauses = append(clauses, "t.due_date = ?")
		args = append(args, tomorrow())
	case "week":
		clauses = append(clauses, "t.due_date <= ?")
		args = append(args, time.Now().AddDate(0, 0, 7).Format("2006-01-02"))
	}

	query := fmt.Sprintf(
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring, t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus, t.source, t.source_ref, t.created_at, t.completed_at, d.name
		 FROM tasks t LEFT JOIN domains d ON t.domain_id = d.id
		 WHERE %s ORDER BY t.position ASC, t.due_date ASC`,
		strings.Join(clauses, " AND "),
	)

	return db.queryTasks(query, args...)
}

var taskUpdateFields = map[string]bool{
	"title": true, "domain_id": true, "goal_id": true, "context": true,
	"due_date": true, "recurring": true, "priority": true, "status": true,
	"micro_steps": true, "notify": true, "position": true, "is_focus": true,
	"source": true, "source_ref": true, "completed_at": true,
}

func (db *DB) UpdateTask(id string, updates map[string]any) error {
	sets := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)

	for k, v := range updates {
		if !taskUpdateFields[k] {
			return fmt.Errorf("unsupported task update field: %s", k)
		}
		sets = append(sets, k+" = ?")
		args = append(args, v)
	}

	if len(sets) == 0 {
		return nil
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE tasks SET %s WHERE id = ?", strings.Join(sets, ", "))
	result, err := db.conn.Exec(query, args...)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) CompleteTask(id string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin complete task tx: %w", err)
	}
	defer tx.Rollback()

	var goalID sql.NullString
	var recurring sql.NullString
	var dueDate sql.NullString
	err = tx.QueryRow(`SELECT goal_id, recurring, due_date FROM tasks WHERE id = ?`, id).Scan(&goalID, &recurring, &dueDate)
	if err != nil {
		return fmt.Errorf("lookup task for complete: %w", err)
	}

	ts := now()

	if _, err := tx.Exec(
		`UPDATE tasks SET status = 'done', completed_at = ? WHERE id = ?`, ts, id,
	); err != nil {
		return fmt.Errorf("mark task done: %w", err)
	}

	completionID := newID()
	if _, err := tx.Exec(
		`INSERT INTO task_completions (id, task_id, goal_id, completed_at) VALUES (?, ?, ?, ?)`,
		completionID, id, goalID, ts,
	); err != nil {
		return fmt.Errorf("log task completion: %w", err)
	}

	if recurring.Valid {
		nextDue := bumpDueDate(dueDate, recurring.String)
		if _, err := tx.Exec(
			`UPDATE tasks SET status = 'pending', completed_at = NULL, due_date = ?, is_focus = 0 WHERE id = ?`,
			nextDue, id,
		); err != nil {
			return fmt.Errorf("reset recurring task: %w", err)
		}
	}

	return tx.Commit()
}

func bumpDueDate(dueDate sql.NullString, recurring string) string {
	base := time.Now()
	if dueDate.Valid {
		if parsed, err := time.Parse("2006-01-02", dueDate.String); err == nil {
			base = parsed
		}
	}

	switch recurring {
	case "daily":
		return base.AddDate(0, 0, 1).Format("2006-01-02")
	case "weekly":
		return base.AddDate(0, 0, 7).Format("2006-01-02")
	case "monthly":
		return base.AddDate(0, 1, 0).Format("2006-01-02")
	default:
		return base.AddDate(0, 0, 1).Format("2006-01-02")
	}
}

func (db *DB) ReorderTask(id string, position int) error {
	result, err := db.conn.Exec(`UPDATE tasks SET position = ? WHERE id = ?`, position, id)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) SoftDeleteTask(id string) error {
	result, err := db.conn.Exec(`UPDATE tasks SET status = 'deleted' WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) TodaysTasks() ([]Task, error) {
	return db.queryTasks(
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring, t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus, t.source, t.source_ref, t.created_at, t.completed_at, d.name
		 FROM tasks t LEFT JOIN domains d ON t.domain_id = d.id
		 WHERE t.status = 'pending'
		   AND (t.due_date <= ? OR (t.recurring IS NOT NULL AND t.due_date IS NULL) OR t.is_focus = 1)
		 ORDER BY
		   t.is_focus DESC,
		   CASE WHEN t.recurring IS NOT NULL AND t.is_focus = 0 THEN 1 ELSE 0 END ASC,
		   CASE t.priority WHEN 'high' THEN 0 WHEN 'normal' THEN 1 WHEN 'low' THEN 2 ELSE 1 END ASC,
		   t.position ASC`,
		today(),
	)
}

func (db *DB) SetFocus(id string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE tasks SET is_focus = 0 WHERE is_focus = 1`); err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE tasks SET is_focus = 1 WHERE id = ? AND status = 'pending'`, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (db *DB) ClearFocus() error {
	_, err := db.conn.Exec(`UPDATE tasks SET is_focus = 0 WHERE is_focus = 1`)
	return err
}

func (db *DB) TasksByGoal(goalID string) ([]Task, error) {
	return db.queryTasks(
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring, t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus, t.source, t.source_ref, t.created_at, t.completed_at, d.name
		 FROM tasks t LEFT JOIN domains d ON t.domain_id = d.id
		 WHERE t.goal_id = ? ORDER BY t.position ASC`,
		goalID,
	)
}

func (db *DB) queryTasks(query string, args ...any) ([]Task, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Task, 0)
	for rows.Next() {
		var t Task
		var domainName sql.NullString
		if err := rows.Scan(&t.ID, &t.Title, &t.DomainID, &t.GoalID, &t.Context, &t.DueDate,
			&t.Recurring, &t.Priority, &t.Status, &t.MicroSteps, &t.Notify, &t.Position, &t.IsFocus,
			&t.Source, &t.SourceRef, &t.CreatedAt, &t.CompletedAt, &domainName); err != nil {
			return nil, err
		}
		if domainName.Valid {
			t.DomainName = domainName.String
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (db *DB) TaskBySourceRef(ref string) (*Task, error) {
	row := db.conn.QueryRow(
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring, t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus, t.source, t.source_ref, t.created_at, t.completed_at, d.name
		 FROM tasks t LEFT JOIN domains d ON t.domain_id = d.id
		 WHERE t.source_ref = ?`, ref,
	)

	var t Task
	var domainName sql.NullString
	err := row.Scan(&t.ID, &t.Title, &t.DomainID, &t.GoalID, &t.Context, &t.DueDate,
		&t.Recurring, &t.Priority, &t.Status, &t.MicroSteps, &t.Notify, &t.Position, &t.IsFocus,
		&t.Source, &t.SourceRef, &t.CreatedAt, &t.CompletedAt, &domainName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if domainName.Valid {
		t.DomainName = domainName.String
	}
	return &t, nil
}
