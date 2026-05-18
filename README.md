# gobump

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Automates Go toolchain and dependency upgrades with safety gates, vulnerability awareness, and VCS integration.

## What it does

- Bumps the `go` directive in `go.mod` once a new release has soaked for 90 days (configurable).
- Runs `govulncheck` and upgrades any dependency with a reachable ("Called") vulnerability to its fixed version.
- Validates changes by running your test suite before committing anything.
- Optionally commits, pushes, and opens a PR — or rolls back cleanly on failure.

## Install

```sh
go install github.com/josvazg/gobump@latest
```

## Usage

```
gobump [path] [flags]

Flags:
  -push        commit and push changes to current remote/branch
  -pr string   shell command to run after push (e.g. "gh pr create --fill")
  -test string test command (default "go test ./...")
  -soak dur    soak duration before bumping go toolchain (default 90d)
  -protected   comma-separated branches to protect (default "main,master,trunk")
  -force       override branch protection and dirty-tree checks
  -skip        skip steps: all | major | govulncheck | custom
  -custom      extra shell command to run after bumping, before testing
```

## Development

Requires [devbox](https://www.jetify.com/devbox). All tools (Go, govulncheck, golangci-lint, goreleaser, mage) are pinned in `devbox.json`.

```sh
devbox shell          # enter the dev environment
mage test             # run tests
mage build            # build ./gobump
mage check            # test + lint (CI gate)
mage install          # install to GOPATH/bin
```

Available mage targets: `build`, `test`, `lint`, `install`, `check`.

## License

Apache 2.0 — Copyright 2026 MongoDB, Inc. and the gobump contributors.
See [LICENSE](LICENSE).
