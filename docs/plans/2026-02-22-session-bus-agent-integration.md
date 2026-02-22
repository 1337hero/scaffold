# Session Bus Agent Integration Guide

Date: 2026-02-22  
Status: Active MVP

## Purpose
This guide explains how other agents and runtimes (Codex, Gemini, Anthropic, custom bots) should integrate with Scaffold's session bus for local multi-agent communication.

The session bus is implemented in the daemon and exposed over authenticated HTTP.

## What It Is
Scaffold runs a single in-memory message bus with:
- session registration
- session discovery
- point-to-point message delivery
- long-poll message receive

Implementation paths:
- Runtime bus: `daemon/sessionbus/bus.go`
- API handlers: `daemon/api/session_bus.go`
- Route registration: `daemon/api/server.go`
- Scaffold agent tools: `daemon/brain/tools.go`
- CLI helper: `daemon/cmd/sessionctl/main.go`

## Transport and Auth
- Base URL: `http://127.0.0.1:$API_PORT` (default port is typically `46873`)
- Auth: `Authorization: Bearer $API_TOKEN`
- Content type: `application/json`

All session bus endpoints are authenticated daemon API routes.

## Session Model
Each agent instance should have one stable session id while it runs.

Session fields:
- `session_id`: unique identifier for the running agent instance
- `provider`: runtime family (`codex`, `gemini`, `anthropic`, `scaffold`, etc.)
- `name` (optional): human-friendly display name

Session ID validation:
- regex: `^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$`
- max length: 128 chars

## API Endpoints

### `POST /api/session-bus/register`
Registers or refreshes a session heartbeat.

Request:
```json
{
  "session_id": "codex-main",
  "provider": "codex",
  "name": "Codex Main"
}
```

Response:
```json
{
  "session": {
    "session_id": "codex-main",
    "provider": "codex",
    "name": "Codex Main",
    "queue_depth": 0,
    "last_seen_at": "2026-02-22T18:32:00Z"
  }
}
```

### `GET /api/session-bus/sessions`
Lists currently active sessions.

Response:
```json
{
  "sessions": [
    {
      "session_id": "codex-main",
      "provider": "codex",
      "name": "Codex Main",
      "queue_depth": 0,
      "last_seen_at": "2026-02-22T18:32:00Z"
    }
  ]
}
```

### `POST /api/session-bus/send`
Sends one message from one session to another.

Request:
```json
{
  "from_session_id": "codex-main",
  "to_session_id": "gemini-worker",
  "mode": "steer",
  "message": "Summarize these notes"
}
```

Response:
```json
{
  "message": {
    "id": "8f9f4f2e-7f0a-4c20-b6be-6ad9b4c6ab19",
    "from_session_id": "codex-main",
    "to_session_id": "gemini-worker",
    "mode": "steer",
    "message": "Summarize these notes",
    "created_at": "2026-02-22T18:32:14Z"
  }
}
```

### `POST /api/session-bus/poll`
Receives queued messages for a session (optionally long-polling).

Request:
```json
{
  "session_id": "gemini-worker",
  "limit": 10,
  "wait_seconds": 30
}
```

Response:
```json
{
  "messages": [
    {
      "id": "8f9f4f2e-7f0a-4c20-b6be-6ad9b4c6ab19",
      "from_session_id": "codex-main",
      "to_session_id": "gemini-worker",
      "mode": "steer",
      "message": "Summarize these notes",
      "created_at": "2026-02-22T18:32:14Z"
    }
  ]
}
```

## Runtime Limits and Behavior (MVP)
- Queue per target session is bounded (default/current runtime config: 128 messages).
- Oldest messages are dropped when queue overflows.
- Message size is capped (default/current runtime config: 32 KB).
- Poll `limit` is capped internally (50 max).
- Poll `wait_seconds` is capped internally (120 max).
- Sessions are pruned when stale (runtime currently configured to ~15 minutes in `daemon/main.go`).
- Queue is in-memory only (daemon restart clears sessions/messages).

## Error Codes
- `400`: invalid input (`session_id`, payload shape, message too large, etc.)
- `401`: missing/invalid bearer token
- `404`: target session not found
- `503`: session bus not configured

## Recommended Integration Pattern

### 1) Startup
1. Generate/select local session id.
2. `register`.
3. Start a poll loop.

### 2) Poll loop
1. `poll` with `wait_seconds` (20-60 is typical).
2. For each message:
   - pass `message` to your local model/runtime
   - optionally respond using `send`

### 3) Heartbeat
Re-run `register` periodically (for example every 60-120s) even if idle.

### 4) Routing discipline
- Use stable naming conventions for ids, for example:
  - `codex-main`
  - `gemini-worker-1`
  - `anthropic-planner`

## Request/Reply Convention
The bus payload is plain text. If you need richer semantics, encode JSON into `message`.

Example message envelope:
```json
{
  "type": "task",
  "request_id": "req-123",
  "reply_to": "codex-main",
  "body": "Summarize latest deployment notes"
}
```

Receiver can parse this and respond with:
```json
{
  "type": "result",
  "request_id": "req-123",
  "status": "ok",
  "body": "Summary text..."
}
```

## `sessionctl` CLI (Recommended for Adapters/Scripts)
Build:
```bash
cd /home/mikekey/Builds/scaffold/daemon
go build -o bin/sessionctl ./cmd/sessionctl
```

Register:
```bash
./bin/sessionctl register --session-id codex-main --provider codex --name "Codex Main"
```

List:
```bash
./bin/sessionctl list
```

Send:
```bash
./bin/sessionctl send \
  --from codex-main \
  --to gemini-worker \
  --mode steer \
  --message "Summarize latest notes"
```

Poll:
```bash
./bin/sessionctl poll --session-id codex-main --wait-seconds 30
```

JSON output mode:
```bash
./bin/sessionctl list --json
```

Common flags:
- `--base-url` (default: `$SCAFFOLD_API_BASE_URL` or `http://127.0.0.1:$API_PORT`)
- `--api-token` (default: `$API_TOKEN`)
- `--timeout` (default: `20s`)
- `--json`

## Minimal Curl Examples
```bash
BASE_URL="http://127.0.0.1:46873"
TOKEN="$API_TOKEN"

curl -sS -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"session_id":"codex-main","provider":"codex","name":"Codex Main"}' \
  "$BASE_URL/api/session-bus/register"
```

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/api/session-bus/sessions"
```

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"from_session_id":"codex-main","to_session_id":"gemini-worker","mode":"steer","message":"ping"}' \
  "$BASE_URL/api/session-bus/send"
```

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"session_id":"gemini-worker","limit":10,"wait_seconds":30}' \
  "$BASE_URL/api/session-bus/poll"
```

## Agent-Specific Notes

### Codex adapter
- Register at startup.
- On each poll message, inject message text into your Codex task loop.
- Send replies back to `from_session_id`.

### Gemini adapter
- Same pattern as Codex.
- If your worker is asynchronous, use one poll thread + one model worker queue.

### Anthropic adapter
- Same pattern as Codex/Gemini.
- Keep token/timeout control outside the bus. The bus only routes payloads.

## Current MVP Tradeoffs
- No durable persistence.
- No per-session ACL/token layer.
- No delivery receipts or retries.
- No push transport (SSE/WebSocket) yet.

For architecture and roadmap context, see:
- `docs/plans/2026-02-22-session-bus-mvp.md`
