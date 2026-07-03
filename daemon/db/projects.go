package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Project is a v2 organizational unit: project (has end date + milestones),
// area (ongoing, no end date), retainer (ongoing with monthly checklists).
type Project struct {
	ID             string
	Name           string
	Type           string // project|area|retainer
	Surface        string // life|business
	DomainID       sql.NullInt64
	Status         string // active|on_hold|completed|archived
	StartDate      sql.NullString
	EndDate        sql.NullString
	Description    sql.NullString
	LastActivityAt sql.NullString
	LastResetAt    sql.NullString
	CreatedAt      string
	UpdatedAt      string
}

// Milestone is a step toward completing a project.
type Milestone struct {
	ID        string
	ProjectID string
	Title     string
	Position  int
	Completed int // 0|1
	CreatedAt string
}

// Checklist is a named list of items on a project (or a template when IsTemplate=1).
type Checklist struct {
	ID         string
	ProjectID  sql.NullString // null for templates
	Title      string
	Items      string // JSON array of {text string, completed bool}
	IsTemplate int    // 0|1
	CreatedAt  string
}

// Activity is one log entry against a project (work done, hours billable, etc.).
type Activity struct {
	ID          string
	ProjectID   string
	Description string
	Hours       sql.NullFloat64
	CreatedAt   string
}

const projectCols = `id, name, type, surface, domain_id, status,
	start_date, end_date, description, last_activity_at, last_reset_at, created_at, updated_at`

const milestoneCols = `id, project_id, title, position, completed, created_at`

const checklistCols = `id, project_id, title, items, is_template, created_at`

const activityCols = `id, project_id, description, hours, created_at`

// --- Projects ---

