# Scaffold

Agent-driven executive function system:
- Signal conversation as the front door
- Cortex runtime for background memory maintenance
- Desktop UI as the presentation layer
- Session bus for cross-agent coordination
- SQLite brain as the shared source of truth

## Current Status

Core build phases complete, LifeOS layer operational:
- `Phase 2A` foundation: config loader, conversation log, suppression model, mutation APIs in DB layer
- `Phase 2B` agent rewrite: tool-use conversational loop + config-driven prompts/tools
- `Phase 2C` cortex runtime: singleton scheduler + bulletin cache + boot wiring
- `Phase 2D` inbox/desktop wiring: confirm/override/archive endpoints + frontend actions
- `Phase 2E` maintenance tasks: prioritize/session cleanup/consolidation/decay/prune/reindex
- `Calendar` Google Calendar integration: OAuth2 flow, event fetching, agent tool
- `LifeOS` Goals (binary/measurable/habit), tasks (recurring), notes — all domain-scoped
- `Dashboard` Primary view: calendar, domain health, completions, goals overview, task list
- `Notebooks` Domain-scoped notebooks with goals/tasks/notes tabs
- `Search` Global search across memories, goals, tasks, notes
- `Session Bus` In-memory cross-agent messaging with TTL, queues, long-poll
- `Agents` scaffold-worker: YAML-driven chain runner, config-driven prompts/skills, per-step tool restrictions, SSE live stream, `#/agents` UI panel, `dispatch_code_task` agent tool
- `Planned` Phase 3A: hybrid memory search (FTS5 + vector retrieval with score fusion)

## What Is Running

Systemd user units:
- `scaffold-signal-cli.service` (Signal bridge)
- `scaffold-daemon.service` (Go daemon: API + Signal handling + Cortex runtime)

Daemon responsibilities:
- Reads Signal messages, stores `conversation_log`, generates replies via tool-use agent (14 tools)
- Hosts API for web UI — dashboard, inbox, desk, notebooks, search, capture, calendar, agents
- Hosts session bus API for cross-agent coordination (`/api/session-bus/*`)
- Runs Cortex scheduler (9 tasks) and bulletin generation in background
- Runs scaffold-worker goroutine — consumes `code_task` from session bus, runs YAML-defined chains via `pi` subprocesses, streams via SSE
- Manages Google Calendar OAuth2 tokens and event proxying
- Persists state in SQLite (`daemon/scaffold.db` by default, configurable)

Frontend:
- `app` is Preact + Vite + Tailwind
- Hash routing: `#/dashboard` (default), `#/inbox`, `#/notebooks`, `#/notebooks/{id}`, `#/search`, `#/agents`
- Talks to daemon API (default `127.0.0.1:46873`)
- Built frontend (`app/dist`) can be served directly by daemon on the same port

## Architecture Snapshot

Five active surfaces over one brain:
- `Agent` (Signal conversation + 14 tool calls)
- `Desktop UI` (Dashboard, Inbox, Notebooks, Search, Agents, Capture)
- `Cortex` (9 periodic maintenance and synthesis tasks)
- `Session Bus` (cross-agent coordination with long-poll)
- `Agents` (YAML-driven chain runner via `pi` subprocesses, registered as `scaffold-worker`)

Shared storage and mutation boundary:
- SQLite tables: `memories`, `edges`, `captures`, `desk`, `domains`, `goals`, `tasks`, `task_completions`, `notes`, `conversation_log`, `sessions`, `memory_centrality`, `memories_fts`, `memory_embeddings`, `embedding_jobs`, `oauth_tokens`, `ingestion_files`, `ingestion_progress`
- Suppression model uses `memories.suppressed_at` (not hard delete by default)
- All writes are funneled through daemon packages (`db`, `brain`, `capture`, `cortex`)

Life domains (`domains` table) organize goals, tasks, and notes. Each domain has health tracking and drift detection (active/drifting/neglected/cold/overactive).

## API (Current)

Public:
- `GET /api/health`
- `POST /api/login`
- `GET /api/auth/check`
- `POST /api/webhook`

