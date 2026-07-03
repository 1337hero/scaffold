# Project State

_Last updated: 2026-07-03 by PRD 05 session_

## What this project is
Scaffold — personal agent-driven LifeOS/executive-function system (Go daemon + SQLite brain + Signal agent + Preact web UI). v2 pivots it to a "notification surface with a brain": the agent is a witness, not an optimizer. Spec: `specs/v2-notification-surface.md`.

## Current focus
PRD 05 complete — Projects (projects/milestones/checklists/activity) db layer + API routes. PR #45 open, awaiting review/merge.

## Next up
1. PRD 06: Facts (db/facts.go — entity-keyed facts with trust scores, fact_edges).
2. Re-auth Google Calendar: revoke Scaffold's access at myaccount.google.com/permissions (kills the archived v1 token's Gmail scope), then `cd daemon && ./bin/scaffold-daemon auth google` (fresh grant is calendar-only per config defaults)

## In flight
PRD 05 PR #45 open — https://github.com/1337hero/scaffold/pull/45.

## Current branch / PR
Branch model: everything off `main` by PR. PRD 05 on `feature/prd-05-projects`. PR #45 open: https://github.com/1337hero/scaffold/pull/45

## Last shipped
PRD 05: Projects — `db/projects.go` (Project/Milestone/Checklist/Activity CRUD, CloneChecklist, MilestoneCompletion, InsertActivity tx bumps last_activity_at, ProjectsSlipping/AreasSlipping, ResetRetainerChecklists), `api/handlers_projects.go` (15 routes), route registrations in `server.go`. 16 db tests (12 new + 4 schema).

## Last verified
- `go build ./...`, `go vet ./...`, `go test ./...` — all green (8 packages)
- No raw SQL in `api/` handlers; all 15 project routes `protected`; list methods pre-seed `make([]T,0)`
- New: project CRUD with type/surface/status/domain filters, milestones with position ordering + completion % calc, checklists with templates and clone-with-reset, activity tx bumps last_activity_at forward-only, slipping detection (7d projects, 14d areas), retainer checklist auto-reset at ~month boundary

## Known issues / watch list
- **api package has no tests** — returns with Phase 2 rebuild
- **v1 daemon incident (2026-07-02 ~14:11-14:12):** stale binary deleted, clean v2 binary rebuilt. systemd units inactive + linked-only.
- **Database reset 2026-07-02 ~22:19:** v1 scaffold.db archived. Fresh DB created. oauth_tokens empty → calendar re-auth required.
- v1 frontend (`app/src`) still calls removed `/api/goals` and `/api/dashboard` — full rebuild is Phase 3

## Key files / commands

| File / command | Why it matters |
|---|---|
| `specs/v2-notification-surface.md` | the v2 master spec |
| `specs/v2-prds/` | 18 actionable PRDs, execute in order |
| `daemon/db/schema_v2.go` | v2 new tables schema + column definitions |
| `daemon/db/schema_v2_test.go` | 14 schema tests |
| `cd daemon && go test ./...` | backend tests |
| `cd daemon && go build -o bin/scaffold-daemon .` | build |
| `cd app && bun run dev` | frontend dev on :4002 |