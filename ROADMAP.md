# Scaffold Roadmap

*Living checklist. Update as things ship or priorities shift.*

## Recently Shipped

- [x] Webhook extractors + agent-driven task creation — pluggable extractor layer, GitHub HMAC verification, 10 event types, agent triage pipeline with dedup via source_ref, capture fallback
- [x] Agent system (Phase 1-3): session bus, pi RPC runner, YAML-driven chains, SSE live events, web UI
- [x] Agent dispatch validation — pre-enqueue chain/cwd checks, 422 errors, worker error replies
- [x] Config-driven chains — add chains via YAML + prompt .md, no Go rebuild
- [x] os.Root sandboxing for agent run directories
- [x] Edit layer — task/goal modals, cascade delete, triage model bump
- [x] CSS theme token extraction
- [x] LifeOS layer — goals, tasks, notes, domains, notebooks UI
- [x] Webhook ingest system
- [x] LLM routing layer (multi-provider, config-driven)

## Up Next

### Agent Chain Improvements
*Needs a focused session — read current prompts, think through new chains.*

- [ ] Prompt tuning — scout/planner/worker prompts are thin, need structured output format, project context (`{cwd}`), stronger conventions
- [ ] Reviewer prompt — decouple "review" from "open PR" (not always wanted)
- [ ] New chain: `review` — read-only analysis, scout-only, dispatched from Signal
- [ ] New chain: `refactor` — restructure without behavior change, extra cleanup pass
- [ ] New chain: `test` — scout what's untested, worker writes tests
- [ ] Per-step skill files — worker/verify steps have no skill overrides yet
- [ ] Smart retry — on chain failure, re-dispatch with failure output + error context injected into prompt (not just re-run blind)
- [ ] Multi-model review — reviewer step fans out to 2-3 models (different models catch different things), aggregate pass/fail. LLM routing already supports multiple providers.
- [ ] Proactive agent dispatch (see section below)

### Proactive Notifications (spec: `docs/plans/2026-02-23-proactive-notifications.md`)
- [ ] `notification_log` table
- [ ] `Notifier` interface + wire Signal client into Cortex
- [ ] `config/notifications.yaml` + loader
- [ ] Cortex `notifications` task (15min interval)
- [ ] Task reminders + overdue nudges
- [ ] Daily briefing
- [ ] UI control panel (later phase)

### Signal Attachment Processing
*Currently detects attachments but rejects them. Knitly bot has reusable vision pipeline.*

- [ ] Parse/store full attachment metadata (file path, MIME type) in `daemon/signal/client.go`
- [ ] Images: port Ollama vision call from Knitly (`qwen3-vl:8b`) → new `daemon/media/ollama_vision.go`
- [ ] Documents: route PDFs through existing `daemon/ingestion` extractor
- [ ] Voice memos: speech-to-text via Whisper/faster-whisper
- [ ] Inject extracted text into message thread before `b.Respond()` in `main.go`
- [ ] Remove "cannot access images/attachments/audio" guardrails from agent prompt + `handleMessage`

*Reference: Knitly bot vision at `~/Builds/Knitly/bot/bot.ts:467`, pipeline split at `test-pipeline.ts:57`*
*NOTE: Sonnet is multimodal — images could go directly to the agent instead of a separate vision pass. Decide at spec time: direct (simpler, one round trip) vs Ollama local (no API cost, no data leaving box).*

### Webhook Extractors — Remaining
*Core pipeline shipped. Extractors are pluggable — adding a new source = implement `Extractor` interface + register in `init()`. Agent triage rules in `config/agent.yaml`.*

- [x] Extractor interface + registry (`daemon/webhook/`)
- [x] Per-token source config with type/secret (backward-compat YAML)
- [x] GitHub extractor: issues, PRs, push, workflow_run, discussions (10 event types, HMAC-SHA256)
- [x] Agent-driven triage pipeline with dedup via `source_ref`
- [x] Schema: `tasks.source`, `tasks.source_ref`, `notes.task_id`
- [x] Live test — Tailscale funnel on `scaffold.1337hero.com`, 3 repos connected (CEP, Knitly landing-page, adding more). Ping confirmed, `issue.opened` delivered. Agent triage fell back to capture due to Anthropic API outage (2026-02-25) — needs retest when API is stable.
- [ ] Verify agent triage path — confirm agent creates tasks (not capture fallback) with source/source_ref, dedup, priority levels, noise filtering
- [ ] Monday.com extractor: `change_status_column_value`, `create_item`, `create_update` (+ challenge handshake)
- [ ] Auto-dispatch agent chains from webhook events (e.g. CI failure → `fix` chain) — connects to Proactive Task Discovery

