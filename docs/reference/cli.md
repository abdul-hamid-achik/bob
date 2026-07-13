---
description: Complete, verified reference for every Bob command, its flags, its repository effect, and its JSON envelope.
---

# CLI Reference

All repository commands accept an optional workspace path. When omitted, Bob
uses the current directory. Bob does not ask where you are; it checks.

## Commands

| Command | Repository effect | Purpose |
|---|---|---|
| `bob new <name>` | Preview by default; writes with `--write` | Create a new repository contract and initial files. |
| `bob init [path]` | Preview by default; writes `bob.yaml` with `--write` | Initialize Bob in an existing directory without generating files yet. |
| `bob plan [path]` | Read-only | Compare desired and observed state; `--content` adds bounded previews, `--conflicts-only` trims to conflicts. |
| `bob apply [path]` | Writes | Apply one fresh, complete, conflict-free plan; a refusal reports `data.conflicts` directly. |
| `bob check [path]` | Read-only | Exit non-zero when managed state or the lock would change; also accepts `--conflicts-only`. |
| `bob doctor [path]` | Runs bounded version probes | Check required and selected optional development tools. |
| `bob inspect [path]` | Read-only by default | Summarize Bob state and binary availability. |
| `bob config show` | Read-only | Show effective settings and resolved XDG paths. |
| `bob config init` | Preview by default; writes with `--write` | Initialize private user settings; `--telemetry` opts in. |
| `bob stats [path]` | Reads local XDG state | Return privacy-bounded usage aggregates. |
| `bob studio [path]` | Repository-read-only interactive UI | Monitor Overview, Plan, and aggregate Stats. |
| `bob explain` | Read-only | Describe product ownership and ecosystem boundaries. |
| `bob learn` | Read-only, no network | One-shot onboarding brief for coding agents. |
| `bob recipe list` | Read-only | List embedded recipes (`files@1`, `go-agent-tool@3`). |
| `bob recipe show <id>` | Read-only | Describe one recipe's schema and print a copyable example. |
| `bob version` | Read-only | Print build version, commit, and date. |
| `bob mcp serve` | Long-running stdio server | Expose six typed repository-read-only tools. |

