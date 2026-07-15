# Architecture

Bob is a deterministic repository planner and whole-file reconciler. The
implemented architecture separates the manifest, recipe, observed files,
planning, mutation, and drift checks so generation remains inspectable.

## Implemented system

```text
                  human or agent
                        |
       CLI / JSON / context / Studio / read-only MCP
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

The CLI provides human-readable and versioned JSON output. Studio presents the
same coherent offline inspection and plan snapshot interactively. A typed stdio
MCP projection exposes nine repository-read-only operations. MCP mutation is not
implemented.

## Implemented components

### Manifest

The human-owned `bob.yaml` declares project identity, the `go-agent-tool` recipe,
CLI/JSON surfaces, optional integration seams, and distribution choices. Schema
version 1 is strict: unknown fields and unsupported combinations fail
validation. MCP and Studio must be disabled.

### Embedded recipe

The `go-agent-tool` recipe renders the complete desired artifact set in memory.
It is deterministic and versioned. The current `go-agent-tool@4` recipe has a
static human-owned command extension contract and no third-party recipe or
plugin runtime.

Every artifact has a repository-relative path, complete content, and file mode.
Recipe output cannot own `.git`, `bob.yaml`, or `bob.lock`.

Recipe metadata is a separate versioned, deterministic projection resolved from
the same validated manifest. It assigns stable artifact and capability IDs,
declares invariants and honest human extension points, and cross-validates every
artifact reference against rendered output. Metadata resolution never observes
the workspace and does not change rendered bytes.

### Workspace context

The `internal/context` service resolves one canonical workspace, loads the
manifest, renders artifacts, computes one engine plan, resolves recipe metadata,
and projects capability states, entry points, extension points, invariants,
typed notices, and structured actions. Compact, standard, and full profiles are
deterministic views of one full semantic result and share contract, plan, and
context digests.

Context performs offline executable lookup only. It never uses the integration
runner, launches a subprocess, contacts a provider, or stores a snapshot.

### Path classification and playbooks

The engine classifies an exact canonical relative path using the same desired
artifacts, lock ownership, destination observation, symlink rules, and action
codes as planning. `internal/pathinfo` composes that result with exact recipe
artifact and extension-pattern metadata; extension patterns can never override
managed or reserved ownership.

`internal/playbook` lists, shows, and resolves the closed playbook definitions
owned by `internal/recipe`. It validates typed inputs, resolves only named path
and argv placeholders, and returns ordered steps with explicit effects and
authority requirements. The service executes no step and writes nothing.

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

Plan digest version 1 now lives in the engine. CLI plan/check and MCP
`bob_plan`/`bob_check` use the same implementation; filtering and previews do
not change the identity.

### Whole-file ownership lock

`bob.lock` lives at the repository root. It records the lock schema, recipe ID
and version, and the SHA-256 digest of every Bob-owned file. It contains no
commands, environment, secrets, plans, or execution history.

Bob has one ownership mode: the complete file. It has no managed-block
merge behavior.

An older positive lock version for the same recipe ID is accepted as an upgrade
input. Its content hashes remain the ownership proof while the current recipe
renders desired files and a new lock version. A future lock version is rejected
so an older Bob binary cannot reinterpret newer state.

### Applier

`apply` calculates a fresh plan, refuses conflicts before mutation, stages
changed files, and rechecks file and lock preconditions. Creates use atomic
no-replace publication; updates use atomic replacement after a final content and
mode check. `bob.lock` is written last.

With `--expect-plan-digest`, apply validates an exact qualified SHA-256 value,
loads and renders `bob.yaml` while holding the workspace apply lock, then
fresh-plans and compares identities before conflict preflight, staging, or
writes. Its exact manifest source is rechecked before staging and publication.
A mismatch is a distinct zero-write result. Success returns a bounded immediate
apply receipt with complete counts and deterministic path omissions; Bob does
not persist it or interpret it as behavioral verification.

Multi-file apply is not globally transactional. A process crash can leave some
files published before the lock is written. A later plan observes the exact
state and may classify already-published matching files as `adopt`.

Bob does not delete files.

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

### XDG settings and local telemetry

The paths package resolves Bob-specific and XDG configuration, data, state, and
cache locations without creating them. Settings are strict schema-v1 YAML; a
missing file produces defaults with telemetry disabled. Initialization is an
explicit, private, no-overwrite operation.

When enabled, telemetry stores one schema-v1 event per recorded operation under
the XDG state directory. The durable type contains only closed surface,
operation, outcome, reason, and recipe values, duration, aggregate action
counts, and a machine-local HMAC workspace pseudonym. It cannot represent
paths, argv, filenames, content, labels, or raw errors. Each event claims an
atomic daily slot; retention and daily caps are local. A best-effort recorder
keeps telemetry failure outside product-command outcomes.

Aggregation scans only retained private event files and returns totals grouped
by the closed operation vocabulary. Malformed files are counted as skipped; a
newer schema is refused rather than reinterpreted.

### Studio

`bob studio` is a Bubble Tea v2 projection with Overview, Plan, and Stats
views. Its source performs one coherent inspect/plan load per refresh with
specialist probing disabled. A telemetry adapter supplies only a 30-day
workspace aggregate. The model exposes no apply, shell, editor, search,
indexing, probe, or repair command.

The TUI responds to terminal size, supports a forced single-pane layout, and
keeps the last successful snapshot visible after a failed refresh. It rejects
non-interactive and dumb terminals so automation receives an ordinary error
instead of control sequences.

### MCP projection

`bob mcp serve` uses the official Go MCP SDK and newline-delimited stdio. It
exposes context, exact-path, closed-playbook, inspect, plan, check,
manifest-validation, recipe-description, and aggregate-stats tools. All publish
repository-read-only, non-destructive, idempotent, closed-world annotations and
inferred input/output schemas.

The MCP inspector never enables specialist probes. Planning is bounded by both
an action count and a byte budget, excludes unchanged actions by default, and
returns truncation metadata plus a digest of the complete plan. Check computes
the same digest. Manifest validation accepts exactly one authorized workspace
or a 64-KiB inline document. Context defaults to compact; path and playbook
inputs are closed and length-bounded; every guidance result has a deterministic
byte cap and explicit truncation. Stats returns aggregates, never events. MCP
publishes the exact validated contract in `structuredContent`. To keep compact
context below the gateway page threshold, `bob_context` uses a small
identity/state JSON text projection that points clients to `structuredContent`
instead of duplicating the complete contract; the other typed tools retain the
SDK's equivalent JSON text projection.

The server's canonical startup workspace is its exact allowlist by default.
Repeatable additional paths and explicit any-workspace mode expand that read
authority. Agents must use the separately approved CLI path for `bob apply`,
preferably guarded by the reviewed plan digest, then check or plan again.

### Output

Normal non-interactive commands support a global `--json` flag. Structured
stdout uses a versioned envelope containing command data, warnings, and next
actions. Studio is interactive-only; MCP reserves stdout for JSON-RPC. The CLI
does not persist plans or receipts.

## Implemented package boundaries

```text
cmd/bob/             process entry point
internal/cli/        Cobra commands and human/JSON rendering
internal/manifest/   strict schema, load, validation, and write
internal/recipe/     embedded recipe and artifact rendering
internal/engine/     plan, whole-file ownership, safe apply, and lock
internal/context/    bounded offline workspace-contract composition
internal/guidance/   shared typed notices, actions, and truncation contracts
internal/pathinfo/   exact ownership plus extension-metadata projection
internal/playbook/   typed non-executing playbook resolution
internal/doctor/     bounded dependency probes
internal/inspect/    offline inventory and explicit specialist status probes
internal/paths/      side-effect-free XDG layout resolution
internal/settings/   strict user settings and private initialization
internal/telemetry/  local-only event storage, pruning, and aggregation
internal/studio/     read-only Bubble Tea projection
internal/mcp/        typed repository-read-only stdio projection
internal/version/    build metadata
internal/workspace/  shared canonical workspace resolution
```

The CLI coordinates these packages. There is no verifier, MCP mutation handler,
remote telemetry transport, or integration orchestrator.

## Ecosystem ownership map

Bob declares optional seams without absorbing specialist behavior.

| Concern | Owner | Current Bob behavior |
|---|---|---|
| Repository desired state | Bob | Render, plan, apply, and check whole files |
| Agent reasoning and goals | Agent runtime | Invoke Bob through CLI/JSON |
| Evidence-guided investigation | Reasoning kernel | Outside Bob; may inspect Bob output |
| MCP aggregation and harness sync | MCP gateway | Bob supplies nine typed repository-read-only tools; gateway owns routing, authorization, and sync |
| Local Bob operations view | Bob Studio | Offline Overview/Plan plus aggregate local Stats; no action execution |
| Local product usage | Bob telemetry | Disabled by default; private XDG events and aggregate-only public projections |
| Structural code impact | Code graph tool | Optional generated seam and doctor probe |
| Semantic search | Search tool | Optional generated seam and doctor probe |
| Secrets | Secret broker | Optional generated seam; Bob stores no secret values |
| Terminal behavior | Terminal spec runner | Optional generated spec and doctor probe |
| Browser behavior | Browser spec runner | Manifest selection only; no Bob runner |
| Evidence preservation | Artifact store | Manifest selection only; no Bob receipt export |
| Resource observation | System monitor | Outside Bob |

Optional integrations are declared honestly. Selecting one does not imply that
Bob ran it or verified application behavior.

## Repository state

Bob's repository-visible state remains:

- `bob.yaml`, owned by the user;
- `bob.lock`, written at the repository root by Bob;
- whole files generated by the selected recipe.

Plans and verification receipts are not stored. Separately, if and only if
telemetry is enabled, Bob stores schema-bounded operational events under its XDG
state directory. The user settings file is machine-local rather than repository
intent.

## Implemented safety invariants

1. `context`, `path`, `playbook`, `plan`, `check`, plain `inspect`, `stats`, Studio, and all nine MCP tools do
   not mutate repository files. Optional telemetry writes are isolated to XDG
   state.
2. `new` and `init` preview unless `--write` is explicit.
3. The same validated manifest, recipe version, files, and lock produce the same
   plan.
4. Any conflict blocks apply before file publication begins.
5. Bob updates a managed file only while it still matches its recorded lock
   digest.
6. Bob adopts only an unmanaged regular file with exactly matching content.
7. Absolute paths, parent traversal, reserved files, pre-existing symlinks, and
   special files are rejected from recipe ownership. Concurrent same-user parent
   replacement remains a documented OS-containment boundary.
8. Apply rechecks observed files and `bob.lock` after staging and before
   publication.
9. `bob.lock` is published last.
10. Bob never executes generated project commands during plan or apply.

## Future architecture

Future work may add:

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
