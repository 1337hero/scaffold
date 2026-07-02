# 2026-07-02 — v2 spec → 18 PRDs

## What happened
- Broke `specs/v2-notification-surface.md` (4 build-order phases) into 18 numbered PRDs, written to `specs/v2-prds/01-…18.md`. Template: Problem / Solution / User Stories / Implementation Decisions / Testing Decisions / Out of Scope / Notes.
- Drafted via 4 parallel subagents (one per phase), then stripped Go source paths from bodies (template forbids file paths; YAML config names kept as stable contracts).
- Testing decisions: isolated tests for five deep modules (slipping detector, birthday-window calc, fact store, brief assembler, notification dedup) + standard CRUD tests for new db packages.

## Course correction
Initially published all 18 as GitHub issues with a `ready-for-agent` label — this required ENABLING issues on the repo, which had them disabled. Mike vetoed: issues deleted, issues re-disabled on 1337hero/scaffold, PRDs live as files in `specs/v2-prds/` instead. Lesson recorded: disabled tracker = not the tracker.

## Not done
- specs/v2-prds/ files are uncommitted (working tree also has unrelated dirty files on `v2`).
- The `ready-for-agent` label still exists on the repo (harmless; issues disabled).
