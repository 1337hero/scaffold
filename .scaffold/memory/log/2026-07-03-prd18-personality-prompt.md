# 2026-07-03 — PRD 18: Agent personality and system prompt assembly

**Task:** GitHub issue #38. Final v2 PRD: identity, prompt assembly, surface detection, fact injection, and fact trust/conflict loop.

**Shipped:** PR #59 merged (closes #38).

- `config/agent-identity.yaml` — new static identity config for voice, values, posture, cannot-do boundaries, identity rules, and prompt fact limit.
- `daemon/agentprompt` — new pure prompt package with `AssembleSystemPrompt` and `DetectSurface`; tests cover identity, bulletin, surface, high-trust facts, cannot-do boundaries, explicit switches, time fallback, calendar context, and query inference.
- `daemon/brain` — response prompts now assemble from identity, cortex bulletin, detected surface, and injected prompt facts; Google Calendar titles are used for detection when available and ignored safely when unavailable.
- `daemon/db/facts.go` — added `PromptFacts` with trust-floor filtering, relevance ranking, top-N limiting, and `retrieval_count` bumps for injected facts.
- `save_fact` now checks same-entity conflicts with conservative negation/exclusive-value heuristics and reports conflicts without saving until Mike decides.
- Existing trust feedback endpoint remains the PRD18 feedback loop: helpful +0.05 and `helpful_count`, unhelpful -0.10, clamped 0.0-1.0.
- Retired stale Gmail triage prompt text in `config/agent-email.md`.
- Removed automatic webhook task creation instructions from the runtime agent prompt; webhook events now ask before task creation.

**YAGNI skips:** no LLM-based contradiction detection; no dynamic personality learning; no multi-user identity; no fact expiration; no browser E2E because this PRD is backend/config/prompt behavior.

**Verified:** `go test ./...`, `go vet ./...`, `go build ./...`, `bun run build`.

**Next session picks up:** v2 PRD loop is complete. Re-auth Google Calendar, then decide the next track (Phase 3/frontend cleanup or runtime deployment).