Authenticated — core:
- `POST /api/logout`
- `POST /api/capture`
- `POST /api/ingest`
- `GET /api/inbox`
- `POST /api/inbox/{id}/confirm`
- `POST /api/inbox/{id}/override`
- `POST /api/inbox/{id}/archive`
- `PUT /api/inbox/{id}/process`
- `GET /api/memories`
- `GET /api/desk`
- `PATCH /api/desk/{id}`
- `POST /api/desk/{id}/defer`
- `GET /api/dashboard`
- `GET /api/search`
- `GET /api/calendar/upcoming`

Authenticated — domains:
- `GET /api/domains`
- `GET /api/domains/dump`
- `GET /api/domains/{id}`
- `POST /api/domains`
- `PATCH /api/domains/{id}`
- `DELETE /api/domains/{id}`

Authenticated — goals:
- `GET /api/goals`
- `GET /api/goals/{id}`
- `POST /api/goals`
- `PUT /api/goals/{id}`
- `DELETE /api/goals/{id}`

Authenticated — tasks:
- `GET /api/tasks`
- `POST /api/tasks`
- `PUT /api/tasks/{id}`
- `PUT /api/tasks/{id}/complete`
- `PUT /api/tasks/{id}/reorder`
- `DELETE /api/tasks/{id}`

Authenticated — notes:
- `GET /api/notes`
- `GET /api/notes/{id}`
- `POST /api/notes`
- `PUT /api/notes/{id}`
- `DELETE /api/notes/{id}`

Authenticated — session bus:
- `POST /api/session-bus/register`
- `GET /api/session-bus/sessions`
- `POST /api/session-bus/send`
- `POST /api/session-bus/poll`

Authenticated — agents:
- `GET /api/agents/tasks`
- `GET /api/agents/tasks/{id}`
- `DELETE /api/agents/tasks/{id}`
- `GET /api/agents/tasks/{id}/steps/{step_num}/events`
- `GET /api/agents/chains`
- `GET /api/agents/stream` (SSE)
- `POST /api/agents/dispatch`

## Agent Tools

| Tool | Purpose |
|------|---------|
| `save_to_inbox` | Synthesize + persist capture → memory |
| `search_memories` | FTS + hybrid vector search |
| `get_inbox` | Read pending captures |
| `get_calendar_events` | Google Calendar (today/upcoming) |
| `list_sessions` | Session bus listing |
| `send_to_session` | Cross-agent message delivery (optional long-poll reply) |
| `create_goal` | Create goal (binary/measurable/habit) |
| `update_goal` | Partial goal update |
| `list_goals` | Goals with progress |
| `create_task` | Create task (domain+goal scoped) |
| `update_task` | Update task; status=done triggers completion logic |
| `list_tasks` | Filtered task list |
| `create_note` | Create note |
| `dispatch_code_task` | Dispatch coding chain to scaffold-worker (chains defined in coder.yaml) |

## Cortex Tasks

| Task | Interval | Purpose |
|------|----------|---------|
| `bulletin` | 60 min | Synthesizes memory into agent system prompt context |
| `prioritize` | 24h | Populates desk from Todo memories via LLM |
| `consolidation` | 6h | Exact dedup + semantic similarity merge (LLM-assisted) |
| `decay` | 24h | Importance score decay (factor 0.95, floor 0.1) |
| `prune` | 24h | Hard-deletes suppressed memories older than 30 days |
| `reindex` | 12h | Recomputes memory centrality scores |
| `embedding_backfill` | 6h | Batch Ollama embeddings for unembedded memories |
| `observations` | 24h | Generates Observation memories from patterns |
| `drift` | 6h | Classifies domain drift state |
| `session_cleanup` | 24h | Purges expired sessions |

## Config

YAML files in `config/`:
- `agent.yaml` (assistant identity, behavior, response model/tokens)
- `tools.yaml` (14 provider-agnostic tool schemas)
- `triage.yaml` (capture triage prompt/model)
- `cortex.yaml` (bulletin + maintenance task intervals and thresholds)
- `llm.yaml` (provider registry + route/profile model selection for startup binding)
- `embedding.yaml` (Ollama embedding provider config — nomic-embed-text, 768-dim)
- `google.yaml` (Google OAuth2 client config — client_id, client_secret, calendar_id)
- `webhooks.yaml` (webhook auth tokens)
- `coder.yaml` (agent chains: chain definitions, step prompts, CWD allowlist, max_concurrent)
- `coder-prompts/{step}.md` (per-step prompt templates: scout, planner, worker, reviewer, verify)
- `coder-skill.md` (base skill injected into every agent step prompt)
- `coder-skills/{step}.md` (optional per-step skill overrides)

