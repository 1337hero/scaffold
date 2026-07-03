# 2026-07-03 — PRD 08: Today aggregate and slipping detection

**Task:** GitHub issue #30 (PRD 08). Part of the merge → next-PRD loop session (PRDs 06, 07 shipped earlier today).

**Shipped:** PR #48 merged (closes #30).

- `db/today.go` — GetTop3Tasks/SetTop3Tasks (tx clear-then-set, rejects unknown/non-pending/duplicate/>3), SlippingTasks (days_overdue), SlippingAll, TodayNotifications (reminders + birthdays 3d + follow-ups + note reviews).
- Surface params added to ProjectsSlipping/AreasSlipping (consolidated into querySlippingProjects) and PeopleSlipping.
- `BirthdayHit` gained Relationship; `brain.CalendarToday()` added (full-day TodayEvents).
- `api/handlers_today.go` — GET /api/today, PUT /api/today/top3, GET /api/slipping; existing project/area slipping handlers take ?surface=.
- Shared `taskCols` const — removed 7 duplicated 19-column SELECT lists.
- 20 new tests in db/today_test.go.

**Review catches (fresh-context go-reviewer):** calendar used 8h-lookahead + process-local TZ (both fixed by switching to TodayEvents which day-bounds in the calendar's own TZ); birthday test had invalid Go date layout "2000-01-02" that only passed on single-digit days; starring a done task was a silent no-op (now 404s).

**Notification semantics worth remembering:** reminder suppression = notification_log row with ref_type='reminder' AND sent_at >= reminder_at (re-setting a later reminder re-arms). Surface filter deliberately skips birthdays/follow-ups/reviews.

**Verified:** build/vet/test green.
