# AGENTS.md

This is the source of truth for people and coding agents working on Bob.
`CLAUDE.md` defers here. Public product behavior is defined by the reference
pages under `docs/reference/` (published at <https://bobcli.dev>); design
decisions live in `docs/adr/`.

## Product boundary

Bob is a deterministic repository factory and lifecycle reconciler. It compiles
`bob.yaml` through a versioned recipe, compares desired artifacts with the
working tree and `bob.lock`, and applies only changes whose ownership is proven.

Bob is not an LLM runtime, planning agent, evidence authority, secret manager,
package manager, or generic task runner. Optional tools remain separate and are
reached through explicit public contracts.

## Architecture

```text
cmd/bob            thin process entrypoint
internal/cli       Cobra commands and human/JSON rendering
internal/manifest  schema, strict YAML loading, validation, atomic writes
internal/recipe    deterministic desired-artifact generation
internal/engine    ownership lock, plan, conflict detection, safe apply
internal/doctor    bounded optional-tool capability probes
internal/inspect   offline workspace inventory and explicit specialist probes
internal/paths     side-effect-free XDG path resolution
internal/settings  strict per-user settings load and private initialization
internal/telemetry privacy-bounded local event store and aggregates
internal/studio    read-only Bubble Tea repository and usage projection
internal/mcp       typed repository-read-only stdio projection
internal/version   build metadata injected by ldflags
internal/workspace shared canonical workspace resolution
```

Keep command handlers thin. Filesystem ownership and mutation rules belong in
`internal/engine`; recipe rendering must not inspect or mutate the workspace.

## Invariants

- `plan`, `check`, plain `inspect`, `stats`, `studio`, `explain`, and `learn`
  do not mutate repositories.
- `inspect --probe-integrations` is explicit subprocess authority. It never
  initializes, indexes, resets, searches, or repairs a specialist tool.
- The six MCP tools never mutate repositories or run specialist probes.
- MCP defaults to an exact startup workspace allowlist. Broader read authority
  requires `--allow-workspace` or explicit `--allow-any-workspace`.
- Telemetry is disabled by default, has no network transport, and never stores
  paths, arguments, filenames, manifest content, or raw errors.
- When telemetry is enabled, CLI and MCP operations may append private XDG
  state; `studio`, `stats`, and configuration commands never record events.
- Studio exposes no apply, shell, editor, indexing, probing, or repair action.
- MCP stdout is JSON-RPC-only. Diagnostics belong on stderr.
- `apply` preflights the complete plan and writes nothing when any conflict
  exists.
- Bob never overwrites an unmanaged differing file.
- A managed file may update only if its current hash matches the prior lock.
- Recipe paths cannot be absolute, escape the workspace, target `.git`, or own
  `bob.yaml`/`bob.lock`.
- Existing symlinks and special files are conflicts.
- Repeated apply converges to a no-op.
- JSON stdout is machine-clean; warnings and errors go to stderr.
- Wire formats are versioned and reject unsupported versions.

## Development commands

```bash
task build        # ./bin/bob with version metadata
task test         # go test ./...
task race         # go test -race ./...
task lint         # golangci-lint v2, with vet/gofmt fallback
task verify       # canonical non-mutating code/security/build gate
task specs        # Glyphrun behavior specs (local)
task docs-build   # VitePress production build and link validation
task ship         # verify + specs + docs + release snapshot
```

Run `gofmt -s` on Go changes. Return lowercase wrapped errors from library code;
only `cmd/bob` may exit the process. Tests must use temporary directories and
must never touch a real user's repositories or tool configuration. Opt-in live
integration tests must isolate telemetry and state explicitly.

## Documentation discipline

`docs/` is the published website. Every Markdown file in it (plus
`docs/.vitepress/` and `docs/public/`) ships verbatim to <https://bobcli.dev>.
Never place anything in `docs/` that is not intended for that site: no working
notes, scratch files, plans, handoffs, TODO lists, or generated reports. If a
file should not appear on bobcli.dev, it does not belong under `docs/`.

User-facing tutorials, how-to guides, and reference pages belong in `docs/`.
Normative product behavior belongs in the reference pages under
`docs/reference/`; architecture decisions belong in `docs/adr/`. `README.md` stays an orientation page rather than a
second complete manual. Do not commit VitePress build output or dependencies.
Coding agents should run `bob learn --json` (or read
<https://bobcli.dev/agents>) before driving Bob.

When a public contract changes, update the relevant README/docs page, the
reference pages when normative behavior changes, the changelog, and any
Glyphrun contract that proves the surface.

## Public repository hygiene

Working notes and handoffs do not belong in this repository. Keep root Markdown
limited to public orientation, contribution, security, changelog, specification,
and agent instructions. Never commit binaries, generated release archives,
credentials, or private filesystem paths.

GitHub-facing generated files are part of the recipe contract. Add or change
them through a new recipe version and preserve safe upgrades from older lock
versions; never change a published recipe version in place.
