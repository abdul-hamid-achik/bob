---
description: Normative contract for Bob's closed, typed, non-executing repository playbooks.
---

# Deterministic Playbooks

A Bob playbook is versioned recipe metadata. It describes a bounded procedure;
it never writes a file, edits a manifest, launches a command, probes a tool, or
claims verification. There is no natural-language matcher.

```bash
bob playbook list [workspace] --json
bob playbook show <id> [workspace] --json
bob playbook plan <id> [workspace] --set key=value --json
```

The stable initial IDs are:

```text
add-cli-command
enable-github-actions
enable-goreleaser
enable-homebrew
enable-terminal-verification
resolve-ownership-conflict
upgrade-recipe
```

`files@1` publishes only the generic conflict-resolution and recipe-upgrade
procedures. It does not infer language, framework, commands, or business
meaning from declared paths or content.

## Availability and recipe state

`applicable` means the playbook belongs to the active recipe contract.
`available` means its deterministic preconditions are currently met.
`blocked_by` is a list of stable reason codes.

`add-cli-command` belongs to `go-agent-tool@4` and becomes available only after
the Bob-owned version-4 root and registry contract is materialized and
converged. An older lock or pending composition-file reconciliation reports the
stable `extension_contract_not_materialized` blocker. Once available, the
playbook creates a human-owned command implementation and test through
`cli.command_files`; the Bob-owned root and registry files are explicitly
forbidden. Registration and command mounting are deterministic, and duplicate
IDs or command names fail visibly.

Enablement playbooks modify only human-owned `bob.yaml`, then describe plan,
explicit apply, and convergence-check steps. Homebrew reports each prerequisite
and adds a human-decision step instead of silently enabling other fields.
`upgrade-recipe` distinguishes no lock, an already-current lock, and an older
lock using the coherent plan snapshot.

## Typed inputs

Each definition publishes a closed `inputs` array with name, required flag,
type, validation code, and optional enum. Resolution:

- rejects unknown and duplicate keys;
- lists all missing required keys together;
- validates lowercase-kebab identifiers, safe relative paths, and closed enums;
- rejects recipe-declared reserved values, including the lazy Cobra `help`
  command name for `add-cli-command`;
- treats values as data, never as shell fragments;
- substitutes only named placeholders in path fields and argv elements.

A request accepts at most 32 values; keys are limited to 128 bytes and values
to 4 KiB.

`resolve-ownership-conflict` requires both `path` and `action_code` and refuses
resolution when the supplied action code does not equal the current planner
action for that path. Its inspection step places flags before an argv `--`
delimiter and the resolved path after it, so a repository filename such as
`--help` remains data rather than becoming a command-line flag.

## Step contract

Each ordered step has a stable `id`, `kind`, `effect`, `paths`, `argv`,
dependencies, blockers, explicit-authority flag, and success condition. `kind`
uses `inspect`, `agent_edit`, `manifest_edit`, `command`, `bob_plan`,
`bob_apply`, `bob_check`, or `human_decision`. `effect` uses the shared
`read_only`, `subprocess_probe`, `repository_mutation`, or
`user_configuration_mutation` vocabulary.

Scope is deterministic: `metadata_only`, `single_file`, `small`,
`multi_surface`, or `repository_wide`. Risk is `low`, `medium`, or `high`.
Neither field estimates time or model ability.

List output is capped at 8 KiB. Show and resolved-plan output are capped at 24
KiB and carry an explicit `truncation` object. Current built-in definitions fit
without omission. No output contains raw file bodies or subprocess output.
