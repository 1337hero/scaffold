package db

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	DriftThreshold   = 10
	NeglectThreshold = 25
)

type Domain struct {
	ID            int
	Name          string
	Importance    int
	LastTouchedAt string
	StatusLine    sql.NullString
	Briefing      sql.NullString
	CreatedAt     string
}

type DomainDetail struct {
	Domain
	DeskItems      []DeskItem
	OpenCaptures   []Capture
	RecentMemories []Memory
}

type DomainDrift struct {
	Domain
	DaysSinceTouch int
	DriftScore     float64
	State          string
	Label          string
	OpenTaskCount  int
}

var defaultDomains = []struct {
	Name       string
	Importance int
}{
	{"Work/Business", 5},
	{"Personal Projects", 5},
	{"Homelife", 5},
	{"Relationships", 5},
	{"Personal Development", 5},
	{"Finances", 5},
	{"Hobbies", 3},
}

func (db *DB) SeedDefaultDomains() error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin seed defaults tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, d := range defaultDomains {
		ts := now()
		_, err := tx.Exec(
			`INSERT OR IGNORE INTO domains (name, importance, last_touched_at, created_at) VALUES (?, ?, ?, ?)`,
			d.Name, d.Importance, ts, ts,
		)
		if err != nil {
			return fmt.Errorf("seed domain %q: %w", d.Name, err)
		}
	}

	legacyDomainToDefault := map[string]string{
		"1337 Hero":  "Work/Business",
		"Health":     "Personal Development",
		"Family":     "Relationships",
		"Faith":      "Personal Development",
		"Shenandoah": "Personal Projects",
		"Vera":       "Personal Projects",
		"Homelab":    "Personal Projects",
	}

	for legacy, mapped := range legacyDomainToDefault {
		legacyID, err := resolveDomainIDTx(tx, legacy)
		if err != nil {
			return fmt.Errorf("resolve legacy domain %q: %w", legacy, err)
		}
		if legacyID == nil {
			continue
		}

		mappedID, err := resolveDomainIDTx(tx, mapped)
		if err != nil {
			return fmt.Errorf("resolve mapped domain %q: %w", mapped, err)
		}
		if mappedID == nil {
			return fmt.Errorf("mapped domain %q missing during seed", mapped)
		}
		if *legacyID == *mappedID {
			continue
		}

		for _, table := range []string{"memories", "captures", "desk"} {
			if _, err := tx.Exec(
				fmt.Sprintf("UPDATE %s SET domain_id = ? WHERE domain_id = ?", table),
				*mappedID, *legacyID,
			); err != nil {
				return fmt.Errorf("reassign %s domain %q->%q: %w", table, legacy, mapped, err)
			}
		}

		if _, err := tx.Exec(`DELETE FROM domains WHERE id = ?`, *legacyID); err != nil {
			return fmt.Errorf("delete legacy domain %q: %w", legacy, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit seed defaults tx: %w", err)
	}
	return nil
}

func resolveDomainIDTx(tx *sql.Tx, name string) (*int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	var id int
	err := tx.QueryRow(
		`SELECT id FROM domains WHERE LOWER(name) = LOWER(?)`, name,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (db *DB) ListDomains() ([]Domain, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, importance, last_touched_at, status_line, briefing, created_at
		 FROM domains ORDER BY importance DESC, name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Name, &d.Importance, &d.LastTouchedAt, &d.StatusLine, &d.Briefing, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (db *DB) GetDomain(id int) (*Domain, error) {
	row := db.conn.QueryRow(
		`SELECT id, name, importance, last_touched_at, status_line, briefing, created_at
		 FROM domains WHERE id = ?`, id,
	)
	var d Domain
	err := row.Scan(&d.ID, &d.Name, &d.Importance, &d.LastTouchedAt, &d.StatusLine, &d.Briefing, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (db *DB) CreateDomain(name string, importance int) (*Domain, error) {
	ts := now()
	result, err := db.conn.Exec(
		`INSERT INTO domains (name, importance, last_touched_at, created_at) VALUES (?, ?, ?, ?)`,
		name, importance, ts, ts,
	)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Domain{
		ID:            int(id),
		Name:          name,
		Importance:    importance,
		LastTouchedAt: ts,
		CreatedAt:     ts,
	}, nil
}

func (db *DB) UpdateDomain(id int, statusLine, briefing *string, importance *int) error {
	sets := make([]string, 0, 3)
	args := make([]any, 0, 4)

	if statusLine != nil {
		sets = append(sets, "status_line = ?")
		args = append(args, *statusLine)
	}
	if briefing != nil {
		sets = append(sets, "briefing = ?")
		args = append(args, *briefing)
	}
	if importance != nil {
		sets = append(sets, "importance = ?")
		args = append(args, *importance)
	}

	if len(sets) == 0 {
		return nil
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE domains SET %s WHERE id = ?", strings.Join(sets, ", "))
	result, err := db.conn.Exec(query, args...)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (db *DB) TouchDomain(id int) error {
	_, err := db.conn.Exec(`UPDATE domains SET last_touched_at = ? WHERE id = ?`, now(), id)
	return err
}

func (db *DB) DomainDeskItems(domainID int) ([]DeskItem, error) {
	return db.queryDesk(
		`SELECT id, memory_id, title, position, status, micro_steps, date, created_at, completed_at, domain_id
		 FROM desk WHERE domain_id = ? AND status = 'active' ORDER BY position ASC`, domainID,
	)
}

func (db *DB) DomainOpenCaptures(domainID int) ([]Capture, error) {
	return db.queryCaptures(
		`SELECT id, raw, source, processed, triage_action, memory_id, created_at, confirmed, domain_id
		 FROM captures WHERE domain_id = ? AND processed = 0 ORDER BY created_at DESC`, domainID,
	)
}

func (db *DB) DomainRecentMemories(domainID int, limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at, domain_id
		 FROM memories WHERE domain_id = ? AND suppressed_at IS NULL ORDER BY created_at DESC LIMIT ?`,
		domainID, limit,
	)
}

func (db *DB) DomainDetailByID(id int) (*DomainDetail, error) {
	domain, err := db.GetDomain(id)
	if err != nil {
		return nil, err
	}
	if domain == nil {
		return nil, nil
	}

	deskItems, err := db.DomainDeskItems(id)
	if err != nil {
		return nil, fmt.Errorf("domain desk items: %w", err)
	}

	captures, err := db.DomainOpenCaptures(id)
	if err != nil {
		return nil, fmt.Errorf("domain open captures: %w", err)
	}

	memories, err := db.DomainRecentMemories(id, 10)
	if err != nil {
		return nil, fmt.Errorf("domain recent memories: %w", err)
	}

	return &DomainDetail{
		Domain:         *domain,
		DeskItems:      deskItems,
		OpenCaptures:   captures,
		RecentMemories: memories,
	}, nil
}

func (db *DB) DumpItems(limit int) ([]Capture, error) {
	return db.queryCaptures(
		`SELECT id, raw, source, processed, triage_action, memory_id, created_at, confirmed, domain_id
		 FROM captures WHERE domain_id IS NULL ORDER BY created_at DESC LIMIT ?`, limit,
	)
}

func (db *DB) DumpMemories(limit int) ([]Memory, error) {
	return db.queryMemories(
		`SELECT id, type, content, title, importance, source, tags, created_at, updated_at, accessed_at, access_count, archived, suppressed_at, domain_id
		 FROM memories WHERE domain_id IS NULL AND suppressed_at IS NULL ORDER BY created_at DESC LIMIT ?`, limit,
	)
}

func (db *DB) CountDumpItems() (int, error) {
	var capturesCount int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM captures WHERE domain_id IS NULL`).Scan(&capturesCount); err != nil {
		return 0, err
	}

	var memoriesCount int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM memories WHERE domain_id IS NULL AND suppressed_at IS NULL`).Scan(&memoriesCount); err != nil {
		return 0, err
	}

	return capturesCount + memoriesCount, nil
}

func (db *DB) TouchDomainByMemory(memoryID string) error {
	var domainID sql.NullInt64
	err := db.conn.QueryRow(`SELECT domain_id FROM memories WHERE id = ?`, memoryID).Scan(&domainID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("lookup domain by memory %s: %w", memoryID, err)
	}
	if !domainID.Valid {
		return nil
	}
	if err := db.TouchDomain(int(domainID.Int64)); err != nil {
		return fmt.Errorf("touch domain by memory %s: %w", memoryID, err)
	}
	return nil
}

func (db *DB) TouchDomainByDesk(deskID string) error {
	var domainID sql.NullInt64
	err := db.conn.QueryRow(`SELECT domain_id FROM desk WHERE id = ?`, deskID).Scan(&domainID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("lookup domain by desk %s: %w", deskID, err)
	}
	if !domainID.Valid {
		return nil
	}
	if err := db.TouchDomain(int(domainID.Int64)); err != nil {
		return fmt.Errorf("touch domain by desk %s: %w", deskID, err)
	}
	return nil
}

func (db *DB) TouchDomainByCapture(captureID string) error {
	var domainID sql.NullInt64
	err := db.conn.QueryRow(`SELECT domain_id FROM captures WHERE id = ?`, captureID).Scan(&domainID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("lookup domain by capture %s: %w", captureID, err)
	}
	if !domainID.Valid {
		return nil
	}
	if err := db.TouchDomain(int(domainID.Int64)); err != nil {
		return fmt.Errorf("touch domain by capture %s: %w", captureID, err)
	}
	return nil
}

func (db *DB) ResolveDomainID(name string) (*int, error) {
	name = canonicalDomainName(name)
	if name == "" {
		return nil, nil
	}
	var id int
	err := db.conn.QueryRow(
		`SELECT id FROM domains WHERE LOWER(name) = LOWER(?)`, name,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func canonicalDomainName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	aliases := map[string]string{
		"work business":         "Work/Business",
		"work/business":         "Work/Business",
		"personal project":      "Personal Projects",
		"personal projects":     "Personal Projects",
		"home life":             "Homelife",
		"homelife":              "Homelife",
		"relationship":          "Relationships",
		"relationships":         "Relationships",
		"personal development":  "Personal Development",
		"health":                "Personal Development",
		"faith":                 "Personal Development",
		"finance":               "Finances",
		"finances":              "Finances",
		"finanaces":             "Finances",
		"hobby":                 "Hobbies",
		"hobbies":               "Hobbies",
		"1337 hero":             "Work/Business",
		"shenandoah":            "Personal Projects",
		"vera":                  "Personal Projects",
		"homelab":               "Personal Projects",
		"family":                "Relationships",
	}
	if mapped, ok := aliases[lower]; ok {
		return mapped
	}
	return trimmed
}

func (db *DB) ComputeDriftStates() ([]DomainDrift, error) {
	domains, err := db.ListDomains()
	if err != nil {
		return nil, err
	}

	nowTime := time.Now().UTC()

	var out []DomainDrift
	for _, d := range domains {
		touched, err := time.Parse(time.RFC3339, d.LastTouchedAt)
		if err != nil {
			touched = nowTime
		}
		daysSince := int(math.Floor(nowTime.Sub(touched).Hours() / 24))
		if daysSince < 0 {
			daysSince = 0
		}

		driftScore := float64(d.Importance) * float64(daysSince)

		var openTasks int
		if err := db.conn.QueryRow(
			`SELECT COUNT(*) FROM desk WHERE domain_id = ? AND status = 'active'`, d.ID,
		).Scan(&openTasks); err != nil {
			return nil, fmt.Errorf("count open tasks for domain %d: %w", d.ID, err)
		}

		state, label := classifyDrift(d.Importance, daysSince, driftScore, openTasks)

		out = append(out, DomainDrift{
			Domain:         d,
			DaysSinceTouch: daysSince,
			DriftScore:     driftScore,
			State:          state,
			Label:          label,
			OpenTaskCount:  openTasks,
		})
	}

	return out, nil
}

func classifyDrift(importance, daysSince int, driftScore float64, openTasks int) (string, string) {
	if importance <= 2 && daysSince <= 1 && openTasks > 3 {
		return "overactive", "Overactive"
	}
	if daysSince <= 2 {
		return "active", "Active"
	}
	if importance >= 4 && driftScore > float64(NeglectThreshold) {
		return "neglected", "You said this was core."
	}
	if driftScore > float64(DriftThreshold) {
		return "drifting", fmt.Sprintf("Drifting \u2014 %d days", daysSince)
	}
	if importance <= 2 && daysSince > 7 {
		return "cold", "Cold (low priority)"
	}
	return "drifting", fmt.Sprintf("Drifting \u2014 %d days", daysSince)
}
