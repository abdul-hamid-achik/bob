---
description: Normative reference for every Bob command, its flags, its repository effect, and its JSON envelope.
---

# CLI Reference

All repository commands accept an optional workspace path. When omitted, Bob
uses the current directory. Bob does not ask where you are; it checks.

## Commands

| Command | Repository effect | Purpose |
|---|---|---|
| `bob new <name>` | Preview by default; writes with `--write` | Create a new repository contract and initial files. |
| `bob init [path]` | Preview by default; writes `bob.yaml` with `--write` | Initialize Bob in an existing directory without generating files yet. |
| `bob context [path]` | Read-only, offline | Return the bounded repository contract; `--profile compact\|standard\|full` controls projection. |
| `bob path <repository-relative-path> [workspace]` | Read-only, offline | Classify one exact path through Bob's planner, lock, and extension metadata. |
| `bob playbook list\|show\|plan` | Read-only, offline | Inspect or resolve a closed, typed repository procedure without executing it. |
| `bob plan [path]` | Read-only | Compare desired and observed state; `--content` adds bounded previews, `--conflicts-only` trims to conflicts. |
| `bob apply [path]` | Writes | Apply one fresh, complete, conflict-free plan; `--expect-plan-digest` binds authority to a reviewed plan. |
| `bob check [path]` | Read-only | Exit non-zero when managed state or the lock would change; also accepts `--conflicts-only`. |
| `bob doctor [path]` | Runs bounded version probes | Check required and selected optional development tools. |
| `bob inspect [path]` | Read-only by default | Summarize Bob state and binary availability. |
| `bob config show` | Read-only | Show effective settings and resolved XDG paths. |
| `bob config init` | Preview by default; writes with `--write` | Initialize private user settings; `--telemetry` opts in. |
| `bob stats [path]` | Reads local XDG state | Return privacy-bounded usage aggregates. |
| `bob studio [path]` | Repository-read-only interactive UI | Monitor Overview, Plan, and aggregate Stats. |
| `bob explain` | Read-only | Describe product ownership and ecosystem boundaries. |
| `bob learn` | Read-only, no network | One-shot onboarding brief for coding agents. |
| `bob recipe list` | Read-only | List embedded recipes (`files@1`, `go-agent-tool@4`, and the stack hygiene recipes). |
| `bob recipe show <id>` | Read-only | Describe one recipe; `files` includes its schema and a copyable example. |
| `bob version` | Read-only | Print build version, commit, and date. |
| `bob mcp serve` | Long-running stdio server | Expose nine typed repository-read-only tools. |

`bob inspect --probe-integrations` is an explicit exception to the plain
read-only inventory: it launches selected Codemap and Vecgrep status commands.
See [Ownership & Safety](../ownership-and-safety.md#commands-and-authority).

## The recipe catalog

```text
$ bob recipe list
files@1  declare any file tree inline; bob materializes it with plan/apply safety
go-agent-tool@4  Public-ready Go and Cobra CLI with docs, CI, release plumbing, and optional ecosystem seams
js-app@1  Seed-once hygiene for a plain JavaScript Node app or workspace: docs presence, .gitignore, and a CI stub; never owns application source
lua-lib@1  Seed-once hygiene for a Lua library or Neovim plugin: docs presence, .gitignore, and a busted CI stub; never owns application source
python-app@1  Seed-once hygiene for a Python project: docs presence, .gitignore, and a pytest CI stub; never owns application source
ruby-app@1  Seed-once hygiene for a Ruby app or gem: docs presence, .gitignore, and a bundler/rake CI stub; never owns application source
rust-cli@1  Seed-once hygiene for a Rust CLI: docs presence, .gitignore, and a cargo CI stub; never owns application source
static-web@1  Seed-once hygiene for a static web site: docs presence, .gitignore, and a validation CI stub; never owns site content
ts-app@1  Seed-once hygiene for a TypeScript app or Bun/Turborepo monorepo: docs presence, .gitignore, and a CI stub; never owns application source
vue-app@1  Seed-once hygiene for a Vue application: docs presence, .gitignore, and a Vite-oriented CI stub; never owns application source
```

`bob recipe show <id>` describes one recipe: `files` prints its manifest
schema and a copyable example, and each stack hygiene recipe prints its stack
and the exact seed-once artifact paths. `bob new` scaffolds `go-agent-tool` only.
`bob init` detects the repository's stack (Go, TypeScript/Bun, JavaScript,
Vue, Python, Ruby, Lua, Rust, or a static web site) and defaults to the
matching recipe; pass `--recipe <id>` to choose explicitly. When the chosen
recipe does not match the detected stack, the preview prints a prominent
warning and `--write` refuses with exit code `4` (`input_invalid`) unless
`--force` is passed. `--module` is required only by `go-agent-tool`; for the
stack hygiene recipes it is optional repository identity. `bob init --json`
carries the full detection result in `data.detection` — `stacks` (each with
its proving marker files), `primary`, `monorepo`, `kind_hint`,
`package_manager`, and `signals` — alongside the previewed or written
manifest. A `files` manifest
is hand- or agent-authored. Stack hygiene recipes render only seed-once
artifacts: each file is created when missing, never recorded in `bob.lock`,
and never updated or overwritten afterwards — application source is never
touched. See the [Manifest Reference](./manifest.md) for the schemas and the
[any-repository guide](../guides/any-repository.md) for a worked `files`
example.

