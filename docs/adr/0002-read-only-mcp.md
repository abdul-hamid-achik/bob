# ADR-0002: expose a compact read-only MCP surface

- Status: superseded by ADR-0003
- Date: 2026-07-12

## Context

Bob already has a deterministic CLI and JSON contract, but local-agent reaches
specialist tools through MCPHub. Registering only the executable would still
force agents to treat Bob as an opaque shell command. Exposing every CLI command
over MCP would make discovery noisy and could hide filesystem mutation behind a
generic gateway permission.

Current Codemap and Vecgrep status commands also cannot be assumed to be purely
offline: Codemap may open tool-owned stores, while Vecgrep may construct and
contact its configured embedding provider. Bob must not advertise those effects
as an implicit read-only inspection.

## Decision

Bob exposes exactly two stdio MCP tools:

- `bob_inspect`, an offline summary of Bob drift and selected binary
  availability;
- `bob_plan`, a compact projection of the deterministic repository plan.

Both tools are read-only, idempotent, non-destructive, and closed-world. The MCP
surface never runs Codemap or Vecgrep status probes. The CLI offers the explicit
`bob inspect --probe-integrations` authority boundary for those calls.

`bob apply` remains a CLI-only mutation invoked through the agent runtime's
normal approval path. A future MCP apply requires an engine-level plan digest,
stale-plan rejection under the apply lock, and a durable receipt contract.

## Consequences

- MCPHub can register and individually pin Bob's useful agent tools.
- local-agent can inspect and plan without Bob duplicating Cortex investigation.
- Filesystem mutation stays visible to the runtime's existing shell approval
  policy.
- Agents need one explicit shell call to apply a reviewed plan.
- Index freshness is unknown in MCP inventory until specialist tools provide
  guaranteed offline/read-only status contracts.
