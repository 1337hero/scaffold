# Project State

_Last updated: 2026-07-03 by calendar/frontend cleanup session_

## What this project is
Scaffold — personal agent-driven LifeOS/executive-function system (Go daemon + SQLite brain + Signal agent + Preact web UI). v2 pivots it to a "notification surface with a brain": the agent is a witness, not an optimizer. Spec: `specs/v2-notification-surface.md`.

## Current focus
PRDs 01-18 are complete. Google Calendar was re-authed with calendar-only scope, and the first frontend cleanup removed the v1 Domains/Areas/goals surface. Latest PR #60 removed the old frontend route, dead goals helpers, and orphaned backend `domains/health`/goal-search code.

## Next up
1. Runtime deployment or production smoke testing.
2. Continue frontend cleanup only if a new stale surface is found.

## In flight
Nothing — main branch is current.

## Current branch / PR
`main` — no in-flight branch.

## Last shipped
Post-PRD cleanup: Google Calendar OAuth token refreshed with calendar-only scope. PR #60 removed the old frontend Domains/Areas route, deleted the v1 areas/goals component tree, trimmed stale query/color helpers, and removed backend `domains/health` plus goal-table search.

## Last verified
- `go build ./...`, `go vet ./...`, `go test ./...` — all green
- `bun run build` — green
- Frontend cleanup covered by app build plus DB search regression test.

## Known issues / watch list
- **api package has no tests** — returns with Phase 2 rebuild
- **v1 daemon incident (2026-07-02 ~14:11-14:12):** stale binary deleted, clean v2 binary rebuilt. systemd units inactive + linked-only.
- **Database reset 2026-07-02 ~22:19:** v1 scaffold.db archived. Fresh DB created. Google Calendar token re-auth completed 2026-07-03.
- **Controlled Preact inputs:** Direct DOM value setting bypasses React state for template selection in ProjectForm — E2E test uses API-based create+clone as workaround. Form UI tested via `page.type()` + button click.
- **People pagination:** PRD 13 kept filtering/sorting client-side for personal-scale CRM; server-side pagination remains deferred.
- **Library surface:** Notes still do not have a first-class surface column; PRD 14 filters by linked domain surface and keeps domainless notes visible on both surfaces.

## Key files / commands

| File / command | Why it matters |
|---|---|
| `specs/v2-notification-surface.md` | the v2 master spec |
| `specs/v2-prds/` | 18 actionable PRDs, execute in order |
| `config/agent-identity.yaml` | PRD18 static agent identity and prompt fact limit |
| `daemon/db/schema_v2.go` | v2 new tables schema + column definitions |
| `daemon/db/schema_v2_test.go` | 14 schema tests |
| `daemon/agentprompt` | PRD18 pure prompt assembly and surface detection |
| `daemon/notify` | PRD17 notification dedupe and message assembly |
| `app/src/pages/{Today,Tasks,Projects,People,Library}.jsx` | active v2 frontend surfaces |
| `cd daemon && go test ./...` | backend tests |
| `cd daemon && go build -o bin/scaffold-daemon .` | build |
| `cd app && bun run dev` | frontend dev on :4002 |
| `$SCRATCHPAD/harness/main.go` | E2E verification harness (:4009, seeded DB, puppeteer) |
