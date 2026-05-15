# gobump — Claude Working Rules

## Test-Driven Development

Write the test first, always. No production code without a failing test that demands it.

## Small, Focused Commits

One logical change per commit. Keep diffs reviewable. Commit messages follow
`type: short description` (e.g. `feat: soak time fetcher`, `test: add modfile parser cases`).

## Testing Strategy

Combine unit tests and end-to-end tests where practical:
- Unit tests for pure logic (version comparison, JSON parsing, flag validation).
- E2E tests against a **fake project environment** — a temporary directory with real
  `go.mod` / `go.sum` files, a bare git repo, and stubbed external commands
  (`govulncheck`, `git push`). No mocking frameworks; use real files and processes.

## Architecture: Thin `main`

`main.go` does exactly three things:

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()
    os.Exit(internal.Run(ctx, os.Args[1:], os.Getenv))
}
```

All logic lives in `internal/`. The runner receives:
- A **cancelable context** (handles Ctrl-C / SIGINT cleanly).
- **CLI args** as `[]string` (keeps `main` testable without subprocess tricks).
- An **env lookup func** `func(string) string` (makes env vars injectable in tests).

## Development Environment

Use **devbox** for all tooling — `devbox shell` gives a reproducible env with Go, govulncheck, golangci-lint, mage, and goreleaser pinned in `devbox.json`.

Use **mage** as the task runner. Prefer `mage <target>` over raw `go` commands so the full pipeline (test → lint → build) is consistent:

| Target | What it does |
|---|---|
| `mage test` | `go test ./...` |
| `mage build` | builds `./gobump` |
| `mage lint` | `golangci-lint run ./...` |
| `mage check` | test then lint (the CI gate) |
| `mage install` | `go install .` |

## Safety First

1. **Test before commit** — `go test ./...` must pass before any `git commit`.
2. **Commit locally before push or PR** — never push uncommitted or dirty state.
3. Branch protection and dirty-tree checks are enforced by the tool itself; respect
   the same discipline when developing it.
