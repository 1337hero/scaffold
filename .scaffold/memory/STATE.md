# Project State

_Last updated: 2026-07-02 by PRD-breakdown session_

## What this project is
Scaffold — personal agent-driven LifeOS/executive-function system (Go daemon + SQLite brain + Signal agent + Preact web UI). v2 pivots it to a "notification surface with a brain": the agent is a witness, not an optimizer. Spec: `specs/v2-notification-surface.md`.

## Current focus
v2 rebuild on the `v2` branch. The v2 spec has been broken into 18 numbered PRDs in `specs/v2-prds/` (01–18), covering the 4 build-order phases: Demolition (01–02), Taxonomy Backend (03–08), Frontend (09–14), Signal Agent (15–18).

## Next up
Execute PRDs in order, starting with 01-demolition. Each is one-PR-sized. Deep modules mandated for isolated tests: slipping detector, birthday-window calculator, fact store, brief assembler, notification dedup.

## In flight
Uncommitted on `v2`: config/llm.yaml changes, daemon/api/auth.go, daemon/api/server.go, daemon/main.go, plus the new specs/v2-prds/ files (not yet committed).

## Current branch / PR
`v2` — no PR.

## Last shipped
0236392 "Minor readme tweaks" (main history). Nothing shipped this session — PRD authoring only.

## Last verified
Nothing runtime-verified this session.

## Known issues / watch list
- GitHub Issues are intentionally DISABLED on 1337hero/scaffold — the tracker is `specs/v2-prds/` files, not issues. (18 issues were briefly created and deleted 2026-07-02.)
- v1 database considered corrupt; v2 boots a fresh DB, no migration.
- Spec Build Order has 4 phases (Mike once said 5 — spec wins).

## Key files / commands

| File / command | Why it matters |
|---|---|
| `specs/v2-notification-surface.md` | the v2 master spec |
| `specs/v2-prds/` | 18 actionable PRDs, execute in order |
| `cd daemon && go test ./...` | backend tests |
| `cd daemon && go build -o bin/scaffold-daemon .` | build |
| `cd app && bun run dev` | frontend dev on :4002 |
| `systemctl --user restart scaffold-daemon.service` | deploy locally |
