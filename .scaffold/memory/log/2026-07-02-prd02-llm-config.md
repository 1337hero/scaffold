# Session log — PRD 02: LLM config rework

**Date:** 2026-07-02  
**Task:** PRD 02 — LLM config rework (llama.cpp provider, dead route removal, cortex.semantic route)  
**Shape:** Feature (config + Go constants)

## Changed files

| File | Change |
|------|--------|
| `config/llm.yaml` | Added `llama_cpp` provider (openai_compatible, localhost:8080/v1), `background_llama` profile (qwen2.5-7b-instruct), `cortex.semantic` route; removed `background_haiku` profile (no more cloud Haiku for background tasks) |
| `daemon/config/llm.go` | Added `LLMRouteCortexSemantic` constant |
| `daemon/llm/runtime.go` | Added `cortex.semantic` to `requirementForRoute` (completionText) |
| `daemon/config/config_test.go` | Removed dead routes from test YAML strings (brain.triage, brain.prioritize, cortex.observations); updated base_url to 8080 for llama.cpp; updated model to qwen2.5-7b-instruct |
| `README.md` | Updated Stack, Prerequisites, and Quick Start to replace Ollama with llama.cpp |

## Verification ledger

| Check | Result |
|-------|--------|
| `go build ./...` | Pass |
| `go vet ./...` | Pass |
| `go test ./...` | Pass — all 5 packages green |
| Config load (`Load(configDir, "Mike")`) | Pass — 2 providers, 2 profiles, 3 routes |
| Runtime BindResponder(brain.respond) | Pass → model=claude-sonnet-4-6 |
| Runtime BindCompletion(cortex.bulletin) | Pass → model=qwen2.5-7b-instruct |
| Runtime BindCompletion(cortex.semantic) | Pass → model=qwen2.5-7b-instruct |

## Derived decisions

- **cortex.semantic route defined but not wired into consolidation task.** The consolidation handler in `cortex.go` currently does exact dedup only (no LLM call). The route exists in config so semantic merging can use it when implemented. If this needs wiring now, that's a separate scope addition.
- **llama_cpp model chosen as qwen2.5-7b-instruct** — a capable local model for bulletin synthesis and consolidation. The user can override via `config/llm.yaml` or `LLM_ROUTE_*` env vars. Ratify in PR.

## Open / next

1. PRD 03: v2 schema — new tables and column modifications
2. Google Calendar re-auth (blocked on user action: revoke at myaccount.google.com/permissions)
