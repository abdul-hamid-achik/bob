# MCPHub & local-agent

Bob exposes six typed MCP tools for repository orientation, validation,
planning, convergence, recipe discovery, and aggregate local usage. Repository
mutation stays on the agent runtime's normal approved shell path.

## Tools

| Tool | Purpose | Effect |
|---|---|---|
| `bob_inspect` | Return Bob state and offline availability of selected binaries. | Read-only; never runs specialist probes. |
| `bob_plan` | Return a bounded deterministic plan and digest. | Repository-read-only; omits desired-content previews. |
| `bob_check` | Return convergence, conflict, and lock-drift state. | Repository-read-only; shares the complete-plan digest. |
| `bob_validate_manifest` | Strictly validate workspace or bounded inline YAML. | Repository-read-only; never writes a manifest. |
| `bob_recipe_describe` | Describe the embedded recipe contract. | Read-only; does not require a workspace. |
| `bob_stats` | Return aggregate opt-in local usage. | Reads XDG state; never returns individual events. |

Use `bob apply <workspace>` through an explicitly approved shell call after
reviewing a conflict-free plan.

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
mcphub pin bob__bob_inspect bob__bob_plan bob__bob_check \
  bob__bob_validate_manifest bob__bob_recipe_describe bob__bob_stats
mcphub doctor --server bob --probe
```

Pinning keeps the tools directly advertised when MCPHub uses lazy exposure. It
does not name, authorize, or execute them.

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

With the current gateway integration, local-agent sees the pinned names as:

```text
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