Environment in `daemon/.env`:
- Core runtime: `AGENT_NUMBER`, `USER_NUMBER`, `SIGNAL_URL`, `API_PORT`, `API_TOKEN`
- LLM provider keys are route-dependent from `config/llm.yaml` (for example `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`)
- Browser auth: `APP_USERNAME`, `APP_PASSWORD_HASH`, cookie/session settings
- Paths: `DB_PATH`, `CONFIG_DIR`, `FRONTEND_DIST_DIR`

## Change Providers By Subsystem

Provider/model selection is controlled by `config/llm.yaml` using `providers`, `profiles`, and `routes`.

Routes map to system parts:
- `brain.respond` = Signal conversation reply loop (tool-use)
- `brain.triage` = capture triage JSON classification
- `brain.prioritize` = desk prioritization JSON generation
- `cortex.bulletin` = memory bulletin synthesis
- `cortex.semantic` = consolidation decisions
- `cortex.observations` = observation extraction

How to switch one part:
1. Add or edit a provider in `providers` (for example `anthropic`, `openai`, `openai_compatible`).
2. Add a `profile` that points to that provider and model.
3. Point the target route to that profile in `routes`.
4. Restart daemon: `systemctl --user restart scaffold-daemon.service`.

Example: keep Ollama only for semantic/observations, Anthropic for respond/triage/bulletin.

```yaml
profiles:
  respond_default:
    provider: anthropic_main
    model: claude-haiku-4-5
  ollama_semantic:
    provider: ollama_specialist
    model: qwen2.5:14b-instruct

routes:
  brain.respond:
    profile: respond_default
  brain.triage:
    profile: respond_default
  cortex.bulletin:
    profile: respond_default
  cortex.semantic:
    profile: ollama_semantic
    lock_provider: true
  cortex.observations:
    profile: ollama_semantic
    lock_provider: true
```

## Daily Operations

Restart services:

```bash
cd /home/mikekey/Builds/scaffold/daemon
systemctl --user daemon-reload
systemctl --user restart scaffold-signal-cli.service
systemctl --user restart scaffold-daemon.service
```

Status:

```bash
systemctl --user --no-pager status scaffold-signal-cli.service
systemctl --user --no-pager status scaffold-daemon.service
```

Health check:

```bash
curl -sS http://127.0.0.1:46873/api/health
```

Expected:

```json
{"status":"ok"}
```

Logs:

```bash
journalctl --user -u scaffold-daemon.service -f
journalctl --user -u scaffold-signal-cli.service -f
```

Rebuild daemon binary:

```bash
cd /home/mikekey/Builds/scaffold/daemon
go build -o bin/scaffold-daemon .
systemctl --user restart scaffold-daemon.service
```

## Dev Commands

Backend:

```bash
cd /home/mikekey/Builds/scaffold/daemon
go test ./...
go vet ./...
go build ./...
```

Frontend:

```bash
cd /home/mikekey/Builds/scaffold/app
bun run dev
bun run build
```

## Agents (scaffold-worker) Quick Reference

Chains defined in `config/coder.yaml`. Default 4: `implement` (scout→planner→worker→reviewer) · `fix` (worker→verify) · `spec` (scout→planner) · `single` (worker). Add new chains by editing YAML + creating a prompt .md in `config/coder-prompts/`.

Trigger via Signal: `implement issue #47 — add X` → agent calls `dispatch_code_task`

Run dirs: `/tmp/scaffold-worker/{uuid}/` — `task.md`, `status.json`, `steps/N-{name}/{prompt.md,events.jsonl,output.md}`

Session bus flow: `code_task` → `scaffold-worker` → runs chain → `coder_result` → `scaffold-agent`

Web UI: `#/agents` — live step pipeline + SSE event log + kill button + dynamic chain select

## Key Docs

- `docs/build-context.md`
- `docs/hybrid-memory-system-spec.md` (Phase 3A — hybrid FTS5 + vector search)
- `docs/plans/2026-02-22-session-bus-agent-integration.md` (cross-agent session bus integration + usage)
- `docs/plans/2026-02-22-session-bus-mvp.md`
- `specs/code-agent.md` (scaffold-worker / agents implementation spec)