## `bob learn`

One-shot onboarding brief for coding agents. It takes no arguments, mutates
nothing, and makes no network call. Human mode prints a compact text
briefing; `--json` emits Bob's standard envelope with `command: "learn"` and a
`data` object covering the product, a summary, the lifecycle order
(init/new preview → plan → apply → check), every command's name, purpose,
mutation status, and JSON support, a field guide to the envelope itself, the
safety invariants, the MCP surface, the boundaries Bob refuses to own, the
recipe catalog (id, version, description for every embedded recipe), the
`exit_codes` and `error_codes` maps documented below, and the docs URLs. See
[Bob for coding agents](../agents.md) for the full contract and a worked
bootstrap sequence.

```bash
bob learn
bob learn --json
```

## `bob context`

`bob context [workspace]` is the workspace counterpart to `bob learn`. It
loads and validates `bob.yaml`, renders recipe metadata, computes one plan, and
projects the repository contract without writing files, launching subprocesses,
or contacting a network provider.

```bash
bob context .
bob context . --json
bob context . --profile full --json
```

Human output defaults to `standard`. JSON defaults to `compact`; an explicit
`--profile compact|standard|full` overrides either default. Compact data has a
6,144-byte limit, standard 24 KiB, and full 64 KiB. Every result carries a
`truncation` object and never truncates recipe identity, repository state,
conflict count, digests, or continuation actions.

The command distinguishes capability `selection`, `materialization`, local
binary `availability`, and `verification`. Verification is always
`not_assessed`: Bob does not turn a selected integration or a discovered
binary into a behavioral claim. The complete schema and closed vocabularies
are normative in [Workspace Context](./context.md).

## `bob path`

`bob path <repository-relative-path> [workspace]` answers what Bob owns and
what its next plan will do for one exact path. `--workspace <workspace>` is an
equivalent workspace form; do not supply both forms. It returns no file body
and does not claim an unmanaged path is globally safe.

```bash
bob path internal/cli/root.go . --json
bob path internal/domain/service.go --workspace .
```

Absolute paths, parent traversal, empty paths, and invalid UTF-8 fail as
`input_invalid`. `.git`, `bob.yaml`, `bob.lock`, and `.bob.apply.lock` are
classified explicitly as reserved. Symlinks and special files are observed
without following them. See [Path Classification](./path.md) for the schema
and closed vocabularies.

## `bob playbook`

Playbooks are deterministic recipe metadata, not a task runner and not
natural-language routing:

```bash
bob playbook list . --json
bob playbook show add-cli-command . --json
bob playbook plan resolve-ownership-conflict . \
  --set path=internal/cli/root.go \
  --set action_code=managed_hash_mismatch --json
```

