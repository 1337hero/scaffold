---
name: prepare-ingest-content
description: Prepare raw business notes, transcripts, logs, and planning docs into ingestion-ready Markdown briefs for Scaffold memory ingestion. Use when Codex needs to distill durable context, remove noise, redact secrets, preserve uncertainty, and produce high-signal files suitable for `ingest/` or `POST /api/ingest`.
---

# Prepare Ingest Content

## Overview

Convert messy source material into concise, durable memory briefs that ingest cleanly and retrieve well later.

Read `references/ingestion-requirements.md` first, then use the template in `assets/templates/` that best matches the source material.

## Workflow

1. Identify the source type and intended use:
- Choose one: business context, project brief, people context, or mixed operations updates.
- Pick the matching template under `assets/templates/`.

2. Extract durable context only:
- Keep goals, decisions, constraints, preferences, commitments, important facts, and key events.
- Drop repetition, social chatter, and low-signal status noise.

3. Apply safety and truthfulness checks:
- Redact secrets and credentials.
- Label uncertain items as `Unverified`.
- Keep claims concrete and attributable to source notes.

4. Normalize output structure:
- Use sections from the selected template.
- Add `Suggested Tags` as comma-separated values.
- Keep each output file focused on one coherent topic.

5. Optimize for ingestion mechanics:
- Keep files plain UTF-8 Markdown.
- Prefer 300-1200 words per file.
- Preserve meaningful line breaks and headings.

6. Deliver the final artifact:
- Return transformed Markdown only unless the user asks for commentary.
- Suggest filename pattern: `YYYY-MM-DD-topic-brief.md`.

## Output Contract

- Include these sections unless the chosen template defines tighter structure:
- `# Title`
- `## Summary` (3-6 bullets)
- `## Facts`
- `## Decisions`
- `## Constraints`
- `## Open Loops`
- `## Suggested Tags`

## Template Selection

- `assets/templates/business-context.md`: Quarterly focus, priorities, risks, and decisions.
- `assets/templates/project-brief.md`: Scope, milestones, owners, dependencies, and execution plan.
- `assets/templates/people-context.md`: Stakeholders, relationship context, preferences, and commitments.

If the source mixes topics, split into multiple files using the most specific template for each file.

## Ingestion Guardrails

- Do not include passwords, keys, tokens, private credential material, or full secrets.
- Keep personal data minimal and purpose-bound.
- Avoid storing speculative conclusions as facts.

## Quick Prompt

Use this when the user asks for a one-shot transform:

```text
Transform this source into an ingestion-ready Markdown brief.
Use durable context only (facts, decisions, constraints, commitments, preferences, key events).
Remove noise and duplicates, redact secrets, and mark uncertainty as "Unverified".
Use the appropriate template from assets/templates and return Markdown only.
```
