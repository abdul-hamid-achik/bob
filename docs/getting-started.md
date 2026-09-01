---
description: Install Bob, scaffold a Go/Cobra CLI, or bring seed-once hygiene to an existing TypeScript, Python, Rust, or Go repository.
---

# Getting Started

Bob does not brainstorm. Give it a name and a recipe, and it hands back a
plan. Approve the plan, and it writes only what it can prove it owns.

The walkthrough below builds a new Go/Cobra CLI — the deepest recipe. If the
application already exists, use
[Existing Repository](./guides/existing-repository.md) instead.

## Prerequisites

- macOS or Linux
- Git
- Go 1.26.6 or newer if you install from source or scaffold `go-agent-tool`.
  The Homebrew cask does not need a local Go toolchain to run `bob init` on
  an existing TypeScript, Python, or Rust repository.
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

## Existing repository

For TypeScript, Python, Rust, Go, HTML, and the other detected stacks,
`bob init` writes a hygiene recipe and never owns application source:

```bash
bob init --write
bob apply
bob check
```

The full detection table, the Go `go-hygiene` versus `go-agent-tool`
split, and how to keep a conflicting file are in
[Existing Repository](./guides/existing-repository.md). For a declared
file tree, see [Build any repository](./guides/any-repository.md).

## What Bob created

The default `go-agent-tool` walkthrough above creates:

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

Point it at [Bob for coding agents](./agents.md) and have it run
`bob learn --json`, then `bob context --json`. The first read-only call briefs
the agent on Bob; the second compiles the active workspace contract before it
plans anything.

## Next steps

- Bring Bob into a repo that already exists with [Existing Repository](./guides/existing-repository.md).
- Read [Ownership & Safety](./ownership-and-safety.md) before changing managed files.
- Review [Configuration & local telemetry](./configuration.md) before opting into local stats.
- Open [Bob Studio](./studio.md) for a read-only interactive workspace view.
- Read the [Manifest Reference](./reference/manifest.md) before changing capabilities.
- Use [MCPHub & local-agent](./guides/mcphub-local-agent.md) to expose Bob to an agent.
- Onboard a coding agent with [Bob for coding agents](./agents.md).
