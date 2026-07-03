package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Task struct {
	ID           string
	Title        string
	DomainID     sql.NullInt64
	DomainName   string
	GoalID       sql.NullString
	Context      sql.NullString
	DueDate      sql.NullString
	Recurring    sql.NullString
	Priority     string
	Status       string
	MicroSteps   sql.NullString
	Notify       int
	Position     int
	IsFocus      int
	ProjectID    sql.NullString
	ReminderAt   sql.NullString
	Surface      string
	Top3Position sql.NullInt64
	CreatedAt    string
	CompletedAt  sql.NullString
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
	if t.Surface == "" {
		t.Surface = "life"
	}
	if err := validateSurface(t.Surface); err != nil {
		return err
	}
	if err := validateTop3Position(t.Top3Position); err != nil {
		return err
	}

	_, err := db.conn.Exec(
		`INSERT INTO tasks (id, title, domain_id, goal_id, context, due_date, recurring, priority, status, micro_steps, notify, position, created_at, completed_at, project_id, reminder_at, surface, top3_position)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.DomainID, t.GoalID, t.Context, t.DueDate, t.Recurring,
		t.Priority, t.Status, t.MicroSteps, t.Notify, t.Position,
		t.CreatedAt, t.CompletedAt,
		t.ProjectID, t.ReminderAt, t.Surface, t.Top3Position,
	)
	if err != nil {
		return err
	}

	// Bump project activity when task is assigned to a project.
	if t.ProjectID.Valid {
		return bumpProjectActivity(db.conn, t.ProjectID.String)
	}
	return nil
}

func (db *DB) GetTask(id string) (*Task, error) {
	row := db.conn.QueryRow(
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring,
		        t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus,
		        t.created_at, t.completed_at,
		        t.project_id, t.reminder_at, t.surface, t.top3_position,
		        d.name
		 FROM tasks t LEFT JOIN domains d ON t.domain_id = d.id
		 WHERE t.id = ?`, id,
	)

	var t Task
	var domainName sql.NullString
	err := row.Scan(&t.ID, &t.Title, &t.DomainID, &t.GoalID, &t.Context, &t.DueDate,
		&t.Recurring, &t.Priority, &t.Status, &t.MicroSteps, &t.Notify, &t.Position, &t.IsFocus,
		&t.CreatedAt, &t.CompletedAt,
		&t.ProjectID, &t.ReminderAt, &t.Surface, &t.Top3Position,
		&domainName)
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

// validateTop3Position accepts null or positions 1-3.
func validateTop3Position(p sql.NullInt64) error {
	if p.Valid && !validTop3(p.Int64) {
		return fmt.Errorf("%w: top3_position %d (must be 1, 2, or 3)", ErrInvalidEnum, p.Int64)
	}
	return nil
}

func validTop3(n int64) bool {
	return n >= 1 && n <= 3
}

// TaskFilters are the optional AND'd filters for ListTasks; nil skips a filter.
type TaskFilters struct {
	DomainID   *int
	GoalID     *string
	Status     string // defaults to pending
	Due        string // today|tomorrow|week
	ProjectID  *string
	Surface    *string
	Top3       *bool   // true: in the Top 3 (any position); false: not in it
	ReminderAt *string // tasks whose reminder_at is set and <= this ISO timestamp
}

func (db *DB) ListTasks(f TaskFilters) ([]Task, error) {
	if f.Status == "" {
		f.Status = "pending"
	}

	clauses := []string{"t.status = ?"}
	args := []any{f.Status}

	if f.DomainID != nil {
		clauses = append(clauses, "t.domain_id = ?")
		args = append(args, *f.DomainID)
	}
	if f.GoalID != nil {
		clauses = append(clauses, "t.goal_id = ?")
		args = append(args, *f.GoalID)
	}
	if f.ProjectID != nil {
		clauses = append(clauses, "t.project_id = ?")
		args = append(args, *f.ProjectID)
	}
	if f.Surface != nil {
		clauses = append(clauses, "t.surface = ?")
		args = append(args, *f.Surface)
	}
	if f.Top3 != nil {
		if *f.Top3 {
			clauses = append(clauses, "t.top3_position IS NOT NULL")
		} else {
			clauses = append(clauses, "t.top3_position IS NULL")
		}
	}
	if f.ReminderAt != nil {
		clauses = append(clauses, "t.reminder_at IS NOT NULL AND t.reminder_at <= ?")
		args = append(args, *f.ReminderAt)
	}

	switch f.Due {
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
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring,
		        t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus,
		        t.created_at, t.completed_at,
		        t.project_id, t.reminder_at, t.surface, t.top3_position,
		        d.name
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
	"completed_at": true,
	"project_id":   true, "reminder_at": true, "surface": true, "top3_position": true,
}

func (db *DB) UpdateTask(id string, updates map[string]any) error {
	sets := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)

	for k, v := range updates {
		if !taskUpdateFields[k] {
			return fmt.Errorf("unsupported task update field: %s", k)
		}
		switch k {
		case "surface":
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("%w: surface must be a string", ErrInvalidEnum)
			}
			if err := validateSurface(s); err != nil {
				return err
			}
		case "top3_position":
			// null clears the star; JSON numbers arrive as float64.
			if v != nil {
				n, ok := v.(float64)
				if !ok || !validTop3(int64(n)) || n != float64(int64(n)) {
					return fmt.Errorf("%w: top3_position %v (must be 1, 2, 3, or null)", ErrInvalidEnum, v)
				}
			}
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
	if err := requireRowsAffected(result); err != nil {
		return err
	}

	// Bump project activity if the task belongs to a project.
	var projectID sql.NullString
	if err := db.conn.QueryRow(`SELECT project_id FROM tasks WHERE id = ?`, id).Scan(&projectID); err != nil {
		return fmt.Errorf("lookup task project_id: %w", err)
	}
	if projectID.Valid {
		return bumpProjectActivity(db.conn, projectID.String)
	}
	return nil
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
	var projectID sql.NullString
	err = tx.QueryRow(`SELECT goal_id, recurring, due_date, project_id FROM tasks WHERE id = ?`, id).Scan(&goalID, &recurring, &dueDate, &projectID)
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

	// Bump project activity when completing a project-assigned task.
	if projectID.Valid {
		if err := bumpProjectActivity(tx, projectID.String); err != nil {
			return fmt.Errorf("bump project activity: %w", err)
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
	if err := requireRowsAffected(result); err != nil {
		return err
	}

	// Bump project activity when deleting a project-assigned task.
	var projectID sql.NullString
	if err := db.conn.QueryRow(`SELECT project_id FROM tasks WHERE id = ?`, id).Scan(&projectID); err != nil {
		return fmt.Errorf("lookup task project_id: %w", err)
	}
	if projectID.Valid {
		return bumpProjectActivity(db.conn, projectID.String)
	}
	return nil
}

func (db *DB) TodaysTasks() ([]Task, error) {
	return db.queryTasks(
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring,
		        t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus,
		        t.created_at, t.completed_at,
		        t.project_id, t.reminder_at, t.surface, t.top3_position,
		        d.name
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
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring,
		        t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus,
		        t.created_at, t.completed_at,
		        t.project_id, t.reminder_at, t.surface, t.top3_position,
		        d.name
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
			&t.CreatedAt, &t.CompletedAt,
			&t.ProjectID, &t.ReminderAt, &t.Surface, &t.Top3Position,
			&domainName); err != nil {
			return nil, err
		}
		if domainName.Valid {
			t.DomainName = domainName.String
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
