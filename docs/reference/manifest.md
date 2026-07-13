# Manifest Reference

`bob.yaml` is a strict, human-owned product contract. Unknown fields and
unsupported values are errors.

## Complete example

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

## Top-level fields

| Field | Current value | Meaning |
|---|---|---|
| `schema_version` | `1` | Selects the strict manifest schema. |
| `recipe` | `go-agent-tool` | Selects the embedded repository recipe. |

## Product

| Field | Valid values | Effect |
|---|---|---|
| `name` | lowercase letter followed by lowercase letters, digits, or hyphens | Binary and command directory name. |
| `module` | non-empty Go module path | Written to `go.mod`; GitHub coordinates enable GitHub-specific repository files. |
| `description` | non-empty text | Used in public README and package metadata. |
| `visibility` | `public`, `private` | Public is required for Homebrew in schema 1. |
| `license` | `MIT` | Schema 1 supports MIT only. |

## Runtime and surfaces

The current recipe requires `runtime.language: go`, `runtime.kind: cli`,
`surfaces.cli: true`, and `surfaces.json: true`.

Generated project MCP and Studio surfaces remain unsupported in schema 1, so
`surfaces.mcp` and `surfaces.studio` must be `false`. This does not conflict with
Bob itself exposing read-only MCP tools.

## Integrations

| Field | Values | What selection means |
|---|---|---|
| `code_structure` | `none`, `codemap` | Generates a structural-intelligence seam and doctor check. |
| `semantic_search` | `none`, `vecgrep` | Generates a semantic-search seam and doctor check. |
| `terminal_verification` | `none`, `glyphrun` | Generates a terminal behavior spec and doctor check. |
| `browser_verification` | `none`, `cairntrace` | Declares a browser-verification seam and doctor check. |
| `secrets` | `none`, `tinyvault` | Declares secret-safe execution availability; no secret values are stored. |
| `artifacts` | `none`, `fcheap` | Declares portable artifact storage availability. |

An integration selection creates repository guidance and capability checks. It
does not mean Bob ran the tool, initialized an index, or verified behavior.

## Distribution

| Field | Values | Effect |
|---|---|---|
| `github_actions` | boolean | Generates GitHub CI; a release workflow is generated only when this and `goreleaser` are both true. |
| `goreleaser` | boolean | Generates GoReleaser configuration. |
| `homebrew` | boolean | Generates a Homebrew cask; requires GoReleaser, public visibility, and a GitHub module. |
| `docs` | `none`, `markdown` | Generates a Markdown documentation entry page. |

Changing a field changes only files declared by the recipe. `bob plan` shows
the exact ownership decision before `bob apply` writes anything.
