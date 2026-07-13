# Architecture

Bob is a deterministic repository planner and whole-file reconciler. The
implemented architecture separates the manifest, recipe, observed files,
planning, mutation, and drift checks so generation remains inspectable.

## Implemented system

```text
                  human or agent
                        |
              CLI / JSON / read-only MCP
                        |
                        v
                  manifest loader
                        |
                 embedded recipe
                        |
                        v
       observed files + root bob.lock
                        |
                        v
                  engine.Plan
                        |
            review actions and conflicts
                        |
                        v
                  engine.Apply
                        |
                        v
            generated files + bob.lock
```

The CLI provides human-readable and versioned JSON output. A thin stdio MCP
projection exposes offline inspection and compact planning. Studio and MCP
mutation are not implemented.

## Implemented components

### Manifest

The human-owned `bob.yaml` declares project identity, the `go-agent-tool` recipe,
CLI/JSON surfaces, optional integration seams, and distribution choices. Schema
version 1 is strict: unknown fields and unsupported combinations fail
validation. MCP and Studio must be disabled.

### Embedded recipe

The `go-agent-tool` recipe renders the complete desired artifact set in memory.
It is deterministic and versioned. Version 0.1 has no third-party recipe or
plugin runtime.

Every artifact has a repository-relative path, complete content, and file mode.
Recipe output cannot own `.git`, `bob.yaml`, or `bob.lock`.

### Planning engine

The engine validates the workspace, renders desired artifacts, reads the
repository-root `bob.lock`, and observes each destination. It emits a sorted,
versioned plan projection with exactly one action per desired file:

| Action | Meaning |
|---|---|
| `create` | The destination does not exist. |
| `adopt` | An unmanaged regular file already matches exactly. |
| `unchanged` | A managed file already matches the desired content. |
| `update` | A managed file still matches its old lock hash and the recipe changed. |
| `conflict` | Ownership is absent or stale, or the destination is unsafe. |

The planner is read-only. A conflict blocks the complete apply.

### Whole-file ownership lock

`bob.lock` lives at the repository root. It records the lock schema, recipe ID
and version, and the SHA-256 digest of every Bob-owned file. It contains no
commands, environment, secrets, plans, or execution history.

Version 0.1 has one ownership mode: the complete file. It has no managed-block
merge behavior.

### Applier

`apply` calculates a fresh plan, refuses conflicts before mutation, stages
changed files, and rechecks file and lock preconditions. Creates use atomic
no-replace publication; updates use atomic replacement after a final content and
mode check. `bob.lock` is written last.

Multi-file apply is not globally transactional. A process crash can leave some
files published before the lock is written. A later plan observes the exact
state and may classify already-published matching files as `adopt`.

Bob does not delete files in version 0.1.

### Drift check

`check` runs the same planning path without mutation and succeeds only when every
action is `unchanged`. It is suitable for CI drift detection.

### Doctor

`doctor` probes Go and Git plus selected optional tools using bounded direct
version commands. Missing required tools make the result not ready. Missing or
failed optional probes produce an explicit degraded result.

### Workspace inspection

`inspect` combines Bob's existing plan engine with integration readiness. Its
default path is offline: it reads the workspace and discovers selected binaries
without launching them. `--probe-integrations` explicitly calls Codemap and
Vecgrep status commands through direct argv, ten-second deadlines, and bounded
stdout/stderr capture.

Probe results are normalized into Bob-owned schema values. Upstream raw output
is neither persisted nor treated as investigation or verification evidence.
Bob validates that each reported project root matches the canonical requested
workspace and never initializes, indexes, resets, migrates, searches, or repairs
a specialist tool.

### MCP projection

`bob mcp serve` uses the official Go MCP SDK and newline-delimited stdio. It
exposes exactly `bob_inspect` and `bob_plan`; both publish read-only,
non-destructive, idempotent, closed-world annotations and inferred input/output
schemas.

The MCP inspector never enables specialist probes. The MCP planner returns a
compact action projection without desired-content previews or mutation. Agents
must use the separately approved CLI path for `bob apply`, then re-plan.

### Output

All commands support a global `--json` flag. Structured stdout uses a versioned
envelope containing command data, warnings, and next actions. The current CLI
does not persist plans or receipts.

## Implemented package boundaries

```text
cmd/bob/             process entry point
internal/cli/        Cobra commands and human/JSON rendering
internal/manifest/   strict schema, load, validation, and write
internal/recipe/     embedded recipe and artifact rendering
internal/engine/     plan, whole-file ownership, safe apply, and lock
internal/doctor/     bounded dependency probes
internal/inspect/    offline inventory and explicit specialist status probes
internal/mcp/        typed read-only stdio projection
internal/version/    build metadata
internal/workspace/  shared canonical workspace resolution
```

The CLI coordinates these packages. There is no Studio package, persistent
store, verifier, MCP mutation handler, or integration orchestrator.

## Ecosystem ownership map

Bob declares optional seams without absorbing specialist behavior.

| Concern | Owner | Bob 0.1 behavior |
|---|---|---|
| Repository desired state | Bob | Render, plan, apply, and check whole files |
| Agent reasoning and goals | Agent runtime | Invoke Bob through CLI/JSON |
| Evidence-guided investigation | Reasoning kernel | Outside Bob; may inspect Bob output |
| MCP aggregation and harness sync | MCP gateway | Bob supplies two read-only tools; gateway owns routing and sync |
| Structural code impact | Code graph tool | Optional generated seam and doctor probe |
| Semantic search | Search tool | Optional generated seam and doctor probe |
| Secrets | Secret broker | Optional generated seam; Bob stores no secret values |
| Terminal behavior | Terminal spec runner | Optional generated spec and doctor probe |
| Browser behavior | Browser spec runner | Manifest selection only; no Bob runner |
| Evidence preservation | Artifact store | Manifest selection only; no Bob receipt export |
| Resource observation | System monitor | Outside Bob 0.1 |

Optional integrations are declared honestly. Selecting one does not imply that
Bob ran it or verified application behavior.

## Repository state

Bob 0.1 persists only repository-visible state:

- `bob.yaml`, owned by the user;
- `bob.lock`, written at the repository root by Bob;
- whole files generated by the selected recipe.

Plans, command executions, and verification receipts are not stored.

## Implemented safety invariants

1. `plan`, `check`, and `explain` do not mutate repository files.
2. `new` and `init` preview unless `--write` is explicit.
3. The same validated manifest, recipe version, files, and lock produce the same
   plan.
4. Any conflict blocks apply before file publication begins.
5. Bob updates a managed file only while it still matches its recorded lock
   digest.
6. Bob adopts only an unmanaged regular file with exactly matching content.
7. Absolute paths, parent traversal, reserved files, pre-existing symlinks, and
   special files are rejected from recipe ownership. Concurrent same-user parent
   replacement remains a documented v0.1 OS-containment boundary.
8. Apply rechecks observed files and `bob.lock` after staging and before
   publication.
9. `bob.lock` is published last.
10. Bob never executes generated project commands during plan or apply.

## Future architecture

Future work may add:

- Studio projections over the same deterministic engine;
- digest-gated, receipt-bearing MCP apply after its race and retry semantics are specified;
- standalone `adopt` and `verify` workflows;
- bounded, redacted persistent verification receipts;
- additional recipe versions and languages;
- explicit deletion and migration plans;
- new ownership strategies, including managed blocks, only after their merge
  semantics are separately specified and tested.

Future features must remain clearly distinguishable from implemented behavior.
They must preserve plan-before-mutation, explicit authority, workspace safety,
and honest degraded states.
