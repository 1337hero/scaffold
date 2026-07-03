# 2026-07-02 — PRD 01: Demolition complete

## What happened
- Read Issue #21 (PRD 01: Demolition) and local spec `specs/v2-prds/01-demolition.md`
- Created branch `prd/01-demolition` from `v2`
- Removed all dead v1 subsystems:
  - **Gmail pipeline**: gmail.go, gmail_triage.go, gmail_test.go, config/gmail.yaml, config/triage.yaml
  - **Coding agents**: daemon/agents/, config/coder.*, config/coder-prompts/, config/coder-skills/, config/coder-skill.md
  - **Session bus**: daemon/sessionbus/, daemon/cmd/sessionctl/
  - **Desk**: daemon/api/desk.go, daemon/db/desk.go, db queries
  - **Inbox/capture/ingest**: daemon/capture/, daemon/ingestion/, API handlers, db tables
  - **Embedding**: daemon/embedding/ (stub removed)
  - **Cortex tasks**: removed prioritize, embedding_backfill, observations, reindex, drift, gmail_triage, session_cleanup
  - **Cron scheduler**: daemon/cron/ (desk prioritization cron)
  - **Agent tools**: 8 tools removed from tools.yaml (save_to_inbox, get_inbox, create_goal, update_goal, list_goals, list_sessions, send_to_session, dispatch_code_task)
  - **Webhook**: simplified to health check endpoint
  - **Daemon startup**: removed agents, session bus, embedder, gmail client, ingestor initialization
- Cleaned up imports, structs, and dead references across all packages
- Created `db/legacy_types.go` with ScoredMemory type and requireRowsAffected helper
- Made triage.yaml config optional with defaults
- Removed dead test files (34 deleted)
- Committed and pushed to `prd/01-demolition`

## Build + test status
- `go build ./...` — clean
- `go test ./...` — all passing (7 packages)

## Not done
- `config/gmail.yaml` still exists on disk (unused, gitignored)
- `search_email`/`get_email` tools are stubbed (return "not available")
- API test coverage reduced (domain tests deleted)
- PR not yet created on GitHub

## Next
PRD 02: LLM config rework