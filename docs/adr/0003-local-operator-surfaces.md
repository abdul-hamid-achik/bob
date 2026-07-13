# ADR-0003: add local operator surfaces and a richer read-only MCP contract

- Status: accepted
- Date: 2026-07-12
- Supersedes: ADR-0002's exact two-tool surface

## Context

The initial two-tool MCP projection proved that Bob could expose deterministic
inspection and planning without hiding repository mutation. Daily use also
needs a convergence check, manifest validation before repository creation,
recipe discovery, and product health signals that both users and agents can
consume. A terminal operator needs to correlate the same information without
assembling several command outputs.

Hosted analytics would violate Bob's local-first boundary and risk collecting
repository identifiers or free-form errors. Allowing arbitrary workspace paths
by default would make a gateway registration broader than it appears.

## Decision

Bob adds three coordinated local operator surfaces:

1. Strict XDG-style user settings and disabled-by-default local telemetry. The
   telemetry schema is closed and cannot represent paths, argv, filenames,
   content, free-form labels, or raw errors. Workspace identity is a
   machine-local HMAC pseudonym. There is no network transport.
2. `bob studio`, a read-only Overview, Plan, and aggregate Stats TUI over one
   coherent offline snapshot. It exposes no repository mutation or specialist
   subprocess action.
3. Six typed MCP tools: inspect, plan, check, validate-manifest,
   recipe-describe, and aggregate stats. Repository mutation remains CLI-only.

MCP starts with an exact allowlist containing its canonical startup workspace.
Operators may add exact paths with repeatable `--allow-workspace` flags. The
name `--allow-any-workspace` is reserved for an explicit broad read-authority
choice.

All six MCP tools retain read-only, non-destructive, idempotent, closed-world
annotations for their target repository effects. When telemetry is opted in,
operations other than stats may append a privacy-bounded event to Bob's XDG
state. This side effect is documented separately from repository mutation.

## Consequences

- People and agents see the same deterministic plan and convergence concepts.
- Product usage can be understood offline without transmitting repository
  information or exposing individual events through public commands.
- Studio remains safe to open during investigation because it cannot apply or
  launch specialist tools.
- A multi-repository MCPHub registration must visibly choose broader authority;
  least-privilege registrations remain exact by default.
- `bob apply` still requires the normal approved shell path. A future mutating
  MCP tool requires its own digest, stale-plan, receipt, and authorization ADR.