### Proactive Task Discovery & Auto-Dispatch
*Cortex stops just notifying — it finds work and dispatches agents autonomously. Depends on: proactive notifications (eyes), webhook extractors (ears), chain improvements (hands).*
*Inspiration: [OpenClaw agent swarm architecture](https://x.com/elvissun/status/2025920521871716562) — orchestrator with business context discovers work, spawns coding agents, monitors to completion.*

- [ ] Discovery sources: webhook events (CI failure, new issue), error logs, stale branches, overdue tasks, drift signals
- [ ] Decision layer: heuristic or quick LLM call — "is this auto-fixable?" per source type
- [ ] Auto-dispatch: cortex sends `code_task` to session bus with context from the discovery source
- [ ] Trust gradient: starts as "found X, want me to fix it?" (notify + suggest). Graduates to "fixed X, PR ready" (act + notify) as patterns prove out.
- [ ] Configurable autonomy per source/chain — e.g. CI fixes auto-dispatch, but new feature requests always ask first
- [ ] Audit trail: log every auto-dispatch decision (why it fired, what context it had, what it dispatched)

### Google Integration
- [ ] Gmail integration (reuses OAuth2 plumbing from Calendar)
- [ ] Calendar write scope — find-a-time + create events from agent

### Confidence-Based Escalation
*Inspired by: OpenClaw sponsor triage — agent scores inbound, surfaces uncertain items with reasoning.*

When the agent is uncertain about any decision — triaging a webhook event, classifying an email, interpreting a vague Signal message — it should say so instead of guessing. Today `brain.Respond` picks an action and runs with it. The idea: add a confidence signal to the agent's decision loop. Below a threshold, the agent escalates to Mike via Signal with its reasoning and a suggested action, then waits for confirmation before proceeding.

Applies across every inbound surface:
- **Webhook events** — "Got a GitHub discussion I'm not sure how to triage. Looks like a feature request but could be a bug report. Here's the title + body. Want me to create a task or just note it?"
- **Gmail (future)** — "This email scored 45/100 on relevance. Sender is unknown, no prior conversation history. Archive or capture?"
- **Signal messages** — "You said 'handle that thing from yesterday' — I found 3 candidates. Which one?"
- **Agent chain results** — "The reviewer flagged 2 issues but I'm not sure if they're real problems or style preferences. Here's what it said."

Implementation path:
1. **Prompt-only first pass** — Add instructions to `config/agent.yaml` system prompt: "When uncertain (confidence < 60%), state your confidence, explain your reasoning, suggest an action, and ask before proceeding." Zero Go changes.
2. **Structured confidence field** — Agent tool responses include a `confidence: int` field. Brain captures this in conversation_log. Enables tracking confidence over time.
3. **Threshold config** — `config/agent.yaml` gets `escalation_threshold: 60`. Below threshold = ask. Above = act. Per-surface overrides possible later.
4. **Feedback loop** — When Mike corrects a decision, agent stores the correction pattern. Over time, confidence on similar inputs rises. Maps to cortex observations task.

### Cortex Quota-Aware Scheduling
*Inspired by: OpenClaw overnight cron spreading to preserve daytime quota.*

Cortex tasks currently run on fixed intervals regardless of time of day. Heavy LLM tasks (bulletin, observations, consolidation, drift) compete with interactive Signal usage for API quota and latency. The idea: add time-of-day awareness to the cortex scheduler so expensive tasks prefer overnight windows.

- **Preferred window per task** — `config/cortex.yaml` (or inline in Go) gets optional `preferred_hours: "02:00-06:00"` per task. Scheduler skips the task outside this window unless it hasn't run in 2x its normal interval (safety fallback).
- **Priority flag** — Tasks marked `interactive_priority: true` (like bulletin, which feeds the agent system prompt) always run on schedule. Others can be deferred.
- **Quota signal** — If Anthropic returns 429 / rate-limit headers, cortex backs off all non-critical tasks for the cooldown period. Protects interactive Signal responses.
- **Implementation** — Small change to cortex scheduler loop: check `time.Now().Hour()` against preferred window before firing. ~30 lines of Go + config.

### Self-Healing from Logs
*Inspired by: OpenClaw "morning review" — agent reads overnight errors, fixes them, saves learnings.*

Today errors go to journald as unstructured text. Nobody looks at them unless something visibly breaks. The idea: a cortex task that periodically reads recent errors, uses LLM to analyze root causes, and either auto-creates fix tasks or surfaces a digest via Signal.

Pipeline:
1. **Error collection** — New cortex task (`error_review`, 24h interval, overnight preferred). Reads `journalctl --since "24h ago" -u scaffold-daemon` output. Filters for ERROR/WARN/panic lines.
2. **LLM analysis** — Sends error batch to LLM with system context (known gotchas from CLAUDE.md, recent deploys). Gets back: root cause hypothesis, severity, suggested fix, whether it's a repeat.
3. **Action** — Critical errors → immediate Signal notification. Recurring errors → create a task with fix suggestion. Novel errors → save to a `learnings` memory for future reference.
4. **Learnings accumulation** — A dedicated `error_patterns` memory category. Cortex consults this before analyzing new errors — avoids re-analyzing known issues, tracks whether fixes actually resolved them.

Prerequisites: structured logging would make this much cleaner (JSON log output instead of plain text), but journald parsing works as a v1.

## Backlog

- [ ] User settings panel — change password from the UI instead of editing `.env` + restart. Prerequisite for onboarding.
- [ ] FasterChat onboarding flow — setup wizard on first boot (if no users exist, route to `/setup` instead of `/login`). Holding until multi-user/sharing story is clearer.
- [ ] Domain edit surface in UI (PATCH endpoint exists, low priority)
- [ ] Agent subprocess hardening (seccomp/namespaces for pi process)
- [ ] Push notifications beyond Signal (webpush, pushover — `Notifier` interface supports this)
- [ ] `max_concurrent > 1` — active map supports it, untested in prod. Prerequisite for parallel agent work. Needs worktree isolation per-agent to avoid conflicts.
