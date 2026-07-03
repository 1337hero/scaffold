# Project State

_Last updated: 2026-07-02 (late evening) by PR-39 review session_

## What this project is
Scaffold — personal agent-driven LifeOS/executive-function system (Go daemon + SQLite brain + Signal agent + Preact web UI). v2 pivots it to a "notification surface with a brain": the agent is a witness, not an optimizer. Spec: `specs/v2-notification-surface.md`.

## Current focus
PRD 01 demolition COMPLETE. PR #39 review findings all closed by sweep commit `bae3308` (−2,758/+304): auth bypass removed, goals/triage/webhook/embedding fully cut, corpse schema dropped, fresh-DB breaks fixed. Build + vet + test green. PR #39 ready to merge into `v2`.

## Next up
1. Merge PR #39 into `v2`
2. PRD 02: LLM config rework — llama.cpp provider (`llama-server`, openai_compatible), route cortex.bulletin/cortex.semantic to it
3. Re-auth Google Calendar: revoke Scaffold's access at myaccount.google.com/permissions (kills the archived v1 token's Gmail scope), then `cd daemon && ./bin/scaffold-daemon auth google` (fresh grant is calendar-only per config defaults)

## In flight
Nothing. Branch `prd/01-demolition` pushed clean at `bae3308`.

## Current branch / PR
`prd/01-demolition` — PR #39 open against `v2`, sweep commit pushed + summarized in PR comment. GitHub Issues mirror PRDs (Issue #21 = PRD 01); `specs/` stays canonical.

## Last shipped
bae3308 refactor: complete PRD 01 demolition sweep from PR #39 review

## Last verified
- `go build ./...`, `go vet ./...`, `go test ./...` — all green (config/google/llm/signal + root)
- Fresh-DB migration exercised by google package tests (db.Open on temp file)
- Rebuilt `daemon/bin/scaffold-daemon` from branch: zero gmail_triage/session-bus symbols

## Known issues / watch list
- **v1 daemon incident (2026-07-02 ~14:11-14:12):** a pre-demolition v1 binary ran and triaged Mike's Gmail (labeled "Your payment is scheduled for 07/01/2026" WAITING; captured Tony Bomkamp prison-schedule + $3,104.74 withdrawal emails). Stale binary deleted, clean v2 binary rebuilt in its place. systemd units inactive + linked-only.
- **Database reset 2026-07-02 ~22:19:** v1 `scaffold.db` archived to `daemon/scaffold.db.v1-archive-2026-07-02` (contains old OAuth refresh token w/ Gmail scope — revoke per Next up, then delete archive when comfortable). Fresh DB created and verified: clean v2-base schema, daemon boots green. `oauth_tokens` empty → calendar re-auth required before calendar features work.
- v1 frontend (`app/src`) still calls removed `/api/goals` and `/api/dashboard` — already incompatible with demolished API; full rebuild is Phase 3
- api/brain/cortex/db packages have no test files after PR #39's test deletions — coverage returns with Phase 2 rebuild

## Key files / commands

| File / command | Why it matters |
|---|---|
| `specs/v2-notification-surface.md` | the v2 master spec |
| `specs/v2-prds/` | 18 actionable PRDs, execute in order |
| `cd daemon && go test ./...` | backend tests |
| `cd daemon && go build -o bin/scaffold-daemon .` | build |
| `cd app && bun run dev` | frontend dev on :4002 |
| `systemctl --user restart scaffold-daemon.service` | deploy locally |
