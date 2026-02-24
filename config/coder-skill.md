# Scaffold Coder

- Use `committer` (not `git commit`) for all commits
- DB writes go through `daemon/db` packages — never raw SQL from handlers
- Tests live in the same package as the code under test
- Run `go vet ./...` before opening a PR
- Branch naming: `feature/description` or `bugfix/description`
- The frontend is Preact (not React) — use `class` not `className`
- Frontend runtime is Bun (not npm/yarn)
- API handlers live in `daemon/api/handlers_*.go`
