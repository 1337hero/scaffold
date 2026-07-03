package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Person is a CRM contact. Nullable columns use sql.Null* per package convention.
type Person struct {
	ID                 string
	Name               string
	Surface            string // life|business
	DomainID           sql.NullInt64
	Relationship       sql.NullString // family|friend|colleague|client|mentor|other
	Birthday           sql.NullString // ISO date
	Anniversary        sql.NullString // ISO date
	Spouse             sql.NullString
	Kids               sql.NullString // JSON array of Kid
	Notes              sql.NullString
	LastInteractionAt  sql.NullString
	ContactCadenceDays sql.NullInt64 // slipping threshold; null = default 90
	SuppressedAt       sql.NullString
	CreatedAt          string
	UpdatedAt          string
}

// Interaction is a logged touchpoint with a Person.
type Interaction struct {
	ID           string
	PersonID     string
	Date         string // ISO date
	Summary      string
	FollowUp     sql.NullString
	FollowUpDate sql.NullString
	CreatedAt    string
}

// Kid is one entry in a Person's kids JSON array.
type Kid struct {
	Name     string `json:"name"`
	Birthday string `json:"birthday,omitempty"` // ISO date
}

// BirthdayHit is one upcoming birthday or anniversary, self or kid.
type BirthdayHit struct {
	PersonID  string `json:"person_id"`
	Name      string `json:"name"` // person or kid name
	Kind      string `json:"kind"` // self|kid|anniversary
	Date      string `json:"date"` // the stored ISO date
	DaysUntil int    `json:"days_until"`
	Urgency   string `json:"urgency"` // today|soon|upcoming
}

const dateLayout = "2006-01-02"

