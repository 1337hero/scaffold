package db

import (
	"database/sql"
	"time"
)

type NotificationEntry struct {
	ID      string
	RefType string
	RefID   string
	SentAt  string
	Message string
}

func (db *DB) LogNotification(refType, refID, message string) error {
	_, err := db.conn.Exec(
		`INSERT INTO notification_log (id, ref_type, ref_id, sent_at, message) VALUES (?, ?, ?, ?, ?)`,
		newID(), refType, refID, now(), message,
	)
	return err
}

func (db *DB) LastNotification(refType, refID string) (*NotificationEntry, error) {
	row := db.conn.QueryRow(
		`SELECT id, ref_type, ref_id, sent_at, message FROM notification_log
		 WHERE ref_type = ? AND ref_id = ? ORDER BY sent_at DESC LIMIT 1`,
		refType, refID,
	)
	var e NotificationEntry
	var msg sql.NullString
	err := row.Scan(&e.ID, &e.RefType, &e.RefID, &e.SentAt, &msg)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if msg.Valid {
		e.Message = msg.String
	}
	return &e, nil
}

func (db *DB) NotificationSentSince(refType, refID string, since time.Time) (bool, error) {
	entry, err := db.LastNotification(refType, refID)
	if err != nil {
		return false, err
	}
	if entry == nil {
		return false, nil
	}
	sentAt, err := time.Parse(time.RFC3339, entry.SentAt)
	if err != nil {
		return false, nil
	}
	return sentAt.After(since), nil
}

func (db *DB) NotifiableTasks(today string) ([]Task, error) {
	return db.queryTasks(
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring,
		        t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus,
		        t.source, t.source_ref, t.created_at, t.completed_at, d.name
		 FROM tasks t
		 LEFT JOIN domains d ON t.domain_id = d.id
		 LEFT JOIN goals g ON t.goal_id = g.id
		 WHERE t.status = 'pending'
		   AND (t.notify = 1 OR g.notify = 1)
		 ORDER BY t.priority ASC, t.due_date ASC`,
	)
}
