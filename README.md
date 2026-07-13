# Bob

**A deterministic repository factory for agent-native developer tools.**

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

To adopt an empty or existing directory, initialize only the human-owned
manifest first:

```bash
bob init . --name acme-tool --module github.com/acme/acme-tool --write
bob plan
bob plan --content  # bounded desired-content previews
bob apply
```

`plan` and `check` never write. `apply` refuses the entire operation if even one
target conflicts.

Inspect the complete workspace readiness inventory without running specialist
tools:

```bash
bob inspect .
bob inspect . --json
```

Add `--probe-integrations` only when you explicitly want Bob to call the public
Codemap and Vecgrep status commands. Those commands may open their tool-owned
stores, and Vecgrep may contact its configured embedding provider. Bob never
searches, indexes, repairs, or declares verification.

## Manifest

```yaml
schema_version: 1
recipe: go-agent-tool

product:
  name: acme-tool
  module: github.com/acme/acme-tool
  description: Agent-ready Acme CLI
  visibility: public
  license: MIT

runtime:
  language: go
  kind: cli

surfaces:
  cli: true
  json: true
  mcp: false
  studio: false

integrations:
  code_structure: codemap
  semantic_search: vecgrep
  terminal_verification: glyphrun
  browser_verification: none
  secrets: none
  artifacts: none

distribution:
  github_actions: true
  goreleaser: true
  homebrew: false
  docs: markdown
```

The schema is strict: unknown fields and unsupported capability combinations are
errors. MCP and Studio output recipes are intentionally deferred until the core
ownership contract has real use behind it.

## Ownership and safety

`bob.lock` records the SHA-256 digest of every Bob-owned file. Planning classifies
each desired file as:

| State | Meaning |
|---|---|
| `create` | The path does not exist. |
| `adopt` | An unmanaged regular file already matches exactly. |
| `unchanged` | The managed file matches the recipe. |
| `update` | The file still matches the old lock and the recipe changed. |
| `conflict` | Ownership is absent or stale, or the destination is unsafe. |

Bob never overwrites an unmanaged differing file or a managed file changed by a
person. Absolute paths, parent traversal, `.git`, manifests, locks, symlinks, and
special files are outside recipe ownership. File publication uses atomic sibling
renames and writes the lock last.

## Commands

| Command | Purpose |
|---|---|
| `bob new <name>` | Preview or explicitly create a new project. |
| `bob init [path]` | Preview or write `bob.yaml` in a repository. |
| `bob plan [path]` | Compute desired changes without writing; add `--content` for bounded previews. |
| `bob apply [path]` | Apply one fully conflict-free plan. |
| `bob check [path]` | Exit non-zero when managed state or the lock would change. |
| `bob doctor [path]` | Probe required and selected optional tools. |
| `bob inspect [path]` | Summarize Bob drift and offline integration availability; probing is explicit. |
| `bob mcp serve` | Expose `bob_inspect` and `bob_plan` over stdio. |
| `bob explain` | Describe Bob's contract and integration boundary. |
| `bob recipe list\|show` | Inspect the embedded recipe catalog. |
| `bob version` | Print build metadata. |

Use `--json` for the versioned, machine-readable envelope. JSON goes to stdout;
diagnostics and errors go to stderr.

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

Install a stable binary and register it with MCPHub:

```bash
task install
BOB_BIN="$(go env GOBIN)"
[ -n "$BOB_BIN" ] || BOB_BIN="$(go env GOPATH)/bin"
mcphub add bob "$BOB_BIN/bob" mcp serve \
  --description "Deterministic agent-ready repository builder" \
  --tag builder
mcphub pin bob__bob_inspect bob__bob_plan
```

If the local-agent route uses an explicit server allowlist, add `bob` to
`agents.local-agent.servers` in `~/.config/mcphub/mcphub.yaml`, then reconcile
and probe:

```bash
mcphub sync local-agent
# Run this only if the dry run above reports a diff:
mcphub sync local-agent --write
mcphub doctor --server bob --probe
```

Pinning keeps the two Bob tools directly advertised in MCPHub's lazy mode; it
does not grant permission. With the current local-agent integration they appear
as `mcphub__bob__bob_inspect` and `mcphub__bob__bob_plan`, remain conservatively
classified as unknown-effect MCP calls, and prompt under the default Ask policy.
Avoid a persistent allow for the generic `mcphub__mcphub_call_tool` proxy because
that name can dispatch many downstream tools. Restart an already running
local-agent process after changing gateway configuration.

MCPHub servers and pins are global unless an agent has an explicit allowlist.
Add per-agent `servers` or `tools` scopes if Bob should not be advertised to
other configured gateways.

## Documentation

- [Product direction](docs/product-direction.md)
- [Architecture](docs/architecture.md)
- [ADR-0001: choose a repository factory](docs/adr/0001-repository-factory.md)
- [ADR-0002: compact read-only MCP surface](docs/adr/0002-read-only-mcp.md)
- [Specification](SPEC.md)
- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)

## Development

```bash
task check
task race
task build
task specs   # requires glyph
```

Without Task:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/bob
```

## License

MIT © 2026 Abdul Hamid Achik
