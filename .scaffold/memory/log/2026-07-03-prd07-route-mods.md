# 2026-07-03 — PRD 07: Task/note/domain route modifications

**Task:** GitHub issue #27 (PRD 07, `specs/v2-prds/07-task-note-domain-mods.md`) — plumb v2 columns through db structs and API routes. Part of Mike's "merge → next PRD" loop session.

**Shipped:** PR #47 merged to main (closes #27).

- `db/tasks.go` — `TaskFilters` struct for ListTasks (project_id, surface, top3 bool, reminder_at ≤ due-by); surface + top3_position (1–3|null) validation on insert/update.
- `db/notes.go` — Note gains kind/source/flag_for_review/review_at/person_id; `NoteFilters`; kind enum (note|journal|quote), default note; shared `noteCols`/`scanNote`.
- `db/domains.go` — Surface through struct/ListDomains(surface)/GetDomain/CreateDomain(default life)/UpdateDomain; `ComputeDriftStates(surface)`.
- API handlers: new query params + body fields, ErrInvalidEnum → 400; `brain/tools.go` caller updated.
- 15 new tests in `db/route_mods_test.go` (all PRD Testing Decisions names).

**Bugs found & fixed (fresh-context go-reviewer):**
1. **First-boot seed bug (critical, prod):** `migrate()` added `domains.surface` after `SeedDefaultDomains()`, so a fresh DB seeded the legacy 7-domain set; v2 LifeOS/BusinessOS domains only appeared on second daemon start. Column add moved before seed + regression test. The current live DB was created before this fix — worth checking `SELECT name, surface FROM domains` on the real scaffold.db if domains look off.
2. **Test-schema drift:** `openTestDB` hand-duplicated the schema (missing `tasks.is_focus` etc.) and double-seeded, masking bug 1. Now calls the real `migrate()`.
3. **500s instead of 404s:** `requireRowsAffected` returned a bare error, never matching handlers' `errors.Is(sql.ErrNoRows)` — update/delete of missing tasks/notes/domains/memories returned 500. Fixed in the shared helper (11 call sites) + regression tests.

**Judgment calls:** reminder_at filter is ≤ (due-by) semantics; top3 filter is boolean not exact-position (YAGNI); TaskFilters/NoteFilters structs over 8 positional args.

**Verified:** build/vet/test green; all changed-signature callers updated across daemon/.
