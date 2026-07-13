# Contributing

Thanks for helping improve Bob.

## Development setup

You need Go 1.26 or newer. [Task](https://taskfile.dev/) is recommended but not
required.

```bash
git clone https://github.com/abdul-hamid-achik/bob
cd bob
go test ./...
go build ./cmd/bob
```

With Task installed:

```bash
task check
task race
task build
```

## Pull requests

- Keep changes focused and explain the user-visible outcome.
- Add tests for manifest, planning, ownership, and filesystem safety changes.
- Preserve plan-before-mutation behavior and idempotent apply.
- Update public docs when a command or wire format changes.
- Do not include private working notes, secrets, generated binaries, or local
  state.

Use conventional commit prefixes such as `feat:`, `fix:`, `docs:`, and
`chore:` when practical.
