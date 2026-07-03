# Project State

_Last updated: 2026-07-03 by PRD 04 session_

## What this project is
Scaffold — personal agent-driven LifeOS/executive-function system (Go daemon + SQLite brain + Signal agent + Preact web UI). v2 pivots it to a "notification surface with a brain": the agent is a witness, not an optimizer. Spec: `specs/v2-notification-surface.md`.

## Current focus
PRD 04 complete — People CRM db layer + API routes on the feature branch, awaiting push/PR.

## Next up
1. Push `feature/prd-04-people-crm` + open PR (not yet pushed).
2. PRD 05: Projects (db/projects.go — projects/milestones/checklists/activity CRUD + API).
3. Re-auth Google Calendar: revoke Scaffold's access at myaccount.google.com/permissions (kills the archived v1 token's Gmail scope), then `cd daemon && ./bin/scaffold-daemon auth google` (fresh grant is calendar-only per config defaults)

## In flight
PRD 04 committed on `feature/prd-04-people-crm`, local only — needs `git push -u` + `gh pr create`.

## Current branch / PR
Branch model: everything off `main` by PR. PRD 04 work is on `feature/prd-04-people-crm`. PR not yet opened.

## Last shipped
PRD 04: People CRM — `db/people.go` (Person/Interaction/Kid/BirthdayHit; InsertPerson/GetPerson/ListPeople/UpdatePerson/SuppressPerson/InsertInteraction (tx)/ListInteractions/FollowUpsDue/PeopleSlipping/UpcomingBirthdays), pure `birthdayUrgency` calculator, `api/handlers_people.go` (8 routes), `people.suppressed_at` column added to schema_v2.go + v2PeopleColumns migration. 24 db tests (schema 14 unchanged + 10 new incl. DST + backdated regression).

## Last verified
- `go build ./...`, `go vet ./...`, `go test ./...` — all green (7 packages)
- No raw SQL in `api/` handlers; all 8 people routes `protected`; list methods pre-seed `make([]T,0)`
- New: birthday urgency (7/3/today/tomorrow/yesterday/leap-year/year-wrap/DST), CRUD, suppress-hides-from-list, surface+relationship filters, interaction tx bumps last_interaction_at (and does not regress on backdate), kids via json_each, slipping via cadence

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