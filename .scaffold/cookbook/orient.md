# Cookbook — ORIENT (session boot, run once)

Build the **minimum** context to work this session well, without bulk-loading the repo. Pull,
don't dump. Run this **once** when the session starts — not per task (you don't re-orient for a
one-line fix; you oriented at boot).

1. **Read `.scaffold/memory/STATE.md` in full** — current focus, `Next up`, what's in flight, `Current
   branch/PR`, `Last verified`, known issues, and the `Key files / commands` block.
2. **Note the build/test/deploy commands** (STATE's `Key files / commands`, or
   `.scaffold/rails/standards.md` → Stack notes). This is the single highest-value thing you assemble —
   without it, every session rediscovers test/lint/dev/CI names and app-boot behavior. Hold them.
3. **Read `.scaffold/rails/`** — the standards you must hold this session.
4. **Skim the latest 1–2 entries in `.scaffold/memory/log/`** (`ls -t .scaffold/memory/log/`) — what was just done,
   what was left open. STATE's `Next up` is the authoritative pointer; the log is the detail.
5. **Grep for your area, then read the 2–3 most similar existing implementations** before writing
   anything new — match the patterns already here; don't invent a parallel one.
6. **Reconcile against reality** — `git status`/`log`, CI, the running code. If STATE or a doc
   contradicts the code, the **code wins on facts**; note the drift so closeout fixes STATE.

**Output:** a tight working context, the project's commands in hand, and the ability to state
what "done" looks like for the session's first task before you edit anything.
