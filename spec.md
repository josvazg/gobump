# Project Specification: gobump

**Description:** A CLI tool to bump the Go `go` directive and vulnerable third-party dependencies using soak-time rules and safety checks (including `govulncheck`). gobump is purely a Go toolchain automation tool; it does not interact with version control.

---

## 1. Core Logic & Bumping Rules

The tool processes `go.mod` files under the optional `[path]` argument (default `.`). A plain directory path processes only the `go.mod` directly inside that directory (non-recursive). Appending `/...` (e.g. `./...`) walks the full subtree and discovers every `go.mod`, skipping `vendor/` directories.

### A. The "Soak Time" Rule

- Fetch stable Go releases from `https://go.dev/dl/?mode=json&include=all`.
- Enrich the latest stable minor line with the **tag date of `x.y.0`** (via the public GitHub commits API for `golang/go`) so soak is measured from when that minor line first shipped, not from the latest patch.
- **Action:** Bump the `go` line in `go.mod` only when that release is at least `-soak` old (default 90 days).

### B. Vulnerability checking

Govulncheck is invoked with `-json` so its output is parsed as structured JSON. Each finding carries the affected module name and the minimum fixed version. Findings are partitioned into two categories:

**Library findings** (any module other than `stdlib`):
- Run `go get module@fixedVersion` for each affected module (deduplicating by module, using the highest fixed version when a module appears in multiple findings).
- Run `go mod tidy` once after all `go get` calls.

**Stdlib / toolchain findings** (`module == "stdlib"`):
- Refetch stable releases.
- If a **strictly newer** stable patch exists than the current `go` line, bump the `go` directive to it and run `go mod tidy`.
- If no newer patch is available, this finding cannot be automatically resolved.

Both categories are handled in a single pass. After any automated fix is applied, govulncheck is re-run to confirm clean. If **no fix was possible** or the **re-run still fails**, the run aborts and prints diagnostics. Modified files are left as-is; the caller is responsible for any rollback using their VCS.

Govulncheck runs in two situations:
- When the toolchain is **already at latest stable**: run `go mod tidy` and govulncheck (unless `-skip=govulncheck`).
- **After a soak-driven** `go` bump: run `go mod tidy` and govulncheck.

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
| `-test`      | `"go test ./..."`        | Command to run for validation. Must exit 0 to proceed. |
| `-soak`      | `90d`                    | Duration to wait after a Go minor line's `x.y.0` tag before auto-bumping to that line's latest patch. |
| `-skip`      | `""`                     | Options: `all`, `major`, `govulncheck`, `custom`. |
| `-dryrun`    | `false`                  | Print what would be done without writing files. |
| `-custom`    | `""`                     | Extra shell command after all bumps (and per-module checks), **before** `-test`. |

---

## 3. Execution Pipeline

1. **Discovery:** Find `go.mod` files under the path. Non-recursive by default; append `/...` for recursive walk (skips `vendor/`).
2. **Update Phase (per module):**
   - Fetch latest stable metadata.
   - If soak permits and the module is behind latest stable: set `go` to latest stable, then `go mod tidy`.
   - If the module **already** matches latest stable: `go mod tidy` only (no `go` line change).
   - Run `govulncheck -json` unless skipped; parse findings and apply fixes:
     - Library findings: `go get module@fixedVersion` per module + `go mod tidy`.
     - Stdlib findings: refetch releases; if a newer patch exists, bump `go` directive + `go mod tidy`.
   - If any fix was applied, re-run govulncheck to confirm clean.
   - On failure: exit non-zero. Modified files are left as-is for the caller to inspect and roll back via VCS.
3. **Post-bump hook:** If `-custom` is set, run it once from the repository root **after** all modules are processed, **before** `-test`. On failure, exit non-zero.
4. **Verification:** Run the `-test` command. On failure, exit non-zero.

gobump does not commit, push, or create pull requests. Those steps are the caller's responsibility.

---

## 4. Technical Implementation Notes

| Concern | Approach |
|---|---|
| Modfile editing | Use `golang.org/x/mod/modfile` to preserve comments and structure. |
| Release / soak metadata | HTTP GET to go.dev JSON + GitHub commits API for `x.y.0` dates (no credentials in gobump). |
| Vulnerability scanning | Shell out to `govulncheck -json ./...`; parse structured JSON output to partition stdlib vs library findings and apply targeted fixes. |
| Library upgrades | `go get module@fixedVersion` per affected module, deduplicating by module at the highest required fixed version. |
| Version comparison | Custom comparison of `go` version tuples (`x.y.z`). |
