---
description: Install Bob and go from an empty directory to a converged, agent-ready Go repository in about five minutes.
---

# Getting Started

Bob does not brainstorm. Give it a name, a module path, and a description, and
it hands back a plan. Approve the plan, and it builds. Five minutes, no
surprises.

## Prerequisites

- macOS or Linux
- Go 1.26.5 or newer
- Git
- Task is optional; direct Go commands work too

Install the release through the Homebrew tap:

```bash
brew tap abdul-hamid-achik/tap
brew install --cask bob
bob version
```

Alternatively, install with Go:

```bash
go install github.com/abdul-hamid-achik/bob/cmd/bob@latest
```

To build the current branch instead:

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

This is a preview. Bob prints the proposed `bob.yaml` and the number of files
it would create, and it touches nothing on disk. No target directory, no
surprise scaffolding waiting for you tomorrow.

## Create it explicitly

Nothing gets built until you say `--write`. Repeat the command:

```bash
bob new acme-tool \
  --module github.com/acme/acme-tool \
  --description "Agent-ready Acme CLI" \
  --write
```

Bob writes the manifest, renders the recipe, applies one conflict-free plan,
and publishes `bob.lock` last. The lock is Bob's receipt, not yours to edit.

## Confirm convergence

```bash
cd acme-tool
bob plan
bob check
go test ./...
```

A newly created project reports only `unchanged` actions, with no lock change.
`bob check` exits `0`. Run it again if you don't believe it. Run it a third
time out of spite. It stays `0`. That is the feature.

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
manifest selects them. A selection adds guidance and capability checks. It does
not mean Bob ran the tool, indexed anything, or vouches for it. Bob signs off on
files, not on vendors.

## If a coding agent is driving

Point it at [Bob for coding agents](./agents.md) and have it run `bob learn --json`
first. That single, read-only command briefs the agent on the whole product
contract before it plans anything.

## Next steps

- Read [Ownership & Safety](./ownership-and-safety.md) before changing managed files.
- Review [Configuration & local telemetry](./configuration.md) before opting into local stats.
- Open [Bob Studio](./studio.md) for a read-only interactive workspace view.
- Read the [Manifest Reference](./reference/manifest.md) before changing capabilities.
- Use [MCPHub & local-agent](./guides/mcphub-local-agent.md) to expose Bob to an agent.
- Onboard a coding agent with [Bob for coding agents](./agents.md).
