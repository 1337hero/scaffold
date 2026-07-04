# 2026-07-03 — PRD 17: Signal push notifications

**Task:** GitHub issue #33. Deterministic Signal push notifications after daily briefs.

**Shipped:** PR #58 merged (closes #33).

- `daemon/notify` — new pure notification package for candidate shape, trigger-date dedupe decisions, birthday trigger selection, task/follow-up/birthday message templates, and batch formatting.
- `daemon/cortex/push_notifications.go` — Cortex now checks due task reminders and follow-ups every 5 minutes, and runs birthday notifications once daily at 08:00 America/Denver.
- Task reminders log with `ref_type='reminder'`, so pushed reminders continue to suppress the existing Today-page reminder surface.
- Follow-up pushes use the interaction id as the dedupe entity and skip suppressed people.
- Birthday pushes send only 3-day planning prompts and day-of reminders, with per-occurrence trigger dates.
- `notification_log` now stores `trigger_date` and `suppressed_at`, with DB helpers for trigger-aware logging, log reads, and suppression.
- `main` now wires a shared Signal sender into Cortex for both daily briefs and push notifications.
- Tests cover notify template/dedupe behavior, notification log trigger metadata, push batching, task-reminder Today-page suppression, and birthday dedupe.

**YAGNI skips:** no real Signal integration test; no in-app notification suppression UI; no surface-specific follow-up push filtering; no LLM-generated notification copy; no notification history page.

**Verified:** `go test ./...`, `go vet ./...`, `go build ./...`, `bun run build`.

**Next session picks up:** PRD 18 Agent personality and system prompt assembly (#38).
