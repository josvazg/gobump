# gobump

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Automates Go toolchain and dependency bumps with soak-time rules and vulnerability checks.

## What it does

- **Scans modules:** the path argument defaults to `.` and recursively finds every `go.mod` in the subtree, skipping `vendor/` directories. Append `/...` to a path (e.g. `./...`) for the same recursive walk; a bare path (e.g. `.` or `sub`) checks only that directory's `go.mod`.
- **Soak time for the toolchain:** reads stable releases from [go.dev/dl](https://go.dev/dl/) (JSON) and uses GitHub commit metadata to date the `x.y.0` tag, so gobump only bumps the `go` directive after your configured soak window (default 90 days).
- **`go` directive bump:** updates `go` in each discovered `go.mod` only when the latest stable release has completed the soak window (and optional `-skip=major` rules allow the bump). Runs `go mod tidy` afterward.
- **Vulnerability gate:** runs `govulncheck ./...` while a module is soaking or when its `go` line already matches latest stable (unless `-skip=govulncheck`). `go mod tidy` runs in the at-latest health check and as needed for automated fixes; it is not run merely because a module is soaking. govulncheck is skipped right after a successful bump to the latest Go release, since the scan would target the previous module graph. With `-dryrun`, no scans or writes happen — decisions are only printed. Govulncheck output is parsed as structured JSON to distinguish finding types:
  - **Library findings** (third-party modules): gobump runs `go get module@fixedVersion` for each affected dependency and then `go mod tidy`.
  - **Stdlib / toolchain findings:** gobump refetches release metadata; if a strictly newer stable patch exists, it bumps the `go` directive, tidies, and re-runs govulncheck.
  - After any automated fix, govulncheck is re-run to confirm clean. If no fix is possible or the re-run still fails, gobump exits non-zero. Any modified files are left as-is; use your VCS to roll back if needed.
- **Validates** with your `-test` command (default `go test ./...`). Test and `-custom` steps run only when at least one module's `go.mod`/`go.sum` actually changed — for example, vulnerability remediation may modify dependencies while soak still blocks the toolchain bump.
- **Optionally** runs a `-custom` shell command after the changed modules are processed, before `-test`.

gobump does not touch version control. Commit, push, and PR creation are left to the caller.

## Install

```sh
go install github.com/josvazg/gobump@latest
```

> **Note:** gobump uses `govulncheck` from the target module's `tool` directive when available; otherwise it requires `govulncheck` on PATH. To install it as a fallback:

```sh
go install golang.org/x/vuln/cmd/govulncheck@latest
```

## Usage

```
gobump [path] [flags]

Flags:
  -test string  test command (default "go test ./...")
  -soak dur     soak duration before bumping go toolchain (default 90d)
  -dryrun       print bump decisions without writing files
  -skip         skip steps: all | major | govulncheck | custom
  -custom       extra shell command to run after all bumps, before -test
```

`-custom` runs **after** any module changes (bumps, tidies, or vulnerability remediation that modified `go.mod`/`go.sum`), but **before** the suite in `-test`, so generators cannot reach the test gate without passing the same validation as a normal change. If nothing changed, both `-custom` and `-test` are skipped.

## CI integration example

```sh
gobump ./... -soak=30d -test="make test" && \
  git add -u && git commit -m "chore: gobump updates" && git push
```

## Development

### Nix setup

If you have [Nix](https://nixos.org/) with flakes enabled basic tools (Go, mage & git) are pinned in `flake.lock`.

```sh
nix develop           # enter the dev environment
```

### Custom setup

You will need to install [Go](https://go.dev/dl) and [mage](https://magefile.org/). Or you can just install Go and use `mage` as `go tool mage ...`:

### Mage flow

```sh
mage test             # run tests
mage build            # build ./gobump
mage ci               # build + test + lint (CI gate)
mage install          # install to GOPATH/bin
```

Available mage targets: `build`, `test`, `lint`, `install`, `ci`.

## License

Apache 2.0 — Copyright 2026 MongoDB, Inc. and the gobump contributors.
See [LICENSE](LICENSE).
