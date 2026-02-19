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

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func tomorrow() string {
	return time.Now().AddDate(0, 0, 1).Format("2006-01-02")
}

func (db *DB) migrate() error {
	_, err := db.conn.Exec(schema)
	return err
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

CREATE TABLE IF NOT EXISTS captures (
  id            TEXT PRIMARY KEY,
  raw           TEXT NOT NULL,
  source        TEXT NOT NULL,
  processed     INTEGER DEFAULT 0,
  triage_action TEXT,
  memory_id     TEXT REFERENCES memories(id),
  created_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS desk (
  id           TEXT PRIMARY KEY,
  memory_id    TEXT REFERENCES memories(id),
  title        TEXT NOT NULL,
  position     INTEGER NOT NULL,
  status       TEXT DEFAULT 'active',
  micro_steps  TEXT,
  date         TEXT NOT NULL,
  created_at   TEXT NOT NULL,
  completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(importance DESC);
CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_memories_archived ON memories(archived);
CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_id);
CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_id);
CREATE INDEX IF NOT EXISTS idx_captures_processed ON captures(processed);
CREATE INDEX IF NOT EXISTS idx_desk_date ON desk(date);
CREATE INDEX IF NOT EXISTS idx_desk_status ON desk(status);
`