// MarshalKids encodes a kids slice for storage. An empty slice stores as SQL NULL.
func MarshalKids(kids []Kid) (sql.NullString, error) {
	if len(kids) == 0 {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(kids)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("marshal kids: %w", err)
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

// KidList decodes the person's stored kids JSON. Empty/NULL yields an empty slice.
func (p Person) KidList() ([]Kid, error) {
	if !p.Kids.Valid || strings.TrimSpace(p.Kids.String) == "" {
		return []Kid{}, nil
	}
	var out []Kid
	if err := json.Unmarshal([]byte(p.Kids.String), &out); err != nil {
		return nil, fmt.Errorf("unmarshal kids: %w", err)
	}
	return out, nil
}

// birthdayUrgency computes whole days until the next occurrence of the given
// ISO date's month/day on or after today, plus its urgency tier. ok is false only
// for unparseable input. daysUntil is always >= 0; a date that just passed folds
// to next year's occurrence (~364 days) and so falls out of any near-term window
// — that is how "expired" is expressed: it simply stops being upcoming.
//
// Feb 29 birthdays resolve to Feb 28 in non-leap target years, so they surface
// every year rather than only in leap years.
func birthdayUrgency(dateISO string, today time.Time) (daysUntil int, urgency string, ok bool) {
	t, err := time.Parse(dateLayout, dateISO)
	if err != nil {
		return 0, "", false
	}

	// Do the civil-day math in UTC so DST transitions (a 23h or 25h local day)
	// can't skew the whole-day count. Only the calendar Y/M/D of today matters.
	utcToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	occ := occurrence(t.Month(), t.Day(), utcToday.Year())
	if occ.Before(utcToday) {
		occ = occurrence(t.Month(), t.Day(), utcToday.Year()+1)
	}

	daysUntil = int(occ.Sub(utcToday).Hours() / 24)
	switch {
	case daysUntil == 0:
		urgency = "today"
	case daysUntil <= 3:
		urgency = "soon"
	case daysUntil <= 7:
		urgency = "upcoming"
	}
	return daysUntil, urgency, true
}

// occurrence returns month/day in the given year (UTC midnight), folding Feb 29
// to Feb 28 when year is not a leap year.
func occurrence(month time.Month, day, year int) time.Time {
	if month == time.February && day == 29 && !isLeap(year) {
		day = 28
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func isLeap(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

func (db *DB) InsertPerson(p Person) error {
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
	if p.Surface == "" {
		p.Surface = "life"
	}

	_, err := db.conn.Exec(
		`INSERT INTO people (id, name, surface, domain_id, relationship, birthday, anniversary,
		    spouse, kids, notes, last_interaction_at, contact_cadence_days, suppressed_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Surface, p.DomainID, p.Relationship, p.Birthday, p.Anniversary,
		p.Spouse, p.Kids, p.Notes, p.LastInteractionAt, p.ContactCadenceDays, p.SuppressedAt, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

const personCols = `id, name, surface, domain_id, relationship, birthday, anniversary,
	spouse, kids, notes, last_interaction_at, contact_cadence_days, suppressed_at, created_at, updated_at`

func scanPerson(s interface {
	Scan(...any) error
}) (Person, error) {
	var p Person
	err := s.Scan(&p.ID, &p.Name, &p.Surface, &p.DomainID, &p.Relationship, &p.Birthday, &p.Anniversary,
		&p.Spouse, &p.Kids, &p.Notes, &p.LastInteractionAt, &p.ContactCadenceDays, &p.SuppressedAt, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (db *DB) GetPerson(id string) (*Person, error) {
	row := db.conn.QueryRow(`SELECT `+personCols+` FROM people WHERE id = ?`, id)
	p, err := scanPerson(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (db *DB) ListPeople(surface, relationship *string) ([]Person, error) {
	q := `SELECT ` + personCols + ` FROM people WHERE suppressed_at IS NULL`
	var args []any
	if surface != nil {
		q += ` AND surface = ?`
		args = append(args, *surface)
	}
	if relationship != nil {
		q += ` AND relationship = ?`
		args = append(args, *relationship)
	}
	q += ` ORDER BY name`
	return db.queryPeople(q, args...)
}

var peopleUpdateCols = map[string]bool{
	"name":                 true,
	"surface":              true,
	"domain_id":            true,
	"relationship":         true,
	"birthday":             true,
	"anniversary":          true,
	"spouse":               true,
	"kids":                 true,
	"notes":                true,
	"last_interaction_at":  true,
	"contact_cadence_days": true,
}

func (db *DB) UpdatePerson(id string, updates map[string]any) error {
	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	var setClauses []string
	var args []any
	for col, val := range updates {
		if !peopleUpdateCols[col] {
			return fmt.Errorf("unsupported update column: %s", col)
		}
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}

	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now(), id)

	q := fmt.Sprintf("UPDATE people SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	return rowsAffectedOrNotFound(db.conn.Exec(q, args...))
}

// SuppressPerson soft-deletes a person; they drop out of ListPeople and queries.
func (db *DB) SuppressPerson(id string) error {
	ts := now()
	return rowsAffectedOrNotFound(
		db.conn.Exec(`UPDATE people SET suppressed_at = ?, updated_at = ? WHERE id = ?`, ts, ts, id),
	)
}

func (db *DB) queryPeople(query string, args ...any) ([]Person, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Person, 0)
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PeopleSlipping returns non-suppressed people whose last interaction is older
// than their contact cadence (default 90 days when unset).
func (db *DB) PeopleSlipping() ([]Person, error) {
	return db.queryPeople(
		`SELECT `+personCols+` FROM people
		 WHERE suppressed_at IS NULL
		   AND last_interaction_at IS NOT NULL
		   AND julianday(?) - julianday(last_interaction_at) > COALESCE(contact_cadence_days, 90)
		 ORDER BY last_interaction_at`,
		today(),
	)
}

// InsertInteraction logs a touchpoint and bumps the person's last_interaction_at
// atomically. A bad person_id fails the FK inside the tx and rolls back.
func (db *DB) InsertInteraction(i Interaction) error {
	if i.ID == "" {
		i.ID = newID()
	}
	if i.CreatedAt == "" {
		i.CreatedAt = now()
	}
	if i.Date == "" {
		i.Date = today()
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin insert interaction tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO interactions (id, person_id, date, summary, follow_up, follow_up_date, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		i.ID, i.PersonID, i.Date, i.Summary, i.FollowUp, i.FollowUpDate, i.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert interaction: %w", err)
	}

	// Only advance last_interaction_at — a backdated interaction must not pull it
	// backward, or PeopleSlipping would treat a fresh contact as overdue.
	if _, err := tx.Exec(
		`UPDATE people
		   SET last_interaction_at = CASE
		         WHEN last_interaction_at IS NULL OR ? > last_interaction_at THEN ?
		         ELSE last_interaction_at END,
		       updated_at = ?
		 WHERE id = ?`,
		i.Date, i.Date, now(), i.PersonID,
	); err != nil {
		return fmt.Errorf("update last_interaction_at: %w", err)
	}

	return tx.Commit()
}

func (db *DB) ListInteractions(personID string) ([]Interaction, error) {
	return db.queryInteractions(
		`SELECT id, person_id, date, summary, follow_up, follow_up_date, created_at
		 FROM interactions WHERE person_id = ? ORDER BY date DESC`,
		personID,
	)
}

// FollowUpsDue returns interactions whose follow_up_date has arrived.
func (db *DB) FollowUpsDue() ([]Interaction, error) {
	return db.queryInteractions(
		`SELECT id, person_id, date, summary, follow_up, follow_up_date, created_at
		 FROM interactions WHERE follow_up_date IS NOT NULL AND follow_up_date <= ?
		 ORDER BY follow_up_date`,
		today(),
	)
}

func (db *DB) queryInteractions(query string, args ...any) ([]Interaction, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Interaction, 0)
	for rows.Next() {
		var i Interaction
		if err := rows.Scan(&i.ID, &i.PersonID, &i.Date, &i.Summary, &i.FollowUp, &i.FollowUpDate, &i.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// UpcomingBirthdays returns people birthdays, anniversaries, and kids' birthdays
// falling within the next `days` days, each tagged with an urgency tier. Kids are
// flattened out of the kids JSON via json_each.
func (db *DB) UpcomingBirthdays(days int) ([]BirthdayHit, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, 'self' AS kind, birthday AS d FROM people
		  WHERE suppressed_at IS NULL AND birthday IS NOT NULL
		UNION ALL
		SELECT id, name, 'anniversary' AS kind, anniversary AS d FROM people
		  WHERE suppressed_at IS NULL AND anniversary IS NOT NULL
		UNION ALL
		SELECT p.id, json_extract(k.value, '$.name') AS name, 'kid' AS kind,
		       json_extract(k.value, '$.birthday') AS d
		  FROM people p, json_each(p.kids) k
		  WHERE p.suppressed_at IS NULL AND p.kids IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().In(localLocation)
	out := make([]BirthdayHit, 0)
	for rows.Next() {
		var hit BirthdayHit
		var name, date sql.NullString
		if err := rows.Scan(&hit.PersonID, &name, &hit.Kind, &date); err != nil {
			return nil, err
		}
		if !date.Valid {
			continue
		}
		daysUntil, urgency, ok := birthdayUrgency(date.String, now)
		if !ok || daysUntil > days {
			continue
		}
		hit.Name = name.String
		hit.Date = date.String
		hit.DaysUntil = daysUntil
		hit.Urgency = urgency
		out = append(out, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].DaysUntil < out[j].DaysUntil })
	return out, nil
}

// rowsAffectedOrNotFound maps a zero-row UPDATE/DELETE to sql.ErrNoRows so API
// handlers can return 404 via errors.Is.
func rowsAffectedOrNotFound(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
