package db

import (
	"database/sql"
	"fmt"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()

	d := &DB{}
	conn, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	conn.SetMaxOpenConns(1)
	if _, err := conn.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	d.conn = conn

	// Apply full schema exactly as migrate() does
	if _, err := conn.Exec(schema); err != nil {
		t.Fatalf("apply base schema: %v", err)
	}

	// Add suppressed_at column (simulates migrateAddColumn)
	if _, err := conn.Exec("ALTER TABLE memories ADD COLUMN suppressed_at TEXT"); err != nil {
		t.Fatalf("add suppressed_at: %v", err)
	}

	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS domains (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		name            TEXT NOT NULL UNIQUE,
		importance      INTEGER NOT NULL DEFAULT 3 CHECK(importance BETWEEN 1 AND 5),
		last_touched_at TEXT NOT NULL,
		status_line     TEXT,
		briefing        TEXT,
		created_at      TEXT NOT NULL
	);`); err != nil {
		t.Fatalf("apply domains schema: %v", err)
	}

	// Add domain_id column (simulates migrateAddColumn)
	if _, err := conn.Exec("ALTER TABLE memories ADD COLUMN domain_id INTEGER REFERENCES domains(id)"); err != nil {
		t.Fatalf("add domain_id to memories: %v", err)
	}

	if _, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS conversation_log (
			id         TEXT PRIMARY KEY,
			sender     TEXT NOT NULL,
			role       TEXT NOT NULL,
			content    TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_memories_suppressed ON memories(suppressed_at);
		CREATE INDEX IF NOT EXISTS idx_conversation_log_created ON conversation_log(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_conversation_log_sender ON conversation_log(sender);
	`); err != nil {
		t.Fatalf("apply extended schema: %v", err)
	}

	if _, err := conn.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
			memory_id UNINDEXED, title, content, tags
		);
		CREATE TRIGGER IF NOT EXISTS memories_fts_insert AFTER INSERT ON memories BEGIN
			INSERT INTO memories_fts(memory_id, title, content, tags)
			VALUES (new.id, COALESCE(new.title,''), new.content, COALESCE(new.tags,''));
		END;
		CREATE TRIGGER IF NOT EXISTS memories_fts_update
		AFTER UPDATE OF title, content, tags ON memories BEGIN
			DELETE FROM memories_fts WHERE memory_id = old.id;
			INSERT INTO memories_fts(memory_id, title, content, tags)
			VALUES (new.id, COALESCE(new.title,''), new.content, COALESCE(new.tags,''));
		END;
		CREATE TRIGGER IF NOT EXISTS memories_fts_delete AFTER DELETE ON memories BEGIN
			DELETE FROM memories_fts WHERE memory_id = old.id;
		END;
	`); err != nil {
		t.Fatalf("apply memories_fts schema: %v", err)
	}

	if _, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS oauth_tokens (
			provider      TEXT PRIMARY KEY,
			access_token  TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_type    TEXT NOT NULL DEFAULT 'Bearer',
			expiry        TEXT NOT NULL,
			created_at    TEXT NOT NULL,
			updated_at    TEXT NOT NULL
		);
	`); err != nil {
		t.Fatalf("apply oauth schema: %v", err)
	}

	if _, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id          TEXT PRIMARY KEY,
			title       TEXT NOT NULL,
			domain_id   INTEGER REFERENCES domains(id),
			goal_id     TEXT,
			context     TEXT,
			due_date    TEXT,
			recurring   TEXT,
			priority    TEXT DEFAULT 'normal',
			status      TEXT DEFAULT 'pending',
			micro_steps TEXT,
			notify      INTEGER DEFAULT 0,
			position    INTEGER DEFAULT 0,
			created_at  TEXT NOT NULL,
			completed_at TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_tasks_domain ON tasks(domain_id);
		CREATE INDEX IF NOT EXISTS idx_tasks_goal ON tasks(goal_id);
		CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
		CREATE INDEX IF NOT EXISTS idx_tasks_due ON tasks(due_date);
		CREATE TABLE IF NOT EXISTS task_completions (
			id           TEXT PRIMARY KEY,
			task_id      TEXT NOT NULL REFERENCES tasks(id),
			goal_id      TEXT,
			completed_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS notes (
			id         TEXT PRIMARY KEY,
			title      TEXT NOT NULL,
			domain_id  INTEGER REFERENCES domains(id),
			goal_id    TEXT,
			task_id    TEXT REFERENCES tasks(id),
			content    TEXT,
			tags       TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_notes_domain ON notes(domain_id);
		CREATE INDEX IF NOT EXISTS idx_notes_goal ON notes(goal_id);
		CREATE INDEX IF NOT EXISTS idx_notes_task_id ON notes(task_id);
	`); err != nil {
		t.Fatalf("apply tasks/notes schema: %v", err)
	}

	if _, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS notification_log (
			id       TEXT PRIMARY KEY,
			ref_type TEXT NOT NULL,
			ref_id   TEXT NOT NULL,
			sent_at  TEXT NOT NULL,
			message  TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_notification_log_ref ON notification_log(ref_type, ref_id, sent_at DESC);
	`); err != nil {
		t.Fatalf("apply notification_log schema: %v", err)
	}

	// v2 new tables
	if _, err := conn.Exec(schemaV2); err != nil {
		t.Fatalf("apply v2 schema: %v", err)
	}

	// v2 column additions (mimics migrateAddColumn)
	for _, col := range v2TaskColumns {
		ct, _, _, err := pragmaColumnInfo(conn, "tasks", col.name)
		if err != nil {
			t.Fatalf("check tasks.%s: %v", col.name, err)
		}
		if ct == "" {
			if _, err := conn.Exec(fmt.Sprintf("ALTER TABLE tasks ADD COLUMN %s %s", col.name, col.def)); err != nil {
				t.Fatalf("add tasks.%s: %v", col.name, err)
			}
		}
	}
	for _, col := range v2NoteColumns {
		ct, _, _, err := pragmaColumnInfo(conn, "notes", col.name)
		if err != nil {
			t.Fatalf("check notes.%s: %v", col.name, err)
		}
		if ct == "" {
			if _, err := conn.Exec(fmt.Sprintf("ALTER TABLE notes ADD COLUMN %s %s", col.name, col.def)); err != nil {
				t.Fatalf("add notes.%s: %v", col.name, err)
			}
		}
	}
	ct, _, _, err := pragmaColumnInfo(conn, "domains", "surface")
	if err != nil {
		t.Fatalf("check domains.surface: %v", err)
	}
	if ct == "" {
		if _, err := conn.Exec("ALTER TABLE domains ADD COLUMN surface TEXT NOT NULL DEFAULT 'life'"); err != nil {
			t.Fatalf("add domains.surface: %v", err)
		}
	}

	if err := d.SeedDefaultDomains(); err != nil {
		t.Fatalf("seed default domains: %v", err)
	}

	return d
}

