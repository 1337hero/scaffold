package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// trustFloor is the retrieval floor: facts below it are quarantined — excluded
// from all agent retrieval ops (probe/reason/related/contradict) but still
// visible to ListFacts/GetFact so they can be reviewed and corrected.
const trustFloor = 0.3

// Trust deltas for feedback. Unhelpful carries a 2:1 penalty.
const (
	trustHelpfulDelta   = 0.05
	trustUnhelpfulDelta = -0.10
)

// Fact is one entity-keyed piece of relational memory. Nullable columns use
// sql.Null* per package convention.
type Fact struct {
	ID             string
	Entity         string // person name, project name, concept, "Mike"
	Content        string
	Category       sql.NullString // user_pref|project|tool|general
	Tags           sql.NullString // comma-separated
	Trust          float64        // 0.0-1.0, default 0.5
	RetrievalCount int
	HelpfulCount   int
	SuppressedAt   sql.NullString
	CreatedAt      string
	UpdatedAt      string
}

// FactEdge links a fact to another entity it relates to.
type FactEdge struct {
	ID           string
	SourceFact   string
	TargetEntity string
	Relation     sql.NullString // about|connects|contradicts|derived_from
	CreatedAt    string
}

const factCols = `id, entity, content, category, tags, trust,
	retrieval_count, helpful_count, suppressed_at, created_at, updated_at`

const factEdgeCols = `id, source_fact, target_entity, relation, created_at`

func validateFactCategory(category string) error {
	switch category {
	case "user_pref", "project", "tool", "general":
		return nil
	default:
		return fmt.Errorf("%w: fact category %q (must be user_pref|project|tool|general)", ErrInvalidEnum, category)
	}
}

