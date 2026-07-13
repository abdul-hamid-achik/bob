# CLI Reference

All repository commands accept an optional workspace path. When omitted, Bob
uses the current directory.

## Commands

| Command | Repository effect | Purpose |
|---|---|---|
| `bob new <name>` | Preview by default; writes with `--write` | Create a new repository contract and initial files. |
| `bob init [path]` | Preview by default; writes `bob.yaml` with `--write` | Initialize Bob in an existing directory without generating files yet. |
| `bob plan [path]` | Read-only | Compare desired and observed state; `--content` adds bounded previews. |
| `bob apply [path]` | Writes | Apply one fresh, complete, conflict-free plan. |
| `bob check [path]` | Read-only | Exit non-zero when managed state or the lock would change. |
| `bob doctor [path]` | Runs bounded version probes | Check required and selected optional development tools. |
| `bob inspect [path]` | Read-only by default | Summarize Bob state and binary availability. |
| `bob config show` | Read-only | Show effective settings and resolved XDG paths. |
| `bob config init` | Preview by default; writes with `--write` | Initialize private user settings; `--telemetry` opts in. |
| `bob stats [path]` | Reads local XDG state | Return privacy-bounded usage aggregates. |
| `bob studio [path]` | Repository-read-only interactive UI | Monitor Overview, Plan, and aggregate Stats. |
| `bob explain` | Read-only | Describe product ownership and ecosystem boundaries. |
| `bob recipe list` | Read-only | List embedded recipes. |
| `bob recipe show <id>` | Read-only | Describe one recipe. |
| `bob version` | Read-only | Print build version, commit, and date. |
| `bob mcp serve` | Long-running stdio server | Expose six typed repository-read-only tools. |

`bob inspect --probe-integrations` is an explicit exception to the plain
read-only inventory: it launches selected Codemap and Vecgrep status commands.
See [Ownership & Safety](../ownership-and-safety.md#commands-and-authority).

`stats`, Studio, and MCP never mutate repositories. When local telemetry is
explicitly enabled, normal CLI and MCP operations may append privacy-bounded
events beneath Bob's XDG state directory. Studio and stats do not record their
own use. See [Configuration & local telemetry](../configuration.md).

## Machine-readable output

Pass the global `--json` flag before or after a normal CLI command:

```bash
bob plan --json
bob --json inspect .
```

The stdout document has a versioned envelope:

```json
{
  "schema_version": 1,
  "ok": true,
  "command": "plan",
  "data": {},
  "warnings": [],
  "next_actions": []
}
```

Normal JSON output and machine-readable failures go to stdout. Cobra diagnostics
and process errors go to stderr. JSON contains no ANSI color or progress output.

Studio intentionally rejects `--json`. MCP is also different: `bob mcp serve`
reserves stdout entirely for newline-delimited JSON-RPC and never emits the CLI
JSON envelope around transport errors.

## Exit behavior

- `0` means the requested operation completed successfully.
- `check` exits non-zero when it detects drift, even though its JSON body
  contains the useful plan.
- validation, unsafe paths, conflicts during apply, failed required doctor
  checks, and transport failures exit non-zero.
- optional integration absence degrades readiness without inventing success.

## Workspace paths

CLI write commands reject a symlink at the selected workspace boundary. The MCP
server canonicalizes its startup `--workspace` and uses it as an exact allowlist
by default. Repeat `--allow-workspace <path>` for additional exact existing
workspaces. `--allow-any-workspace` explicitly accepts any existing workspace
the process can read. The hosting process and agent runtime remain responsible
for filesystem access control.

## Configuration and stats flags

`bob config init` previews by default. `--write` creates the file without
overwriting an existing path; `--telemetry` writes an enabled opt-in setting.

`bob stats` defaults to the selected workspace and a seven-day window. Use
`--since 24h`, `--since 30d`, or `--since all`; use `--all` instead of a
workspace to aggregate every retained pseudonymous workspace.

## Studio flags

`bob studio [workspace]` requires an interactive terminal. `--single-pane`
forces the compact layout. Studio has no mutation or subprocess shortcuts; all
suggested actions are inert text.

## MCP tools

| Tool | Result |
|---|---|
| `bob_inspect` | Repository state, drift summary, and offline selected-binary availability. |
| `bob_plan` | Bounded actions, exact counts, truncation metadata, and deterministic plan digest. |
| `bob_check` | Convergence, conflict, and lock-drift summary using the same plan digest. |
| `bob_validate_manifest` | Strict normalized validation of workspace `bob.yaml` or bounded inline YAML. |
| `bob_recipe_describe` | Embedded recipe schema, version, surfaces, and supported choices. |
| `bob_stats` | Aggregate local usage for one authorized workspace or all pseudonymous workspaces. |

`bob_plan` excludes unchanged actions by default, accepts at most 500 requested
actions, and also enforces a transport byte budget. `bob_validate_manifest`
accepts exactly one of `workspace` and `manifest_yaml`; inline YAML is limited
to 64 KiB. `bob_stats` accepts a one-to-365-day window and never exposes
individual events.
