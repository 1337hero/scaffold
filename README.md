# Scaffold

Agent-driven executive function system:
- Signal conversation as the front door
- Cortex runtime for background memory maintenance
- Desktop UI as the presentation layer
- SQLite brain as the shared source of truth

## Current Status

Core build phases are in place:
- `Phase 2A` foundation: config loader, conversation log, suppression model, mutation APIs in DB layer
- `Phase 2B` agent rewrite: tool-use conversational loop + config-driven prompts/tools
- `Phase 2C` cortex runtime: singleton scheduler + bulletin cache + boot wiring
- `Phase 2D` inbox/desktop wiring: confirm/override/archive endpoints + frontend actions
- `Phase 2E` maintenance tasks: prioritize/session cleanup/consolidation/decay/prune/reindex

## What Is Running

Systemd user units:
- `scaffold-signal-cli.service` (Signal bridge)
- `scaffold-daemon.service` (Go daemon: API + Signal handling + Cortex runtime)

Daemon responsibilities:
- Reads Signal messages, stores `conversation_log`, generates replies via tool-use agent
- Hosts API for web UI auth/inbox/desk/capture
- Hosts session bus API for cross-agent coordination (`/api/session-bus/*`)
- Runs Cortex scheduler and bulletin generation in background
- Persists state in SQLite (`daemon/scaffold.db` by default, configurable)

Frontend:
- `app` is Preact + Vite + Tailwind
- Talks to daemon API (default `127.0.0.1:46873`)
- Built frontend (`app/dist`) can be served directly by daemon on the same port

## Architecture Snapshot

Three active surfaces over one brain:
- `Agent` (Signal conversation + tool calls)
- `Desktop UI` (Inbox, Desk, Capture, auth)
- `Cortex` (periodic maintenance and synthesis)

Shared storage and mutation boundary:
- SQLite tables include `memories`, `edges`, `captures`, `desk`, `conversation_log`, `sessions`, `memory_centrality`
- Suppression model uses `memories.suppressed_at` (not hard delete by default)
- All writes are funneled through daemon packages (`db`, `brain`, `capture`, `cortex`)

## API (Current)

Public:
- `GET /api/health`
- `POST /api/login`
- `GET /api/auth/check`

Authenticated:
- `POST /api/logout`
- `GET /api/inbox`
- `POST /api/inbox/{id}/confirm`
- `POST /api/inbox/{id}/override`
- `POST /api/inbox/{id}/archive`
- `GET /api/memories`
- `GET /api/desk`
- `PATCH /api/desk/{id}`
- `POST /api/desk/{id}/defer`
- `POST /api/capture`
- `POST /api/session-bus/register`
- `GET /api/session-bus/sessions`
- `POST /api/session-bus/send`
- `POST /api/session-bus/poll`

## Config

YAML files in `config/`:
- `agent.yaml` (assistant identity, behavior, response model/tokens)
- `tools.yaml` (provider-agnostic tool schemas)
- `triage.yaml` (CaptureModal triage prompt/model)
- `cortex.yaml` (bulletin + maintenance task intervals and thresholds)
- `llm.yaml` (provider registry + route/profile model selection for startup binding)

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

## Key Docs

- `docs/build-context.md`
- `docs/Scaffold — Build Roadmap.html`
- `docs/three-surfaces-architecture.html`
- `docs/ref/go-dhh-principles.md`
- `docs/plans/2026-02-22-session-bus-agent-integration.md` (cross-agent session bus integration + usage)
