# Project Specification: gobump

**Description:** A CLI tool to bump the Go `go` directive using soak-time rules, run safety checks (including `govulncheck`), and optionally integrate with Git. **Full dependency auto-upgrades are planned**; the current release automates the toolchain line and fails clearly when vulnerabilities require manual work.

---

## 1. Core Logic & Bumping Rules

The tool walks the tree under the optional `[path]` argument (default `.`), discovers every `go.mod` (skipping `vendor/`), and processes each module. **Discovery is recursive for the subtree you pass in;** a narrower explicit scope (e.g. only the current module vs nested modules) is a future enhancement.

### A. The "Soak Time" Rule

- Fetch stable Go releases from `https://go.dev/dl/?mode=json&include=all`.
- Enrich the latest stable minor line with the **tag date of `x.y.0`** (via the public GitHub commits API for `golang/go`) so soak is measured from when that minor line first shipped, not from the latest patch.
- **Action:** Bump the `go` line in `go.mod` only when that release is at least `-soak` old (default 90 days).

### B. Vulnerability checking (current)

- When the toolchain is **already at latest stable** (same patch family as `go.dev`’s latest stable tag): still run `go mod tidy` and `govulncheck ./...` (unless `-skip=govulncheck`).
- After a **soak-driven** `go` bump: run `go mod tidy` and `govulncheck ./...`.
- **If `govulncheck` exits non-zero:** refetch stable releases. If a **strictly newer** stable patch exists than the `go` line in `go.mod`, bump to it, tidy, and run `govulncheck` once more.
- **If there is no newer Go patch, or the second `govulncheck` still fails:** abort, print diagnostics, revert `go.mod` / `go.sum` for affected work, and treat the finding as **needing manual dependency work** (automatic library bumps are still roadmap).
- **No automatic `go get` for third-party modules today.**

### C. Minor-line jumps driven only by vulns (planned)

- **Soak** still governs adopting a **new `X.Y` line** when you are behind latest stable.
- **Roadmap:** optionally allow a **cross-minor** `go` bump when `govulncheck` shows only a stdlib/toolchain fix in a newer line, without waiting for soak (policy / flags TBD).
- **Patch-level** relief is implemented via the refetch-and-retry step in §B.

---

## 2. CLI Command Interface

```
gobump [path] [flags]
```

| Flag         | Default                  | Description |
|--------------|--------------------------|-------------|
| `-push`      | `false`                  | If true, commits and pushes changes to `{current-remote}/{current-branch}`. |
| `-pr`        | `""`                     | A shell command to run after pushing (e.g., `-pr="gh pr create --fill"`). |
| `-test`      | `"go test ./..."`        | Command to run for validation. Must exit 0 to proceed. |
| `-soak`      | `90d`                    | Duration to wait after a Go minor line’s `x.y.0` tag before auto-bumping to that line’s latest patch. |
| `-protected` | `main,master,trunk`      | Comma-separated list of branches to never use without `-force`. |
| `-force`     | `false`                  | Overrides branch protection and "dirty tree" checks. |
| `-skip`      | `""`                     | Options: `all`, `major`, `govulncheck`, `custom`. |
| `-dryrun`    | `false`                  | Print what would be done without writing files. |
| `-custom`    | `""`                     | Extra shell command after all bumps (and per-module checks), **before** `-test`. |

---

## 3. Execution Pipeline (The "Transaction")

1. **Environment Check:** Ensure the Git tree is clean (unless `-force`). Check if current branch is in the protected list.
2. **Discovery:** Find all `go.mod` files under the path.
3. **Update Phase (per module):**
   - Fetch latest stable metadata (again after a failed `govulncheck` when retrying with a newer patch).
   - If soak permits and the module is behind latest stable: set `go` to latest stable, then `go mod tidy`.
   - If the module **already** matches latest stable: `go mod tidy` only (no `go` line change yet).
   - Run `govulncheck` unless skipped; on failure, optionally bump to a **newer patch** from a refetch and re-run `govulncheck` once.
4. **Post-bump hook:** If `-custom` is set, run it once from the repository root **after** all modules are processed, **before** `-test`. On failure, revert `go.mod` / `go.sum` in bumped dirs (changes made only by `-custom` are **not** reverted).
5. **Verification:** Run the `-test` command.
6. **Finalization:**
   - **On Failure:** Revert `go.mod` and `go.sum` to original state where applicable. Exit with error.
   - **On Success:**
     - If `-push`: `git add`, `git commit -m "chore: gobump updates"`, and `git push`.
     - If `-pr`: Execute the provided shell string.

---

## 4. Technical Implementation Notes

| Concern | Approach |
|---|---|
| Modfile editing | Use `golang.org/x/mod/modfile` to preserve comments and structure. |
| Release / soak metadata | HTTP GET to go.dev JSON + GitHub commits API for `x.y.0` dates (no credentials in gobump). |
| Vulnerability scanning | Shell out to `govulncheck ./...`; **roadmap:** structured parsing and automated upgrades. |
| Reversion logic | Use `git checkout -- go.mod go.sum` as the primary rollback mechanism. |
| Version comparison | Custom comparison of `go` version tuples (`x.y.z`). |
