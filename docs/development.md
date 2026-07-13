---
description: How to build, test, lint, and ship Bob and its VitePress docs site from a fresh checkout.
---

# Development

Bob is a Go 1.26.5 module with a small VitePress documentation site and optional
Glyphrun terminal contract. Building the site requires Node.js 22 or newer.
Nothing here mutates a real repository; that discipline applies to Bob's own
build too.

## Setup

```bash
git clone https://github.com/abdul-hamid-achik/bob
cd bob
go mod download
npm --prefix docs ci
```

Read the root
[`AGENTS.md`](https://github.com/abdul-hamid-achik/bob/blob/main/AGENTS.md)
before changing package boundaries or safety behavior.

## Verification

```bash
task verify          # non-mutating canonical Go/security/build gate
task specs           # terminal behavior contract
task docs-build      # documentation links and production build
task ship            # complete local pre-release gate
```

Use `task fmt` deliberately when you want to rewrite Go formatting. Verification
never formats or tidies files in place — it audits, it doesn't clean up after
you.

## Documentation

```bash
task docs            # start the local site at 127.0.0.1
task docs-build      # production build
task docs-preview    # preview the production build
```

The development and preview servers bind to loopback intentionally. Do not
expose them with `--host 0.0.0.0`; the publishable documentation artifact is
the static output from `task docs-build`.

User-facing guides and reference pages belong in `docs/`. Normative product
behavior lives on this site's reference pages — [Manifest](./reference/manifest.md)
and [CLI](./reference/cli.md) — rather than a root `SPEC.md`; repository-visible
design decisions belong in `docs/adr/`. Temporary notes, handoffs, and private
filesystem details do not belong in the public repository.

## Live integration test

The normal test suite never reads a user's MCPHub configuration. One opt-in
smoke test exercises the installed Bob binary through MCPHub's local-agent
scope while isolating telemetry:

```bash
BOB_TEST_MCPHUB=1 go test ./internal/mcp -run TestMCPHubLocalAgentScopeRoute -v
```

Run it only when Bob is installed and MCPHub has the intended local-agent route.
