# Contributing

Thanks for helping improve Bob.

Read [AGENTS.md](AGENTS.md) before changing package boundaries, ownership rules,
subprocess behavior, or public contracts.

## Development setup

You need Go 1.26.5 or newer. Node.js 22 or newer is required only for the
documentation site. [Task](https://taskfile.dev/) is recommended but not
required.

```bash
git clone https://github.com/abdul-hamid-achik/bob
cd bob
go test ./...
go build ./cmd/bob
npm --prefix docs ci
```

With Task installed:

```bash
task verify
task docs-build
```

Run `task specs` when CLI or MCP behavior changes. Run `task ship` before a
release-oriented change. Verification is non-mutating; use `task fmt`
deliberately when you want to rewrite Go formatting.

## Pull requests

- Keep changes focused and explain the user-visible outcome.
- Add tests for manifest, planning, ownership, and filesystem safety changes.
- Preserve plan-before-mutation behavior and idempotent apply.
- Update public docs when a command or wire format changes.
- Describe compatibility and recipe-version impact.
- Do not include private working notes, secrets, generated binaries, or local
  state.

Security-sensitive reports belong in [GitHub private vulnerability
reporting](https://github.com/abdul-hamid-achik/bob/security/advisories/new), not
in a public issue. Community participation follows the
[Code of Conduct](CODE_OF_CONDUCT.md).

By contributing, you agree that your contribution is licensed under the
repository's MIT License.

Use conventional commit prefixes such as `feat:`, `fix:`, `docs:`, and
`chore:` when practical.