The caller chooses a stable ID. `plan` validates a closed input schema and
returns ordered, argv-shaped steps; Bob executes none of them. Unknown keys,
duplicate keys, invalid values, and all missing required keys are reported as
input errors. See [Deterministic Playbooks](./playbooks.md).

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
| `plan_digest_mismatch` | A guarded apply fresh-planned a different repository state than the caller reviewed. |
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

## Plan identity

`plan --json` adds `plan_digest_version` and `plan_digest` to the complete plan.
`check --json` exposes the same fields both beside `clean` and inside its plan.
Version 1 is the unchanged identity previously used by MCP `bob_plan` and
`bob_check`: it covers recipe identity, lock drift, desired lock, and every
complete action without previews or output filtering. The digest is lowercase
hexadecimal with a `sha256:` prefix in CLI plan/check and workspace context.
For wire compatibility, MCP keeps its original raw `plan_digest` and adds the
directly consumable `plan_digest_qualified` field.

## Digest-gated apply

`bob apply [workspace] --expect-plan-digest sha256:<64-lowercase-hex>` binds
explicit mutation authority to one reviewed complete plan. CLI and MCP
plan/check expose a qualified value that can be copied directly: CLI uses
`plan_digest`, while MCP uses `plan_digest_qualified`. Bob accepts no
whitespace, uppercase, or unqualified form.

While holding the existing workspace apply lock, Bob loads and renders the
current `bob.yaml`, computes a fresh plan, and compares its identity before
conflict preflight, staging, or repository writes. The exact manifest source is
rechecked before staging and publication. A mismatch exits `5`, emits
`plan_digest_mismatch` with both the expected and actual qualified digests, and
leaves no staged paths, lock change, or apply-lock file behind.

Successful JSON apply output contains an apply receipt with schema version,
plan-digest version, expected and applied digests, written/adopted/unchanged
paths and complete counts, `lock_written`, `converged_after_apply`, an
argv-shaped `next_check`, and explicit truncation metadata. The encoded receipt
is capped at 16 KiB and retains at most 256 path entries, prioritizing written,
then adopted, then unchanged paths; omitted suffix counts are deterministic.
Identity, digests, complete counts, convergence, and `next_check` are never
omitted.

Apply JSON now returns this receipt directly in `data`; it does not echo the
complete plan a second time. Callers that need actions retain the separately
reviewed `plan --json` result identified by `applied_plan_digest`. The receipt
is returned once and is not persisted. It proves which Bob reconciliation ran,
not that generated application behavior passed.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. `bob plan` always exits `0`, even when it finds conflicts — plan is a read-only report, not a gate. |
| `1` | Unclassified command failure (`command_failed`), a workspace path that could not be resolved (`workspace_invalid`), or `doctor` reporting that required tools are missing or unusable — a determinate not-ready result, not a crash. |
| `2` | `apply` refused a conflicted plan, or `check` found an ownership conflict. |
| `3` | `check` found drift with no ownership conflict. |
| `4` | Invalid input: a missing or invalid manifest (including an unrecognized `recipe:` id in `bob.yaml`), a bad flag or argument, or `bob init --write` refusing a recipe that does not match the detected stack without `--force`. |
| `5` | `apply --expect-plan-digest` refused because the fresh plan differs from the reviewed plan; zero repository writes occurred. |

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
| `bob_context` | Bounded workspace contract; defaults to the compact profile. |
| `bob_path` | Exact Bob relationship to one repository-relative path, with no file body. |
| `bob_playbook` | Closed `list`, `show`, or `plan` procedure projection; never executes a step. |
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

`bob_context` accepts only `compact`, `standard`, or `full` and defaults to
compact. `bob_path.path` is repository-relative and limited to 4 KiB.
`bob_playbook.operation` is `list`, `show`, or `plan`; IDs are limited to 128
bytes and a plan accepts at most 32 values with 128-byte keys and 4-KiB values.
Each guidance request is limited to 64 KiB. Context targets less than 8 KiB
end-to-end in compact mode, path and playbook-list results are capped at 8 KiB,
and playbook show/plan results are capped at 24 KiB with explicit truncation.
