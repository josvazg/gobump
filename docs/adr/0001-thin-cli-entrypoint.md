# 0001: Thin CLI Entrypoint

## Status

Accepted

## Context

`main.go` risks accumulating CLI wiring, orchestration, and business logic, making it hard to test.

## Decision

Keep `main.go` thin. Delegate application orchestration to `internal.Run`, passing in a context, CLI args, and an environment lookup function for testability.

## Consequences

- Application behavior is testable without building the binary.
- `main.go` changes rarely; logic changes live in `internal/`.
- Slight indirection between the entrypoint and orchestration code.
