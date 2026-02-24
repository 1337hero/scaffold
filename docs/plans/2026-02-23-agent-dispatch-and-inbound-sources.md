# Agent Dispatch & Inbound Sources

**Date:** 2026-02-23
**Status:** Approved

## The Frame

Scaffold surfaces everything in Mike's life in one place. When something surfaces that needs action, he launches an agent on it from there — without touching the terminal.

**Real-world example:** GitHub issue #23 on CEP → shows up in Scaffold inbox → one click → coder reviews it and drafts a response or opens a PR.

Signal stays as low-friction input capture. FasterChat stays for research. Scaffold is the action layer.

---

## Phase 1 — Fix Dispatching (immediate)

Everything else depends on the coder actually working.

### 1a. PATH fix
The daemon runs as a systemd service and doesn't inherit the user PATH. `claude` lives at `~/.local/bin/claude` and isn't found.

Fix: add `Environment=PATH=...` to `daemon/systemd/scaffold-daemon.service` to include `~/.local/bin`.

### 1b. UI dispatch endpoint
Currently only Signal can trigger coder chains. Add `POST /api/coder/dispatch` so the UI can too.

Payload: `{ task, chain, cwd }` — same shape as `CodeTaskMessage`.

### 1c. Dispatch button on Inbox items
Each Inbox item gets an "→ Dispatch" action. Opens a small modal: chain picker (implement / fix / spec / single), confirm. Fires `POST /api/coder/dispatch`. No new page — action lives inline on the item.

---

## Phase 2 — Inbound Webhook Sources

The Scaffold webhook handler already exists. What's missing is sources that know to call it, and a schema that maps incoming payloads to meaningful Inbox items.

### GitHub
- Webhook on repo (or org) → fires on issue opened, issue commented, PR opened, PR reviewed
- Payload maps to capture: title = "GH: [event] on [repo]#[num]", content = body/comment, tags = [repo, type], domain = Work/Business
- Link preserved in capture so agent can fetch full context via WebFetch

### Fitness app (custom)
- Simple inbound webhook: `{ type: "weight_log", value, date }` or `{ type: "workout", ... }`
- Maps to task or note in Scaffold: "Log weight today" task auto-created, or habit goal progress updated
- Low-code: Mike configures the endpoint URL in the fitness app, done

### Future sources (not in scope now)
- Slack: slash command or app mention → capture
- Email (Gmail): unread important → capture (shares OAuth2 plumbing with Calendar)
- Calendar: write scope for find-a-time + event creation from agent

---

## Phase 3 — More Agent Tools

Expand what the agent can act on, not just observe.

### Calendar write
- Already have OAuth2 + read scope
- Add write scope: create events, find-a-time
- New agent tools: `create_calendar_event`, `find_available_time`

### Gmail
- Same OAuth2 flow as Calendar, different scope
- New agent tools: `search_email`, `send_email`, `archive_email`
- Requires permission gate — no bulk operations without confirmation

### SSH / server ops
- Simple use case: "reload Caddy on my server"
- Implement as a registered webhook or a restricted SSH tool with an allowlist of safe commands
- Not `--dangerously-skip-permissions` territory — explicit allowlist only

---

## What This Is NOT

- Not a chat UI (FasterChat handles research)
- Not replacing Signal (still lowest-friction input)
- Not autonomous agents running while you sleep (yet — trust + permission model comes first)
- Not multi-chat with folders (separate conversation, separate day)

---

## Build Order

1. PATH fix → coder works
2. `POST /api/coder/dispatch` → UI can trigger chains
3. Dispatch button on Inbox items → end-to-end UI flow
4. GitHub webhook source → real items flowing in
5. Fitness app webhook → habit loop closes
6. Calendar write tools
7. Gmail tools (with permission gates)
