---
description: How a coding agent should onboard to Bob — bob learn, the JSON envelope, exit codes, and the read-only MCP surface.
---

# Bob for coding agents

Bob does not know what an agent is. It knows commands, plans, and locks. That
is a feature: an agent gets the exact same deterministic contract a human
does, no special pleading, no hidden mode. This page is the fast path to using
that contract well.

If you are an agent reading this to decide what to do next: run
`bob learn --json`, then `bob context --json`, before you plan anything. The
first call describes Bob; the second describes the repository contract.

## `bob learn`

`bob learn` is a one-shot onboarding brief. It takes no arguments. It mutates
nothing and makes no network call — it is a summary of what Bob already knows
about itself, not a probe of your workspace.

```bash
bob learn
```

Human mode prints a compact text briefing: what Bob is, the lifecycle order,
the command list, and where to read more. Good for a person skimming a
terminal. An agent should prefer the JSON form.

```bash
bob learn --json
```

### The `data` object

`bob learn --json` returns Bob's standard envelope (below) with
`"command": "learn"`. Its `data` field carries the whole brief:

| Field | Contents |
|---|---|
| `product` | What Bob is: a deterministic repository factory, contract compiler, guidance provider, and lifecycle reconciler. |
| `summary` | A short prose description of the product contract. |
| `lifecycle` | The ordered workflow: `init`/`new` preview → `plan` → `apply` → `check`. |
| `commands` | One entry per command: name, purpose, whether it mutates, whether it supports `--json`. |
| `recommended_agent_bootstrap` | Ordered `learn`, `context`, `plan`, and `check` argv guidance. |
| `json_envelope` | A field guide to the envelope itself — what `ok`, `data`, `warnings`, and `next_actions` mean. |
| `invariants` | The safety guarantees an agent can rely on without re-deriving them. |
| `mcp` | The `bob mcp serve` command and the nine read-only tools it exposes. |
| `boundaries` | What Bob explicitly refuses to own — see [Non-goals](#what-bob-refuses-to-own). |
| `recipes` | The embedded recipe catalog: id, version, and description for each (`files@1`, `go-agent-tool@4`, and the eight stack hygiene recipes `ts-app@1`, `js-app@1`, `vue-app@1`, `python-app@1`, `ruby-app@1`, `lua-lib@1`, `rust-cli@1`, `static-web@1`). |
| `exit_codes` | The same table as [Exit codes](#exit-codes), keyed by code. |
| `error_codes` | The same table as [Error codes](#error-codes), keyed by code. |
| `docs` | Canonical documentation URLs: `https://bobcli.dev` and `https://bobcli.dev/agents`. |

Nothing in `data` requires a workspace. `bob learn` describes the product, not
a repository, so it works identically from any directory.

## Recommended agent bootstrap

Run `bob learn --json` once at session start. Cache the brief for the rest of
the session — it does not change between commands, only between Bob versions.
Then drive the actual work with the normal read-only commands:

```bash
bob learn --json          # once, at session start
bob context --json        # recipe, capability, ownership, and digest context
bob path <relative-path> --json  # before editing a path whose ownership is unclear
bob playbook list --json   # discover stable procedures; select by ID, never prose
bob plan --json           # before proposing or reviewing any change
bob check --json          # to confirm convergence, exits non-zero on drift
```

Context is bounded, read-only, and offline. Branch on its stable capability
facets and action codes; never treat `verification: not_assessed` as success.
For a converged `go-agent-tool@4` workspace, add commands through the advertised
`cli.command_files` extension point. If `add-cli-command` reports
`extension_contract_not_materialized`, reconcile the version-4 root and
registry contract before creating an extension. Those composition files remain
Bob-owned; editing either creates an ownership conflict. See the [workspace
context schema](./reference/context.md).

Before editing a path, branch on `bob path` codes. `will_conflict` means the
current Bob contract owns or intends to own the complete file;
`outside_bob_ownership` is not a global safety claim. When a related playbook
is advertised, select its exact ID and supply only its named inputs:

```bash
bob path internal/cli/root.go --json
bob playbook show add-cli-command --json
bob playbook plan resolve-ownership-conflict \
  --set path=internal/cli/root.go \
  --set action_code=managed_hash_mismatch --json
```

Playbook steps are guidance. Route each effect through the normal approval
policy; never auto-execute the first mutation step. See the
[path](./reference/path.md) and [playbook](./reference/playbooks.md) contracts.

Only call `bob apply` after a human or an explicit policy has approved a
conflict-free plan. Approval-aware callers should copy the reviewed CLI
`plan_digest`, MCP `plan_digest_qualified`, or the qualified digest from
context into the guarded apply:

```bash
bob apply . --expect-plan-digest sha256:<64-lowercase-hex> --json
```

Bob fresh-plans while holding its apply lock. A stale digest exits `5` with
`plan_digest_mismatch` and writes nothing. A successful apply receipt proves
which reconciliation Bob performed; it does not verify generated behavior.

## The `--json` envelope contract

Every non-interactive command that supports `--json` writes one versioned
envelope to stdout and nothing else:

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

- `schema_version` is `1`. Bob rejects wire formats it does not recognize
  rather than guessing at a shape.
- `ok` is `true` on success. On failure it is `false`, and `data` carries an
  error code and message instead of the command's normal result.
- `command` names the command that produced the envelope.
- `warnings` and `next_actions` are always arrays, even when empty.
- stdout carries only this JSON document. Diagnostics, Cobra usage text, and
  process errors go to stderr. Parse stdout without stripping anything first.

`bob mcp serve` is the one exception: its stdout is reserved entirely for
newline-delimited JSON-RPC, and it never wraps transport errors in the CLI
envelope above.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. `bob plan` always exits `0`, even with conflicts — plan is a read-only report, not a gate. |
| `1` | Unclassified command failure (`command_failed`), or an unresolvable workspace path (`workspace_invalid`). |
| `2` | `apply` refused a conflicted plan, or `check` found an ownership conflict. |
| `3` | `check` found drift with no ownership conflict. |
| `4` | Invalid input: a missing or invalid manifest (including an unrecognized `recipe:` id), or a bad flag or argument. |
| `5` | Guarded apply refused because the fresh plan no longer matches the reviewed digest. No repository write occurred. |

`bob check` is the deliberate special case: it exits non-zero when the
repository or lock *would* change, even though its JSON body is a normal,
successful plan. Non-zero here is the answer, not an error. Note that `2` and
`3` are both "non-zero, drift found" — `2` means a conflict is in the mix and
apply would refuse; `3` means every change is safe and apply would proceed.

An agent scripting Bob should branch on the exit code first and only inspect
`data` for detail — do not infer success from the presence of JSON alone.

## Error codes

Every failure envelope carries `data.error.code`, one of:

| Code | Meaning |
|---|---|
| `missing_manifest` | No `bob.yaml` was found at the resolved workspace path. |
| `manifest_invalid` | `bob.yaml` failed to parse or failed `Validate`; the message lists every problem. |
| `conflicts` | The plan contains one or more ownership conflicts; apply refused every write. |
| `input_invalid` | A flag, argument, or recipe id was invalid. |
| `workspace_invalid` | The workspace path could not be resolved safely. |
| `plan_digest_mismatch` | The fresh apply plan differs from the explicitly reviewed plan digest; review a new plan before applying. |
| `command_failed` | An unclassified failure; read the message for detail. |

Validation errors echo the offending value and, when one exists, the nearest
valid option — `unknown recipe "fils"; did you mean "files"?` — rather than
just failing quietly.

## Action codes

Every action in a `bob_plan`/`bob_check`/`plan`/`check` result carries a
stable `code` alongside its human `kind`, so an agent can branch on the code
instead of parsing the `reason` prose (the prose can change; the code is the
contract):

Non-conflict codes (safe to act on):

| Code | Kind | Meaning |
|---|---|---|
| `missing` | `create` | Destination does not exist. |
| `content_update` | `update` | Managed file's content would change; lock hash still matches. |
| `mode_drift` | `update` | Managed file's mode would change; content already matches. |
| `in_sync` | `unchanged` | Managed file already matches desired content and mode. |
| `identical_content` | `adopt` | Unmanaged file already byte-matches; Bob may adopt it without writing. |

Conflict codes (apply refuses the *entire* plan if any of these appear):

| Code | Meaning |
|---|---|
| `unmanaged_differs` | An unmanaged file exists and differs from the desired content. |
| `managed_hash_mismatch` | A managed file's current hash no longer matches `bob.lock` — it was hand-edited. |
| `managed_missing` | A file `bob.lock` says Bob manages is gone from disk. |
| `unmanaged_mode_differs` | An unmanaged file's content matches but its mode doesn't, and Bob can't prove ownership to change it. |
| `retired_owned` | A previously-managed path is no longer desired but Bob still owns it in the lock. |
| `symlink` | A symlink sits at the destination path. |
| `special_file` | A device, socket, or named pipe sits at the destination path. |

Symlinks and special files used to abort the whole plan; they are now reported
as ordinary per-path conflicts, so one odd file doesn't blind you to the rest
of the plan.

## Reading an apply refusal

An `apply` refused by conflicts does not make you round-trip through `plan`
to find out why. Its failure `data` carries `data.conflicts` directly, one
entry per blocked path:

```json
{
  "ok": false,
  "command": "apply",
  "data": {
    "conflicts": [
      { "path": "conflict.json", "code": "unmanaged_differs", "reason": "unmanaged file differs from the desired content" }
    ],
    "error": { "code": "conflicts", "message": "apply: plan contains conflicts; run bob plan for details" }
  }
}
```

Branch on `data.conflicts[].code`. Reserve the `reason` string for a log line
or a human, not for control flow.

## `--conflicts-only`

`plan` and `check` both accept `--conflicts-only`, which drops every
non-conflict action from the output. It exists for output-capped agent
harnesses that loop on "is anything blocking apply?" without wanting the full
action list back each time:

```bash
bob check --conflicts-only --json
```

The summary counts still report the full totals; only the listed actions are
trimmed.

## Recovering from failure

Map the error code straight to a corrective command — this is exactly what
`next_actions` already gives you, spelled out per code:

| Error code | Corrective command |
|---|---|
| `missing_manifest` | `bob init --module <module> --write` |
| `manifest_invalid` | Fix every problem the message lists, then rerun `bob plan --json`. `bob recipe show <recipe-id>` describes the recipe, and the [Manifest Reference](./reference/manifest.md) documents the schema you're validating against. |
| `conflicts` | Inspect `data.conflicts` (apply) or actions with `kind: conflict` (plan/check), resolve each path deliberately, then rerun `bob apply`. |
| `input_invalid` | Fix the flag, argument, or recipe id the message names — check for a "did you mean" suggestion first. |
| `workspace_invalid` | Pass an existing, non-symlink directory as the workspace path. |
| `plan_digest_mismatch` | Run `bob plan --json`, review the new digest, then issue a new guarded apply. |
| `command_failed` | Read the message; it is the whole diagnosis. If it looks like a bug, `bob learn --json` won't help further — that's a report, not a retry loop. |

Every failure envelope's `next_actions` array already contains this same
guidance as literal, copy-pasteable commands — this table exists so an agent
can branch on the code without parsing prose.

## The read-only MCP surface

`bob mcp serve` runs Bob's MCP server over stdio. It exposes nine tools, and
none of them mutate a repository:

| Tool | Result |
|---|---|
| `bob_context` | Bounded recipe, capability, ownership, invariant, playbook-summary, and current-plan context. |
| `bob_path` | Exact Bob ownership relationship for one repository-relative path, without file bodies. |
| `bob_playbook` | Closed `list`, `show`, or `plan` guidance with typed values and no execution. |
| `bob_plan` | Bounded plan actions, counts, truncation metadata, and a deterministic plan digest. |
| `bob_check` | Convergence, conflict, and lock-drift state, sharing the same plan digest. |
| `bob_inspect` | Bob state and offline binary availability, without running specialist probes. |
| `bob_stats` | Aggregate opt-in local usage for one workspace or all retained pseudonymous workspaces. |
| `bob_recipe_describe` | The embedded recipe schema, version, and supported choices. |
| `bob_validate_manifest` | Strict validation of a workspace manifest or bounded inline YAML. |

A computed conflict from `bob_plan` or `bob_check` is still a successful,
read-only result. It carries no mutation authority. Apply always goes through
the normal approved shell path. Prefer
`bob apply <workspace> --expect-plan-digest sha256:<digest>` after review, then
call `bob_check` to confirm convergence.

### Workspace allowlist

The server starts in exact-allowlist mode. Its startup workspace is the only
one the tools may read by default:

```bash
bob mcp serve --help
```

```text
Flags:
      --allow-any-workspace           allow MCP tools to read any existing workspace accessible to Bob
      --allow-workspace stringArray   additional exact existing workspace allowed to MCP tools (repeatable)
      --workspace string              default existing workspace (defaults to startup cwd)
```

- `--workspace` sets the default workspace explicitly instead of relying on
  the process's startup directory.
- `--allow-workspace` is repeatable and adds specific additional existing
  workspaces to the allowlist.
- `--allow-any-workspace` is the explicit broad mode. It allows the tools to
  read any existing workspace the Bob process can access. It is read
  authority, not a sandbox — the hosting process and agent runtime remain
  responsible for what the Bob process itself can reach.

Pick the narrowest mode that gets the job done. One repository needs
`--workspace` and nothing else.

### Example: Claude Code setup

```bash
claude mcp add bob -- bob mcp serve <workspace>
```

Replace `<workspace>` with the absolute path to the repository Bob should
read. Add `--allow-workspace <path>` (repeatably) for a small fixed set of
additional repositories, or `--allow-any-workspace` only on a trusted,
machine-local setup that is meant to serve arbitrary local repositories.

## What Bob refuses to own

Bob is not an LLM runtime, a planning agent, an evidence authority, a secret
manager, a package manager, or a generic task runner. It does not run models,
schedule agents, hold credentials, or declare that generated code behaves
correctly. Optional tools — Codemap, Vecgrep, Cairntrace, TinyVault,
file.cheap, Glyphrun — stay behind explicit public contracts that Bob
describes but never operates on your behalf.

## Safety invariants agents can rely on

These hold across the CLI and MCP surfaces, and an agent can build on them
without re-verifying each session:

- `context`, `path`, `playbook`, `plan`, `check`, plain `inspect`, `stats`, `learn`, and Studio never mutate a
  repository.
- All nine MCP tools are read-only; none of them run specialist probes or
  write to the target repository.
- `apply` preflights the complete plan first and writes nothing if any single
  action is a conflict.
- Bob never overwrites an unmanaged file that differs from the desired
  content — it reports `conflict` and stops instead of guessing.
- A managed file only updates if its current hash still matches the prior
  lock; a hand-edited managed file is a conflict, not a silent overwrite.
- Repeated `apply` converges to a no-op. Once a plan reports only `unchanged`
  actions, running `apply` again writes nothing.
- JSON stdout is machine-clean on every surface; warnings, errors, and
  diagnostics go to stderr (or, for MCP, stay outside the JSON-RPC stream).

## See also

- [Build any repository](./guides/any-repository.md) for the `files` recipe —
  the second embedded recipe, for anything that isn't a Go/Cobra CLI — with an
  agent-focused section on looping `check --json --conflicts-only`.
- [MCPHub & local-agent](./guides/mcphub-local-agent.md) for wiring the same
  nine tools through MCPHub and scoping them to a local-agent gateway.
- [Ownership & Safety](./ownership-and-safety.md) for the full plan-state
  vocabulary (`create`, `adopt`, `unchanged`, `update`, `conflict`).
- [CLI Reference](./reference/cli.md) for every command's flags in one table.
- Canonical docs: [bobcli.dev](https://bobcli.dev) and
  [bobcli.dev/agents](https://bobcli.dev/agents).
