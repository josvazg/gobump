# gobump

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Automates Go toolchain bumps with soak-time rules, vulnerability checks, and optional VCS integration. **Automatic library upgrades are planned**; today the tool focuses on the `go` directive and failing loudly when `govulncheck` reports issues.

## What it does

- **Scans modules recursively:** walks the directory tree under the given path (default `.`) and finds every `go.mod`, skipping `vendor/`. There is no separate non-recursive mode yet; pointing gobump at `.` means “all nested modules under this root.”
- **Soak time for the toolchain:** reads stable releases from [go.dev/dl](https://go.dev/dl/) (JSON) and uses GitHub commit metadata to date the `x.y.0` tag line so `gobump` only bumps the `go` directive after your configured soak window (default 90 days).
- **`go` directive bump:** updates `go` in each discovered `go.mod` when the latest stable release satisfies soak (and optional `-skip=major` rules). Runs `go mod tidy` afterward.
- **Vulnerability gate:** runs `go mod tidy` and `govulncheck ./...` when the `go` line already matches latest stable **or** after a soak-driven bump (unless `-skip=govulncheck`). If `govulncheck` fails, gobump **refetches** release metadata; when a **newer stable patch** exists, it bumps the `go` directive to that patch, tidies, and runs `govulncheck` again. If there is **no** newer Go patch (or the second run still fails), the run **fails** and `go.mod` / `go.sum` are reverted—likely a **dependency** issue; automatic library bumps are not implemented yet.
- **Roadmap:** parse `govulncheck` findings and apply safe `go get` upgrades for third-party modules; optional policy for crossing a new **minor** Go line early when only a toolchain fix addresses the finding.
- **Validates** with your `-test` command (default `go test ./...`) before any `-push`.
- **Optionally** commits, pushes, and runs a `-pr` shell command—or rolls back `go.mod` / `go.sum` on failure.

`gobump` does not handle credentials; `git` / `gh` / the network behavior of `go` tools use your existing environment.

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
  -dryrun      print bump decisions without writing files
  -skip        skip steps: all | major | govulncheck | custom
  -custom      extra shell command to run after all bumps, before -test
```

`-custom` runs **after** every module has been bumped and tidied (and after per-module `govulncheck`), but **before** the suite in `-test`, so generators cannot reach `-push` without passing the same gate as a normal change.

## Development

Requires [devbox](https://www.jetify.com/devbox). All tools (Go, govulncheck, golangci-lint, mage) are pinned in `devbox.json`.

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
