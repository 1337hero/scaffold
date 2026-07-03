# Session log — PRD 04: People CRM

**Date:** 2026-07-03
**Task:** PRD 04 — People CRM (people + interactions CRUD, birthdays, cadence). GH issue #22, spec `specs/v2-prds/04-people-crm.md`.
**Shape:** Feature (db package + API layer + tests)

## Process

Ran `/plan_w_team` after two Explore agents mapped db + API conventions. Plan saved to `specs/prd-04-people-crm-plan.md` (gitignored). **Subagent deployment (Agent tool) was blocked in this environment** — every `Agent` launch was denied — so the plan's builder/reviewer/validator agents were executed directly by the lead instead. Fresh-context review done via `/code-review high` (also runs finders as subagents, which were blocked, so the review passes were run directly against the diff). Flagging for next time: if subagents are needed, confirm the Agent tool is permitted first.

## What was done

- **Schema:** added `suppressed_at TEXT` + `idx_people_suppressed` to the `people` CREATE TABLE in `schema_v2.go` (people is a v2-native table; the PRD requires soft-delete but PRD 03 shipped the table without the column). Added `v2PeopleColumns` migration list (applied in `db.go` migrate()) to backfill any DB already created by PRD 03. Updated `schema_v2_test.go` to expect the column + index.
- **`daemon/db/people.go`** (new): `Person`, `Interaction`, `Kid`, `BirthdayHit` structs; pure `birthdayUrgency(dateISO, today)` calculator (next-occurrence, UTC civil-day math, Feb-29→Feb-28 fold); `InsertPerson`, `GetPerson`, `ListPeople(surface, relationship)`, `UpdatePerson` (whitelist), `SuppressPerson`, `InsertInteraction` (tx: insert + advance last_interaction_at), `ListInteractions`, `FollowUpsDue`, `PeopleSlipping`, `UpcomingBirthdays` (json_each UNION for kids/anniversaries, urgency computed in Go). `MarshalKids`/`KidList` helpers. `rowsAffectedOrNotFound` maps 0-rows → `sql.ErrNoRows` so the API 404 path fires.
- **`daemon/api/handlers_people.go`** (new): 8 handlers modelled on `handlers_notes.go` + `domains.go` PATCH convention. Interaction-create pre-checks person existence for a clean 404.
- **`daemon/api/server.go`**: wired 8 `s.protected` routes under `// People` (birthdays literal registered before `{id}` — Go 1.22 mux resolves literal-over-wildcard; verified no registration panic).
- **`daemon/db/people_test.go`** (new): 10 tests.

## Changed files

| File | Change |
|------|--------|
| `daemon/db/people.go` | **New** — people/interactions CRUD, birthday calculator, tx, json_each birthdays |
| `daemon/db/people_test.go` | **New** — 10 tests |
| `daemon/api/handlers_people.go` | **New** — 8 HTTP handlers |
| `daemon/api/server.go` | +8 People routes in New() |
| `daemon/db/schema_v2.go` | people.suppressed_at + idx + v2PeopleColumns |
| `daemon/db/db.go` | apply v2PeopleColumns in migrate() |
| `daemon/db/schema_v2_test.go` | expect suppressed_at + idx_people_suppressed |

## Review findings (self, fresh pass over the diff — all fixed)

1. **DST off-by-one in `birthdayUrgency`** (correctness) — `Sub(...).Hours()/24` on two local midnights truncates a 23h spring-forward day to 0 days. Fixed by doing the civil-day math in UTC. Regression: `TestBirthdayUrgency_DSTBoundary` (Boise 2025-03-08→09).
2. **`last_interaction_at` regressed on backdated interaction** (correctness) — a backdated log pulled it earlier, breaking `PeopleSlipping`. Fixed: tx `UPDATE` now only advances (CASE). Regression: `TestInsertInteraction_DoesNotRegressLastInteraction`.
3. **Empty PATCH → 500** (minor) — aligned to 400 like `domains`.

## Derived decisions

- **"Expired" birthday = out of window, not negative.** `birthdayUrgency` always returns the *next* occurrence (daysUntil ≥ 0); a just-passed birthday folds to ~364 days and drops out of any near-term window. `TestBirthdayUrgency_Yesterday` asserts empty urgency, not an error.
- **`suppressed_at` added to a PRD-03 table.** Judged in-scope: soft-delete is the PRD's requirement, the column is its prerequisite. Belt-and-suspenders: in CREATE TABLE (fresh installs) + migration (existing).
- **DB models keep the package convention (no json tags, `sql.Null*`).** `Person`/`Interaction` serialize like `Note`/`Task` (nullable fields as `{String,Valid}`) — consistent with existing API; frontend is Phase 3. Only `BirthdayHit` (a bespoke API result) got json tags.
- **`notes.person_id` wiring (user story 14) deferred** — the column exists from PRD 03 but exposing it is a notes-layer change, not people-layer. Left for a small follow-up.

## Verification ledger

| Check | Result |
|-------|--------|
| `go build ./...` / `go vet ./...` | Pass |
| `go test ./...` | Pass — 7 packages green |
| db tests | 24 pass (14 schema + 10 new) |
| no raw SQL in `api/` | clean |
| 8/8 people routes `protected` | yes |
| mux registration | no panic; literal `birthdays` beats `{id}` |

## Open / next

1. **Push `feature/prd-04-people-crm` + open PR** — committed locally, not pushed (waiting on Mike's go-ahead).
2. PRD 05: Projects.
3. Google Calendar re-auth (still blocked on user action).