// InsertFact stores a fact and, in the same transaction, one 'about' edge per
// related entity. Zero Trust defaults to 0.5 — a deliberate 0.0 insert is not
// a real case; feedback is what drives trust down.
func (db *DB) InsertFact(f Fact, relatedEntities []string) error {
	if f.ID == "" {
		f.ID = newID()
	}
	ts := now()
	if f.CreatedAt == "" {
		f.CreatedAt = ts
	}
	if f.UpdatedAt == "" {
		f.UpdatedAt = ts
	}
	if f.Trust == 0 {
		f.Trust = 0.5
	}
	// Trim the key: probe/reason/related match on exact string equality, so a
	// stray space would silently orphan the fact.
	f.Entity = strings.TrimSpace(f.Entity)
	if f.Category.Valid {
		if err := validateFactCategory(f.Category.String); err != nil {
			return err
		}
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin insert fact tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO facts (id, entity, content, category, tags, trust,
		    retrieval_count, helpful_count, suppressed_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.Entity, f.Content, f.Category, f.Tags, f.Trust,
		f.RetrievalCount, f.HelpfulCount, f.SuppressedAt, f.CreatedAt, f.UpdatedAt,
	); err != nil {
		return fmt.Errorf("insert fact: %w", err)
	}

	for _, entity := range relatedEntities {
		entity = strings.TrimSpace(entity)
		if entity == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO fact_edges (id, source_fact, target_entity, relation, created_at)
			 VALUES (?, ?, ?, 'about', ?)`,
			newID(), f.ID, entity, ts,
		); err != nil {
			return fmt.Errorf("insert fact edge for %q: %w", entity, err)
		}
	}

	return tx.Commit()
}

func scanFact(s interface {
	Scan(...any) error
}) (Fact, error) {
	var f Fact
	err := s.Scan(&f.ID, &f.Entity, &f.Content, &f.Category, &f.Tags, &f.Trust,
		&f.RetrievalCount, &f.HelpfulCount, &f.SuppressedAt, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

func (db *DB) GetFact(id string) (*Fact, error) {
	row := db.conn.QueryRow(`SELECT `+factCols+` FROM facts WHERE id = ?`, id)
	f, err := scanFact(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

var factUpdateCols = map[string]bool{
	"entity":   true,
	"content":  true,
	"category": true,
	"tags":     true,
}

func (db *DB) UpdateFact(id string, updates map[string]any) error {
	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	var setClauses []string
	var args []any
	for col, val := range updates {
		if !factUpdateCols[col] {
			return fmt.Errorf("unsupported fact update column: %s", col)
		}
		if col == "category" {
			if s, ok := val.(string); ok {
				if err := validateFactCategory(s); err != nil {
					return err
				}
			}
		}
		if col == "entity" {
			if s, ok := val.(string); ok {
				val = strings.TrimSpace(s)
			}
		}
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}

	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now(), id)

	q := fmt.Sprintf("UPDATE facts SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	return rowsAffectedOrNotFound(db.conn.Exec(q, args...))
}

// SuppressFact hides a fact from all queries without deleting it.
func (db *DB) SuppressFact(id string) error {
	ts := now()
	return rowsAffectedOrNotFound(
		db.conn.Exec(`UPDATE facts SET suppressed_at = ?, updated_at = ? WHERE id = ?`, ts, ts, id),
	)
}

// escapeLike escapes LIKE wildcards in a literal value, for use with ESCAPE '\'.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func (db *DB) queryFacts(query string, args ...any) ([]Fact, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Fact, 0)
	for rows.Next() {
		f, err := scanFact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ListFacts returns non-suppressed facts sorted by trust DESC, optionally
// filtered. No trust floor here — this is the browse/review surface, so
// quarantined facts stay inspectable. limit <= 0 means no limit; the trust
// ordering makes limit the "top N facts for prompt injection" query.
func (db *DB) ListFacts(entity, category, tag *string, limit int) ([]Fact, error) {
	q := `SELECT ` + factCols + ` FROM facts WHERE suppressed_at IS NULL`
	var args []any
	if entity != nil {
		q += ` AND entity = ?`
		args = append(args, *entity)
	}
	if category != nil {
		q += ` AND category = ?`
		args = append(args, *category)
	}
	if tag != nil {
		// Comma-wrap both sides so tag "go" doesn't match tags "golang";
		// escape LIKE wildcards so tag "a_b" doesn't match "axb".
		q += ` AND (',' || tags || ',') LIKE ('%,' || ? || ',%') ESCAPE '\'`
		args = append(args, escapeLike(*tag))
	}
	q += ` ORDER BY trust DESC, updated_at DESC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	return db.queryFacts(q, args...)
}

// PromptFacts returns high-trust facts for system-prompt injection. It ranks
// same-entity and query-relevant facts ahead of generic high-trust facts, then
// bumps retrieval_count for facts that actually enter the prompt.
func (db *DB) PromptFacts(entities []string, query string, limit int) ([]Fact, error) {
	if limit <= 0 {
		limit = 10
	}
	facts, err := db.queryFacts(
		`SELECT `+factCols+` FROM facts
		 WHERE suppressed_at IS NULL AND trust >= ?
		 ORDER BY trust DESC, updated_at DESC`,
		trustFloor,
	)
	if err != nil {
		return nil, err
	}
	if len(facts) == 0 {
		return make([]Fact, 0), nil
	}

	entitySet := normalizedSet(entities)
	queryTokens := factTokens(query)
	scored := make([]scoredFact, 0, len(facts))
	for _, fact := range facts {
		scored = append(scored, scoredFact{
			Fact:  fact,
			Score: promptFactScore(fact, entitySet, queryTokens),
		})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		if scored[i].Trust != scored[j].Trust {
			return scored[i].Trust > scored[j].Trust
		}
		return scored[i].UpdatedAt > scored[j].UpdatedAt
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}
	out := make([]Fact, 0, len(scored))
	ids := make([]string, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.Fact)
		ids = append(ids, item.ID)
	}
	if err := db.bumpFactsRetrieved(ids); err != nil {
		return nil, err
	}
	return out, nil
}

// ProbeFacts returns all facts about an entity (trust floor applied), sorted by
// trust DESC, and bumps retrieval_count on every returned fact.
func (db *DB) ProbeFacts(entity string) ([]Fact, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin probe tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE facts SET retrieval_count = retrieval_count + 1
		 WHERE entity = ? AND suppressed_at IS NULL AND trust >= ?`,
		entity, trustFloor,
	); err != nil {
		return nil, fmt.Errorf("bump retrieval_count: %w", err)
	}

	rows, err := tx.Query(
		`SELECT `+factCols+` FROM facts
		 WHERE entity = ? AND suppressed_at IS NULL AND trust >= ?
		 ORDER BY trust DESC`,
		entity, trustFloor,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Fact, 0)
	for rows.Next() {
		f, err := scanFact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, tx.Commit()
}

// ReasonFacts returns facts that touch every given entity simultaneously — a
// fact matches an entity if it is keyed to it or has an edge targeting it.
// These are the bridging facts for "what connects X to Y?".
func (db *DB) ReasonFacts(entities []string) ([]Fact, error) {
	if len(entities) == 0 {
		return make([]Fact, 0), nil
	}

	q := `SELECT ` + factCols + ` FROM facts f WHERE f.suppressed_at IS NULL AND f.trust >= ?`
	args := []any{trustFloor}
	for _, entity := range entities {
		q += ` AND (f.entity = ? OR EXISTS (
			SELECT 1 FROM fact_edges e WHERE e.source_fact = f.id AND e.target_entity = ?))`
		args = append(args, entity, entity)
	}
	q += ` ORDER BY f.trust DESC`
	return db.queryFacts(q, args...)
}

// RelatedEntities returns entities structurally adjacent to the given one:
// every entity (fact key or edge target) sharing a fact that touches it.
func (db *DB) RelatedEntities(entity string) ([]string, error) {
	rows, err := db.conn.Query(
		`WITH touching AS (
		    SELECT f.id, f.entity FROM facts f
		    WHERE f.suppressed_at IS NULL AND f.trust >= ?
		      AND (f.entity = ? OR EXISTS (
		          SELECT 1 FROM fact_edges e WHERE e.source_fact = f.id AND e.target_entity = ?))
		)
		SELECT DISTINCT name FROM (
		    SELECT entity AS name FROM touching
		    UNION
		    SELECT e.target_entity AS name FROM fact_edges e JOIN touching t ON e.source_fact = t.id
		)
		WHERE name != ?
		ORDER BY name`,
		trustFloor, entity, entity, entity,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// ContradictingFacts surfaces all facts keyed to the same entity for manual
// review. Semantic contradiction detection needs a model call and is deferred;
// for now the caller judges conflicts across the returned set.
func (db *DB) ContradictingFacts(entity string) ([]Fact, error) {
	return db.queryFacts(
		`SELECT `+factCols+` FROM facts
		 WHERE entity = ? AND suppressed_at IS NULL AND trust >= ?
		 ORDER BY trust DESC`,
		entity, trustFloor,
	)
}

// ConflictingFacts returns same-entity facts that appear to conflict with new
// content under simple launch heuristics. This is intentionally conservative:
// exact duplicates and merely adjacent preferences are allowed through.
func (db *DB) ConflictingFacts(entity, content string) ([]Fact, error) {
	content = strings.TrimSpace(content)
	if strings.TrimSpace(entity) == "" || content == "" {
		return make([]Fact, 0), nil
	}
	facts, err := db.ContradictingFacts(strings.TrimSpace(entity))
	if err != nil {
		return nil, err
	}
	out := make([]Fact, 0)
	for _, fact := range facts {
		if factContentConflicts(fact.Content, content) {
			out = append(out, fact)
		}
	}
	return out, nil
}

// FeedbackFact adjusts trust in place: +0.05 helpful, -0.10 unhelpful, clamped
// to [0.0, 1.0]. Helpful feedback also bumps helpful_count.
func (db *DB) FeedbackFact(id string, helpful bool) error {
	delta := trustUnhelpfulDelta
	bump := 0
	if helpful {
		delta = trustHelpfulDelta
		bump = 1
	}
	return rowsAffectedOrNotFound(db.conn.Exec(
		`UPDATE facts
		 SET trust = MIN(1.0, MAX(0.0, trust + ?)),
		     helpful_count = helpful_count + ?,
		     updated_at = ?
		 WHERE id = ?`,
		delta, bump, now(), id,
	))
}

// ListFactEdges returns the edges originating from a fact.
func (db *DB) ListFactEdges(factID string) ([]FactEdge, error) {
	rows, err := db.conn.Query(
		`SELECT `+factEdgeCols+` FROM fact_edges WHERE source_fact = ? ORDER BY target_entity`,
		factID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]FactEdge, 0)
	for rows.Next() {
		var e FactEdge
		if err := rows.Scan(&e.ID, &e.SourceFact, &e.TargetEntity, &e.Relation, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type scoredFact struct {
	Fact
	Score int
}

func normalizedSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func promptFactScore(f Fact, entities map[string]struct{}, queryTokens []string) int {
	score := 0
	entity := strings.ToLower(strings.TrimSpace(f.Entity))
	if _, ok := entities[entity]; ok && entity != "" {
		score += 100
	}

	haystackTokens := factTokens(strings.Join([]string{f.Entity, f.Content, f.Category.String, f.Tags.String}, " "))
	haystack := make(map[string]struct{}, len(haystackTokens))
	for _, token := range haystackTokens {
		haystack[token] = struct{}{}
	}
	for _, token := range queryTokens {
		if _, ok := haystack[token]; ok {
			score += 8
		}
	}
	if strings.Contains(strings.ToLower(f.Content), entity) && entity != "" {
		score += 5
	}
	return score
}

func (db *DB) bumpFactsRetrieved(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin prompt fact retrieval tx: %w", err)
	}
	defer tx.Rollback()

	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			continue
		}
		if _, err := tx.Exec(`UPDATE facts SET retrieval_count = retrieval_count + 1 WHERE id = ?`, id); err != nil {
			return fmt.Errorf("bump fact retrieval_count: %w", err)
		}
	}
	return tx.Commit()
}

func factContentConflicts(a, b string) bool {
	aNorm := normalizeFactText(a)
	bNorm := normalizeFactText(b)
	if aNorm == "" || bNorm == "" || aNorm == bNorm {
		return false
	}

	if hasNegation(aNorm) != hasNegation(bNorm) && tokenOverlap(factTokens(aNorm), factTokens(bNorm)) >= 3 {
		return true
	}

	aValue, aOK := exclusiveValue(aNorm)
	bValue, bOK := exclusiveValue(bNorm)
	return aOK && bOK && aValue != bValue
}

func normalizeFactText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '\'' {
			return r
		}
		return ' '
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func hasNegation(s string) bool {
	for _, marker := range []string{
		" not ", " never ", " cannot ", " can't ", " wont ", " won't ",
		" does not ", " do not ", " doesn't ", " don't ", " dislike ", " dislikes ",
		" hate ", " hates ", " avoid ", " avoids ",
	} {
		if strings.Contains(" "+s+" ", marker) {
			return true
		}
	}
	return false
}

func exclusiveValue(s string) (string, bool) {
	for _, marker := range []string{" prefers ", " preferred ", " favorite ", " default "} {
		if idx := strings.Index(" "+s+" ", marker); idx >= 0 {
			value := strings.TrimSpace((" " + s + " ")[idx+len(marker):])
			value = strings.TrimPrefix(value, "is ")
			value = strings.TrimPrefix(value, "to ")
			value = strings.TrimSpace(value)
			return value, value != ""
		}
	}
	return "", false
}

func tokenOverlap(a, b []string) int {
	seen := make(map[string]struct{}, len(a))
	for _, token := range a {
		seen[token] = struct{}{}
	}
	count := 0
	for _, token := range b {
		if _, ok := seen[token]; ok {
			count++
		}
	}
	return count
}

func factTokens(s string) []string {
	normalized := normalizeFactText(s)
	if normalized == "" {
		return nil
	}
	raw := strings.Fields(normalized)
	out := make([]string, 0, len(raw))
	for _, token := range raw {
		token = normalizeFactToken(token)
		if token == "" || factStopWords[token] {
			continue
		}
		out = append(out, token)
	}
	return out
}

func normalizeFactToken(token string) string {
	token = strings.Trim(token, "'")
	switch token {
	case "likes":
		return "like"
	case "dislikes":
		return "like"
	case "prefers", "preferred":
		return "prefer"
	case "hates":
		return "hate"
	case "avoids":
		return "avoid"
	default:
		return token
	}
}

var factStopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "but": true, "by": true, "do": true, "does": true, "for": true,
	"he": true, "his": true, "i": true, "is": true, "it": true, "not": true,
	"of": true, "on": true, "or": true, "she": true, "that": true, "the": true,
	"to": true, "was": true, "with": true,
}
