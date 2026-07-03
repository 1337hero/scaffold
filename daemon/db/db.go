package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	if _, err := conn.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func newID() string {
	return uuid.New().String()
}

var localLocation *time.Location

func init() {
	loc, err := time.LoadLocation("America/Boise")
	if err != nil {
		loc = time.UTC
	}
	localLocation = loc
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func localMidnightUTC() string {
	now := time.Now().In(localLocation)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, localLocation)
	return midnight.UTC().Format(time.RFC3339)
}

func localYesterday() string {
	return time.Now().In(localLocation).AddDate(0, 0, -1).Format("2006-01-02")
}

func today() string {
	return time.Now().In(localLocation).Format("2006-01-02")
}

func tomorrow() string {
	return time.Now().In(localLocation).AddDate(0, 0, 1).Format("2006-01-02")
}

func (db *DB) migrate() error {
	_, err := db.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	if err := db.migrateAddColumn("memories", "suppressed_at", "TEXT"); err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS domains (
		  id              INTEGER PRIMARY KEY AUTOINCREMENT,
		  name            TEXT NOT NULL UNIQUE,
		  importance      INTEGER NOT NULL DEFAULT 3 CHECK(importance BETWEEN 1 AND 5),
		  last_touched_at TEXT NOT NULL,
		  status_line     TEXT,
		  briefing        TEXT,
		  created_at      TEXT NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("apply domains schema: %w", err)
	}

	if err := db.migrateAddColumn("memories", "domain_id", "INTEGER REFERENCES domains(id)"); err != nil {
		return err
	}

	if err := db.SeedDefaultDomains(); err != nil {
		return fmt.Errorf("seed default domains: %w", err)
	}

	_, err = db.conn.Exec(`
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
	`)
	if err != nil {
		return fmt.Errorf("apply extended schema: %w", err)
	}

	_, err = db.conn.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		  memory_id UNINDEXED,
		  title,
		  content,
		  tags
		);

CREATE TRIGGER IF NOT EXISTS memories_fts_insert
		AFTER INSERT ON memories BEGIN
		  INSERT INTO memories_fts(memory_id, title, content, tags)
		  VALUES (new.id, COALESCE(new.title,''), new.content, COALESCE(new.tags,''));
		END;

		CREATE TRIGGER IF NOT EXISTS memories_fts_update
		AFTER UPDATE OF title, content, tags ON memories BEGIN
		  DELETE FROM memories_fts WHERE memory_id = old.id;
		  INSERT INTO memories_fts(memory_id, title, content, tags)
		  VALUES (new.id, COALESCE(new.title,''), new.content, COALESCE(new.tags,''));
		END;

		CREATE TRIGGER IF NOT EXISTS memories_fts_delete
		AFTER DELETE ON memories BEGIN
		  DELETE FROM memories_fts WHERE memory_id = old.id;
		END;
	`)
	if err != nil {
		return fmt.Errorf("apply semantic schema: %w", err)
	}

	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS oauth_tokens (
		  provider      TEXT PRIMARY KEY,
		  access_token  TEXT NOT NULL,
		  refresh_token TEXT NOT NULL,
		  token_type    TEXT NOT NULL DEFAULT 'Bearer',
		  expiry        TEXT NOT NULL,
		  created_at    TEXT NOT NULL,
		  updated_at    TEXT NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("apply oauth schema: %w", err)
	}

	_, err = db.conn.Exec(`
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
	`)
	if err != nil {
		return fmt.Errorf("apply lifeos schema: %w", err)
	}

	if err := db.migrateAddColumn("domains", "icon", "TEXT"); err != nil {
		return err
	}
	if err := db.migrateAddColumn("domains", "color", "TEXT"); err != nil {
		return err
	}
	if err := db.migrateAddColumn("domains", "position", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := db.migrateAddColumn("domains", "status", "TEXT DEFAULT 'active'"); err != nil {
		return err
	}
	if err := db.migrateAddColumn("tasks", "is_focus", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS notification_log (
		  id       TEXT PRIMARY KEY,
		  ref_type TEXT NOT NULL,
		  ref_id   TEXT NOT NULL,
		  sent_at  TEXT NOT NULL,
		  message  TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_notification_log_ref ON notification_log(ref_type, ref_id, sent_at DESC);
	`)
	if err != nil {
		return fmt.Errorf("apply notification_log schema: %w", err)
	}

	// v2 schema: new tables
	if _, err := db.conn.Exec(schemaV2); err != nil {
		return fmt.Errorf("apply v2 schema: %w", err)
	}

	// v2 schema: new columns on existing tables (safe via IF NOT EXISTS pattern using migrateAddColumn)
	for _, col := range v2TaskColumns {
		if err := db.migrateAddColumn("tasks", col.name, col.def); err != nil {
			return err
		}
	}
	for _, col := range v2NoteColumns {
		if err := db.migrateAddColumn("notes", col.name, col.def); err != nil {
			return err
		}
	}
	for _, col := range v2PeopleColumns {
		if err := db.migrateAddColumn("people", col.name, col.def); err != nil {
			return err
		}
	}
	if err := db.migrateAddColumn("domains", "surface", "TEXT NOT NULL DEFAULT 'life'"); err != nil {
		return err
	}

	return nil
}

func (db *DB) migrateAddColumn(table, column, colDef string) error {
	exists, err := db.columnExists(table, column)
	if err != nil {
		return fmt.Errorf("check column %s.%s: %w", table, column, err)
	}
	if exists {
		return nil
	}
	if _, err := db.conn.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colDef)); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

func (db *DB) columnExists(table, column string) (bool, error) {
	rows, err := db.conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			defaultV   sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultV, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS memories (
  id           TEXT PRIMARY KEY,
  type         TEXT NOT NULL,
  content      TEXT NOT NULL,
  title        TEXT,
  importance   REAL NOT NULL DEFAULT 0.5,
  source       TEXT NOT NULL DEFAULT 'web',
  tags         TEXT,
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL,
  accessed_at  TEXT,
  access_count INTEGER DEFAULT 0,
  archived     INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS edges (
  id         TEXT PRIMARY KEY,
  from_id    TEXT NOT NULL REFERENCES memories(id),
  to_id      TEXT NOT NULL REFERENCES memories(id),
  relation   TEXT NOT NULL,
  weight     REAL NOT NULL DEFAULT 1.0,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(importance DESC);
CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_memories_archived ON memories(archived);
CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_id);
CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_id);

CREATE TABLE IF NOT EXISTS sessions (
  token_hash   TEXT PRIMARY KEY,
  created_at   TEXT NOT NULL,
  expires_at   TEXT NOT NULL,
  last_seen_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
`
