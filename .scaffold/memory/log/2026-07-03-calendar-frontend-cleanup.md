# 2026-07-03 — Calendar re-auth and v1 frontend cleanup

**Task:** Re-auth Google Calendar, then start frontend cleanup after the v2 PRD loop.

**Shipped:** PR #60 merged.

- Rebuilt `daemon/bin/scaffold-daemon` from current `main`.
- Removed stale `gmail.modify` from local untracked `config/google.yaml` before auth.
- Ran `./bin/scaffold-daemon auth google` with the local callback server; Google OAuth token saved to `daemon/scaffold.db` with Calendar scope only.
- Removed the old frontend Domains/Areas route from desktop and mobile nav.
- Deleted the v1 `Area`/`Areas` pages and `app/src/components/areas/*` goal/task/note management tree.
- Removed dead frontend query helpers for `/api/goals`, `/api/domains/health`, and unused calendar-upcoming fetches.
- Updated Search to target tasks and notes only, with no links to the removed Domains route.
- Removed the orphaned `/api/domains/health` route and DB helper that still queried the removed `goals` table.
- Removed goal-table search from `SearchAll` and added a regression test proving unfiltered search works on the v2 schema.

**Verified:** `bun run build`, `go test ./...`, `go vet ./...`, `go build ./...`.

**Next session picks up:** runtime deployment or production smoke testing.
