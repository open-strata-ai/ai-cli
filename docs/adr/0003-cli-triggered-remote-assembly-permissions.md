# ADR-0003: CLI triggered remote assembly permissions

- **Status**: Accepted — see R-005 in `openstrata-meta/contracts/adr-resolutions.md`
- **Date**: 2026-07-17
- **Suggested by**: OpenStrata Architecture Group
- **Repository**: ai-cli
- **Source**: `docs/DESIGN.md` §14 Open Issue
- **Association**: `ai-platform-api`

##Context

`aictl apply` changes the running state through resolver/provisioner, whose authentication/RBAC is enforced (ai-platform-api?).

## Decision Options (Options Considered)

1. **Maintain status quo / conservative default**: Maintain current behavior, controlled by configuration switches or explicit parameters, and do not introduce destructive changes.
2. **Unified implementation after cross-repository alignment**: Make a clear contract with the relevant service (`ai-platform-api`) before implementation.
3. **Phased introduction**: Leave a placeholder/default switch in the current stage, and solidify it in subsequent stages after the dependent capabilities are ready (see Related Architecture §).

## Recommended decision (Decision)

This ADR solidifies "CLI-triggered remote assembly permissions" as an architectural decision record and incorporates it into `docs/adr/` for continuous tracking. This issue stems from the `docs/DESIGN.md` §14 open issue and is still open.

**Conservative Default Principle**: Before the final decision is made, the "minimum available + explicit configuration switch" shall prevail, maintain the current behavior, and not destroy the existing contract and cross-repository SPI interface; this ADR status will be written back after review by the relevant team.



## To be aligned / Follow-ups (Follow-ups)

- Alignment confirmation with `ai-platform-api`: clarify responsibility boundaries/interface contracts/data flow direction to avoid double writing or semantic drift.
- **Resolution (R-005)**: Accepted — `aictl apply` enforces server-side RBAC at `ai-platform-api`; the CLI holds no authority and presents the caller's OIDC token. Unauthorized requests are rejected with `403`. See `openstrata-meta/contracts/adr-resolutions.md`.

## Traceback

- Upstream design: `docs/DESIGN.md` §14 Open issue
- Relevance index: see `docs/adr/README.md`
