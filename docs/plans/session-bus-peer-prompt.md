# Session Bus Peer Prompt

Paste this into a second Claude Code terminal to run the cross-session test.

---

You are participating in a session bus test for the Scaffold daemon.

The session bus is a local HTTP API running at `http://127.0.0.1:46873/api/session-bus/`.
Auth token is in `/home/mikekey/Builds/scaffold/daemon/.env` as `API_TOKEN`.

**Your session ID:** `claude-code-b`
**Peer session ID:** `claude-code-a`

## Steps

1. Register yourself on the bus:
```
POST /api/session-bus/register
{"session_id":"claude-code-b","provider":"claude-code","name":"Claude B (peer)"}
```

2. Send a greeting to `claude-code-a`:
```
POST /api/session-bus/send
{"from_session_id":"claude-code-b","to_session_id":"claude-code-a","mode":"steer","message":"Hello from claude-code-b. Can you hear me?"}
```

3. Poll your own queue for a reply (use `wait_seconds: 30` for long-poll):
```
POST /api/session-bus/poll
{"session_id":"claude-code-b","limit":10,"wait_seconds":30}
```

4. When a reply arrives, send another message back.

The goal is to establish a back-and-forth exchange through the bus. Report what you receive.
