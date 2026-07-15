---
description: Normative schema for bob context, capability state facets, digests, profiles, notices, and structured actions.
---

# Workspace Context

`bob context [workspace]` compiles repository intent into a bounded operating
contract. It is read-only and offline: it does not run Codemap, Vecgrep,
Glyphrun, or another subprocess, and it does not read generated file bodies
beyond the planner's existing bounded observations.

`bob learn --json` describes Bob. `bob context --json` describes the current
repository.

## Envelope and schema

The standard CLI envelope has `command: "context"`. Its `data` object has
schema version 1:

```json
{
  "schema_version": 1,
  "profile": "compact",
  "workspace": "/canonical/workspace",
  "contract_digest": "sha256:...",
  "context_digest": "sha256:...",
  "recipe": {"id": "go-agent-tool", "version": 4},
  "product": {"name": "acme", "module": "github.com/acme/acme", "runtime": "go", "kind": "cli"},
  "repository": {
    "state": "clean",
    "clean": true,
    "lock_changed": false,
    "conflict_count": 0,
    "managed_files": 24,
    "plan_digest_version": 1,
    "plan_digest": "sha256:..."
  },
  "capabilities": [],
  "entry_points": [],
  "extension_points": [],
  "invariants": [],
  "playbooks": [],
  "notices": [],
  "actions": [],
  "truncation": {"profile": "compact", "byte_limit": 6144, "truncated": false, "omitted": {}}
}
```

The playbook array contains bounded summaries with stable IDs, applicability,
availability, blockers, required input names, scope class, and risk. The
catalog is selected by recipe metadata and current manifest state; Bob never
matches natural-language tasks. Full procedures are available through
`bob playbook show|plan`.

## Repository states

`repository.state` is one of:

| State | Meaning |
|---|---|
| `clean` | Every desired artifact is unchanged and the lock is current. |
| `drifted` | A conflict-free plan would create, adopt, or update state. |
| `conflicted` | At least one ownership conflict blocks apply. |

`repository.plan_digest` is the `sha256:`-labelled version of the exact plan
identity shared by CLI and MCP plan/check. It is not an approval or a
verification receipt.

## Capability facets

Every capability separates four facts:

| Facet | Values |
|---|---|
| `selection` | `required`, `enabled`, `disabled`, `not_applicable` |
| `materialization` | `in_sync`, `drifted`, `conflicted`, `missing`, `not_applicable`, `unknown` |
| `availability` | `available`, `unavailable`, `not_checked`, `not_applicable` |
| `verification` | `not_assessed` |

Materialization is derived only from the complete Bob plan for artifacts named
by versioned recipe metadata. Availability is an offline executable lookup; Bob
does not run the executable. A selected or materialized integration is never
reported as verified.

The `go-agent-tool@4` catalog exposes these stable capability IDs:

```text
surface.cli
surface.json
distribution.github_actions
distribution.goreleaser
distribution.homebrew
docs.markdown
integration.codemap
integration.vecgrep
integration.glyphrun
integration.cairntrace
integration.tinyvault
integration.fcheap
repository.public_hygiene
repository.whole_file_ownership
```

`files@1` exposes only `repository.declared_file_tree` and
`repository.whole_file_ownership`; it does not borrow application-specific
capabilities from `go-agent-tool`.

`go-agent-tool@4` advertises `cli.command_files` as a human-owned extension
point. New command implementation and test files match its declared patterns;
`internal/cli/root.go`, `internal/cli/root_test.go`, `internal/cli/registry.go`,
and `internal/cli/registry_test.go` remain Bob-owned forbidden paths. The
extension declaration describes the desired versioned contract, not proof that
its registry is already materialized. Consumers must respect the related
playbook's `available` and `blocked_by` fields before creating command files.
The `files@1` metadata uses `files:<canonical-path>` artifact IDs and the generic
`declared_file` role; it does not infer application semantics from names or
content.

## Digests

- `contract_digest` covers the normalized manifest, recipe identity, and the
  behavioral portion of resolved recipe metadata.
- `plan_digest` covers the exact current plan using plan digest version 1.
- `context_digest` covers the complete untruncated semantic context, including
  projected availability.

Absolute workspace location, timestamps, output profile, truncation metadata,
and explanatory prose are excluded. Consequently, changing only the profile or
moving an otherwise identical converged repository does not change contract or
context identity.

## Profiles and bounds

| Profile | Limit | Projection |
|---|---:|---|
| `compact` | 6,144 bytes | All capability IDs and state facets, short invariants, relevant entry/extension points, notices, and actions. |
| `standard` | 24 KiB | Complete capability evidence and extension constraints; default for people. |
| `full` | 64 KiB | Full resolved artifact metadata without raw file content. |

Byte-limit truncation is deterministic and records omitted counts by field.
Identity, state, conflict counts, digests, capabilities, and actions are never
removed to satisfy a limit. If those non-omittable fields alone cannot fit,
Bob returns a bounded failure instead of emitting an oversized or partially
clipped contract.

## Structured actions and notices

Actions are argv-shaped rather than shell strings. `effect` is one of
`read_only`, `subprocess_probe`, `repository_mutation`, or
`user_configuration_mutation`. Context currently emits only `read_only` plan
review actions. `requires_explicit_authority` and `blocked_by` remain explicit
so an agent does not infer authority from ordering.

Notices carry stable `id`, `code`, and `severity` fields plus explanatory text.
Consumers branch on codes, not messages.
