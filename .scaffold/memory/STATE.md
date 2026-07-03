# Project State

_Last updated: 2026-07-03 by PRD 06 session_

## What this project is
Scaffold — personal agent-driven LifeOS/executive-function system (Go daemon + SQLite brain + Signal agent + Preact web UI). v2 pivots it to a "notification surface with a brain": the agent is a witness, not an optimizer. Spec: `specs/v2-notification-surface.md`.

## Current focus
PRD 06 complete — Facts (facts/fact_edges db layer + 10 API routes). PR #46 open (closes issue #24), awaiting review/merge. PR #45 (PRD 05 Projects) merged.

## Next up
1. PRD 07 (next in `specs/v2-prds/` order) once PR #46 merges.
2. Re-auth Google Calendar: revoke Scaffold's access at myaccount.google.com/permissions (kills the archived v1 token's Gmail scope), then `cd daemon && ./bin/scaffold-daemon auth google` (fresh grant is calendar-only per config defaults)

## In flight
PRD 06 PR #46 open — https://github.com/1337hero/scaffold/pull/46 (Closes #24).

## Current branch / PR
Branch model: everything off `main` by PR. PRD 06 on `feature/prd-06-facts`. PR #46 open: https://github.com/1337hero/scaffold/pull/46

## Last shipped
PRD 06: Facts — `db/facts.go` (InsertFact with related_entities→edges in tx, Get/Update/Suppress/ListFacts, ProbeFacts bumps retrieval_count, ReasonFacts bridging, RelatedEntities adjacency via shared facts, ContradictingFacts same-entity surface, FeedbackFact single-UPDATE clamp +0.05/-0.10), `api/handlers_facts.go` (10 routes), route registrations in `server.go`. 22 db tests.

## Last verified
- `go build ./...`, `go vet ./...`, `go test ./...` — all green (8 packages)
- gofmt clean on new files; no raw SQL in `api/`; all 10 fact routes `protected`; list methods pre-seed `make([]T,0)`
- New: trust floor 0.3 on agent ops (probe/reason/related/contradict) but NOT ListFacts (browse surface — quarantined facts stay reviewable, PR documents the call), category enum validated via ErrInvalidEnum, entity keys trimmed on insert/update (exact-match orphan guard), tag filter escapes LIKE wildcards
- Fresh-context go-reviewer pass on the diff; both real findings fixed with regression tests

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