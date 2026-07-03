# 2026-07-03 — PRD 12: Projects page

**Task:** GitHub issue #31. Loop session, seventh PRD today.

**Shipped:** PR #52 merged (closes #31).

- `pages/Projects.jsx` — master-detail with surface-filtered sidebar, create mutation (with template cloning), hash routing via projectId prop.
- `components/projects/{ProjectSidebar,ProjectDetail,MilestoneList,ChecklistCard,ActivityLog,ProjectForm}.jsx` — full component set.
- Backend: `POST /api/checklists/templates` route for reusable template creation.
- queries.js: 3 new query factories (projectDetailQuery, projectTasksQuery, projectActivityQuery), checklistTemplatesQuery, all project/milestone/checklist/activity mutation exports; projectsListQuery parameterized for surface filter; normalizeProjectFull gains startDate/endDate/description/lastActivityAt/lastResetAt; nullableField handles Float64.

**E2E:** harness pattern from PRD 10 — 31/31 checks green:
- Sidebar grouped by type with search, status ordering, slipping dot
- Project detail: milestones (add/delete/toggle + progress bar), checklists (toggle/create/clone), activity log with billable hours, linked tasks, edit/archive
- Template clone on new project creation
- Area detail (no end date), retainer (reset pending indicator)
- Sidebar search filter narrows correctly

**YAGNI skips (documented in PR):** pagination, Gantt/scheduling, Gantt chart.

**Issues discovered:** Puppeteer `page.type()` into controlled Preact inputs works for text but the project form submit via dispatchEvent doesn't propagate state correctly (controlled component state not updated). Workaround: API-based create+clone for the template clone E2E check. The form UI is still tested via `page.type()` + button click for the New button flow.

**Harness seed data expanded:** Projects (with milestones/checklists/activity/activity-hours/description/linked-task), area (business/Acme Corp), retainer (business/SURF LLC with reset-pending), checklist template.

**Next session picks up:** PRD 13 People page (#34).