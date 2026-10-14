---
description: Normative reference for every Bob command, its flags, its repository effect, and its JSON envelope.
---

# CLI Reference

All repository commands accept an optional workspace path. When omitted, Bob
uses the current directory. Bob does not ask where you are; it checks.

## Commands

| Command | Repository effect | Purpose |
|---|---|---|
| `bob new <name>` | Preview by default; writes with `--write` | Create a repository contract and initial files; stack hygiene recipes require a target whose application stack already exists. |
| `bob init [path]` | Preview by default; writes `bob.yaml` with `--write` | Initialize Bob in an existing directory without generating files yet. |
| `bob context [path]` | Read-only, offline | Return the bounded repository contract; `--profile compact\|standard\|full` controls projection, `--allow-retire` previews the retire case. |
| `bob path <repository-relative-path> [workspace]` | Read-only, offline | Classify one exact path through Bob's planner, lock, and extension metadata. |
| `bob playbook list\|show\|plan` | Read-only, offline | Inspect or resolve a closed, typed repository procedure without executing it. |
| `bob plan [path]` | Read-only | Compare desired and observed state; `--content` adds bounded previews, `--conflicts-only` trims to conflicts, `--diff` shows unified content diffs, `--sarif` emits a SARIF 2.1.0 log for code scanning, `--junit` emits a JUnit XML report for CI test reporting, `--allow-retire` classifies dropped lock-owned files as retire actions, and `--watch` refreshes human output when `bob.yaml` changes. |
| `bob apply [path]` | Writes | Apply one fresh, complete, conflict-free plan; `--expect-plan-digest` binds authority to a reviewed plan, `--allow-retire` deletes dropped lock-owned files. |
| `bob remove [path]` | Writes | Remove lock-owned regular files and `bob.lock`; `--force` removes drifted files, `--dry-run` previews without writing and reports a removal digest, `--expect-removal-digest` binds removal to that reviewed decision. |
| `bob upgrade [path]` | Writes | Upgrade `bob.lock` to the current recipe version; `--dry-run` previews (with `--json` it reports the `plan_digest` to review), `--expect-plan-digest` gates authority, `--allow-retire` deletes dropped lock-owned files. |
| `bob check [path]` | Read-only | Exit non-zero when managed state or the lock would change; also accepts `--conflicts-only`, `--sarif`, `--junit`, and `--allow-retire`. |
| `bob doctor [path]` | Runs bounded version probes | Check required and selected optional development tools. |
| `bob inspect [path]` | Read-only by default | Summarize Bob state and binary availability. |
| `bob config show` | Read-only | Show effective settings and resolved XDG paths. |
| `bob config init` | Preview by default; writes with `--write` | Initialize private user settings; `--telemetry` opts in. |
| `bob stats [path]` | Reads local XDG state | Return privacy-bounded usage aggregates. |
| `bob studio [path]` | Repository-read-only interactive UI | Monitor Overview, Plan, and aggregate Stats. |
| `bob explain` | Read-only | Describe product ownership and ecosystem boundaries. |
| `bob learn` | Read-only, no network | One-shot onboarding brief for coding agents. |
| `bob recipe list` | Read-only | List embedded recipes (`files@1`, `go-agent-tool@6`, `java-tool@1`, `python-tool@1`, `rust-tool@1`, `ts-tool@1`, and the stack hygiene recipes). |
| `bob recipe show <id>` | Read-only | Describe one recipe; `files` includes its schema and a copyable example. |
| `bob version` | Read-only | Print build version, commit, and date. |
| `bob mcp serve` | Long-running stdio server | Expose nine typed repository-read-only tools plus `bob_apply`, a digest-gated mutation tool disabled unless `--allow-apply-workspace` names the exact workspace. |

