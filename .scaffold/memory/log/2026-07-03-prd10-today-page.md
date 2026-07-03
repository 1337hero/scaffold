# 2026-07-03 — PRD 10: Today page

**Task:** GitHub issue #29. Loop session, fifth PRD today.

**Shipped:** PR #50 merged (closes #29).

- `components/today/{Top3Section,CalendarSection,SlippingSection,NotificationsSection}.jsx`, `pages/Today.jsx` (single todayQuery fetch, sections take props).
- `queries.js`: v2 normalizers (Go PascalCase + sql.Null → clean client shapes, one place), todayQuery, top3CandidatesQuery, setTop3.
- `lib/utils.js`: daysSince helper.
- Backend one-liner: `/api/today` returns Top 3 globally — SetTop3Tasks clears ALL positions on write, so a surface-filtered view would silently wipe the other surface's stars. Top 3 = one daily commitment; the rest stays surface-scoped.

**Verification pattern worth reusing (PRDs 11–14):** scratch harness at `scratchpad/harness/main.go` — real `api.New` + seeded temp SQLite + built frontend on :4009, bcrypt test creds, no live daemon (no Signal side effects). Drive with puppeteer-core (`bun add puppeteer-core`, system `/usr/bin/google-chrome-stable`, headless new, `--no-sandbox`). 13/13 checks green incl. surface toggle, star/unstar/reorder mutations, reload persistence.

**Gotchas hit:** hash-only `page.goto` doesn't reload the SPA (auth state persisted — reload after login); CSS `uppercase` changes `innerText` casing (match case-insensitively); default viewport 800px hides the `lg:` sidebar.

**Verified:** bun build green, 13/13 E2E checks, screenshot reviewed (layout matches wireframe).
