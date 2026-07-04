package db

import (
	"database/sql"
	"fmt"
)

// --- Top 3 ---

// GetTop3Tasks returns pending tasks starred into the Top 3, in position order.
func (db *DB) GetTop3Tasks(surface *string) ([]Task, error) {
	q := `SELECT ` + taskCols + `
	 FROM tasks t LEFT JOIN domains d ON t.domain_id = d.id
	 WHERE t.status = 'pending' AND t.top3_position IS NOT NULL`
	var args []any
	if surface != nil {
		q += ` AND t.surface = ?`
		args = append(args, *surface)
	}
	q += ` ORDER BY t.top3_position`
	return db.queryTasks(q, args...)
}

// SetTop3Tasks clears all existing top3 positions and stars the given task IDs
// as positions 1..n (max 3). An empty list just clears the Top 3. An unknown
// or non-pending task ID fails the whole transaction with sql.ErrNoRows —
// starring a done task would be a silent no-op in GetTop3Tasks otherwise.
func (db *DB) SetTop3Tasks(taskIDs []string) error {
	if len(taskIDs) > 3 {
		return fmt.Errorf("%w: top3 accepts at most 3 task ids, got %d", ErrInvalidEnum, len(taskIDs))
	}
	seen := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		if seen[id] {
			return fmt.Errorf("%w: duplicate task id %q", ErrInvalidEnum, id)
		}
		seen[id] = true
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin set top3 tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE tasks SET top3_position = NULL WHERE top3_position IS NOT NULL`); err != nil {
		return fmt.Errorf("clear top3: %w", err)
	}

	for i, id := range taskIDs {
		res, err := tx.Exec(
			`UPDATE tasks SET top3_position = ? WHERE id = ? AND status = 'pending'`, i+1, id,
		)
		if err != nil {
			return fmt.Errorf("set top3 position %d: %w", i+1, err)
		}
		if err := rowsAffectedOrNotFound(res, nil); err != nil {
			return fmt.Errorf("task %q not found or not pending: %w", id, err)
		}
	}

	return tx.Commit()
}

// --- Slipping ---

// SlippingTask is an overdue pending task with its staleness.
type SlippingTask struct {
	Task
	DaysOverdue int
}

// SlippingTasks returns pending tasks past their due date (oldest first).
func (db *DB) SlippingTasks(surface *string) ([]SlippingTask, error) {
	q := `SELECT ` + taskCols + `,
	        CAST(julianday(?) - julianday(t.due_date) AS INTEGER)
	 FROM tasks t LEFT JOIN domains d ON t.domain_id = d.id
	 WHERE t.status = 'pending' AND t.due_date IS NOT NULL AND t.due_date < ?`
	args := []any{today(), today()}
	if surface != nil {
		q += ` AND t.surface = ?`
		args = append(args, *surface)
	}
	q += ` ORDER BY t.due_date ASC`

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SlippingTask, 0)
	for rows.Next() {
		var st SlippingTask
		var domainName sql.NullString
		if err := rows.Scan(&st.ID, &st.Title, &st.DomainID, &st.GoalID, &st.Context, &st.DueDate,
			&st.Recurring, &st.Priority, &st.Status, &st.MicroSteps, &st.Notify, &st.Position, &st.IsFocus,
			&st.CreatedAt, &st.CompletedAt,
			&st.ProjectID, &st.ReminderAt, &st.Surface, &st.Top3Position,
			&domainName, &st.DaysOverdue); err != nil {
			return nil, err
		}
		if domainName.Valid {
			st.DomainName = domainName.String
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// Slipping groups every category of stale item for the Today page.
type Slipping struct {
	Projects []Project
	Tasks    []SlippingTask
	People   []Person
	Areas    []Project
}

// SlippingAll runs all four slipping detectors, optionally scoped to a surface.
func (db *DB) SlippingAll(surface *string) (*Slipping, error) {
	projects, err := db.ProjectsSlipping(surface)
	if err != nil {
		return nil, fmt.Errorf("slipping projects: %w", err)
	}
	tasks, err := db.SlippingTasks(surface)
	if err != nil {
		return nil, fmt.Errorf("slipping tasks: %w", err)
	}
	people, err := db.PeopleSlipping(surface)
	if err != nil {
		return nil, fmt.Errorf("slipping people: %w", err)
	}
	areas, err := db.AreasSlipping(surface)
	if err != nil {
		return nil, fmt.Errorf("slipping areas: %w", err)
	}
	return &Slipping{Projects: projects, Tasks: tasks, People: people, Areas: areas}, nil
}

// --- Notifications ---

// TodayNotification is one actionable item for the Today view.
type TodayNotification struct {
	Type      string `json:"type"` // reminder|birthday|follow_up|review
	RefID     string `json:"ref_id"`
	Title     string `json:"title"`
	Detail    string `json:"detail,omitempty"`
	Date      string `json:"date,omitempty"`
	DaysUntil *int   `json:"days_until,omitempty"` // birthdays only
}

// TodayNotifications assembles reminders, birthdays (3-day window), follow-ups
// due, and notes due for review. Reminders respect the surface filter; the
// rest are people/notes-level and surface-agnostic — a birthday matters no
// matter which view you're in.
func (db *DB) TodayNotifications(surface *string) ([]TodayNotification, error) {
	out := make([]TodayNotification, 0)

	// Reminders: reminder_at has arrived and no notification has been logged
	// since it was set (re-setting a later reminder re-arms it).
	q := `SELECT t.id, t.title, t.reminder_at FROM tasks t
	 WHERE t.status = 'pending' AND t.reminder_at IS NOT NULL AND t.reminder_at <= ?
	   AND NOT EXISTS (
	       SELECT 1 FROM notification_log nl
	       WHERE nl.ref_type = 'reminder' AND nl.ref_id = t.id AND nl.sent_at >= t.reminder_at)`
	args := []any{now()}
	if surface != nil {
		q += ` AND t.surface = ?`
		args = append(args, *surface)
	}
	q += ` ORDER BY t.reminder_at`

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("reminders due: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, title, reminderAt string
		if err := rows.Scan(&id, &title, &reminderAt); err != nil {
			return nil, err
		}
		out = append(out, TodayNotification{Type: "reminder", RefID: id, Title: title, Date: reminderAt})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Birthdays and anniversaries within 3 days.
	hits, err := db.UpcomingBirthdays(3)
	if err != nil {
		return nil, fmt.Errorf("upcoming birthdays: %w", err)
	}
	for _, h := range hits {
		detail := h.Kind
		if h.Relationship != "" {
			detail = h.Relationship + " · " + h.Kind
		}
		out = append(out, TodayNotification{
			Type:      "birthday",
			RefID:     h.PersonID,
			Title:     h.Name,
			Detail:    detail,
			Date:      h.Date,
			DaysUntil: &h.DaysUntil,
		})
	}

	// Follow-ups due.
	followUps, err := db.FollowUpsDue()
	if err != nil {
		return nil, fmt.Errorf("follow-ups due: %w", err)
	}
	for _, f := range followUps {
		n := TodayNotification{Type: "follow_up", RefID: f.ID, Title: f.FollowUp.String, Detail: f.Summary, Date: f.FollowUpDate.String}
		if n.Title == "" {
			n.Title = f.Summary
			n.Detail = ""
		}
		out = append(out, n)
	}

	// Notes due for review.
	noteRows, err := db.conn.Query(
		`SELECT id, title, review_at FROM notes
		  WHERE suppressed_at IS NULL AND review_at IS NOT NULL AND review_at <= ?
		  ORDER BY review_at`,
		today(),
	)
	if err != nil {
		return nil, fmt.Errorf("notes due for review: %w", err)
	}
	defer noteRows.Close()
	for noteRows.Next() {
		var id, title, reviewAt string
		if err := noteRows.Scan(&id, &title, &reviewAt); err != nil {
			return nil, err
		}
		out = append(out, TodayNotification{Type: "review", RefID: id, Title: title, Date: reviewAt})
	}
	return out, noteRows.Err()
}
