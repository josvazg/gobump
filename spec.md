# Project Specification: gobump

**Description:** A CLI tool to automate Go toolchain and dependency upgrades with safety gates, vulnerability awareness, and VCS integration.

---

## 1. Core Logic & Bumping Rules

The tool evaluates every `go.mod` file in the target path (defaulting to current directory or recursive with `./...`).

### A. The "Soak Time" Rule

- Fetch latest Go versions from `https://go.dev/dl/?mode=json&include=all`.
- **Action:** Bump the `go` line in `go.mod` ONLY if the latest stable Go release is older than 3 months (configurable via `-soak`).
- **Exception:** If a vulnerability is found (see below), this rule is overridden to meet security requirements.

### B. Vulnerability-Driven Bumps

- Run `govulncheck -json ./...`.
- Parse the JSON stream (specifically `Finding` objects).
- **Action:** If a "Called" vulnerability exists, identify the `FixedVersion` and execute `go get <module>@<version>`.
- **Constraint:** By default, only perform minor/patch updates. If a fix requires a Major Version change (e.g., `v1` to `v2`), log a warning and skip unless a `--major` flag is provided.

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
| `-soak`      | `90d`                    | Duration to wait after a Go release before auto-bumping. |
| `-protected` | `main,master,trunk`      | Comma-separated list of branches to never push to without `-force`. |
| `-force`     | `false`                  | Overrides branch protection and "dirty tree" checks. |
| `-skip`      | `""`                     | Options: `all`, `major`, `govulncheck`, `custom`. |
| `-custom`    | `""`                     | Extra shell command to run (e.g., `make generate`) after bumping but before testing. |

---

## 3. Execution Pipeline (The "Transaction")

1. **Environment Check:** Ensure the Git tree is clean (unless `-force`). Check if current branch is in the protected list.
2. **Discovery:** Find all `go.mod` files in the target path.
3. **Update Phase:**
   - Update `go` version based on soak time.
   - Update dependencies based on `govulncheck`.
   - Run `go mod tidy`.
   - Execute `-custom` command if provided.
4. **Verification (The Gate):** Run the `-test` command.
5. **Finalization:**
   - **On Failure:** Revert `go.mod` and `go.sum` to original state. Exit with error.
   - **On Success:**
     - If `-push`: `git add`, `git commit -m "chore: gobump updates"`, and `git push`.
     - If `-pr`: Execute the provided PR string.

---

## 4. Technical Implementation Notes

| Concern | Approach |
|---|---|
| Modfile editing | Use `golang.org/x/mod/modfile` to preserve comments and structure. |
| Vulnerability parsing | Use `golang.org/x/vuln/scan` or parse the streaming JSON output of `govulncheck`. |
| Reversion logic | Use `git checkout -- go.mod go.sum` as the primary rollback mechanism. |
| Version comparison | Use `golang.org/x/mod/semver` for all version comparisons. |