`bob inspect --probe-integrations` is an explicit exception to the plain
read-only inventory: it launches selected Codemap and Vecgrep status commands.
See [Ownership & Safety](../ownership-and-safety.md#commands-and-authority).

## Two recipes

```text
$ bob recipe list
files@1  declare any file tree inline; bob materializes it with plan/apply safety
go-agent-tool@3  Public-ready Go and Cobra CLI with docs, CI, release plumbing, and optional ecosystem seams
```

`bob recipe show <id>` describes one recipe's manifest schema and prints an
example manifest verbatim — copy it, don't retype it. `bob new`/`bob init`
still scaffold `go-agent-tool` only; a `files` manifest is hand- or
agent-authored. See the [Manifest Reference](./manifest.md) for both schemas
and the [any-repository guide](../guides/any-repository.md) for a worked
`files` example.

## `bob learn`

One-shot onboarding brief for coding agents. It takes no arguments, mutates
nothing, and makes no network call. Human mode prints a compact text
briefing; `--json` emits Bob's standard envelope with `command: "learn"` and a
`data` object covering the product, a summary, the lifecycle order
(init/new preview → plan → apply → check), every command's name, purpose,
mutation status, and JSON support, a field guide to the envelope itself, the
safety invariants, the MCP surface, the boundaries Bob refuses to own, the
recipe catalog (id, version, description for both embedded recipes), the
`exit_codes` and `error_codes` maps documented below, and the docs URLs. See
[Bob for coding agents](../agents.md) for the full contract and a worked
bootstrap sequence.

```bash
bob learn
bob learn --json
```

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
and process errors go to stderr. JSON contains no ANSI color or progress
output — pipe it straight into `jq` without cleaning up after Bob first.

Studio intentionally rejects `--json`. MCP is also different: `bob mcp serve`
reserves stdout entirely for newline-delimited JSON-RPC and never emits the CLI
JSON envelope around transport errors.

### Failure envelopes

A failed command still writes one JSON document, with `ok: false` and the
command's normal `data` replaced by an error:

```json
{
  "schema_version": 1,
  "ok": false,
  "command": "plan",
  "data": {
    "error": {
      "code": "missing_manifest",
      "message": "plan: no bob.yaml found in .; run: bob init --module <module> --write to create one: file does not exist"
    }
  },
  "warnings": [],
  "next_actions": [
    "run: bob init --module <module> --write",
    "run: bob learn --json"
  ]
}
```

`next_actions` is never empty on failure — it holds copy-pasteable corrective
commands, not advice prose. Human mode prints the same steps on stderr as
`next: ...` lines after the error line, so a person reads the identical
recovery path a script would parse.

`data.error.code` is one of:

| Code | Meaning |
|---|---|
| `missing_manifest` | No `bob.yaml` was found at the resolved workspace path. |
| `manifest_invalid` | `bob.yaml` failed to parse or failed `Validate`; the message lists every problem. |
| `conflicts` | The plan contains one or more ownership conflicts; apply refused every write. |
| `input_invalid` | A flag, argument, or recipe id was invalid. |
| `workspace_invalid` | The workspace path could not be resolved safely (for example, a symlink at the workspace boundary). |
| `command_failed` | An unclassified failure — read the message for detail. |

An `apply` refused by conflicts skips the round-trip back through `plan`: its
failure `data` also carries `data.conflicts`, one entry per blocked path:

```json
{
  "data": {
    "conflicts": [
      { "path": "conflict.json", "code": "unmanaged_differs", "reason": "unmanaged file differs from the desired content" }
    ],
    "error": { "code": "conflicts", "message": "apply: plan contains conflicts; run bob plan for details" }
  }
}
```

Validation failures echo the offending value and, where one exists, the
nearest valid option, instead of just failing:

```text
bob: recipe show: unknown recipe "fils"; did you mean "files"?
bob: plan: validate manifest: product.name must start with a letter and
  contain only lowercase letters, digits, and hyphens (got "My-App")
```

## `--conflicts-only`

`plan` and `check` both accept `--conflicts-only`, which drops every
non-conflict action from the output — human or JSON. It exists for harnesses
with a capped output budget that only need to know what's blocking apply:

```bash
bob check --conflicts-only --json
```

The action count line still reports the full `create`/`update`/`adopt`/
`unchanged`/`conflict` totals; only the listed actions are filtered.

## `--content`

`plan --content` (and `check --content`) adds bounded, 2048-byte content
previews to `create`, `update`, and `conflict` actions:

- `desired_preview` — always present for create/update/conflict.
- `current_preview` — present alongside `desired_preview` whenever a current
  file exists, so a conflict or update shows both sides without a second
  read:

```json
{
  "path": "scripts/run.sh",
  "kind": "update",
  "code": "content_update",
  "desired_preview": "#!/usr/bin/env bash\necho \"listening on 9090\"\n",
  "current_preview": "#!/usr/bin/env bash\necho \"listening on 8080\"\n"
}
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. `bob plan` always exits `0`, even when it finds conflicts — plan is a read-only report, not a gate. |
| `1` | Unclassified command failure (`command_failed`), or a workspace path that could not be resolved (`workspace_invalid`). |
| `2` | `apply` refused a conflicted plan, or `check` found an ownership conflict. |
| `3` | `check` found drift with no ownership conflict. |
| `4` | Invalid input: a missing or invalid manifest (including an unrecognized `recipe:` id in `bob.yaml`), or a bad flag or argument. |

An agent scripting Bob should branch on the exit code first, then read
`data.error.code` for detail — do not infer success from the mere presence of
JSON on stdout. See [Bob for coding agents](../agents.md#recovering-from-failure)
for a code-by-code recovery playbook.

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
