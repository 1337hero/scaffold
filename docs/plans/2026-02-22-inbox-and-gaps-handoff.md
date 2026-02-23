# Scaffold LifeOS — Handoff Prompt

> Pass this to the next Claude instance to continue work.

## What Just Happened (Feb 22 evening session)

Mike found that capturing "lose 50lbs" showed as "processed" but no goal was ever created. Root cause: `PersistTriageResult` in `daemon/db/captures.go` was setting `processed = 1` during triage (classification), so captures skipped the inbox entirely.

### Fixes Applied This Session

**Backend (`daemon/`):**
1. `db/captures.go:170` — Removed `processed = 1` from `PersistTriageResult`. Triage now only sets `triage_action`, `memory_id`, `domain_id` on captures, leaving `processed = 0` so items stay in inbox.
2. `api/inbox.go` — `inferCaptureType` now checks `memoryType == "Goal"` first → returns `"goal"`. Previously ignored memory type entirely.
3. `api/inbox.go` — `inboxProcessRequest` now accepts goal-specific fields: `goal_type`, `target_value`, `current_value`, `habit_type`, `schedule_days`.
4. `api/inbox.go` — `handleInboxProcess` goal case now uses all goal fields when creating.
5. `brain/triage.go` — `TriageResult` has new `GoalType` field. Triage prompt updated with goal detection heuristics and `goal_type` classification (binary/measurable/habit).

**Frontend (`app/src/`):**
6. `components/inbox/InboxItem.jsx` — Full rewrite. Pre-fills type/title/domain from triage response. Type pills (task/note/goal). Full goal form: goal type pills (binary/measurable/habit), target/current for measurable, habit type + schedule days for habit. Goal alignment dropdown for tasks/notes.
7. `pages/Inbox.jsx` — Removed "Processed" section. Only shows unprocessed captures. Fetches goals list for alignment dropdown. Invalidates goals/tasks/notes/dashboard queries on process.

**DB:** Reset two existing captures to `processed=0` so they show in inbox.

**Daemon builds clean** (`go build .` exits 0).

### Full Spec Audit Completed

Visual audit report at `~/.agent/diagrams/scaffold-spec-audit.html`. Summary:

**Backend: 95%** — All CRUD endpoints built. Minor: notes use hard delete not soft, domains use PATCH not PUT.

**Dashboard: 65%** — Structure works. Gaps:
- Task queries don't JOIN domains → DomainName undefined, badges never render
- No task tabs (Today/Tomorrow/This Week) — only shows today+overdue
- Goal type visualizations missing — measurable (current/target), habit/streak (flame+count), habit/schedule (dot grid) all show generic progress bars
- No agent suggestion card (no component, API, or data model)
- No "Due Soon" section under calendar
- Goal click → no detail page yet

**Notebooks: 65%** — Core flow works. Gaps:
- Notebook header missing health bar, drift badge, stats row
- Goal cards don't expand to show child tasks/notes (API supports GET /goals/:id with children)
- Goal type-specific visuals missing (same as dashboard)
- No markdown rendering for notes (raw text only)
- No inline title/date editing
- No drag handles for task reordering (backend endpoint exists)
- Missing Recurring filter tab (2 of 4 built)
- Micro-steps stored in schema but no UI

**Search: 85%** — Works. Gaps:
- Results not grouped by type with section dividers
- Domain filter is dropdown, spec says chips

**Capture: 90%** — Works. Gap:
- No confirmation toast ("Captured. It's in your inbox.")

**Inbox: Fixed this session** — Remaining gaps:
- No card slide-out animation on process
- No bulk archive (select multiple)

## Key Files

```
daemon/db/captures.go       — PersistTriageResult fix
daemon/db/goals.go          — Goal struct, InsertGoal, GoalsWithProgress
daemon/api/inbox.go         — inferCaptureType, handleInboxProcess, inboxProcessRequest
daemon/api/handlers_dashboard.go — dashboard endpoint
daemon/api/handlers_tasks.go     — task CRUD
daemon/db/tasks.go          — queryTasks (needs domain JOIN)
daemon/db/dashboard.go      — DashboardData struct
daemon/brain/triage.go      — triage prompt, TriageResult

app/src/pages/Inbox.jsx
app/src/pages/Dashboard.jsx
app/src/pages/Notebook.jsx
app/src/pages/Notebooks.jsx
app/src/pages/Search.jsx
app/src/components/inbox/InboxItem.jsx
app/src/components/dashboard/TaskList.jsx
app/src/components/dashboard/GoalsOverview.jsx
app/src/components/dashboard/DomainHealth.jsx
app/src/components/dashboard/CalendarPanel.jsx
app/src/components/CaptureModal.jsx
app/src/api/queries.js
```

## Specs

```
docs/lifeos-ux-brief.md     — Full UX spec (sections 5.1-5.5)
docs/lifeos-spec.md         — Data model, API spec, build phases
```

## Suggested Next Steps (Priority Order)

1. **Task DomainName JOIN** — `daemon/db/tasks.go` queryTasks needs `LEFT JOIN domains` so dashboard task badges render
2. **Goal type visualizations** — GoalsOverview.jsx needs measurable (current/target values), habit/streak (flame+count), habit/schedule (weekly dot grid). Data is in GoalWithProgress struct already.
3. **Task tabs** — Today/Tomorrow/This Week in TaskList.jsx. Backend already supports `due` filter param (today/tomorrow/week).
4. **Notebook header** — Add health bar, drift state badge, stats row to Notebook.jsx header
5. **Goal expansion** — Click goal card in notebook → show child tasks/notes inline. API: GET /goals/:id returns Tasks and Notes arrays.
6. **Capture confirmation toast** — Quick win, add after successful capture
7. **"Due Soon" section** — Under CalendarPanel, show upcoming task/goal deadlines

## Dev Notes

- Mike runs his own dev server. NEVER start `bun run dev` or similar.
- Runtime: Bun. Package manager: Bun.
- Preact, not React. TanStack Query for server state.
- No TypeScript (except where forced). No barrel files. No useCallback.
- Read the specs before building. The previous agent clearly didn't.
- Commit with `committer` if available in repo, otherwise conventional commits.
