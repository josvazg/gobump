# 0003: Soak Window for Go Toolchain Upgrades

## Status

Accepted

## Context

Upgrading the `go` directive to the latest stable release immediately exposes projects to brand-new toolchains before the ecosystem has caught up. Some guardrail is needed, while still keeping upgrades automatic.

## Decision

gobump only bumps the `go` directive to the latest stable Go release after a configurable soak window (default 90 days since that release). Cross-minor upgrades can be blocked with `-skip=major`. Release dates are derived from stable release metadata plus GitHub tag commit metadata.

## Consequences

- Projects adopt new Go toolchains only after a proven-in-the-wild period, reducing breakage risk.
- Callers can tune the soak window or block cross-minor upgrades entirely via `-skip=major`.
- Upgrade timing depends on external metadata availability (stable release data and GitHub tag commits).
