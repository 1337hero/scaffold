# Session Bus MVP (Global Cross-Agent Tooling)

Date: 2026-02-22
Status: Validated — Claude ↔ Codex round-trip confirmed 2026-02-22

Integration guide:
- `docs/plans/2026-02-22-session-bus-agent-integration.md`

## Goal
Create a shared local message bus so any agent runtime can coordinate with any other runtime through one consistent interface.

This replaces per-agent ad hoc session sockets with one daemon-hosted hub.

## MVP Scope
- In-memory session registry with heartbeat-by-activity
- Direct message delivery (`from_session_id -> to_session_id`)
- Pull-based receiving with optional long-poll wait
- Shared API endpoints under daemon auth
- Agent tool support for `send_to_session` and `list_sessions`

Out of scope for MVP:
- Persistent queues across daemon restarts
- Per-session ACL policy graph
- Message signatures / per-session auth tokens
- Fanout topics, workflows, retries, dead-letter queues

## Implemented Components

### 1) Bus runtime package
Path: `daemon/sessionbus/bus.go`

Capabilities:
- `Register(session_id, provider, name)`
- `List()`
- `Send(from, to, mode, message)`
- `Poll(session_id, limit, wait)`

Safety defaults:
- Session ID validation (`[a-zA-Z0-9._:-]`, 1-128 chars)
- Bounded per-session queue
- Bounded message bytes
- Stale-session TTL pruning

### 2) Daemon API surface
Path: `daemon/api/session_bus.go`

Authenticated endpoints:
- `POST /api/session-bus/register`
- `GET /api/session-bus/sessions`
- `POST /api/session-bus/send`
- `POST /api/session-bus/poll`

These are intentionally provider-agnostic, so external runtimes can adopt them without coupling to Scaffold internals.

### 3) Agent tool wiring
Paths:
- `daemon/brain/tools.go`
- `config/tools.yaml`

New tools:
- `list_sessions`
- `send_to_session`

`send_to_session` supports optional `wait_seconds` for a simple request/reply pattern.

### 4) Runtime wiring
Path: `daemon/main.go`

- Single `sessionbus.Bus` instance is created at startup
- Wired into both API server and brain tool handlers
- Default agent registration: `scaffold-agent`

## External Agent Integration (Codex/Gemini/Anthropic)
Any external agent can integrate by calling the API:

1. Register itself (`/register`)
2. List sessions (`/sessions`) or target known IDs
3. Send messages (`/send`)
4. Poll for messages (`/poll`, optional long-poll)

This gives immediate cross-provider communication without custom socket protocols.

## Protocol Shape (HTTP JSON)

Register:
```json
{
  "session_id": "codex-main",
  "provider": "codex",
  "name": "Codex Main"
}
```

Send:
```json
{
  "from_session_id": "codex-main",
  "to_session_id": "gemini-worker",
  "mode": "steer",
  "message": "Summarize the latest plan"
}
```

Poll:
```json
{
  "session_id": "codex-main",
  "limit": 10,
  "wait_seconds": 20
}
```

## Known MVP Tradeoffs
- Queue is in-memory only; daemon restart drops state.
- Auth is daemon-level bearer/cookie, not per-session credentials.
- Delivery is at-most-once once polled (no explicit ack/retry layer).
- No loop-prevention metadata yet (hop count/correlation policy not enforced).

## Next Iteration (Recommended)
1. Add optional durable storage for envelopes.
2. Add per-session secret/token and ACL checks.
3. Add correlation IDs and reply-to semantics.
4. Add event streaming transport (SSE/WebSocket) for push delivery.
5. Add policy hooks for max rate, allowed peer matrix, and audit log.

## Future: Broadcast + Gather (Deliberation Pattern)
Fan out a single message to multiple sessions simultaneously, then aggregate responses.
Use case: design decision points — ask Claude, Gemini, Ollama, and a second Claude instance
the same question and collect divergent perspectives before committing to an approach.
Needs: `POST /api/session-bus/broadcast` with a list of target session IDs.
Not needed until a second provider (Gemini, Ollama) is actually wired up.
