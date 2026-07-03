# Project State

_Last updated: 2026-07-02 by demolition session_

## What this project is
Scaffold — personal agent-driven LifeOS/executive-function system (Go daemon + SQLite brain + Signal agent + Preact web UI). v2 pivots it to a "notification surface with a brain": the agent is a witness, not an optimizer. Spec: `specs/v2-notification-surface.md`.

## Current focus
PRD 01 (Demolition) complete — dead v1 subsystems removed from `v2` branch. Ready for PRD 02 (LLM config rework).

## Next up
PRD 02: LLM config rework — simplify LLM routing config, remove triage/prioritize routes, clean up profiles.

## In flight
Branch `prd/01-demolition` — committed and pushed. Not yet merged to main.

## Current branch / PR
`prd/01-demolition` — PR not yet created. Blocks: none.

## Last shipped
5732175 feat: PRD 01 demolition — remove dead v1 subsystems

## Last verified
- `go build ./...` — clean
- `go test ./...` — all passing (7 packages)
- Removed: Gmail pipeline, coding agents, session bus, desk, inbox/capture/ingest, embedding, cron scheduler, 8 agent tools, triage.yaml, dead test files
- Kept: cortex bulletin/consolidation/decay/prune, search_email/get_email (stubbed out), calendar, auth sessions, domain CRUD, frontend

## Known issues / watch list
- `search_email` and `get_email` tools return "not available in v2" stubs — email tools need rework or removal in a later PRD
- `config/gmail.yaml` still exists on disk (unused) — should be removed
- Legacy types file `db/legacy_types.go` was created to satisfy build (ScoredMemory, requireRowsAffected) — these should be absorbed into proper locations
- Several API test files were deleted — frontend domain API tests need restoration in Phase 3

## Key files / commands

| File / command | Why it matters |
|---|---|
| `specs/v2-notification-surface.md` | the v2 master spec |
| `specs/v2-prds/` | 18 actionable PRDs, execute in order |
| `cd daemon && go test ./...` | backend tests |
| `cd daemon && go build -o bin/scaffold-daemon .` | build |
| `cd app && bun run dev` | frontend dev on :4002 |
| `systemctl --user restart scaffold-daemon.service` | deploy locally |

## Key files / commands

| File / command | Why it matters |
|---|---|
| `specs/v2-notification-surface.md` | the v2 master spec |
| `specs/v2-prds/` | 18 actionable PRDs, execute in order |
| `cd daemon && go test ./...` | backend tests |
| `cd daemon && go build -o bin/scaffold-daemon .` | build |
| `cd app && bun run dev` | frontend dev on :4002 |
| `systemctl --user restart scaffold-daemon.service` | deploy locally |
