package db

const schemaV2 = `
-- New v2 tables: people, interactions, projects, milestones, checklists, activity, facts, fact_edges

CREATE TABLE IF NOT EXISTS people (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    surface             TEXT NOT NULL DEFAULT 'life',  -- life|business
    domain_id           INTEGER REFERENCES domains(id),
    relationship        TEXT,                           -- family|friend|colleague|client|mentor|other
    birthday            TEXT,                           -- ISO date
    anniversary         TEXT,                           -- ISO date
    spouse              TEXT,
    kids                TEXT,                           -- JSON array: [{"name":"Marcus","birthday":"2014-03-15"}]
    notes               TEXT,
    last_interaction_at TEXT,
    contact_cadence_days INTEGER,                      -- slipping threshold; null = default 90
    suppressed_at       TEXT,                           -- soft-delete, same model as memories
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_people_surface ON people(surface);
CREATE INDEX IF NOT EXISTS idx_people_domain ON people(domain_id);
CREATE INDEX IF NOT EXISTS idx_people_birthday ON people(birthday);
CREATE INDEX IF NOT EXISTS idx_people_suppressed ON people(suppressed_at);

CREATE TABLE IF NOT EXISTS interactions (
    id              TEXT PRIMARY KEY,
    person_id       TEXT NOT NULL REFERENCES people(id),
    date            TEXT NOT NULL,
    summary         TEXT NOT NULL,
    follow_up       TEXT,
    follow_up_date  TEXT,
    created_at      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_interactions_person ON interactions(person_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_interactions_follow_up ON interactions(follow_up_date) WHERE follow_up_date IS NOT NULL;

CREATE TABLE IF NOT EXISTS projects (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL DEFAULT 'project',  -- project|area|retainer
    surface         TEXT NOT NULL DEFAULT 'life',       -- life|business
    domain_id       INTEGER REFERENCES domains(id),
    status          TEXT NOT NULL DEFAULT 'active',     -- active|on_hold|completed|archived
    start_date      TEXT,
    end_date        TEXT,                                -- null for areas
    description     TEXT,
    last_activity_at TEXT,
    last_reset_at   TEXT,                                -- last retainer checklist reset date
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_projects_surface ON projects(surface);
CREATE INDEX IF NOT EXISTS idx_projects_domain ON projects(domain_id);
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_projects_type ON projects(type);

CREATE TABLE IF NOT EXISTS project_milestones (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id),
    title       TEXT NOT NULL,
    position    INTEGER NOT NULL DEFAULT 0,
    completed   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_milestones_project ON project_milestones(project_id, position);

CREATE TABLE IF NOT EXISTS project_checklists (
    id          TEXT PRIMARY KEY,
    project_id  TEXT REFERENCES projects(id),        -- null for templates
    title       TEXT NOT NULL,
    items       TEXT NOT NULL,                         -- JSON array of {text, completed}
    is_template INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_checklists_project ON project_checklists(project_id);

CREATE TABLE IF NOT EXISTS project_activity (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id),
    description TEXT NOT NULL,
    hours       REAL,
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_activity_project ON project_activity(project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS facts (
    id              TEXT PRIMARY KEY,
    entity          TEXT NOT NULL,              -- person name, project name, concept, "Mike"
    content         TEXT NOT NULL,              -- the fact itself
    category        TEXT,                        -- user_pref|project|tool|general
    tags            TEXT,                        -- comma-separated tags
    trust           REAL NOT NULL DEFAULT 0.5,  -- 0.0-1.0; +0.05 helpful, -0.10 unhelpful; queries floor at 0.3
    retrieval_count INTEGER NOT NULL DEFAULT 0, -- bumped on every query hit
    helpful_count   INTEGER NOT NULL DEFAULT 0, -- bumped on helpful feedback
    suppressed_at   TEXT,                        -- soft-delete, same model as memories
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_facts_entity ON facts(entity);
CREATE INDEX IF NOT EXISTS idx_facts_category ON facts(category);
CREATE INDEX IF NOT EXISTS idx_facts_tags ON facts(tags);
CREATE INDEX IF NOT EXISTS idx_facts_trust ON facts(trust DESC) WHERE suppressed_at IS NULL;

CREATE TABLE IF NOT EXISTS fact_edges (
    id              TEXT PRIMARY KEY,
    source_fact     TEXT NOT NULL REFERENCES facts(id),
    target_entity   TEXT NOT NULL,               -- entity name this fact relates to
    relation        TEXT,                         -- about|connects|contradicts|derived_from
    created_at      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_fact_edges_source ON fact_edges(source_fact);
CREATE INDEX IF NOT EXISTS idx_fact_edges_target ON fact_edges(target_entity);
`

// v2TaskColumns are the columns added to the existing tasks table in v2.
var v2TaskColumns = []struct {
	name, def string
}{
	{"project_id", "TEXT REFERENCES projects(id)"},
	{"reminder_at", "TEXT"},
	{"surface", "TEXT NOT NULL DEFAULT 'life'"},
	{"top3_position", "INTEGER"},
}

// v2NoteColumns are the columns added to the existing notes table in v2.
var v2NoteColumns = []struct {
	name, def string
}{
	{"kind", "TEXT NOT NULL DEFAULT 'note'"},
	{"source", "TEXT"},
	{"flag_for_review", "INTEGER NOT NULL DEFAULT 0"},
	{"review_at", "TEXT"},
	{"person_id", "TEXT REFERENCES people(id)"},
	{"project_id", "TEXT REFERENCES projects(id)"},
	{"suppressed_at", "TEXT"},
}

// v2PeopleColumns backfills the people table for databases that were created by
// an earlier v2 migration before suppressed_at existed. Fresh installs get the
// column from the CREATE TABLE above; this is a no-op there.
var v2PeopleColumns = []struct {
	name, def string
}{
	{"suppressed_at", "TEXT"},
}

// v2ProjectsColumns backfills the projects table for databases that were created
// by PRD 05 before last_reset_at existed.
var v2ProjectsColumns = []struct {
	name, def string
}{
	{"last_reset_at", "TEXT"},
}