`bob inspect --probe-integrations` is an explicit exception to the plain
read-only inventory: it launches selected Codemap and Vecgrep status commands.
See [Ownership & Safety](../ownership-and-safety.md#commands-and-authority).

## The recipe catalog

```text
$ bob recipe list
ansible-app@2  Seed-once hygiene for Ansible playbooks/roles: docs, .gitignore, and a lint CI stub; never owns playbook content
cpp-cmake@2  Seed-once hygiene for a C/C++ project: docs, .gitignore, and a CMake CI stub; never owns application source
dart-app@2  Seed-once hygiene for a Dart/Flutter project: docs, .gitignore, and an analyze/test CI stub; never owns application source
deno-app@2  Seed-once hygiene for a Deno project: docs, .gitignore, and a Deno CI stub; never owns application source
docker-app@2  Seed-once hygiene for a Docker image: docs, .dockerignore, and a build CI stub; never owns application source
dotnet-app@2  Seed-once hygiene for a .NET application or library: docs, .gitignore, and a dotnet CI stub; never owns application source
elixir-app@2  Seed-once hygiene for an Elixir application or umbrella: Mix formatting, tests, ignores, and CI; never owns application source
files@1  declare any file tree inline; bob materializes it with plan/apply safety
go-agent-tool@6  Public-ready Go and Cobra CLI with docs, CI, release plumbing, and optional ecosystem seams
go-hygiene@2  Seed-once hygiene for an existing Go repository: docs, .gitignore, and golangci-lint CI; never owns application source
helm-app@2  Seed-once hygiene for a Helm chart: docs, .gitignore, and a lint/template CI stub; never owns chart content
java-app@2  Seed-once hygiene for a Java application or library: docs, .gitignore, and a Maven CI stub; never owns application source
java-tool@1  Public-ready Java (Maven) CLI scaffold with docs, CI, and a thin main entry point over a tested pure function
js-app@2  Seed-once hygiene for a JavaScript Node app or workspace, using the declared package manager; never owns application source
kotlin-app@2  Seed-once hygiene for a Kotlin application or library: docs, .gitignore, and a Gradle CI stub; never owns application source
kubernetes-app@2  Seed-once hygiene for Kubernetes manifests: docs, .gitignore, and a render/validate CI stub; never owns manifest content
lua-lib@2  Seed-once hygiene for a Lua library or Neovim plugin with kind-aligned runtime configuration; never owns application source
nix-app@2  Seed-once hygiene for a Nix flake or package: docs, .gitignore, and a flake-check CI stub; never owns application source
nuxt-app@2  Seed-once hygiene for a Nuxt application, preserving JavaScript or TypeScript and its package manager; never owns application source
php-app@2  Seed-once hygiene for a PHP project: docs, .gitignore, and a PHPUnit CI stub; never owns application source
python-app@2  Seed-once hygiene for a Python project: docs, aligned Python tooling, and pytest CI; never owns application source
python-tool@1  Public-ready Python CLI scaffold with docs, CI, and an argparse entry point over a tested pure function
ruby-app@2  Seed-once hygiene for a Ruby app or gem with kind-aware Bundler defaults; never owns application source
rust-cli@2  Seed-once hygiene for a Rust CLI, library, or workspace: Cargo checks and CI; never owns application source
rust-tool@1  Public-ready Rust CLI scaffold with docs, CI, and a thin binary over a tested library core
scala-app@2  Seed-once hygiene for a Scala sbt project: docs, .gitignore, and a test CI stub; never owns application source
static-web@2  Seed-once hygiene for a static web site: docs presence, .gitignore, and a validation CI stub; never owns site content
swift-package@2  Seed-once hygiene for a Swift package: SwiftPM commands, ignores, and CI; never owns package source
terraform-app@2  Seed-once hygiene for Terraform/IaC: docs, .gitignore, and a validate CI stub; never owns application source
ts-app@2  Seed-once hygiene for a TypeScript app or workspace, using the declared package manager; never owns application source
ts-tool@1  Public-ready TypeScript (Node) CLI scaffold with docs, CI, and a process.argv entry point over a tested pure function
vue-app@2  Seed-once hygiene for a Vue application, preserving JavaScript or TypeScript and its package manager; never owns application source
zig-app@2  Seed-once hygiene for a Zig project: docs, .gitignore, and a build/test CI stub; never owns application source
```

`bob recipe show <id>` describes one recipe: `files` prints its manifest
schema and a copyable example, and each stack hygiene recipe prints its stack
and the exact seed-once artifact paths. `bob new` previews any catalog recipe
via `--recipe <id>`. When `--recipe` is omitted, a target directory
whose content matches a detected stack auto-selects that stack's recipe (a
stack recipe may then scaffold into the non-empty target because it only
seeds create-once files), and a greenfield (missing or empty) target keeps
the historical `go-agent-tool` default. A stack-recipe `--write` against a
greenfield target refuses because these recipes never scaffold application
source; create the Swift package, Mix project, Node app, or other application
first, then use `bob init --write` and `bob apply`. An explicit stack
`--recipe` that
does not match the target's detected stack warns on preview and refuses
`--write` (run `bob init --recipe <id> --force` to seed it anyway). For
`bob new`, `--module` is required by `go-agent-tool`, optional repository
identity for `java-tool`, `python-tool`, `rust-tool`, and `ts-tool`, and rejected by every other recipe.
`bob init` detects the repository's stack (Go, TypeScript, JavaScript,
Vue, Nuxt, Deno, Python, PHP, Ruby, Lua, Rust, Java, Kotlin, .NET, C/C++ with
CMake, SwiftPM, Elixir/Mix, Dart, Zig, Scala, Docker, Terraform, Nix, Ansible,
Helm, Kubernetes, or a static web site) and defaults to the
matching recipe; pass `--recipe <id>` to choose explicitly. For Go, a
`recipe:` already declared in an existing `bob.yaml` wins; otherwise a Cobra
layout (`cmd/` plus `internal/cli/root.go`) hints `go-agent-tool` only when
`go.mod` requires `github.com/spf13/cobra`, and any
other Go module defaults to `go-hygiene`. When a regular-file `bob.yaml`
already exists at the target, the preview warns (stderr in human mode,
`warnings[]` in JSON) and points `next_actions` at `bob plan <path> --json`
instead of suggesting `--write`; `bob init --write` refuses with exit code `4`
(`input_invalid`), names `bob plan`, and leaves the existing manifest
byte-for-byte unchanged. When the chosen
recipe does not match the detected stack, the preview prints a prominent
warning and `--write` refuses with exit code `4` (`input_invalid`) unless
`--force` is passed. `--module` is required only by `go-agent-tool`; for
`java-tool`, `python-tool`, `rust-tool`, `ts-tool`, and the stack hygiene recipes it is optional repository identity. `bob init --json`
carries the full detection result in `data.detection` — `stacks` (each with
its proving marker files), `primary`, `monorepo`, `kind_hint`,
`package_manager`, and `signals` — alongside the previewed or written
manifest. For JavaScript-family repositories, `runtime.package_manager`
preserves a detected Bun, npm, pnpm, or Yarn lockfile; Vue also preserves
JavaScript when TypeScript markers are absent. A `files` manifest
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
bob context . --allow-retire --json
```

Human output defaults to `standard`. JSON defaults to `compact`; an explicit
`--profile compact|standard|full` overrides either default. Compact data has a
6,144-byte limit, standard 24 KiB, and full 64 KiB. Every result carries a
`truncation` object and never truncates recipe identity, repository state,
conflict count, digests, or continuation actions.

`--allow-retire` opts the context plan into the retire action: a lock-owned
regular file the recipe dropped is classified as a non-conflict `retire` action
instead of a `retired_owned` conflict, so context previews the `apply_retire`
action and an `info` notice rather than the default `warning`. It is a
read-only preview — it changes the plan projection, not the repository. See
[Retiring dropped files](#retiring-dropped-files).

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

`stats`, Studio, and the nine read-only MCP tools never mutate repositories.
The MCP `bob_apply` tool writes only when `bob mcp serve` was started with
`--allow-apply-workspace` for that workspace and a fresh plan matches the
exact reviewed `expect_plan_digest`. When local telemetry is
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
command's normal `data` replaced by an error. Two outcomes are the exception:
`check` on drift keeps `data.clean: false` with `data.plan` and the plan digest
fields, and `doctor` reporting a required tool missing or unusable keeps its
probe results — both with `ok: false`, no `data.error`, and the non-zero exit
code as the signal. A `check` or `doctor` that cannot load `bob.yaml` returns an
ordinary `data.error` envelope like any other command. The envelope shape below
applies to those ordinary failures:

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
| `plan_digest_mismatch` | A guarded apply or upgrade fresh-planned a different repository state than the caller reviewed. Its `data.converged` is `true` when the fresh plan is already clean. |
| `workspace_busy` | Another mutating Bob operation holds the workspace apply lock; nothing was written. Retry once it finishes. |
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

## SARIF output

`plan` and `check` both accept `--sarif`, which writes a SARIF 2.1.0 log to
stdout instead of the human or JSON plan. It is a drop-in artifact for GitHub
code scanning and other SARIF consumers:

```bash
bob plan --sarif
bob check --sarif
```

The document reports one `result` per non-converged action — `conflict`,
`update`, `retire`, and `create` — keyed by the action's stable `code` as the
`ruleId`. `unchanged` and `adopt` actions are omitted. The `level` follows the
action kind: `error` for conflicts, `warning` for updates and retires, and
`note` for creates. Each result's message carries the action's human `reason`,
and its location is the repository-relative path.

`--sarif` and `--json` are mutually exclusive; combining them is invalid input
(exit `4`). Exit codes are unchanged: `plan --sarif` always exits `0`, and
`check --sarif` still exits `2` on an ownership conflict and `3` on drift, so a
code-scanning upload fails the build exactly as a plain `check` would.

## JUnit output

`plan` and `check` both accept `--junit`, which writes a JUnit XML report to
stdout instead of the human or JSON plan. It is a drop-in artifact for CI test
reporters:

```bash
bob plan --junit
bob check --junit
```

The document is a single `<testsuites>` containing one `<testsuite>` named
`bob plan` or `bob check`, with one `<testcase>` per action. A testcase's
`name` is the repository-relative path and its `classname` is the action kind.
Non-converged actions — `create`, `update`, `retire`, and `conflict` — are
failures carrying a `<failure type="<code>" message="<reason>"/>`; `unchanged`
and `adopt` actions are passing testcases. Paths and reasons are XML-escaped.

`--junit` is mutually exclusive with `--json` and `--sarif`; combining them is
invalid input (exit `4`). Exit codes are unchanged: `plan --junit` always exits
`0`, and `check --junit` still exits `2` on an ownership conflict and `3` on
drift, so a failing CI test report maps to the same exit code as a plain
`check`.

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

## Recipe upgrade

`bob upgrade [workspace]` migrates a lock written by an older version of the
same recipe to the current embedded recipe. It uses the prior lock hashes as
the ownership proof, renders the current desired artifacts, and applies the
same whole-plan conflict and publication rules as `bob apply`. A lock from a
future version or a different recipe is refused.

`--dry-run` returns the observed and target versions, the complete plan, and
whether migration is required without writing. A current lock is a successful
no-op. `--expect-plan-digest` accepts the same qualified digest as apply and
refuses stale authority with `plan_digest_mismatch` and zero writes. Successful
JSON output reports `from_version`, `to_version`, `applied`, and the bounded
apply receipt when a migration ran. `--dry-run --json` also reports
`plan_digest` and `plan_digest_version` for the fresh plan, the same values
`bob plan --json` reports, so the reviewed preview can be passed straight to
`--expect-plan-digest` without a separate `bob plan` call.

Upgrade holds the workspace apply lock for the full fresh-plan and mutation
sequence. Conflicts exit `2`; a missing, invalid, cross-recipe, or future lock
exits `4`; a digest mismatch exits `5`.

## Retiring dropped files

When a `files` manifest drops a path that `bob.lock` still owns, the plan
reports a `retired_owned` conflict by default: the file blocks the entire apply
until the human removes it by hand. `plan`, `check`, `apply`, and `upgrade`
each accept `--allow-retire`, which reclassifies a dropped lock-owned regular
file whose current content still matches its lock hash as a non-conflict
`retire` action (`retired_unchanged`). With the flag set, `apply` deletes the
file and drops it from `bob.lock` (which is still written last). A dropped
file that drifted, or a symlink or special file, remains a conflict that blocks
apply regardless of the flag. The default is unchanged: without
`--allow-retire`, a dropped path stays a `retired_owned` conflict and the plan
digest is identical.

## Plan diff preview

`bob plan [workspace] --diff` appends unified content diffs for every create
and update action after the normal plan summary. The diff is computed by a
bounded LCS algorithm (standard library only, no external dependencies) and
produces standard unified diff output with `--- a/`, `+++ b/`, and `@@` hunk
headers with three lines of context.

Files whose content exceeds 1 MiB or 8192 lines are skipped with a note;
binary (non-UTF-8) content is likewise skipped. The `--diff` flag is
presentation-only: it never affects the plan digest, the action list, or any
engine behavior. JSON output adds a `diffs` array (omitted when `--diff` is
not set).

## Plan watch

`bob plan [workspace] --watch` renders the normal human plan immediately and
refreshes it whenever `bob.yaml` changes. It is a read-only convenience for a
person editing the manifest: invalid or temporarily missing manifests are
shown inline and watching continues until interrupted. Because it is a live
terminal view, `--watch` cannot be combined with `--json`.

## Safe remove

`bob remove [workspace]` releases the files whose ownership is recorded in
`bob.lock`. It removes only tracked files whose content hash still matches the
lock entry — the same ownership proof apply uses. Unmanaged files, seed-once
artifacts, `bob.yaml`, and directories containing unmanaged files are never
touched.

- `--force` removes managed files even when their content has drifted from
  the lock (hash mismatch). Without `--force`, drifted files are skipped and
  reported.
- `--dry-run` previews what a real run would remove without writing anything
  and reports `removal_digest` and `removal_digest_version`.
- `--expect-removal-digest sha256:<64-lowercase-hex>` binds the removal to one
  reviewed decision. See [Digest-gated remove](#digest-gated-remove).
- Symlinks and special files at managed paths are reported as conflicts and
  never removed, even with `--force`.
- Empty directories left behind by removed files are cleaned up bottom-up;
  the workspace root itself is never removed.
- `bob.lock` is removed last, and only when nothing was skipped or
  conflicted. A partial remove retains the lock so a later `--force` run can
  still prove and delete the remaining files.

Immediately before each unlink, Bob rechecks the file type and content hash so
an edit made after the initial inspection is preserved. Immediately before
removing `bob.lock`, Bob also requires its bytes to match the lock it loaded.
These checks close the normal editor-save race; like apply, remove is not a
globally transactional multi-file operation.

Remove acquires the same `.bob.apply.lock` used by apply to prevent
concurrent mutations. The lock records the owning pid and hostname; a lock
left by a crashed Bob on the same host is reclaimed automatically when that
process no longer exists, while a lock from another host or a live process is
refused. Exit codes: `0` clean success, `2` when files were
skipped or conflicted, `4` when no lock exists, the manifest is invalid, or the
expected removal digest is malformed, `5` when a guarded remove's digest no
longer matches.

## Digest-gated remove

`bob remove [workspace] --dry-run --json` reports `removal_digest`, a
`sha256:<64-lowercase-hex>` identity of the complete removal decision, and
`removal_digest_version` (currently `1`). Real runs report the same fields.
The digest covers a `remove` kind marker, the digest schema version, the hash
of the exact `bob.lock` bytes, the `--force` flag, and, for every managed path
in lock order, its path, locked hash, current hash, current mode, and removal
decision. It is a separate namespace from the plan digest: a plan digest never
authorizes a removal, and a removal digest never authorizes an apply or
upgrade.

`bob remove [workspace] --expect-removal-digest sha256:<64-lowercase-hex>`
binds the destructive run to that reviewed decision. Pass the same `--force`
setting used for the dry-run; a digest reviewed without `--force` does not
authorize a `--force` removal. Bob validates the digest form before touching
the workspace (a malformed value exits `4` with `input_invalid`), then
recomputes the decision while holding the workspace apply lock and compares it
before the first unlink. A mismatch exits `5`, emits `plan_digest_mismatch`
with `expected_removal_digest` and `actual_removal_digest`, and deletes
nothing. The per-file recheck immediately before each unlink still applies to a
matching run.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. `bob plan` always exits `0`, even when it finds conflicts — plan is a read-only report, not a gate. |
| `1` | Unclassified command failure (`command_failed`), a workspace path that could not be resolved (`workspace_invalid`), another mutating operation holding the workspace apply lock (`workspace_busy`; retry once it finishes), or `doctor` reporting that required tools are missing or unusable — a determinate not-ready result, not a crash. |
| `2` | `apply` or `upgrade` refused a conflicted plan, `check` found an ownership conflict, or `remove` skipped or conflicted on managed files. |
| `3` | `check` found drift with no ownership conflict. |
| `4` | Invalid input: a missing or invalid manifest (including an unrecognized `recipe:` id in `bob.yaml`), a bad flag or argument, `bob init --write` refusing a `bob.yaml` that already exists or a recipe that does not match the detected stack without `--force`, `bob remove` when no lock exists, or `bob upgrade` when its lock is missing, incompatible, or newer than this binary supports. |
| `5` | `apply` or `upgrade` with `--expect-plan-digest` refused because the fresh plan differs from the reviewed plan, or `remove` with `--expect-removal-digest` refused because the fresh removal decision differs from the reviewed one; zero repository writes occurred. |

An agent scripting Bob should branch on the exit code first, then read
`data.error.code` for detail — except `check` on drift and a not-ready
`doctor`, which carry no `data.error` and are read from `data.clean` or the
probe results instead. Do not infer success from the mere presence of JSON on
stdout. See [Bob for coding agents](../agents.md#recovering-from-failure)
for a code-by-code recovery playbook.

## Workspace paths

CLI write commands reject a symlink at the selected workspace boundary. The MCP
server canonicalizes its startup `--workspace` and uses it as an exact allowlist
by default. Repeat `--allow-workspace <path>` for additional exact existing
workspaces. `--allow-any-workspace` explicitly accepts any existing workspace
the process can read. The hosting process and agent runtime remain responsible
for filesystem access control.

Mutation authority for `bob_apply` is separate and empty by default. Repeat
`--allow-apply-workspace <path>` to name each exact existing workspace it may
mutate; each such workspace is also added to the read allowlist so its plan can
be reviewed. `--allow-workspace` and `--allow-any-workspace` never grant apply
authority.

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
| `bob_apply` | **Mutating, opt-in.** Digest-gated apply of a reviewed plan; disabled unless `--allow-apply-workspace` names the exact workspace. |

The first nine tools are annotated repository-read-only, non-destructive,
idempotent, and closed-world. `bob_apply` is annotated destructive,
non-idempotent, closed-world, and not read-only; annotations describe intent
and grant no permission.

`bob_apply` accepts an optional `workspace` and a required
`expect_plan_digest` in the exact `sha256:<64-lowercase-hex>` form (copy
`plan_digest_qualified`). It calls the same lock-owned engine path as
[digest-gated apply](#digest-gated-apply) and returns a typed `outcome`:
`applied` with the apply `receipt`; `plan_digest_mismatch` with
`expected_plan_digest`, `actual_plan_digest`, and `converged` (the current plan
is clean — Bob keeps no history and never claims "already applied");
`conflicts` with one `conflicts[]` entry per blocked path; `workspace_busy`
with `retryable: true` (lock contention fails immediately, without waiting);
`apply_unauthorized`; `workspace_invalid`; or `input_invalid`. Every outcome
other than `applied` writes nothing. Cancellation is honored only before the
engine starts; once staging begins, apply runs to completion so files are never
left published without their lock. Telemetry records the existing `apply`
operation under the `mcp` surface.

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