func TestSchemaV2NewTables(t *testing.T) {
	db := openTestDB(t)

	tables := []string{
		"people", "interactions", "projects",
		"project_milestones", "project_checklists", "project_activity",
		"facts", "fact_edges",
	}

	for _, name := range tables {
		var exists int
		err := db.conn.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", name, err)
		}
		if exists == 0 {
			t.Errorf("v2 table %q does not exist", name)
		}
	}
}

func TestSchemaV2ExistingTablesPreserved(t *testing.T) {
	db := openTestDB(t)

	preserved := []string{
		"memories", "edges", "conversation_log", "oauth_tokens",
		"tasks", "task_completions", "notes", "domains",
		"notification_log", "sessions",
	}

	for _, name := range preserved {
		var exists int
		err := db.conn.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", name, err)
		}
		if exists == 0 {
			t.Errorf("existing table %q not preserved", name)
		}
	}
}

func TestSchemaV2DroppedTablesAbsent(t *testing.T) {
	db := openTestDB(t)

	dropped := []string{"desk", "captures", "gmail_waiting_threads", "ingestion_files", "ingestion_progress"}

	for _, name := range dropped {
		var exists int
		err := db.conn.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", name, err)
		}
		if exists != 0 {
			t.Errorf("dropped table %q still exists", name)
		}
	}
}

