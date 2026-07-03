# 2026-07-03 — PRD 11: Tasks page

**Task:** GitHub issue #32. Loop session, sixth PRD today.

**Shipped:** PR #51 merged (closes #32).

- `pages/Tasks.jsx` + `components/tasks/{TaskFilters,TaskRow,TaskForm}.jsx`.
- Client-side filter/sort (personal-scale data — one fetch per status+surface); star column reuses setTop3/top3IdsQuery (same code path as Today); row expansion lazy-fetches linked notes; inline create/edit form.
- Backend: ListTasks `status=all` (excludes soft-deleted), notes `task_id` filter — 2 new db tests.
- queries.js: tasksListQuery, top3IdsQuery, projectsListQuery, taskNotesQuery, normalizeNote/normalizeProjectFull; normalizeTask gained context/microSteps/recurring/domainId.

**YAGNI skips (documented in PR):** pagination, #/tasks/{id} deep link, recurring completion count.

**E2E:** harness pattern from PRD 10 — 11/11 checks green (surface filter, create, sort, filter, star→top3 via API, expansion, complete→Done/All filters). Screenshot reviewed. Note: puppeteer `page.type` into `<input type=date>` garbles values (segment jumping) — cosmetic in test only; use page.$eval value-set next time.

**Verified:** go build/vet/test green, bun build green, 11/11 E2E.
