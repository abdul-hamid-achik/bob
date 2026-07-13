# Ownership & Safety

Bob's defining feature is not file generation. It is proving when a generated
file may be changed again.

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
already knows is conflicted.

## Files Bob refuses to own

Recipe output cannot target:

- absolute paths or parent traversal;
- `.git` or anything beneath it;
- `bob.yaml` or `bob.lock`;
- a pre-existing symlink, directory, device, socket, or named pipe.

Bob also refuses to overwrite an unmanaged differing file or a managed file
whose current hash no longer matches `bob.lock`.

## Publication and crash recovery

Changed files are staged as temporary siblings and published with atomic rename
operations. Bob rechecks file and lock preconditions immediately before
publication and writes `bob.lock` last.

The multi-file operation is not globally transactional. A process crash can
publish some matching files before the new lock. The next `bob plan` reports the
actual state and may classify those exact files as safe `adopt` actions. Review
that plan before continuing.

## Commands and authority

- `plan`, `check`, and plain `inspect` do not mutate the repository.
- `inspect --probe-integrations` explicitly launches selected status commands;
  current Codemap may open tool-owned state and Vecgrep may contact its provider.
- `apply` is the explicit repository mutation command.
- `bob_inspect` and `bob_plan` are the only MCP tools; both are read-only.

MCP annotations describe intent but do not grant permission. MCPHub and the
calling agent runtime remain separate authorization boundaries.
