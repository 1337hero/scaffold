#!/usr/bin/env bash
# scaffold-code closeout floor — portable gate for any shell-capable agent. Run from anywhere in the repo.
# note: session ≈ branch; a branch spanning sessions passes on the earlier session's STATE roll.
fail() { echo "FAIL: $1"; exit 1; }
cd "$(git rev-parse --show-toplevel 2>/dev/null)/.scaffold" 2>/dev/null || fail "no .scaffold/ at repo root"

default=$(git symbolic-ref -q --short refs/remotes/origin/HEAD)
default=${default#origin/}
default=${default:-main}
[ "$(git rev-parse --abbrev-ref HEAD)" != "$default" ] || fail "on $default branch"

base=""
for ref in "origin/$default" "$default"; do
  base=$(git merge-base "$ref" HEAD 2>/dev/null) && break
done
[ -n "$base" ] || fail "no $default ref to diff against"

git diff --quiet "$base" -- memory/STATE.md && fail "STATE.md unchanged this session"
ls memory/log/"$(date +%Y-%m-%d)"-*.md >/dev/null 2>&1 || fail "no dated log entry today"
# added lines only; requires a quoted literal value so `apiKey: process.env.X` (reference by name) passes
git diff "$base" | grep -E '^\+' | grep -iqE "(api[_-]?key|secret|password|token)[[:space:]]*[:=][[:space:]]*[\"'][A-Za-z0-9+/_-]{16,}" && fail "possible secret in diff"
echo "OK: closeout floor satisfied"
