# Getting Started

Build and use Bob from source in about five minutes.

## Prerequisites

- macOS or Linux
- Go 1.26 or newer
- Git
- Task is optional; direct Go commands work too

Bob has no tagged release yet, so use a checkout-built binary:

```bash
git clone https://github.com/abdul-hamid-achik/bob
cd bob
go install ./cmd/bob
bob version
```

## Preview a repository

Choose a project name, public Go module, and one-line description:

```bash
bob new acme-tool \
  --module github.com/acme/acme-tool \
  --description "Agent-ready Acme CLI"
```

The preview prints the proposed `bob.yaml` and the number of files Bob would
create. It does not create the target directory.

## Create it explicitly

Repeat the command with `--write`:

```bash
bob new acme-tool \
  --module github.com/acme/acme-tool \
  --description "Agent-ready Acme CLI" \
  --write
```

Bob writes the manifest, renders the recipe, applies one conflict-free plan,
and publishes `bob.lock` last.

## Confirm convergence

```bash
cd acme-tool
bob plan
bob check
go test ./...
```

A newly created project should report only `unchanged` actions, with no lock
change. `bob check` then exits successfully.

## What Bob created

The default manifest creates:

- a Go/Cobra CLI with human and JSON output;
- tests and explicit dependency injection;
- `AGENTS.md` plus a thin `CLAUDE.md` pointer;
- contribution, security, conduct, changelog, and license files;
- GitHub issue and pull-request templates for GitHub modules;
- CI, vulnerability scanning, and tag-driven GoReleaser configuration;
- Codemap and Vecgrep integration guidance plus a Glyphrun terminal contract.

The same recipe can add Cairntrace, TinyVault, and file.cheap seams when the
manifest selects them. A selection adds guidance and capability checks; it does
not mean Bob ran or verified the external tool.

## Next steps

- Read [Ownership & Safety](./ownership-and-safety.md) before changing managed files.
- Read the [Manifest Reference](./reference/manifest.md) before changing capabilities.
- Use [MCPHub & local-agent](./guides/mcphub-local-agent.md) to expose Bob to an agent.
