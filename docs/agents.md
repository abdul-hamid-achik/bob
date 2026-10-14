---
description: How a coding agent should onboard to Bob — bob learn, the JSON envelope, exit codes, and the MCP surface (read-only tools plus opt-in, digest-gated bob_apply).
---

# Bob for coding agents

Bob does not know what an agent is. It knows commands, plans, and locks. That
is a feature: an agent gets the exact same deterministic contract a human
does, no special pleading, no hidden mode. This page is the fast path to using
that contract well.

If you are an agent reading this to decide what to do next: run
`bob learn --json`, then `bob context --json`, before you plan anything. The
first call describes Bob; the second describes the repository contract.
On an existing TypeScript, Python, Rust, or Go application, prefer
[Existing Repository](./guides/existing-repository.md) — stack hygiene,
not `go-agent-tool` — unless the workspace already is a Cobra CLI.

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
| `lifecycle` | The ordered workflow: `init`/`new` preview → `plan` → `apply` → `check`, plus the explicit `upgrade` and `remove` lifecycle paths. |
| `commands` | One entry per command: name, purpose, whether it mutates, whether it supports `--json`. |
| `recommended_agent_bootstrap` | Ordered `learn`, `context`, `plan`, and `check` argv guidance. |
| `json_envelope` | A field guide to the envelope itself — what `ok`, `data`, `warnings`, and `next_actions` mean. |
| `invariants` | The safety guarantees an agent can rely on without re-deriving them. |
| `mcp` | The `bob mcp serve` command, its nine read-only tools, and the opt-in, digest-gated `bob_apply` mutation tool. |
| `boundaries` | What Bob explicitly refuses to own — see [Non-goals](#what-bob-refuses-to-own). |
| `recipes` | The embedded recipe catalog: id, version, and description for each (`files@1`, `go-agent-tool@6`, and the eighteen stack hygiene recipes at version 2: `ts-app`, `js-app`, `vue-app`, `nuxt-app`, `deno-app`, `python-app`, `php-app`, `ruby-app`, `lua-lib`, `rust-cli`, `java-app`, `kotlin-app`, `dotnet-app`, `cpp-cmake`, `swift-package`, `elixir-app`, `static-web`, and `go-hygiene`). |
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
For a converged `go-agent-tool@6` workspace, add commands through the advertised
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
`plan_digest_mismatch` and writes nothing; its `data.converged` is `true` when
the fresh plan is already clean, for example because an earlier guarded run
succeeded but its response was lost. A successful apply receipt proves
which reconciliation Bob performed; it does not verify generated behavior.
Preview recipe migration with `bob upgrade --dry-run --json` and bind a real
upgrade to the reviewed plan digest when policy requires it. Preview teardown
with `bob remove --dry-run --json`; remove affects only proven lock-owned files,
not seed-once or unmanaged files, and preserves `bob.yaml`. The dry-run reports
`removal_digest` and `removal_digest_version`; bind the real teardown to that
reviewed decision by repeating the same flags with
`--expect-removal-digest sha256:<64-lowercase-hex>`.
The removal digest is a separate namespace from the plan digest: a plan digest
never authorizes a removal and a removal digest never authorizes an apply. It
covers the exact `bob.lock` bytes, the `--force` flag, and every managed path's
hashes, mode, and removal decision, so a digest reviewed without `--force`
cannot authorize a `--force` run. A stale digest exits `5` with
`plan_digest_mismatch` and deletes nothing; a malformed digest exits `4`.

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
  error code and message instead of the command's normal result. `check` and
  `doctor` are the exceptions; see [Exit codes](#exit-codes).
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
| `1` | Unclassified command failure (`command_failed`), an unresolvable workspace path (`workspace_invalid`), another mutating operation holding the workspace lock (`workspace_busy`), or `doctor` reporting required tools missing or unusable. |
| `2` | `apply` or `upgrade` refused a conflicted plan, `check` found an ownership conflict, or `remove` skipped or conflicted on managed files. |
| `3` | `check` found drift with no ownership conflict. |
| `4` | Invalid input: a missing or invalid manifest (including an unrecognized `recipe:` id), a bad flag or argument, a missing remove lock, or missing/incompatible upgrade ownership state. |
| `5` | Guarded apply or upgrade refused because the fresh plan no longer matches the reviewed digest, or guarded remove refused because the fresh removal decision no longer matches the reviewed removal digest. No repository write occurred. |

`bob check` is the deliberate special case: it exits non-zero when the
repository or lock *would* change. On drift, `bob check --json` emits `ok: false`
with `data.clean: false`, `data.plan`, `data.plan_digest`, and
`data.plan_digest_version`, and no `data.error`. Non-zero here is the answer,
not an error. Note that `2` and `3` are both "non-zero, drift found" — `2` means
a conflict is in the mix and apply would refuse; `3` means every change is safe
and apply would proceed.

`bob doctor` follows the same pattern. When a required tool is missing or
unusable, it emits `ok: false` with its normal probe data and no `data.error`,
and exits `1`.

An agent scripting Bob should branch on the exit code first and only inspect
`data` for detail — do not infer success from the presence of JSON alone.

## Error codes

Every failure envelope carries `data.error.code`, one of the following — with
two exceptions that keep their normal `data` and carry no `data.error`: `check`
on drift, and `doctor` reporting a required tool missing or unusable (see
[Exit codes](#exit-codes)). A `check` or `doctor` invocation that cannot load
`bob.yaml` still returns an ordinary `data.error` envelope.

| Code | Meaning |
|---|---|
| `missing_manifest` | No `bob.yaml` was found at the resolved workspace path. |
| `manifest_invalid` | `bob.yaml` failed to parse or failed `Validate`; the message lists every problem. |
| `conflicts` | The plan contains one or more ownership conflicts; apply or upgrade refused every write. Remove also uses this code when a managed file is skipped or a managed path is unsafe. |
| `input_invalid` | A flag, argument, or recipe id was invalid. |
| `workspace_invalid` | The workspace path could not be resolved safely. |
| `plan_digest_mismatch` | The fresh apply or upgrade plan differs from the explicitly reviewed plan digest, or the fresh remove decision differs from the reviewed removal digest; review again before authorizing mutation. |
| `workspace_busy` | Another mutating Bob operation holds the workspace apply lock; no repository write occurred. Retry once it finishes. |
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
| `seed_exists` | `unchanged` | A seed-once file already exists; Bob never updates or owns it. |
| `retired_unchanged` | `retire` | A lock-owned regular file the recipe no longer declares still matches `bob.lock`; `--allow-retire` deletes it and drops it from the lock. |

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
| `conflicts` | Inspect `data.conflicts` or actions with `kind: conflict`, resolve each path deliberately, then rerun the originating `bob apply` or `bob upgrade`; for remove, follow its listed `--force` or manual-path guidance. |
| `input_invalid` | Fix the flag, argument, or recipe id the message names — check for a "did you mean" suggestion first. |
| `workspace_invalid` | Pass an existing, non-symlink directory as the workspace path. |
| `plan_digest_mismatch` | Run `bob plan --json`, review the new digest, then issue a new guarded apply or upgrade. For remove, rerun `bob remove --dry-run --json` with the same flags and review the new `removal_digest`. |
| `workspace_busy` | Wait for the other Bob operation on the workspace to finish, then retry the same command. Do not delete `.bob.apply.lock` while a Bob process is running. |
| `command_failed` | Read the message; it is the whole diagnosis. If it looks like a bug, `bob learn --json` won't help further — that's a report, not a retry loop. |

Every failure envelope's `next_actions` array already contains this same
guidance as literal, copy-pasteable commands — this table exists so an agent
can branch on the code without parsing prose.

## The MCP surface

`bob mcp serve` runs Bob's MCP server over stdio. It exposes nine read-only
tools plus one explicitly authorized, digest-gated, opt-in mutation tool
(`bob_apply`) that is disabled unless `--allow-apply-workspace` is set. The
nine read-only tools never mutate a repository:

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
read-only result. It carries no mutation authority.

### Opt-in apply: `bob_apply`

`bob_apply` is the only MCP tool that can write to a repository. It is
listed by every server but refuses with `apply_unauthorized` unless the
server was started with `--allow-apply-workspace <path>` naming that exact
workspace. Its annotations (`readOnlyHint: false`, `destructiveHint: true`,
`idempotentHint: false`, `openWorldHint: false`) describe intent; they do not
grant permission.

| Input | Meaning |
|---|---|
| `workspace` | Optional; defaults to the startup workspace. Must be in the apply allowlist. |
| `expect_plan_digest` | Required. The exact `plan_digest_qualified` value (`sha256:` plus 64 lowercase hex characters) from a reviewed `bob_plan` or `bob_check`. |

It calls the same lock-owned engine path as
`bob apply --expect-plan-digest`: Bob takes the workspace lock, reloads
`bob.yaml` under it, fresh-plans, and applies only when the fresh digest
equals the reviewed one. Every result is typed in `structuredContent` with an
`outcome`:

| `outcome` | Meaning |
|---|---|
| `applied` | Written. `receipt` carries the apply receipt; call `bob_check` next. |
| `plan_digest_mismatch` | Nothing written. `expected_plan_digest`, `actual_plan_digest`, and `converged` (the current plan is already clean). Bob keeps no history, so this never means "already applied" — replan and review. |
| `conflicts` | Nothing written. `conflicts[]` lists each blocked path with its code and reason. |
| `workspace_busy` | Nothing written. Another mutating Bob operation holds the lock; the call fails immediately rather than waiting and is marked `retryable: true`. |
| `apply_unauthorized` | Nothing written. The workspace is not in the apply allowlist (or apply is disabled). |
| `workspace_invalid` | Nothing written. The workspace path could not be resolved safely. |
| `input_invalid` | Nothing written. `expect_plan_digest` is not the exact `sha256:<64-lowercase-hex>` form. |

Cancellation is honored before staging; once publication starts the write
completes, so published files are never left without their lock. A retry
after a lost response reports `plan_digest_mismatch` with `converged: true`.
When the server has no apply authority, use the normal approved shell
path — `bob apply <workspace> --expect-plan-digest sha256:<digest>` after
review — then call `bob_check` to confirm convergence.

### Workspace allowlist

The server starts in exact-allowlist mode. Its startup workspace is the only
one the tools may read by default:

```bash
bob mcp serve --help
```

```text
Flags:
      --allow-any-workspace                 allow MCP tools to read any existing workspace accessible to Bob
      --allow-apply-workspace stringArray   exact existing workspace bob_apply may mutate with a reviewed plan digest (repeatable; never implied by other flags)
      --allow-workspace stringArray         additional exact existing workspace allowed to MCP tools (repeatable)
      --workspace string                    default existing workspace (defaults to startup cwd)
```

- `--workspace` sets the default workspace explicitly instead of relying on
  the process's startup directory.
- `--allow-workspace` is repeatable and adds specific additional existing
  workspaces to the allowlist.
- `--allow-any-workspace` is the explicit broad mode. It allows the tools to
  read any existing workspace the Bob process can access. It is read
  authority, not a sandbox — the hosting process and agent runtime remain
  responsible for what the Bob process itself can reach.
- `--allow-apply-workspace` is repeatable and is the only way to grant
  `bob_apply` mutation authority. It is never implied by `--allow-workspace`
  or `--allow-any-workspace`. Each apply workspace is also readable so its
  plan can be reviewed.

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
- The nine read-only MCP tools never run specialist probes or write to the
  target repository. The one MCP mutation tool, `bob_apply`, is disabled
  unless `--allow-apply-workspace` names the exact workspace, and then writes
  only a plan whose fresh digest equals the reviewed `expect_plan_digest`.
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

- [Build any repository](./guides/any-repository.md) for the three catalog
  paths — stack hygiene on an existing language repo, `files` for a declared
  tree, and `go-agent-tool` for a new Go/Cobra CLI — with an agent-focused
  section on looping `check --json --conflicts-only`.
- [MCPHub & local-agent](./guides/mcphub-local-agent.md) for wiring the same
  tools through MCPHub and scoping them to a local-agent gateway.
- [Ownership & Safety](./ownership-and-safety.md) for the full plan-state
  vocabulary (`create`, `adopt`, `unchanged`, `update`, `conflict`).
- [CLI Reference](./reference/cli.md) for every command's flags in one table.
- Canonical docs: [bobcli.dev](https://bobcli.dev) and
  [bobcli.dev/agents](https://bobcli.dev/agents).
