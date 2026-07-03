# Session log — PRD 05: Projects

**Date:** 2026-07-03
**Task:** PRD 05 — Projects (projects + milestones + checklists + activity CRUD, slipping detection, retainer auto-reset). GH issue #26, spec `specs/v2-prds/05-projects.md`.
**Shape:** Feature (db package + API layer + tests)

## What was done

- **`daemon/db/projects.go`** (new): `Project`, `Milestone`, `Checklist`, `Activity` structs; project CRUD (Insert/Get/Update/Suppress/List with type/surface/status/domain filters), milestone CRUD (Insert/Update/Delete/List + MilestoneCompletion returning completed/total), checklist CRUD (Insert/Update/List/ListTemplates + CloneChecklist with items reset), activity CRUD (Insert in tx bumping last_activity_at forward-only, List), slipping queries (ProjectsSlipping 7d, AreasSlipping 14d), ResetRetainerChecklists (finds retainer projects past ~28d, clones template checklists with items reset).
- **`daemon/db/projects_test.go`** (new): 16 tests — project CRUD (6), milestones (2), checklists (3), activity (2), slipping (2), retainer reset (1).
- **`daemon/api/handlers_projects.go`** (new): 15 HTTP handlers following the same patterns as `handlers_people.go` and `handlers_notes.go`.
- **`daemon/api/server.go`**: registered 15 routes under // Projects (slipping/areas literals before wildcards per Go 1.22 mux resolution rules).

## Changed files

| File | Change |
|------|--------|
| `daemon/db/projects.go` | **New** — projects/milestones/checklists/activity CRUD, slipping, retainer reset |
| `daemon/db/projects_test.go` | **New** — 16 tests |
| `daemon/api/handlers_projects.go` | **New** — 15 HTTP handlers |
| `daemon/api/server.go` | +15 project routes in New() |

## Review findings (self-review over fresh diff — all addressed)

1. **`projects.go` was missing `encoding/json` import** — the `unmarshalJSON`/`marshalJSON` wrappers call `json.Unmarshal`/`json.Marshal`. Fixed by adding the import. Caught by `go build`.
2. **File truncation on write** — both `projects.go` and `projects_test.go` were truncated at write time (likely output length limit). Appended the missing tails with edit.

## Derived decisions

- **`SuppressProject` sets status to `'archived'`** rather than a `suppressed_at` column. The schema has `status` as a natural field with CHECK constraints; using `suppressed_at` would duplicate. Follows the `status` pattern: default list query excludes `status = 'archived'`, explicit filter can include it.
- **ListProjects defaults to `status != 'archived'`** when no status filter supplied. With an explicit status filter, the filter is applied directly (no default suppression).
- **Slipping uses `>=` not `>`** — a project hitting exactly day 7 or 14 is slipping. Consistent with the spec's "no activity in 7+ days" phrasing.
- **Retainer reset threshold is ~28 days** rather than a strict month boundary query. Simpler SQL (`julianday(?) - julianday(last_activity_at) >= 28`) and avoids calendar math issues. Close enough for practical purposes.
- **`ResetRetainerChecklists` looks for per-project templates** (where `project_id` is set AND `is_template=1`) rather than global templates. Logic: a retainer's templates belong to the retainer project, not a global pool. Global templates (project_id null) remain for one-off clones.
- **`InsertActivity` checks project existence before inserting** to give a clean 404, matching the `InsertInteraction` pattern.
- **API routes use `PATCH /api/milestones/{id}` and `PATCH /api/checklists/{id}`** rather than nesting under projects. Milestones/checklists are sub-resources with their own IDs; this matches the `PATCH /api/people/{id}` pattern.

## Verification ledger

| Check | Result |
|-------|--------|
| `go build ./...` / `go vet ./...` | Pass |
| `go test ./...` | Pass — 8 packages green |
| db tests | 40 pass (24 existing + 16 new) |
| no raw SQL in `api/` | clean |
| 15/15 project routes `protected` | yes |
| mux registration | no panic; literals before wildcards |

## Open / next

1. **PR #45 ready for review** — pushes to merge, then PRD 06 (Facts).
2. PRD 06: Facts (entity-keyed fact store with trust scores, fact_edges).
3. Google Calendar re-auth (still blocked on user action).
