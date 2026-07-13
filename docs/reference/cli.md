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
| `bob explain` | Read-only | Describe product ownership and ecosystem boundaries. |
| `bob recipe list` | Read-only | List embedded recipes. |
| `bob recipe show <id>` | Read-only | Describe one recipe. |
| `bob version` | Read-only | Print build version, commit, and date. |
| `bob mcp serve` | Long-running stdio server | Expose `bob_inspect` and `bob_plan` over MCP. |

`bob inspect --probe-integrations` is an explicit exception to the plain
read-only inventory: it launches selected Codemap and Vecgrep status commands.
See [Ownership & Safety](../ownership-and-safety.md#commands-and-authority).

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

MCP is different: `bob mcp serve` reserves stdout entirely for newline-delimited
JSON-RPC. It never emits the CLI JSON envelope around transport errors.

## Exit behavior

- `0` means the requested operation completed successfully.
- `check` exits non-zero when it detects drift, even though its JSON body
  contains the useful plan.
- validation, unsafe paths, conflicts during apply, failed required doctor
  checks, and transport failures exit non-zero.
- optional integration absence degrades readiness without inventing success.

## Workspace paths

CLI write commands reject a symlink at the selected workspace boundary. The
read-only MCP server treats `--workspace` as its default and accepts another
existing absolute workspace per call; this is intentional multi-repository read
authority. The hosting process and agent runtime remain responsible for
filesystem access control.
