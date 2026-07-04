# 2026-07-03 — PRD 13: People page

**Task:** GitHub issue #34. Loop session, eighth frontend PRD in the current run.

**Shipped:** PR #53 merged (closes #34).

- `pages/People.jsx` — surface-aware personal CRM with card/list views, search, relationship/domain filters, birthday strip, slipping count, create flow, and `#/people/{id}` master-detail routing.
- `components/people/{PeopleFilters,PersonCard,PersonDetail,PersonForm,personUtils}.jsx/js` — filter controls, cards, full person form, detail panel, interaction logging, linked note creation, birthday/cadence formatting.
- `queries.js` — People normalizers and query/mutation exports for people, person detail, interactions, linked notes, and birthdays.
- `apiFetch` — handles `204 No Content` success responses so PATCH flows resolve cleanly.
- `AppLayout.jsx` — passes the hash route param into `People`.

**E2E:** harness pattern from PRD 10/12 — 22/22 checks green:
- Business surface filtering, relationship/domain/search filters, grid/list toggle.
- Kid birthday surfaced from `/api/people/birthdays`.
- Slipping indicators on card and detail link to Today.
- Detail shows spouse, kids, notes, interaction history, and linked notes.
- Log interaction updates `last_interaction_at` and due follow-up appears in Today notifications.
- Add linked note, edit person via 204 PATCH, create and delete person.

**YAGNI skips:** server-side pagination, contact import/sync, photos/avatar upload.

**Verified:** `go build ./...`, `go vet ./...`, `go test ./...`, `bun run build`, harness People E2E 22/22.

**Next session picks up:** PRD 14 Library page (#37).
