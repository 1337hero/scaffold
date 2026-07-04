package db

import (
	"database/sql"
	"time"
)

type NotificationEntry struct {
	ID           string
	RefType      string
	RefID        string
	TriggerDate  string
	SentAt       string
	Message      string
	SuppressedAt sql.NullString
}

func (db *DB) LogNotification(refType, refID, message string) error {
	return db.LogNotificationTrigger(refType, refID, today(), message)
}

func (db *DB) LogNotificationTrigger(refType, refID, triggerDate, message string) error {
	_, err := db.conn.Exec(
		`INSERT INTO notification_log (id, ref_type, ref_id, trigger_date, sent_at, message) VALUES (?, ?, ?, ?, ?, ?)`,
		newID(), refType, refID, triggerDate, now(), message,
	)
	return err
}

func (db *DB) LastNotification(refType, refID string) (*NotificationEntry, error) {
	row := db.conn.QueryRow(
		`SELECT id, ref_type, ref_id, trigger_date, sent_at, message, suppressed_at FROM notification_log
		 WHERE ref_type = ? AND ref_id = ? ORDER BY sent_at DESC LIMIT 1`,
		refType, refID,
	)
	var e NotificationEntry
	var triggerDate, msg sql.NullString
	err := row.Scan(&e.ID, &e.RefType, &e.RefID, &triggerDate, &e.SentAt, &msg, &e.SuppressedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if triggerDate.Valid {
		e.TriggerDate = triggerDate.String
	}
	if msg.Valid {
		e.Message = msg.String
	}
	return &e, nil
}

func (db *DB) NotificationLog(refType, refID string) ([]NotificationEntry, error) {
	rows, err := db.conn.Query(
		`SELECT id, ref_type, ref_id, trigger_date, sent_at, message, suppressed_at
		 FROM notification_log
		 WHERE ref_type = ? AND ref_id = ?
		 ORDER BY sent_at DESC`,
		refType, refID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]NotificationEntry, 0)
	for rows.Next() {
		var e NotificationEntry
		var triggerDate, msg sql.NullString
		if err := rows.Scan(&e.ID, &e.RefType, &e.RefID, &triggerDate, &e.SentAt, &msg, &e.SuppressedAt); err != nil {
			return nil, err
		}
		if triggerDate.Valid {
			e.TriggerDate = triggerDate.String
		}
		if msg.Valid {
			e.Message = msg.String
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (db *DB) SuppressNotification(id string) error {
	return rowsAffectedOrNotFound(
		db.conn.Exec(`UPDATE notification_log SET suppressed_at = ? WHERE id = ?`, now(), id),
	)
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
		`SELECT ` + taskCols + `
		 FROM tasks t
		 LEFT JOIN domains d ON t.domain_id = d.id
		 WHERE t.status = 'pending'
		   AND t.notify = 1
		 ORDER BY t.priority ASC, t.due_date ASC`,
	)
}
