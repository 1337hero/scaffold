# 2026-07-03 — PRD 09: Frontend shell

**Task:** GitHub issue #28. Loop session, fourth PRD today (06→07→08→09).

**Shipped:** PR #49 merged (closes #28). Net −3,000 lines.

- Surface state: `hooks/useSurface.jsx` (context; business 8am–6pm default, sessionStorage persistence — deliberate deviation from PRD's localStorage note, intent over letter; Cmd/Ctrl+. toggle), `constants/surfaces.js` (single source for ids/labels/icons/colors), `components/SurfaceToggle.jsx` (fixed top-right pill; life=accent, business=status-info).
- Nav: `constants/nav.js` (Today|Tasks|Projects|People|Library|Domains), Sidebar/MobileBar rewritten with real anchors; search kept reachable (sidebar footer + mobile 7th icon); #/today default; document.title per route.
- Stubs Today/Tasks/Projects/People/Library via shared PagePlaceholder; #/domains → old Areas/Area pages (their /api/goals calls are dead — rebuild owned by later PRDs).
- Deleted: Inbox/Coder/Dashboard pages, CaptureModal, components/{inbox,coder,dashboard,map,desk}, dead queries (dashboard/inbox/agents/capture).
- Logout now `queryClient.clear()` in a finally (no stale personal data).

**Process note:** code-quality-enforcer workflow hit the Claude session rate limit (resets 3pm Boise) — 18 raw findings, 0 machine-verified. Triaged them inline instead; fixed 9 real ones (incl. critical stale `#/areas` link in Search.jsx), declined 4 with reasons in the PR. Until the limit resets, avoid multi-agent workflows; do inline reviews.

**Verified:** `bun run build` green; headless Chrome (`google-chrome-stable --headless=new --dump-dom` against vite dev) renders Login without JS errors. Full data-driven shell verification lands with PRD 10.