func TestSchemaV2TaskNewColumns(t *testing.T) {
	db := openTestDB(t)

	type colSpec struct{ colType string; notNull int; defaultVal string }
	checks := map[string]colSpec{
		"project_id":    {"TEXT", 0, ""},
		"reminder_at":   {"TEXT", 0, ""},
		"surface":       {"TEXT", 1, "'life'"},
		"top3_position": {"INTEGER", 0, ""},
	}
	for col, exp := range checks {
		ct, nn, dv, err := pragmaColumnInfo(db.conn, "tasks", col)
		if err != nil {
			t.Fatalf("get column info for tasks.%s: %v", col, err)
		}
		if ct == "" {
			t.Errorf("tasks.%s column not found", col)
			continue
		}
		if ct != exp.colType {
			t.Errorf("tasks.%s type: got %q, want %q", col, ct, exp.colType)
		}
		if nn != exp.notNull {
			t.Errorf("tasks.%s notNull: got %d, want %d", col, nn, exp.notNull)
		}
		if exp.defaultVal != "" && dv != exp.defaultVal {
			t.Errorf("tasks.%s default: got %q, want %q", col, dv, exp.defaultVal)
		}
	}
}

func TestSchemaV2NoteNewColumns(t *testing.T) {
	db := openTestDB(t)

	type colSpec struct{ colType string; notNull int; defaultVal string }
	checks := map[string]colSpec{
		"kind":            {"TEXT", 1, "'note'"},
		"source":          {"TEXT", 0, ""},
		"flag_for_review": {"INTEGER", 1, "0"},
		"review_at":       {"TEXT", 0, ""},
		"person_id":       {"TEXT", 0, ""},
	}
	for col, exp := range checks {
		ct, nn, dv, err := pragmaColumnInfo(db.conn, "notes", col)
		if err != nil {
			t.Fatalf("get column info for notes.%s: %v", col, err)
		}
		if ct == "" {
			t.Errorf("notes.%s column not found", col)
			continue
		}
		if ct != exp.colType {
			t.Errorf("notes.%s type: got %q, want %q", col, ct, exp.colType)
		}
		if nn != exp.notNull {
			t.Errorf("notes.%s notNull: got %d, want %d", col, nn, exp.notNull)
		}
		if exp.defaultVal != "" && dv != exp.defaultVal {
			t.Errorf("notes.%s default: got %q, want %q", col, dv, exp.defaultVal)
		}
	}
}

func TestSchemaV2DomainSurfaceColumn(t *testing.T) {
	db := openTestDB(t)

	ct, nn, dv, err := pragmaColumnInfo(db.conn, "domains", "surface")
	if err != nil {
		t.Fatalf("get column info for domains.surface: %v", err)
	}
	if ct == "" {
		t.Fatal("domains.surface column not found")
	}
	if ct != "TEXT" {
		t.Errorf("domains.surface type: got %q, want text", ct)
	}
	if nn != 1 {
		t.Errorf("domains.surface notNull: got %d, want 1", nn)
	}
	if dv != "'life'" {
		t.Errorf("domains.surface default: got %q, want 'life'", dv)
	}
}

func TestSchemaV2DomainSeedSurface(t *testing.T) {
	db := openTestDB(t)

	rows, err := db.conn.Query(`SELECT name, surface FROM domains ORDER BY name`)
	if err != nil {
		t.Fatalf("query domains: %v", err)
	}
	defer rows.Close()

	ds := make(map[string]string)
	for rows.Next() {
		var name, surface string
		if err := rows.Scan(&name, &surface); err != nil {
			t.Fatalf("scan domain: %v", err)
		}
		ds[name] = surface
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}

	for _, name := range []string{"Homelife", "Health", "Faith", "Relationships", "Personal Development"} {
		if surface, ok := ds[name]; !ok {
			t.Errorf("LifeOS domain %q not found", name)
		} else if surface != "life" {
			t.Errorf("LifeOS domain %q surface=%q, want 'life'", name, surface)
		}
	}
	for _, name := range []string{"SURF", "CISS", "1337hero", "Consulting"} {
		if surface, ok := ds[name]; !ok {
			t.Errorf("BusinessOS domain %q not found", name)
		} else if surface != "business" {
			t.Errorf("BusinessOS domain %q surface=%q, want 'business'", name, surface)
		}
	}
}

func TestSchemaV2PeopleTableColumns(t *testing.T) {
	db := openTestDB(t)

	for _, col := range []string{
		"id", "name", "surface", "domain_id", "relationship",
		"birthday", "anniversary", "spouse", "kids", "notes",
		"last_interaction_at", "contact_cadence_days", "suppressed_at", "created_at", "updated_at",
	} {
		ct, _, _, err := pragmaColumnInfo(db.conn, "people", col)
		if err != nil {
			t.Fatalf("get column info for people.%s: %v", col, err)
		}
		if ct == "" {
			t.Errorf("people.%s column not found", col)
		}
	}

	for _, idx := range []string{"idx_people_surface", "idx_people_domain", "idx_people_birthday", "idx_people_suppressed"} {
		var exists int
		err := db.conn.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check index %s: %v", idx, err)
		}
		if exists == 0 {
			t.Errorf("index %q not found", idx)
		}
	}
}

