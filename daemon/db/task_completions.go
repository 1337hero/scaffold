package db

import (
	"database/sql"
	"time"
)

type TaskCompletion struct {
	ID          string
	TaskID      string
	GoalID      sql.NullString
	CompletedAt string
}

func (db *DB) LogCompletion(taskID, goalID string) error {
	var gid sql.NullString
	if goalID != "" {
		gid = sql.NullString{String: goalID, Valid: true}
	}

	_, err := db.conn.Exec(
		`INSERT INTO task_completions (id, task_id, goal_id, completed_at) VALUES (?, ?, ?, ?)`,
		newID(), taskID, gid, now(),
	)
	return err
}

func (db *DB) CompletionsSince(goalID string, since time.Time) ([]TaskCompletion, error) {
	rows, err := db.conn.Query(
		`SELECT id, task_id, goal_id, completed_at
		 FROM task_completions
		 WHERE goal_id = ? AND completed_at >= ?
		 ORDER BY completed_at DESC`,
		goalID, since.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TaskCompletion
	for rows.Next() {
		var c TaskCompletion
		if err := rows.Scan(&c.ID, &c.TaskID, &c.GoalID, &c.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *DB) CompletionCount(goalID string) (int, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM task_completions WHERE goal_id = ?`, goalID,
	).Scan(&count)
	return count, err
}
