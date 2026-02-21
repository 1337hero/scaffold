# Webhook Ingestion — Design

**Date:** 2026-02-21
**Status:** Approved

## Summary

Generic authenticated webhook endpoint that accepts captures from external services and routes them through the existing triage pipeline into the inbox.

## Goals

- Accept structured captures from any external service (fitness apps, GitHub, homelab health checks, iOS Shortcuts, cron scripts)
- Per-integration tokens so each source can be revoked independently
- Rate limiting per token to prevent abuse
- Lands in inbox for user review — same as Signal/web captures
- Source identity derived from token name, not caller-supplied

## Non-Goals

- Source-specific payload parsers (e.g. raw GitHub event format) — future work
- DB-backed token management — file config is sufficient for personal use
- Hot-reload of token config — daemon restart on rotation is acceptable

## API

```
POST /api/webhook
Authorization: Bearer <webhook-token>
Content-Type: application/json

{
  "title": "Weight logged",   // optional
  "content": "185 lbs"        // required
}
```

**Responses:**
- `202 Accepted` — `{"id": "<capture-id>", "source": "fitness"}`
- `400 Bad Request` — missing/empty content
- `401 Unauthorized` — missing or invalid token
- `429 Too Many Requests` — rate limit exceeded, includes `Retry-After` header

Source tag in the capture record: `"webhook:<token-name>"` (e.g. `webhook:fitness`). Callers cannot override this.

## Config

`config/webhooks.yaml` — gitignored. `config/webhooks.yaml.example` committed.

```yaml
rate_limit:
  max: 60
  window_minutes: 60

tokens:
  fitness:  <random-token>
  homelab:  <random-token>
  github:   <random-token>
```

- File is optional. If absent, `POST /api/webhook` returns `503 Service Unavailable`.
- Loaded at startup. Restart daemon to rotate tokens.
- `rate_limit` is global, applied per token independently.

## Pipeline

```
POST /api/webhook
  → validate Bearer token against webhooks.yaml
  → check per-token rate limit
  → format text: "[title]\n\n[content]" or just content
  → capture.Ingest(ctx, db, brain, text, "webhook:<name>")
  → triage → inbox (same as web capture)
  → 202 with {id, source}
```

Triage failure is degraded mode — capture still lands in inbox with default classification.

## Rate Limiting

In-memory per-token window counter, same pattern as the existing login rate limiter in `api/auth.go`. Config from `webhooks.yaml` with defaults (60 requests / 60 minutes). Exceeded limit returns `429` with `Retry-After: <seconds>` header.

## Files

| File | Action |
|------|--------|
| `config/webhooks.yaml` | New, gitignored |
| `config/webhooks.yaml.example` | New, committed |
| `.gitignore` | Add `config/webhooks.yaml` |
| `daemon/config/webhooks.go` | New — config loader |
| `daemon/config/webhooks_test.go` | New — loader tests |
| `daemon/api/webhook.go` | New — handler + rate limiter |
| `daemon/api/webhook_test.go` | New — handler tests |
| `daemon/api/server.go` | Register route, wire webhook config |

## Security Notes

- Tokens stored in file, not env — excluded from git via `.gitignore`
- Bearer token comparison uses `crypto/subtle.ConstantTimeCompare`
- Source derived from token name — no caller-supplied source field to prevent spoofing
- Rate limiter prevents bulk replay
- Future: HMAC signature verification for GitHub-style webhooks
