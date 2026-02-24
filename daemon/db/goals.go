package db

import (
	"database/sql"
	"fmt"
	"strings"
)

type Goal struct {
	ID           string
	Title        string
	DomainID     sql.NullInt64
	Context      sql.NullString
	DueDate      sql.NullString
	Type         string
	TargetValue  sql.NullFloat64
	CurrentValue sql.NullFloat64
	HabitType    sql.NullString
	ScheduleDays sql.NullString
	Notify       int
	Status       string
	CreatedAt    string
	CompletedAt  sql.NullString
}

type GoalWithProgress struct {
	Goal
	DomainName     string
	TotalTasks     int
	CompletedTasks int
	Progress       float64
}

func (db *DB) InsertGoal(g Goal) error {
	if g.ID == "" {
		g.ID = newID()
	}
	if g.CreatedAt == "" {
		g.CreatedAt = now()
	}
	if g.Status == "" {
		g.Status = "active"
	}
	if g.Type == "" {
		g.Type = "binary"
	}

	_, err := db.conn.Exec(
		`INSERT INTO goals (id, title, domain_id, context, due_date, type, target_value, current_value, habit_type, schedule_days, notify, status, created_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Title, g.DomainID, g.Context, g.DueDate, g.Type,
		g.TargetValue, g.CurrentValue, g.HabitType, g.ScheduleDays,
		g.Notify, g.Status, g.CreatedAt, g.CompletedAt,
	)
	return err
}

func (db *DB) GetGoal(id string) (*Goal, error) {
	row := db.conn.QueryRow(
		`SELECT id, title, domain_id, context, due_date, type, target_value, current_value, habit_type, schedule_days, notify, status, created_at, completed_at
		 FROM goals WHERE id = ?`, id,
	)
	var g Goal
	err := row.Scan(
		&g.ID, &g.Title, &g.DomainID, &g.Context, &g.DueDate, &g.Type,
		&g.TargetValue, &g.CurrentValue, &g.HabitType, &g.ScheduleDays,
		&g.Notify, &g.Status, &g.CreatedAt, &g.CompletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (db *DB) ListGoals(domainID *int, status string) ([]Goal, error) {
	if status == "" {
		status = "active"
	}

	q := `SELECT id, title, domain_id, context, due_date, type, target_value, current_value, habit_type, schedule_days, notify, status, created_at, completed_at
	      FROM goals WHERE status = ?`
	args := []any{status}

	if domainID != nil {
		q += ` AND domain_id = ?`
		args = append(args, *domainID)
	}

	q += ` ORDER BY created_at DESC`
	return db.queryGoals(q, args...)
}

func (db *DB) UpdateGoal(id string, updates map[string]any) error {
	allowed := map[string]bool{
		"title": true, "domain_id": true, "context": true, "due_date": true,
		"type": true, "target_value": true, "current_value": true,
		"habit_type": true, "schedule_days": true, "notify": true,
		"status": true, "completed_at": true,
	}

	sets := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)

	for k, v := range updates {
		if !allowed[k] {
			return fmt.Errorf("unsupported update key: %s", k)
		}
		sets = append(sets, k+" = ?")
		args = append(args, v)
	}

	if len(sets) == 0 {
		return nil
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE goals SET %s WHERE id = ?", strings.Join(sets, ", "))
	result, err := db.conn.Exec(query, args...)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) SoftDeleteGoal(id string) error {
	// cascade: soft-delete non-done child tasks first
	_, err := db.conn.Exec(
		`UPDATE tasks SET status = 'deleted' WHERE goal_id = ? AND status != 'done' AND status != 'deleted'`,
		id,
	)
	if err != nil {
		return err
	}
	result, err := db.conn.Exec(`UPDATE goals SET status = 'abandoned' WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) GoalsByDomain(domainID int) ([]Goal, error) {
	return db.queryGoals(
		`SELECT id, title, domain_id, context, due_date, type, target_value, current_value, habit_type, schedule_days, notify, status, created_at, completed_at
		 FROM goals WHERE domain_id = ? AND status = 'active' ORDER BY created_at DESC`, domainID,
	)
}

func (db *DB) GoalsWithProgress(domainID *int) ([]GoalWithProgress, error) {
	q := `SELECT g.id, g.title, g.domain_id, g.context, g.due_date, g.type,
	             g.target_value, g.current_value, g.habit_type, g.schedule_days,
	             g.notify, g.status, g.created_at, g.completed_at,
	             d.name,
	             COALESCE(t.total, 0), COALESCE(t.done, 0)
	      FROM goals g
	      LEFT JOIN domains d ON g.domain_id = d.id
	      LEFT JOIN (
	        SELECT goal_id, COUNT(*) AS total, SUM(CASE WHEN status = 'done' THEN 1 ELSE 0 END) AS done
	        FROM tasks GROUP BY goal_id
	      ) t ON t.goal_id = g.id
	      WHERE g.status = 'active'`
	args := []any{}

	if domainID != nil {
		q += ` AND g.domain_id = ?`
		args = append(args, *domainID)
	}

	q += ` ORDER BY g.created_at DESC`

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]GoalWithProgress, 0)
	for rows.Next() {
		var gp GoalWithProgress
		var domainName sql.NullString
		if err := rows.Scan(
			&gp.ID, &gp.Title, &gp.DomainID, &gp.Context, &gp.DueDate, &gp.Type,
			&gp.TargetValue, &gp.CurrentValue, &gp.HabitType, &gp.ScheduleDays,
			&gp.Notify, &gp.Status, &gp.CreatedAt, &gp.CompletedAt,
			&domainName,
			&gp.TotalTasks, &gp.CompletedTasks,
		); err != nil {
			return nil, err
		}
		if domainName.Valid {
			gp.DomainName = domainName.String
		}
		if gp.Type == "measurable" && gp.TargetValue.Valid && gp.TargetValue.Float64 > 0 {
			gp.Progress = gp.CurrentValue.Float64 / gp.TargetValue.Float64
			if gp.Progress > 1.0 {
				gp.Progress = 1.0
			}
		} else if gp.TotalTasks > 0 {
			gp.Progress = float64(gp.CompletedTasks) / float64(gp.TotalTasks)
		}
		out = append(out, gp)
	}
	return out, rows.Err()
}

func (db *DB) queryGoals(query string, args ...any) ([]Goal, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Goal, 0)
	for rows.Next() {
		var g Goal
		if err := rows.Scan(
			&g.ID, &g.Title, &g.DomainID, &g.Context, &g.DueDate, &g.Type,
			&g.TargetValue, &g.CurrentValue, &g.HabitType, &g.ScheduleDays,
			&g.Notify, &g.Status, &g.CreatedAt, &g.CompletedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
