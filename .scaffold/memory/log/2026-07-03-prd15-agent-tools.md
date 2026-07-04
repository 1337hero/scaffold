# 2026-07-03 — PRD 15: Agent tool set update

**Task:** GitHub issue #36. Agent/tool-contract PRD after People and Library pages.

**Shipped:** PR #56 merged (closes #36).

- `config/tools.yaml` — replaced the model-facing tool contract with the ten PRD15 witness tools: `save_memory`, `search_memories`, `create_task`, `update_task`, `list_tasks`, `create_note`, `get_calendar_events`, `query_people`, `query_facts`, `save_fact`.
- `daemon/brain/tools.go` — added handlers for `save_memory`, `query_people`, `query_facts`, and `save_fact`; removed calendar mutation tools from the default registry; extended task tools for `project_id` and `reminder_at`; extended note creation for `person_id`, `project_id`, and `kind`.
- `search_memories` now searches notes as well as memories, while only memory rows get access-count updates.
- `query_people` supports search, interaction logging, and birthday lookups using the existing people/interaction/birthday DB primitives.
- `query_facts` probes entity facts at the trust floor; `save_fact` writes facts and related-entity edges.
- Runtime config now defaults `brain.respond` / agent fallback model to Sonnet and removes loaded prompt references to removed inbox/email tools.
- Added missing `embedding_jobs` migration table, because `InsertMemory` already enqueues embedding jobs and `save_memory` exposed that path.
- Added focused brain handler tests for registry shape, memory save/search, task project/reminder support, note person/project/kind support, people modes, fact edges, and the five-round tool loop cap.

**YAGNI skips:** no browser E2E for this PRD because the surface is backend/tool-contract only; no vector search; no fact contradiction check; no fact feedback tool; no calendar mutation.

**Verified:** `go test ./...`, `go vet ./...`, `go build ./...`, `bun run build`.

**Next session picks up:** PRD 16 Daily brief (#35).
