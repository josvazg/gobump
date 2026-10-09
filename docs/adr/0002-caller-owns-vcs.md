# 0002: Caller Owns VCS Operations

## Status

Accepted

## Context

gobump modifies project files (e.g., the `go` directive and dependencies in `go.mod`/`go.sum`). Automatically committing, pushing, or opening PRs would couple the tool to a specific VCS workflow and complicate rollback.

## Decision

gobump modifies project files only. It does not perform git operations (commit, push, PR); the caller retains VCS workflow and rollback control.

## Consequences

- gobump stays VCS-agnostic and composable in scripts and pipelines.
- Callers must run `git commit`, `git push`, etc. themselves.
- Rollback is a simple `git checkout` of the touched files.
