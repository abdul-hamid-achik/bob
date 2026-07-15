---
description: Normative schema and closed vocabularies for exact Bob path ownership classification.
---

# Path Classification

`bob path <repository-relative-path> [workspace]` is a read-only, offline
explanation of Bob's relationship to one exact path. It uses the same
canonical workspace, rendered recipe, complete plan, `bob.lock`, symlink
checks, special-file checks, and whole-file ownership rules as the engine.
Recipe extension patterns are considered only after managed and reserved paths
have been classified.

The command does not return file bodies and never emits `safe_to_edit`. Outside
Bob ownership means only that Bob does not own the path; it is not a product,
security, or behavioral safety claim.

## Schema version 1

The standard CLI envelope has `command: "path"`. Its `data` includes:

```json
{
  "schema_version": 1,
  "workspace": "/canonical/workspace",
  "path": "internal/cli/root.go",
  "exists": true,
  "classification": "managed",
  "state": "managed_in_sync",
  "human_edit_effect": "will_conflict",
  "ownership": {
    "recipe": {"id": "go-agent-tool", "version": 4},
    "locked_sha256": "...",
    "current_sha256": "..."
  },
  "plan_action": {"kind": "unchanged", "code": "in_sync"},
  "artifact": {"id": "cli.root", "roles": ["cli", "composition_root"], "capability_ids": ["surface.cli", "surface.json"]},
  "extension_points": [],
  "related_playbooks": ["add-cli-command"],
  "notices": [],
  "actions": [],
  "truncation": {"profile": "path", "byte_limit": 8192, "truncated": false, "omitted": {}}
}
```

## Closed vocabularies

`classification` is one of `managed`, `reserved`, `extension_point`,
`unmanaged`, or `missing`.

`state` is one of `managed_in_sync`, `managed_modified`, `managed_missing`,
`retired_owned`, `extension_point`, `unmanaged_present`, `unmanaged_missing`,
`reserved`, `symlink`, or `special_file`.

`human_edit_effect` is one of `will_conflict`, `outside_bob_ownership`,
`reserved_for_bob`, `requires_manifest_change`, or `unsafe`.

- `bob.yaml` is reserved with `requires_manifest_change`: it is the
  human-owned contract surface.
- `bob.lock`, `.bob.apply.lock`, and `.git` are reserved for Bob or repository
  machinery.
- A desired absent artifact is `managed_missing`; manually creating different
  bytes will conflict.
- A lock-owned path retired by the current recipe is `retired_owned`.
- A matching human extension pattern is `extension_point` only when the path
  is not desired, locked, or reserved.
- Absolute paths, traversal, empty paths, NUL, and invalid UTF-8 are rejected.

Consumers branch on these codes and on `plan_action.code`, never on prose.
Results are capped at 8 KiB. Deterministic omission removes only supplemental
artifact detail, notices, and non-blocking continuation actions; the
`truncation` object reports every omission.
