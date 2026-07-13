# MCPHub & local-agent

Bob exposes a compact read-only MCP surface for repository orientation and
planning. Filesystem mutation stays on the agent runtime's normal approved shell
path.

## Tools

| Tool | Purpose | Effect |
|---|---|---|
| `bob_inspect` | Return Bob state and offline availability of selected binaries. | Read-only; never runs specialist probes. |
| `bob_plan` | Return a compact deterministic repository plan. | Read-only; omits desired-content previews. |

Use `bob apply <workspace>` through an explicitly approved shell call after
reviewing a conflict-free plan.

## Install from a checkout

```bash
task install
BOB_BIN="$(go env GOBIN)"
[ -n "$BOB_BIN" ] || BOB_BIN="$(go env GOPATH)/bin"
```

## Register with MCPHub

```bash
mcphub add bob "$BOB_BIN/bob" mcp serve \
  --description "Deterministic agent-ready repository builder" \
  --tag builder
mcphub pin bob__bob_inspect bob__bob_plan
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
```

It conservatively classifies MCP calls as unknown-effect and prompts under the
default Ask policy. Avoid a persistent allow for
`mcphub__mcphub_call_tool`: that generic proxy can dispatch many downstream
tools. PLAN mode blocks MCP; use NORMAL or AUTO with the intended permission
policy.

MCPHub servers and pins are global unless an agent has an explicit allowlist.
Scope other configured gateways if Bob should not be advertised to them.

## Integration probes remain CLI-only

Plain `bob inspect` and `bob_inspect` do not launch Codemap or Vecgrep. Use the
CLI flag only when you explicitly authorize their public status commands:

```bash
bob inspect /path/to/project --probe-integrations
```

Bob normalizes readiness information; it never searches code, calculates
impact, repairs an index, or treats status as verification evidence. Cortex
remains the owner of evidence-guided Vecgrep-to-Codemap investigation.
