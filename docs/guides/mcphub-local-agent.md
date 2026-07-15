---
description: Wire Bob's nine read-only MCP tools into MCPHub and local-agent, and keep digest-gated apply on the approved shell path.
---

# MCPHub & local-agent

Bob exposes nine typed MCP tools for repository context, exact ownership,
closed guidance, validation, planning, convergence, recipe discovery, and
aggregate local usage. None of them mutate a repository or run specialist
probes. Repository mutation stays on the agent runtime's normal approved shell
path, with a human or policy watching.

## Tools

| Tool | Purpose | Effect |
|---|---|---|
| `bob_context` | Return a bounded workspace contract and current plan identity. | Repository-read-only and offline; defaults to `compact`. |
| `bob_path` | Classify one exact repository-relative path. | Repository-read-only; returns no file body. |
| `bob_playbook` | `list`, `show`, or `plan` a closed typed procedure. | Repository-read-only; never executes a step. |
| `bob_inspect` | Return Bob state and offline availability of selected binaries. | Read-only; never runs specialist probes. |
| `bob_plan` | Return a bounded deterministic plan and digest. | Repository-read-only; omits desired-content previews. |
| `bob_check` | Return convergence, conflict, and lock-drift state. | Repository-read-only; shares the complete-plan digest. |
| `bob_validate_manifest` | Strictly validate workspace or bounded inline YAML. | Repository-read-only; never writes a manifest. |
| `bob_recipe_describe` | Describe the embedded recipe contract. | Read-only; does not require a workspace. |
| `bob_stats` | Return aggregate opt-in local usage. | Reads XDG state; never returns individual events. |

After reviewing a conflict-free plan, prefer an explicitly approved guarded
shell call:

```bash
bob apply <workspace> --expect-plan-digest sha256:<64-lowercase-hex> --json
```

Copy MCP `plan_digest_qualified` directly into the apply flag. The original
raw `plan_digest` remains available for wire compatibility. Bob fresh-plans
under its apply lock. A mismatch returns
`plan_digest_mismatch` and writes nothing. The successful apply receipt is not
a behavioral verification receipt.

## Install from a checkout

```bash
task install
BOB_BIN="$(go env GOBIN)"
[ -n "$BOB_BIN" ] || BOB_BIN="$(go env GOPATH)/bin"
```

## Choose workspace authority

The server starts in exact-allowlist mode. Its canonical startup workspace is
allowed; other workspaces are refused even when the Bob process could read
them. For one repository, register that exact default:

```bash
mcphub add bob "$BOB_BIN/bob" \
  --description "Deterministic agent-ready repository builder" \
  --tag builder --tag code -- \
  mcp serve --workspace /absolute/path/to/repository
```

Add other exact existing repositories with a repeatable flag after the `--`
separator:

```bash
mcphub add bob "$BOB_BIN/bob" --force \
  --description "Deterministic agent-ready repository builder" \
  --tag builder --tag code -- \
  mcp serve \
    --workspace /absolute/path/to/default \
    --allow-workspace /absolute/path/to/second
```

For a trusted, machine-local MCPHub that intentionally serves arbitrary local
repositories, choose the broad mode explicitly:

```bash
mcphub add bob "$BOB_BIN/bob" --force \
  --description "Deterministic agent-ready repository builder" \
  --tag builder --tag code -- \
  mcp serve --allow-any-workspace
```

`--allow-any-workspace` is read authority, not an OS sandbox. It allows tools to
read any existing workspace accessible to the Bob process. MCPHub and the
calling agent still own policy and approval.

## Pin and probe

```bash
mcphub pin bob__bob_context bob__bob_plan bob__bob_check
mcphub doctor --server bob --probe
```

This minimal pin set keeps workspace orientation, plan review, and convergence
visible to small models. `bob_path` and `bob_playbook` remain available through
lazy discovery until a task needs them; validation, recipe description,
inspection, and stats remain discoverable as well. Pinning does not name a
workspace, grant authority, or execute a tool.

## Scope local-agent

If local-agent has an explicit MCPHub server allowlist, add `bob` to it:

```yaml
agents:
  local-agent:
    type: local-agent
    mode: gateway
    path: ~/.config/local-agent/config.yaml
    servers:
      - bob
      - cortex
      - obsidian
```

Then reconcile the generated harness configuration:

```bash
mcphub sync local-agent
# Run only if the dry run reports a diff:
mcphub sync local-agent --write
```

Restart an already running local-agent process after changing gateway scope or
pins.

## Approval behavior

With the current gateway integration, Bob's available names appear as:

```text
mcphub__bob__bob_context
mcphub__bob__bob_path
mcphub__bob__bob_playbook
mcphub__bob__bob_inspect
mcphub__bob__bob_plan
mcphub__bob__bob_check
mcphub__bob__bob_validate_manifest
mcphub__bob__bob_recipe_describe
mcphub__bob__bob_stats
```

It conservatively classifies MCP calls as unknown-effect and prompts under the
default Ask policy. Avoid a persistent allow for
`mcphub__mcphub_call_tool`: that generic proxy can dispatch many downstream
tools. PLAN mode blocks MCP; use NORMAL or AUTO with the intended permission
policy.

MCPHub servers and pins are global unless an agent has an explicit allowlist.
Scope other configured gateways if Bob should not be advertised to them.

## Input and output bounds

`bob_context` accepts `compact`, `standard`, or `full` and defaults to compact;
normal compact responses target less than 8 KiB end-to-end. `bob_path` accepts
one repository-relative UTF-8 path of at most 4 KiB and caps output at 8 KiB.
`bob_playbook` accepts only `list`, `show`, or `plan`, an ID of at most 128
bytes, and at most 32 plan values with 128-byte keys and 4-KiB values. Guidance
requests are capped at 64 KiB. Playbook list output is capped at 8 KiB; show and
plan are capped at 24 KiB. Every bounded domain result reports truncation
explicitly and returns no raw file body or subprocess output. The exact
validated object is in MCP `structuredContent`. `bob_context` avoids a second
full copy by returning only identity, repository state, digests, and a
`detail_location: "structuredContent"` marker in its JSON text block. Consumers
must parse the allowlisted structured contract rather than treating that text
summary as the complete context.

## Consumer contract fixtures

Bob publishes versioned JSON examples under `testdata/contracts/` for clean,
drifted, and conflicted context; managed and extension paths; a ready playbook;
missing input; and rejection of an unknown future schema. These fixtures are a
consumer boundary, not persisted workspace snapshots.

A local-agent or Cortex adapter should parse each allowlisted Bob operation
through an exact schema. Successful MCP transport with a malformed or unknown
domain schema must remain an unknown domain result. Persist only a bounded
digest or summary; pass validated compact content to a model transiently. A
structured Bob action is guidance: validate its tool and arguments, fill only
named missing inputs, respect `blocked_by`, and route its declared effect
through normal approval policy.

## Local telemetry remains local

MCP uses the same disabled-by-default settings as the CLI. If telemetry is
enabled, inspect, plan, check, manifest validation, and recipe description may
append one privacy-bounded event to Bob's XDG state. `bob_stats` does not record
itself. No event is sent through MCPHub or over a telemetry network, and the
stats tool returns aggregates rather than individual events.

Use `bob config show` to inspect the effective setting and
`bob stats --all --json` to inspect the same aggregate outside MCP. See
[Configuration & local telemetry](../configuration.md) for the schema and
retention boundary.

## Integration probes remain CLI-only

Plain `bob inspect` and `bob_inspect` do not launch Codemap or Vecgrep. Use the
CLI flag only when you explicitly authorize their public status commands:

```bash
bob inspect /path/to/project --probe-integrations
```

Bob normalizes readiness information; it never searches code, calculates
impact, repairs an index, or treats status as verification evidence. Cortex
remains the owner of evidence-guided Vecgrep-to-Codemap investigation.

## New here as a coding agent

If an agent is reading this file to decide how to behave, start with
[Bob for coding agents](../agents.md) instead. Run `bob learn --json` once,
before touching MCP or the shell, for the full product brief in one read-only
call.
