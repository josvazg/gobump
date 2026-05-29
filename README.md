# gobump

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Automates Go toolchain and dependency bumps with soak-time rules, vulnerability checks, and optional VCS integration.

## What it does

- **Scans modules:** by default processes only the `go.mod` in the given directory. Append `/...` to the path (e.g. `./...`) to walk the full subtree, skipping `vendor/` directories.
- **Soak time for the toolchain:** reads stable releases from [go.dev/dl](https://go.dev/dl/) (JSON) and uses GitHub commit metadata to date the `x.y.0` tag, so gobump only bumps the `go` directive after your configured soak window (default 90 days).
- **`go` directive bump:** updates `go` in each discovered `go.mod` when the latest stable release satisfies soak (and optional `-skip=major` rules). Runs `go mod tidy` afterward.
- **Vulnerability gate:** runs `go mod tidy` and `govulncheck ./...` when the `go` line already matches latest stable **or** after a soak-driven bump (unless `-skip=govulncheck`). Govulncheck output is parsed as structured JSON to distinguish finding types:
  - **Library findings** (third-party modules): gobump runs `go get module@fixedVersion` for each affected dependency and then `go mod tidy`.
  - **Stdlib / toolchain findings:** gobump refetches release metadata; if a strictly newer stable patch exists, it bumps the `go` directive, tidies, and re-runs govulncheck.
  - After any automated fix, govulncheck is re-run to confirm clean. If no fix is possible (no newer Go patch, no library fix version) or the re-run still fails, the run **fails** and `go.mod` / `go.sum` are reverted.
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

Requires [Nix](https://nixos.org/) with flakes enabled. All tools (Go, govulncheck, golangci-lint, mage) are pinned in `flake.lock`.

```sh
nix develop           # enter the dev environment
mage test             # run tests
mage build            # build ./gobump
mage ci               # build + test + lint (CI gate)
mage install          # install to GOPATH/bin
```

Available mage targets: `build`, `test`, `lint`, `install`, `ci`.

## License

Apache 2.0 — Copyright 2026 MongoDB, Inc. and the gobump contributors.
See [LICENSE](LICENSE).