func TestSchemaV2InteractionsTable(t *testing.T) {
	db := openTestDB(t)

	for _, col := range []string{"id", "person_id", "date", "summary", "follow_up", "follow_up_date", "created_at"} {
		ct, _, _, err := pragmaColumnInfo(db.conn, "interactions", col)
		if err != nil {
			t.Fatalf("get column info for interactions.%s: %v", col, err)
		}
		if ct == "" {
			t.Errorf("interactions.%s column not found", col)
		}
	}

	for _, idx := range []string{"idx_interactions_person", "idx_interactions_follow_up"} {
		var exists int
		err := db.conn.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check index %s: %v", idx, err)
		}
		if exists == 0 {
			t.Errorf("index %q not found", idx)
		}
	}
}

func TestSchemaV2ProjectsTable(t *testing.T) {
	db := openTestDB(t)

	for _, col := range []string{
		"id", "name", "type", "surface", "domain_id", "status",
		"start_date", "end_date", "description", "last_activity_at", "last_reset_at",
		"created_at", "updated_at",
	} {
		ct, _, _, err := pragmaColumnInfo(db.conn, "projects", col)
		if err != nil {
			t.Fatalf("get column info for projects.%s: %v", col, err)
		}
		if ct == "" {
			t.Errorf("projects.%s column not found", col)
		}
	}

	for _, idx := range []string{"idx_projects_surface", "idx_projects_domain", "idx_projects_status", "idx_projects_type"} {
		var exists int
		err := db.conn.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check index %s: %v", idx, err)
		}
		if exists == 0 {
			t.Errorf("index %q not found", idx)
		}
	}
}

func TestSchemaV2FactsTable(t *testing.T) {
	db := openTestDB(t)

	for _, col := range []string{
		"id", "entity", "content", "category", "tags", "trust",
		"retrieval_count", "helpful_count", "suppressed_at", "created_at", "updated_at",
	} {
		ct, _, _, err := pragmaColumnInfo(db.conn, "facts", col)
		if err != nil {
			t.Fatalf("get column info for facts.%s: %v", col, err)
		}
		if ct == "" {
			t.Errorf("facts.%s column not found", col)
		}
	}

	for _, idx := range []string{"idx_facts_entity", "idx_facts_category", "idx_facts_tags", "idx_facts_trust"} {
		var exists int
		err := db.conn.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check index %s: %v", idx, err)
		}
		if exists == 0 {
			t.Errorf("index %q not found", idx)
		}
	}
}

func TestSchemaV2ForeignKeys(t *testing.T) {
	db := openTestDB(t)

	fkTests := []struct{ table, column, refTable, refCol string }{
		{"interactions", "person_id", "people", "id"},
		{"project_milestones", "project_id", "projects", "id"},
		{"fact_edges", "source_fact", "facts", "id"},
	}

	for _, fk := range fkTests {
		var cnt int
		err := db.conn.QueryRow(`
			SELECT COUNT(*) FROM pragma_foreign_key_list(?)
			WHERE "from" = ? AND "table" = ? AND "to" = ?
		`, fk.table, fk.column, fk.refTable, fk.refCol).Scan(&cnt)
		if err != nil {
			t.Fatalf("check FK for %s.%s: %v", fk.table, fk.column, err)
		}
		if cnt == 0 {
			t.Errorf("missing FK on %s.%s -> %s(%s)", fk.table, fk.column, fk.refTable, fk.refCol)
		}
	}
}

func TestSchemaV2NotificationsPreserved(t *testing.T) {
	db := openTestDB(t)

	var exists int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='notification_log'`,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("check notification_log: %v", err)
	}
	if exists == 0 {
		t.Fatal("notification_log table missing")
	}
}

func pragmaColumnInfo(conn *sql.DB, table, column string) (colType string, notNull int, defaultVal string, err error) {
	rows, err := conn.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return "", 0, "", err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ct string
		var nn int
		var dv sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ct, &nn, &dv, &pk); err != nil {
			return "", 0, "", err
		}
		if name == column {
			if dv.Valid {
				return ct, nn, dv.String, nil
			}
			return ct, nn, "", nil
		}
	}
	return "", 0, "", rows.Err()
}
