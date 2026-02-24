# Proactive Notifications

**Date:** 2026-02-23
**Status:** Draft

## Goal

Scaffold proactively nudges Mike over Signal — task reminders, recurring check-ins, and optional daily briefings. Opt-in per item, configurable schedule, smart cooldown so it doesn't spam.

---

## What This Is Not

- Not goal-level nagging ("You still haven't lost 25lbs"). Goals are long-horizon; they don't need daily pings.
- Not always-on. Everything is opt-in. Default `notify = 0`.
- Not real-time push (no websockets, no mobile push yet — Signal is the delivery layer).

---

## Notification Types

### 1. Task Reminders
For tasks with `notify = 1` and a due date or scheduled recurring day:
- "Heads up — [task] is due today and not marked done."
- Fires once per day, in the morning window (configurable).

### 2. Recurring Task Check-ins
For recurring tasks with `notify = 1` that are scheduled for today and not yet completed:
- Fires mid-day or at a configured time.
- Conversational tone: "Did you get your workout in this morning? Your Mon/Wed/Fri task isn't checked off yet."
- Uses the task name and schedule context — not generic.

### 3. Overdue Nudge
For tasks with `notify = 1`, `due_date < today`, `status = pending`:
- Single nudge per overdue task, not daily harassment.
- Cooldown: once notified, suppress for 48h unless task is updated.

### 4. Daily Briefing (optional)
A morning message summarizing the day: tasks due today, any overdue items, habit goal progress.
- Driven by a configurable schedule (e.g., "09:00 weekdays").
- Uses existing bulletin content + task layer.
- Opt-in via notifications config, not always-on.

### 5. Custom Nudge Times (future)
User-defined schedule: "remind me at 12pm and 8pm". Not in scope for this pass but the config structure should support it.

---

## Notify Toggle Cascade

`notify` lives on tasks. Goals don't generate direct notifications (no "you haven't lost 25lbs" messages). But if a goal has tasks, and you want to be nudged on that goal's work — set `notify = 1` on the goal and it cascades to all child tasks at query time (not stored per-task, resolved at notification check).

Cascade rule: `effective_notify = task.notify OR goal.notify` — evaluated at runtime, not persisted.

---

## Cooldown / Dedupe

A `notification_log` table tracks what was sent and when. The cortex task queries this before firing.

```sql
CREATE TABLE notification_log (
  id TEXT PRIMARY KEY,
  ref_type TEXT NOT NULL,     -- 'task', 'goal', 'briefing'
  ref_id TEXT NOT NULL,       -- task/goal id, or 'daily_briefing'
  sent_at TEXT NOT NULL,
  message TEXT
);
```

Suppression rules:
- Task reminders: once per day max
- Overdue nudges: once per 48h per task
- Daily briefing: once per day

---

## Notifications Config

New file: `config/notifications.yaml`

```yaml
enabled: true

# Daily briefing
briefing:
  enabled: false
  schedule: "09:00"          # 24h local time
  days: [mon, tue, wed, thu, fri]

# Task reminders window
reminders:
  morning_window: "08:30"    # fire task-due-today reminders at this time
  checkin_window: "13:00"    # fire recurring task check-ins at this time

# Overdue nudge cooldown (hours)
overdue_cooldown_hours: 48
```

Config changes require daemon restart (consistent with rest of system).

---

## Implementation

### 1. Wire Signal client into Cortex

**Problem:** `daemon/cortex/cortex.go` has no Signal dependency.

**Fix:** Add a `Notifier` interface and pass it at startup.

```go
// daemon/cortex/notify.go
type Notifier interface {
    Send(ctx context.Context, recipient, message string) error
}
```

`Cortex` struct gets a `notifier Notifier` and `userNumber string` field.

In `daemon/main.go`, pass the existing `signalcli.Client` when constructing Cortex.

### 2. New Cortex task: `notifications`

Path: `daemon/cortex/notifications.go`

Runs every **15 minutes** (fast enough to hit scheduled windows, lightweight query).

Logic:
1. Load notifications config. If `enabled: false`, return.
2. Check if current time falls in any notification window (briefing, morning reminders, check-in).
3. Query tasks: `notify=1 OR goal.notify=1`, filter by today's schedule/due date, exclude already-done, check `notification_log` for cooldown.
4. For each eligible task, compose message, send via `notifier.Send()`, write to `notification_log`.
5. Daily briefing: if briefing window hit and no `notification_log` entry for today's briefing, compose + send.

Message composition is plain text, not LLM — keep it fast and deterministic. The recurring check-in ("Did you do your workout?") is a simple template based on task name + schedule.

### 3. Register task in cortex scheduler

In `daemon/cortex/cortex.go`, add `notifications` to the task list with a 15-minute interval.

### 4. Notifications Control Panel (UI)

New panel or section in the web UI. Surfaces:
- Global on/off toggle
- Briefing schedule (time + days)
- Reminder windows (morning, check-in time)
- Per-task notify toggle (inline on task cards — already planned in LifeOS UX)

This is a Phase 4 item (after core notifications are wired). CLI/Signal can manage toggles in the interim via the agent.

---

## DB Changes

`notification_log` — new table (see schema above). No changes to `goals` or `tasks` schema; `notify INTEGER DEFAULT 0` is already specced in lifeos-spec.

---

## Build Order

1. Add `notification_log` table to DB migration
2. Add `Notifier` interface + wire Signal client into Cortex
3. Add `config/notifications.yaml` + load in config package
4. Implement `notifications` cortex task (task reminders + overdue nudge)
5. Add daily briefing to the same task
6. Register task in scheduler (15min interval)
7. Test via Signal: create a task with `notify=1`, due today — verify nudge fires
8. UI control panel (Phase 4)

---

## Push Notifications (Future)

Signal is the delivery layer for now. The `Notifier` interface is intentionally minimal — a future `WebPushNotifier` or `PushoverNotifier` can implement the same interface and be swapped in or added as a second delivery target without touching notification logic.

Config would gain a `delivery` block:
```yaml
delivery:
  - signal
  # - webpush
  # - pushover
```
