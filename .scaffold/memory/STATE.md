# Project State

_Last updated: 2026-07-03 by PRD 18 session_

## What this project is
Scaffold — personal agent-driven LifeOS/executive-function system (Go daemon + SQLite brain + Signal agent + Preact web UI). v2 pivots it to a "notification surface with a brain": the agent is a witness, not an optimizer. Spec: `specs/v2-notification-surface.md`.

## Current focus
PRD 18 complete — the agent prompt now assembles from static identity, cortex bulletin, detected surface, and high-trust facts. PRDs 01-18 shipped; latest PR #59 closed issue #38. The v2 PRD loop is complete.

## Next up
1. Re-auth Google Calendar: revoke Scaffold's access at myaccount.google.com/permissions (kills the archived v1 token's Gmail scope), then `cd daemon && ./bin/scaffold-daemon auth google` (fresh grant is calendar-only per config defaults).
2. Decide the next track after v2 PRDs: Phase 3/frontend cleanup, runtime deployment, or production smoke testing.

## In flight
Nothing — main branch is current.

## Current branch / PR
`main` — no in-flight branch.

## Last shipped
PRD 18: Agent personality and system prompt assembly — new `config/agent-identity.yaml`; new pure `daemon/agentprompt` prompt/surface module; Brain now injects identity, cortex bulletin, detected surface, and high-trust facts; DB prompt facts enforce the trust floor and bump retrieval_count; `save_fact` reports likely same-entity conflicts without saving. PR #59 closed #38.

## Last verified
- `go build ./...`, `go vet ./...`, `go test ./...` — all green
- `bun run build` — green
- PRD 18 has no browser E2E surface; covered by agentprompt/config/brain/DB tests plus backend validation.

## Known issues / watch list
- **api package has no tests** — returns with Phase 2 rebuild
- **v1 daemon incident (2026-07-02 ~14:11-14:12):** stale binary deleted, clean v2 binary rebuilt. systemd units inactive + linked-only.
- **Database reset 2026-07-02 ~22:19:** v1 scaffold.db archived. Fresh DB created. oauth_tokens empty → calendar re-auth required.
- **Controlled Preact inputs:** Direct DOM value setting bypasses React state for template selection in ProjectForm — E2E test uses API-based create+clone as workaround. Form UI tested via `page.type()` + button click.
- **People pagination:** PRD 13 kept filtering/sorting client-side for personal-scale CRM; server-side pagination remains deferred.
- **Library surface:** Notes still do not have a first-class surface column; PRD 14 filters by linked domain surface and keeps domainless notes visible on both surfaces.
- v1 frontend (`app/src`) still calls removed `/api/goals` and `/api/dashboard` — full rebuild is Phase 3

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
| `cd daemon && go test ./...` | backend tests |
| `cd daemon && go build -o bin/scaffold-daemon .` | build |
| `cd app && bun run dev` | frontend dev on :4002 |
| `$SCRATCHPAD/harness/main.go` | E2E verification harness (:4009, seeded DB, puppeteer) |
