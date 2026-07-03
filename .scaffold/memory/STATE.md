# Project State

_Last updated: 2026-07-02 (late night) by PRD 03 session_

## What this project is
Scaffold — personal agent-driven LifeOS/executive-function system (Go daemon + SQLite brain + Signal agent + Preact web UI). v2 pivots it to a "notification surface with a brain": the agent is a witness, not an optimizer. Spec: `specs/v2-notification-surface.md`.

## Current focus
PRD 03 complete — v2 schema bootstrap with new tables, modified columns, LifeOS/BusinessOS domain seeding.

## Next up
1. PRD 04: People CRM (db/people.go, API routes for people + interactions)
2. Re-auth Google Calendar: revoke Scaffold's access at myaccount.google.com/permissions (kills the archived v1 token's Gmail scope), then `cd daemon && ./bin/scaffold-daemon auth google` (fresh grant is calendar-only per config defaults)

## In flight
Nothing.

## Current branch / PR
Branch model: everything off `main` by PR. PRD 03 work is on `prd/03-v2-schema` branch. PR #43 open: https://github.com/1337hero/scaffold/pull/43

## Last shipped
PRD 03: v2 schema bootstrap — new schema_v2.go with 8 new tables (people, interactions, projects, milestones, checklists, activity, facts, fact_edges), modified columns on tasks/notes/domains, LifeOS/BusinessOS domain seeding, schema tests

## Last verified
- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — all green (all 7 packages)
- 14 schema tests pass: new tables exist, v1 tables preserved, dropped tables absent, column types/defaults correct, FK constraints present, domain seed with surface values correct

## Known issues / watch list
- **db package has no tests for people/projects/facts CRUD** — these are PRDs 04-06. Schema tests exist now.
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