# Ingestion Requirements (Scaffold)

## Pipeline Facts

- Poll-based ingestion loop runs on a timer (default 30 seconds).
- Input comes from the ingest folder or `POST /api/ingest`.
- Files are processed oldest-first by modified time.
- Content is chunked by line boundaries at 4000 characters max per chunk.
- Dedupe and resume rely on chunk content hashing and SQLite progress tables.
- Source files are deleted only after full successful processing.

## Supported Inputs

- Text-like files: `.txt`, `.md`, `.markdown`, `.json`, `.yaml`, `.yml`, `.csv`, `.log`
- PDF files: `.pdf` (requires `pdftotext` in runtime environment)

## Content Constraints For Best Results

- Prefer UTF-8 plain text or Markdown.
- Keep one coherent topic per file.
- Keep files concise enough to preserve context signal.
- Use explicit headings and short bullets where possible.
- Avoid giant unstructured dumps when smaller topical files are possible.

## Content To Prioritize

- Facts with durable relevance
- Explicit decisions and rationale
- Constraints and non-negotiables
- Preferences and operating norms
- Commitments and deadlines
- Material events affecting plans

## Content To Exclude

- Secrets or private credentials
- Repetitive chatter and filler
- Verbose boilerplate
- Ambiguous claims presented as certainty

## Preferred Section Pattern

- `# Title`
- `## Summary`
- `## Facts`
- `## Decisions`
- `## Constraints`
- `## Open Loops`
- `## Suggested Tags`