func (db *DB) InsertProject(p Project) error {
	if p.ID == "" {
		p.ID = newID()
	}
	ts := now()
	if p.CreatedAt == "" {
		p.CreatedAt = ts
	}
	if p.UpdatedAt == "" {
		p.UpdatedAt = ts
	}
	if p.Type == "" {
		p.Type = "project"
	}
	if p.Surface == "" {
		p.Surface = "life"
	}
	if p.Status == "" {
		p.Status = "active"
	}

	// Validate enum fields.
	switch p.Type {
	case "project", "area", "retainer":
	default:
		return fmt.Errorf("invalid project type: %q (must be project|area|retainer)", p.Type)
	}
	switch p.Surface {
	case "life", "business":
	default:
		return fmt.Errorf("invalid surface: %q (must be life|business)", p.Surface)
	}
	switch p.Status {
	case "active", "on_hold", "completed", "archived":
	default:
		return fmt.Errorf("invalid status: %q (must be active|on_hold|completed|archived)", p.Status)
	}

	_, err := db.conn.Exec(
		`INSERT INTO projects (id, name, type, surface, domain_id, status,
		    start_date, end_date, description, last_activity_at, last_reset_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Type, p.Surface, p.DomainID, p.Status,
		p.StartDate, p.EndDate, p.Description, p.LastActivityAt, p.LastResetAt, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func scanProject(s interface {
	Scan(...any) error
}) (Project, error) {
	var p Project
	err := s.Scan(&p.ID, &p.Name, &p.Type, &p.Surface, &p.DomainID, &p.Status,
		&p.StartDate, &p.EndDate, &p.Description, &p.LastActivityAt, &p.LastResetAt, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (db *DB) GetProject(id string) (*Project, error) {
	row := db.conn.QueryRow(`SELECT `+projectCols+` FROM projects WHERE id = ?`, id)
	p, err := scanProject(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

var projectUpdateCols = map[string]bool{
	"name":        true,
	"type":        true,
	"surface":     true,
	"domain_id":   true,
	"status":      true,
	"start_date":  true,
	"end_date":    true,
	"description": true,
}

func (db *DB) UpdateProject(id string, updates map[string]any) error {
	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	var setClauses []string
	var args []any
	for col, val := range updates {
		if !projectUpdateCols[col] {
			return fmt.Errorf("unsupported update column: %s", col)
		}
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}

	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now(), id)

	q := fmt.Sprintf("UPDATE projects SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	return rowsAffectedOrNotFound(db.conn.Exec(q, args...))
}

// SuppressProject archives a project — it no longer appears in active lists.
func (db *DB) SuppressProject(id string) error {
	ts := now()
	return rowsAffectedOrNotFound(
		db.conn.Exec(`UPDATE projects SET status = 'archived', updated_at = ? WHERE id = ?`, ts, id),
	)
}

func (db *DB) queryProjects(query string, args ...any) ([]Project, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Project, 0)
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListProjects returns active (not archived) projects, optionally filtered.
// All filter pointers are AND'd; pass nil to skip a filter.
func (db *DB) ListProjects(typeFilter, surfaceFilter, statusFilter *string, domainID *int64) ([]Project, error) {
	q := `SELECT ` + projectCols + ` FROM projects WHERE 1=1`
	var args []any
	if typeFilter != nil {
		q += ` AND type = ?`
		args = append(args, *typeFilter)
	}
	if surfaceFilter != nil {
		q += ` AND surface = ?`
		args = append(args, *surfaceFilter)
	}
	if statusFilter != nil {
		q += ` AND status = ?`
		args = append(args, *statusFilter)
	} else {
		// Default: exclude archived
		q += ` AND status != 'archived'`
	}
	if domainID != nil {
		q += ` AND domain_id = ?`
		args = append(args, *domainID)
	}
	q += ` ORDER BY name`
	return db.queryProjects(q, args...)
}

// --- Milestones ---

func (db *DB) InsertMilestone(m Milestone) error {
	if m.ID == "" {
		m.ID = newID()
	}
	if m.CreatedAt == "" {
		m.CreatedAt = now()
	}
	_, err := db.conn.Exec(
		`INSERT INTO project_milestones (id, project_id, title, position, completed, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.ID, m.ProjectID, m.Title, m.Position, m.Completed, m.CreatedAt,
	)
	return err
}

func (db *DB) ListMilestones(projectID string) ([]Milestone, error) {
	rows, err := db.conn.Query(
		`SELECT `+milestoneCols+` FROM project_milestones WHERE project_id = ? ORDER BY position`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Milestone, 0)
	for rows.Next() {
		var m Milestone
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Title, &m.Position, &m.Completed, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MilestoneCompletion returns (completed, total) counts for a project's milestones.
func (db *DB) MilestoneCompletion(projectID string) (completed, total int, err error) {
	row := db.conn.QueryRow(
		`SELECT COALESCE(SUM(completed), 0), COUNT(*) FROM project_milestones WHERE project_id = ?`,
		projectID,
	)
	err = row.Scan(&completed, &total)
	return
}

var milestoneUpdateCols = map[string]bool{
	"title":     true,
	"position":  true,
	"completed": true,
}

func (db *DB) UpdateMilestone(id string, updates map[string]any) error {
	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	var setClauses []string
	var args []any
	for col, val := range updates {
		if !milestoneUpdateCols[col] {
			return fmt.Errorf("unsupported milestone update column: %s", col)
		}
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}

	args = append(args, id)
	q := fmt.Sprintf("UPDATE project_milestones SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	return rowsAffectedOrNotFound(db.conn.Exec(q, args...))
}

func (db *DB) DeleteMilestone(id string) error {
	return rowsAffectedOrNotFound(db.conn.Exec(`DELETE FROM project_milestones WHERE id = ?`, id))
}

// --- Checklists ---

func (db *DB) InsertChecklist(c Checklist) error {
	if c.ID == "" {
		c.ID = newID()
	}
	if c.CreatedAt == "" {
		c.CreatedAt = now()
	}
	_, err := db.conn.Exec(
		`INSERT INTO project_checklists (id, project_id, title, items, is_template, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.ProjectID, c.Title, c.Items, c.IsTemplate, c.CreatedAt,
	)
	return err
}

// CloneChecklist copies a template checklist into a project, resetting all items
// to not completed. Returns the newly created checklist.
func (db *DB) CloneChecklist(templateID, projectID string) (*Checklist, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin clone checklist tx: %w", err)
	}
	defer tx.Rollback()

	row := tx.QueryRow(
		`SELECT title, items FROM project_checklists WHERE id = ? AND is_template = 1`,
		templateID,
	)
	var title, items string
	if err := row.Scan(&title, &items); err == sql.ErrNoRows {
		return nil, nil // template not found
	} else if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}

	// Reset all items to not completed.
	items, err = resetChecklistItems(items)
	if err != nil {
		return nil, fmt.Errorf("reset checklist items: %w", err)
	}

	c := Checklist{
		ID:        newID(),
		ProjectID: sql.NullString{String: projectID, Valid: true},
		Title:     title,
		Items:     items,
		CreatedAt: now(),
	}

	if _, err := tx.Exec(
		`INSERT INTO project_checklists (id, project_id, title, items, is_template, created_at)
		 VALUES (?, ?, ?, ?, 0, ?)`,
		c.ID, c.ProjectID, c.Title, c.Items, c.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("insert cloned checklist: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit clone checklist: %w", err)
	}
	return &c, nil
}

// resetChecklistItems parses a JSON array of checklist items and sets all
// completed fields to false.
func resetChecklistItems(jsonItems string) (string, error) {
	// Simple approach: parse and rebuild.
	var items []struct {
		Text      string `json:"text"`
		Completed bool   `json:"completed"`
	}
	if err := json.Unmarshal([]byte(jsonItems), &items); err != nil {
		return "", err
	}
	for i := range items {
		items[i].Completed = false
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var checklistUpdateCols = map[string]bool{
	"title": true,
	"items": true,
}

func (db *DB) UpdateChecklist(id string, updates map[string]any) error {
	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	var setClauses []string
	var args []any
	for col, val := range updates {
		if !checklistUpdateCols[col] {
			return fmt.Errorf("unsupported checklist update column: %s", col)
		}
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}

	args = append(args, id)
	q := fmt.Sprintf("UPDATE project_checklists SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	return rowsAffectedOrNotFound(db.conn.Exec(q, args...))
}

func (db *DB) ListChecklists(projectID string) ([]Checklist, error) {
	rows, err := db.conn.Query(
		`SELECT `+checklistCols+` FROM project_checklists WHERE project_id = ? AND is_template = 0 ORDER BY created_at`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Checklist, 0)
	for rows.Next() {
		var c Checklist
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Title, &c.Items, &c.IsTemplate, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *DB) ListChecklistTemplates() ([]Checklist, error) {
	rows, err := db.conn.Query(
		`SELECT `+checklistCols+` FROM project_checklists WHERE is_template = 1 ORDER BY title`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Checklist, 0)
	for rows.Next() {
		var c Checklist
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Title, &c.Items, &c.IsTemplate, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// --- Activity ---

func (db *DB) InsertActivity(a Activity) error {
	if a.ID == "" {
		a.ID = newID()
	}
	if a.CreatedAt == "" {
		a.CreatedAt = now()
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin insert activity tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO project_activity (id, project_id, description, hours, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		a.ID, a.ProjectID, a.Description, a.Hours, a.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}

	// Bump last_activity_at on the project — only advance, never regress.
	today := time.Now().In(localLocation).Format("2006-01-02")
	if _, err := tx.Exec(
		`UPDATE projects
		   SET last_activity_at = CASE
		         WHEN last_activity_at IS NULL OR ? > last_activity_at THEN ?
		         ELSE last_activity_at END,
		       updated_at = ?
		 WHERE id = ?`,
		today, today, now(), a.ProjectID,
	); err != nil {
		return fmt.Errorf("update last_activity_at: %w", err)
	}

	return tx.Commit()
}

func (db *DB) ListActivity(projectID string) ([]Activity, error) {
	rows, err := db.conn.Query(
		`SELECT `+activityCols+` FROM project_activity WHERE project_id = ? ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Activity, 0)
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Description, &a.Hours, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// --- Slipping ---

// ProjectsSlipping returns active projects with no activity in 7+ days,
// excluding areas (checked by AreasSlipping).
func (db *DB) ProjectsSlipping() ([]Project, error) {
	return db.queryProjects(
		`SELECT `+projectCols+` FROM projects
		 WHERE status = 'active' AND type = 'project'
		   AND (last_activity_at IS NULL OR julianday(?) - julianday(last_activity_at) >= 7)
		 ORDER BY last_activity_at NULLS FIRST`,
		today(),
	)
}

// AreasSlipping returns active areas with no tasks touched in 14+ days.
func (db *DB) AreasSlipping() ([]Project, error) {
	return db.queryProjects(
		`SELECT `+projectCols+` FROM projects
		 WHERE status = 'active' AND type = 'area'
		   AND (last_activity_at IS NULL OR julianday(?) - julianday(last_activity_at) >= 14)
		 ORDER BY last_activity_at NULLS FIRST`,
		today(),
	)
}

// ResetRetainerChecklists finds active retainer projects whose checklists haven't
// been reset this calendar month and resets all checklist items to not completed.
// Returns the number of retainers whose checklists were reset.
//
// Uses last_reset_at as its own cursor (independent of last_activity_at) so
// logging billable hours to a retainer doesn't block the month-end reset.
func (db *DB) ResetRetainerChecklists() (int, error) {
	rows, err := db.conn.Query(
		`SELECT id FROM projects
		 WHERE type = 'retainer' AND status = 'active'
		   AND (last_reset_at IS NULL
		        OR strftime('%Y-%m', 'now') != strftime('%Y-%m', last_reset_at))`,
	)
	if err != nil {
		return 0, fmt.Errorf("query retainers for reset: %w", err)
	}
	defer rows.Close()

	var projectIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		projectIDs = append(projectIDs, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	reset := 0
	for _, pid := range projectIDs {
		ok, err := db.resetRetainer(pid)
		if err != nil {
			return reset, fmt.Errorf("reset retainer %s: %w", pid, err)
		}
		if ok {
			reset++
		}
	}
	return reset, nil
}

// resetRetainer resets all non-template checklists on a retainer project by
// setting every item's completed to false. Stamps last_reset_at so the same
// month doesn't trigger again. Returns true if any checklist was touched.
func (db *DB) resetRetainer(projectID string) (bool, error) {
	// Find non-template checklists for this project.
	rows, err := db.conn.Query(
		`SELECT id, items FROM project_checklists
		 WHERE project_id = ? AND is_template = 0`,
		projectID,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	type cl struct{ id, items string }
	var checklists []cl
	for rows.Next() {
		var c cl
		if err := rows.Scan(&c.id, &c.items); err != nil {
			return false, err
		}
		checklists = append(checklists, c)
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	if len(checklists) == 0 {
		return false, nil
	}

	// Reset items in-place.
	for _, c := range checklists {
		resetItems, err := resetChecklistItems(c.items)
		if err != nil {
			return false, fmt.Errorf("reset items for %s: %w", c.id, err)
		}
		if _, err := db.conn.Exec(
			`UPDATE project_checklists SET items = ? WHERE id = ?`,
			resetItems, c.id,
		); err != nil {
			return false, fmt.Errorf("update checklist %s: %w", c.id, err)
		}
	}

	// Stamp last_reset_at only — don't touch last_activity_at.
	ts := today()
	if _, err := db.conn.Exec(
		`UPDATE projects SET last_reset_at = ?, updated_at = ? WHERE id = ?`,
		ts, now(), projectID,
	); err != nil {
		return false, err
	}

	return true, nil
}

// resetChecklistItems is above