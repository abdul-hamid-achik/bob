---
description: How Bob proves file ownership, why it refuses conflicts instead of guessing, and how to read a plan.
---

# Ownership & Safety

Bob's defining feature is not file generation. Any template engine can spray
files onto a disk. Bob's job is proving when a generated file may be safely
changed again, and refusing the moment it can't prove that.

## Two repository contracts

`bob.yaml` is human-owned intent. It selects the recipe, product identity,
surfaces, optional integrations, and distribution choices.

`bob.lock` is Bob-owned evidence. It records the recipe ID and version plus the
SHA-256 digest of every whole file Bob manages. It contains no commands,
credentials, environment values, or execution history.

## Plan actions

| Action | Meaning | Apply behavior |
|---|---|---|
| `create` | The desired path does not exist. | Bob may create it. |
| `adopt` | An unmanaged regular file already matches exactly. | Bob may record ownership without replacing it. |
| `unchanged` | The managed file matches the recipe and lock. | No file write. |
| `update` | The file still matches its old lock, but the recipe or mode changed. | Bob may replace it safely. |
| `conflict` | Ownership is absent, stale, or unsafe. | The complete apply is refused. |

One conflict blocks every planned write. Bob never partially applies a plan it
already knows is conflicted. It would rather do nothing than do half a job.

## Files Bob refuses to own

Recipe output cannot target:

- absolute paths or parent traversal;
- `.git` or anything beneath it;
- `bob.yaml` or `bob.lock`;
- a pre-existing symlink, directory, device, socket, or named pipe.

Bob also refuses to overwrite an unmanaged differing file or a managed file
whose current hash no longer matches `bob.lock`. If you hand-edited a
Bob-managed file, Bob notices, and it stops rather than clobbering your edit.

## Publication and crash recovery

Changed files are staged as temporary siblings and published with atomic rename
operations. Bob rechecks file and lock preconditions immediately before
publication and writes `bob.lock` last, because the lock is the receipt and you
don't hand out a receipt before the goods ship.

The multi-file operation is not globally transactional. A process crash can
publish some matching files before the new lock lands. The next `bob plan`
reports the actual state and may classify those exact files as safe `adopt`
actions. Review that plan before continuing. Bob doesn't panic about a crash
mid-apply; it just tells you exactly where the truck stopped.

## Commands and authority

- `context`, `path`, `playbook`, `plan`, `check`, plain `inspect`, `stats`, and Studio do not mutate the
  repository.
- `inspect --probe-integrations` explicitly launches selected status commands;
  current Codemap may open tool-owned state and Vecgrep may contact its provider.
- `apply` is the explicit repository mutation command. Optional
  `--expect-plan-digest` binds it to a freshly recomputed reviewed plan before
  staging or writing.
- All nine MCP tools have read-only repository effects. Manifest validation may
  also operate on bounded inline YAML; recipe description needs no workspace.
- MCP starts with an exact workspace allowlist. `--allow-workspace` adds exact
  paths and `--allow-any-workspace` deliberately broadens read authority.

Repository-read-only does not mean Bob never writes any machine-local state.
When telemetry is explicitly enabled, recorded CLI and MCP operations may
append a privacy-bounded event beneath Bob's XDG state directory. Telemetry is
disabled by default, has no network transport, and cannot represent paths,
arguments, filenames, content, or raw errors. `stats`, `bob_stats`, Studio, and
configuration commands do not record events.

Studio never runs specialist probes and exposes no apply, shell, editor,
indexing, or repair action. A displayed next action is inert text until a person
or agent invokes it through the normal authority path.

MCP annotations describe intent but do not grant permission. MCPHub and the
calling agent runtime remain separate authorization boundaries.
