# Cookbook — CLOSE OUT (session end, run once)

A change is a cause: the next session inherits whatever you leave. Leave it clean.

## 1. Roll `STATE.md` forward (living snapshot — edit it to be currently true)
Update `.scaffold/memory/STATE.md` to match reality now: `Current focus`, `In flight`, `Current
branch/PR`, `Last shipped`, `Last verified`, `Known issues` — and **overwrite `Next up`** with
the new queue. STATE owns *what's true now*; don't let history or design rationale accrete here.

**Routing rule — where an update lands (get this wrong and STATE rots):**
- *Perishable status* (what's live, what's open, current focus, next up) → **`STATE.md`**.
- *Historical record* (what happened this session, the proof, derived decisions) → a **log entry**.
- *Durable design* (an invariant, a standard, the architecture) → **`.scaffold/rails/`** or a design doc — never STATE.

## 2. Append a dated log entry (append-only history — never rewrite a past entry)
`.scaffold/memory/log/YYYY-MM-DD-<slug>.md`:
- **Task** and shape (fix / feature / review / investigation).
- **Changed:** the PR, the key files.
- **Verification ledger:** the checks you ran, their result, and any check you skipped + why.
- **Proof:** the test that protects it.
- **Open / next:** the natural next step (feeds STATE's `Next up`).
- **Derived decisions:** anything you derived rather than asked — flag "ratify in PR?" when material.

## 3. Run the closeout self-check (mechanical — fails loudly)
Any shell-capable agent runs this before declaring done. It turns the keystone rails (not on
the default branch, STATE updated, log written, no secrets) from read-and-trusted prose into a gate:

```bash
bash .scaffold/cookbook/closeout-check.sh   # works from anywhere in the repo
```

It diffs against the merge-base with the default branch, so STATE/log changes count whether
committed or not.

## 4. Re-confirm the hard rails held
Branch+PR (not main), no secrets, in scope, green. (A re-check of REVIEW, not a first look.)

## 5. If something genuinely needs the human
State it as a steer — fork + recommendation + reason — not a vague question.

> The memory **is** the project's continuity across sessions, agents, and runtimes. Never end
> with code shipped and `STATE.md` silent.
