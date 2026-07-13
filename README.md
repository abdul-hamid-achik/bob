# Bob

**A deterministic repository factory for agent-native developer tools.**

[![CI](https://github.com/abdul-hamid-achik/bob/actions/workflows/ci.yml/badge.svg)](https://github.com/abdul-hamid-achik/bob/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Bob turns a small `bob.yaml` product contract into a reviewable repository plan,
applies only the files it can prove it owns, and detects drift in CI. It is local,
model-free, and useful from a terminal, MCPHub, or an existing coding agent.

```text
bob.yaml + embedded recipe + bob.lock + working tree
                         |
                         v
               create / update / conflict
                         |
                    bob apply
                         |
                         v
               public-ready repository
```

> **Status: early alpha.** The current contract supports one recipe,
> `go-agent-tool`, plus a compact read-only MCP surface. Review every plan and
> resulting diff before publishing.

## Why Bob exists

Building a small public tool repeatedly requires the same careful substrate:
command structure, machine-readable output, tests, contributor docs, CI, release
packaging, behavior specs, and safe integration seams. Copying an old repository
also copies its accidents. General app generators own too much business logic.

Bob owns the construction lifecycle instead:

- `bob.yaml` declares intent;
- an embedded, versioned recipe renders desired files;
- `bob plan` compares desired state with the repository and content-hash lock;
- `bob apply` changes only absent, identical, or previously managed files;
- `bob check` fails when generated infrastructure drifts;
- `bob doctor` reports required and optional tool availability honestly.

Bob does not run an LLM, infer application behavior, manage secrets, or declare a
feature verified. Agent runtimes may drive Bob; evidence tools verify the result.

## Quick start

Prerequisites: macOS or Linux and Go 1.26 or newer. Task, GoReleaser, Codemap, Vecgrep, and Glyphrun
are optional and reported by `bob doctor` when selected.

```bash
git clone https://github.com/abdul-hamid-achik/bob
cd bob
go install ./cmd/bob

# Preview a new project. Nothing is written yet.
bob new acme-tool \
  --module github.com/acme/acme-tool \
  --description "Agent-ready Acme CLI"

# Create the manifest and initial repository explicitly.
bob new acme-tool \
  --module github.com/acme/acme-tool \
  --description "Agent-ready Acme CLI" \
  --write

cd acme-tool
bob plan
bob check
```

To initialize Bob in an empty or existing directory, write only the human-owned
manifest first:

```bash
bob init . --name acme-tool --module github.com/acme/acme-tool --write
bob plan
bob plan --content  # bounded desired-content previews
bob apply
```

`plan` and `check` never write. `apply` refuses the entire operation if even one
target conflicts.

Inspect Bob-managed state and offline binary availability without running
specialist tools:

```bash
bob inspect .
bob inspect . --json
```

Add `--probe-integrations` only when you explicitly want Bob to call the public
Codemap and Vecgrep status commands. Those commands may open their tool-owned
stores, and Vecgrep may contact its configured embedding provider. Bob never
searches, indexes, repairs, or declares verification.

## Product contract and ownership

`bob.yaml` is a strict human-owned contract for product identity, surfaces,
local intelligence seams, and distribution. See the complete
[manifest reference](docs/reference/manifest.md) and the tested
[minimal and integrated examples](examples/README.md).

`bob.lock` records the recipe version and SHA-256 digest of every Bob-owned
whole file. Plans classify paths as `create`, `adopt`, `unchanged`, `update`, or
`conflict`; one conflict blocks the complete apply. Bob never overwrites an
unmanaged differing file or a managed file changed by a person. Read the full
[ownership and safety contract](docs/ownership-and-safety.md) and
[CLI reference](docs/reference/cli.md) before initializing Bob in an existing
repository.

Use `--json` for versioned machine output. JSON goes to stdout; diagnostics and
errors go to stderr.

## Intelligence-stack integration

Bob has one seat in a larger local-first toolchain:

```text
local-agent   conversation, permissions, models, scheduling, recovery
MCPHub        tool discovery and harness synchronization
Cortex        evidence cases, change boundaries, canonical verification
Bob           deterministic repository construction and drift
Codemap       structural code understanding
Vecgrep       semantic code discovery
Glyphrun      terminal behavior proof
Cairntrace    browser behavior proof
TinyVault     secret-safe execution
file.cheap    durable evidence artifacts
Monitor       bounded runtime observations
```

Bob exposes a compact stdio MCP projection with exactly two read-only tools:

- `bob_inspect` returns Bob drift plus offline Codemap and Vecgrep availability;
- `bob_plan` returns a compact, structured repository plan without content
  previews.

Mutation deliberately remains on the normal approved command path:
`bob apply <workspace>`. This avoids hiding filesystem effects behind a generic
MCP proxy before Bob has digest-gated apply receipts. Cortex remains the owner of
semantic investigation and verification; Bob does not duplicate its
Vecgrep-to-Codemap routing.

### MCPHub and local-agent

Install a checkout-built binary and register it with MCPHub:

```bash
task install
BOB_BIN="$(go env GOBIN)"
[ -n "$BOB_BIN" ] || BOB_BIN="$(go env GOPATH)/bin"
mcphub add bob "$BOB_BIN/bob" mcp serve \
  --description "Deterministic agent-ready repository builder" \
  --tag builder
mcphub pin bob__bob_inspect bob__bob_plan
mcphub doctor --server bob --probe
```

For allowlists, gateway names, approval behavior, and the explicit integration
probe boundary, follow the [MCPHub & local-agent guide](docs/guides/mcphub-local-agent.md).

## Documentation

- [Documentation home](docs/index.md)
- [Getting started](docs/getting-started.md)
- [Ownership and safety](docs/ownership-and-safety.md)
- [Manifest reference](docs/reference/manifest.md)
- [CLI reference](docs/reference/cli.md)
- [Product direction](docs/product-direction.md)
- [Architecture](docs/architecture.md)
- [ADR-0001: choose a repository factory](docs/adr/0001-repository-factory.md)
- [ADR-0002: compact read-only MCP surface](docs/adr/0002-read-only-mcp.md)
- [Specification](SPEC.md)
- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)

## Development

```bash
task verify       # non-mutating code, security, and build checks
task specs        # terminal behavior; requires glyph
task docs-build   # VitePress production build
task ship         # complete pre-release gate
```

Without Task:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/bob
npm --prefix docs ci
npm --prefix docs run build
```

## License

MIT © 2026 Abdul Hamid Achik
