# AGENTS.md

This is the source of truth for people and coding agents working on Bob.
`CLAUDE.md` defers here. Public product behavior is defined in `SPEC.md`; design
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
internal/version   build metadata injected by ldflags
```

Keep command handlers thin. Filesystem ownership and mutation rules belong in
`internal/engine`; recipe rendering must not inspect or mutate the workspace.

## Invariants

- `plan`, `check`, `doctor`, and `explain` are read-only.
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
task check        # fmt + lint + test
task specs        # Glyphrun behavior specs (local)
```

Run `gofmt -s` on Go changes. Return lowercase wrapped errors from library code;
only `cmd/bob` may exit the process. Tests must use temporary directories and
must never touch a real user's repositories or tool configuration.

## Public repository hygiene

Working notes and handoffs do not belong in this repository. Keep root Markdown
limited to public orientation, contribution, security, changelog, specification,
and agent instructions. Never commit binaries, generated release archives,
credentials, or private filesystem paths.
