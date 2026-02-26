package db

import "time"

type WaitingThread struct {
	ThreadID      string
	Subject       string
	TaskID        *string
	Context       string
	MsgCount      int
	LastMessageID string
	CreatedAt     int64
}

func (db *DB) SaveWaitingThread(threadID, subject string, taskID *string, context string, msgCount int, lastMessageID string) error {
	var tid interface{}
	if taskID != nil {
		tid = *taskID
	}
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO gmail_waiting_threads(thread_id, subject, task_id, context, msg_count, last_message_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		threadID, subject, tid, context, msgCount, lastMessageID, time.Now().Unix(),
	)
	return err
}

func (db *DB) GetWaitingThreads() ([]WaitingThread, error) {
	rows, err := db.conn.Query(`
		SELECT thread_id, subject, task_id, context, msg_count, last_message_id, created_at
		FROM gmail_waiting_threads
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []WaitingThread
	for rows.Next() {
		var t WaitingThread
		if err := rows.Scan(&t.ThreadID, &t.Subject, &t.TaskID, &t.Context, &t.MsgCount, &t.LastMessageID, &t.CreatedAt); err != nil {
			return nil, err
		}
		threads = append(threads, t)
	}
	return threads, rows.Err()
}

func (db *DB) DeleteWaitingThread(threadID string) error {
	_, err := db.conn.Exec(`DELETE FROM gmail_waiting_threads WHERE thread_id = ?`, threadID)
	return err
}

func (db *DB) UpdateWaitingThreadMsgCount(threadID string, msgCount int) error {
	_, err := db.conn.Exec(`UPDATE gmail_waiting_threads SET msg_count = ? WHERE thread_id = ?`, msgCount, threadID)
	return err
}

func (db *DB) UpdateWaitingThreadLastMessageID(threadID, lastMessageID string) error {
	_, err := db.conn.Exec(`UPDATE gmail_waiting_threads SET last_message_id = ? WHERE thread_id = ?`, lastMessageID, threadID)
	return err
}
