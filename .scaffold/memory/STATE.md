# Project State

_Last updated: 2026-07-03 by PRD 17 session_

## What this project is
Scaffold — personal agent-driven LifeOS/executive-function system (Go daemon + SQLite brain + Signal agent + Preact web UI). v2 pivots it to a "notification surface with a brain": the agent is a witness, not an optimizer. Spec: `specs/v2-notification-surface.md`.

## Current focus
PRD 17 complete — deterministic Signal push notifications are generated for due task reminders, follow-ups, and birthday reminders, with trigger-date dedupe. PRDs 01-17 shipped; latest PR #58 closed issue #33.

## Next up
1. PRD 18 — Agent personality and system prompt assembly (#38).
2. Re-auth Google Calendar: revoke Scaffold's access at myaccount.google.com/permissions (kills the archived v1 token's Gmail scope), then `cd daemon && ./bin/scaffold-daemon auth google` (fresh grant is calendar-only per config defaults)

## In flight
Nothing — main branch is current.

## Current branch / PR
`main` — no in-flight branch.

## Last shipped
PRD 17: Signal push notifications — new `daemon/notify` pure package for candidate dedupe and message templates; cortex runs a 5-minute push check for due task reminders and follow-ups plus an 08:00 America/Denver birthday check; `notification_log` now stores trigger dates and suppression metadata; Signal delivery is shared with daily briefs. PR #58 closed #33.

## Last verified
- `go build ./...`, `go vet ./...`, `go test ./...` — all green
- `bun run build` — green
- PRD 17 has no browser E2E surface; covered by notify/cortex/DB tests plus backend validation.

## Known issues / watch list
- **api package has no tests** — returns with Phase 2 rebuild
- **v1 daemon incident (2026-07-02 ~14:11-14:12):** stale binary deleted, clean v2 binary rebuilt. systemd units inactive + linked-only.
- **Database reset 2026-07-02 ~22:19:** v1 scaffold.db archived. Fresh DB created. oauth_tokens empty → calendar re-auth required.
- **Controlled Preact inputs:** Direct DOM value setting bypasses React state for template selection in ProjectForm — E2E test uses API-based create+clone as workaround. Form UI tested via `page.type()` + button click.
- **People pagination:** PRD 13 kept filtering/sorting client-side for personal-scale CRM; server-side pagination remains deferred.
- **Library surface:** Notes still do not have a first-class surface column; PRD 14 filters by linked domain surface and keeps domainless notes visible on both surfaces.
- **Legacy email prompt doc:** `config/agent-email.md` still contains v1 Gmail/session wording, but it is not loaded into the runtime prompt; PRD 18 should remove or rewrite it.
- v1 frontend (`app/src`) still calls removed `/api/goals` and `/api/dashboard` — full rebuild is Phase 3

## Key files / commands

| File / command | Why it matters |
|---|---|
| `specs/v2-notification-surface.md` | the v2 master spec |
| `specs/v2-prds/` | 18 actionable PRDs, execute in order |
| `daemon/db/schema_v2.go` | v2 new tables schema + column definitions |
| `daemon/db/schema_v2_test.go` | 14 schema tests |
| `daemon/notify` | PRD17 notification dedupe and message assembly |
| `cd daemon && go test ./...` | backend tests |
| `cd daemon && go build -o bin/scaffold-daemon .` | build |
| `cd app && bun run dev` | frontend dev on :4002 |
| `$SCRATCHPAD/harness/main.go` | E2E verification harness (:4009, seeded DB, puppeteer) |
