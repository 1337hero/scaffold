# 2026-07-03 — PRD 16: Daily brief

**Task:** GitHub issue #35. Deterministic daily brief PRD after agent tool cleanup.

**Shipped:** PR #57 merged (closes #35).

- `daemon/brief` — new pure assembler package with `AssembleBrief(surface string, queries QueryData) string`, Go `text/template` output, empty-section omission, friendly time/date formatting, and BusinessOS/LifeOS section layouts.
- `daemon/cortex/daily_brief.go` — scheduled morning/evening brief orchestration using America/Denver time: `morning_brief` at 09:00 for BusinessOS and `evening_brief` at 18:00 for LifeOS.
- Cortex query layer fetches calendar events, due/overdue tasks, project/area slipping, reminders, follow-ups, birthdays/anniversaries/kids birthdays, and overdue people, then passes plain data into the assembler.
- Signal delivery is a thin callback set from `main`, keeping Signal out of the assembler.
- Google Calendar stored OAuth tokens are now loaded at daemon startup and attached to Brain, which also gained tomorrow/day-range calendar reads.
- `config/notifications.yaml` now enables deterministic daily briefs with separate morning/evening schedules.
- Tests cover brief assembly, empty-section omission, friendly birthday/date formatting, config defaults, and daily brief schedule dedup.

**YAGNI skips:** no browser E2E for this backend/scheduler PRD; no weekend skip logic; no brief history storage; no Signal integration test; no LLM-generated brief content.

**Verified:** `go test ./...`, `go vet ./...`, `go build ./...`, `bun run build`.

**Next session picks up:** PRD 17 Signal push notifications (#33).
