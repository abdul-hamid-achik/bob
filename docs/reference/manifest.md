---
description: Complete field-by-field reference for bob.yaml, Bob's strict human-owned product manifest.
---

# Manifest Reference

`bob.yaml` is a strict, human-owned product contract. Unknown fields and
unsupported values are errors, not warnings. Bob will not politely ignore a
typo and guess what you meant.

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
| `recipe` | `go-agent-tool`, `files` | Selects the embedded repository recipe. |

Two recipes are embedded: `go-agent-tool@4`, documented below, and `files@1`,
a plain file-tree recipe documented in its own section further down. `bob
recipe list` prints both; an unrecognized recipe id fails manifest validation
and suggests the nearest match rather than guessing.

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
does not mean Bob ran the tool, initialized an index, or verified behavior. Bob
writes the sign on the door; it doesn't open the shop.

## Distribution

| Field | Values | Effect |
|---|---|---|
| `github_actions` | boolean | Generates GitHub CI; a release workflow is generated only when this and `goreleaser` are both true. |
| `goreleaser` | boolean | Generates GoReleaser configuration. |
| `homebrew` | boolean | Generates a Homebrew cask; requires GoReleaser, public visibility, and a GitHub module. |
| `docs` | `none`, `markdown` | Generates a Markdown documentation entry page. |

Changing a field changes only files declared by the recipe. `bob plan` shows
the exact ownership decision before `bob apply` writes anything.

Everything above this line is specific to `recipe: go-agent-tool`. It does not
apply to `recipe: files`, described next.

## The `files` recipe

`recipe: files` declares an arbitrary file tree directly in `bob.yaml`. There
is no Go/Cobra scaffold underneath it — you write the paths, Bob writes the
bytes. Use it for anything `go-agent-tool` doesn't shape: a web service, a
config bundle, a set of scripts, a non-Go project.

```yaml
schema_version: 1
recipe: files
product:
  name: my-app
  description: A generated web service
vars:
  project_name: my-app
  port: "8080"
files:
  - path: package.json
    content: |
      {"name": "${vars.project_name}"}
  - path: scripts/run.sh
    mode: "0755"
    content: |
      #!/usr/bin/env bash
      echo "listening on ${vars.port}"
```

This is `bob recipe show files`'s own example. It is meant to be copied.

### `vars`

A flat `map[string]string`. Keys must match `^[a-z][a-z0-9_]*$` — lowercase,
starts with a letter, digits and underscores after that. A declared-but-unused
var is fine; Bob doesn't nag about it. There is no nesting, no lists, no
numbers-as-numbers — every value is a string, substituted as a string.

### `files`

A list of entries, each `{path, mode?, content}`:

| Field | Constraint | Default |
|---|---|---|
| `path` | must resolve inside the workspace | required |
| `mode` | optional 3–4 digit octal string, e.g. `"0755"` | `"0644"` |
| `content` | written verbatim after substitution | required |

`mode` rejects setuid, setgid, and sticky bits — this recipe hands out file
permissions, not privilege escalation. Duplicate paths, compared after the
same canonicalization Bob uses for ownership everywhere else, are rejected at
validate time, before any rendering happens.

### Substitution

One deterministic, literal-replacement regex pass over `\$\{vars\.([a-z][a-z0-9_]*)\}`.
That's it — no loops, no conditionals, no includes. This is not a template
language wearing a trench coat.

Text that doesn't match the pattern passes through untouched, including a
shell script's own `${FOO}`: `files` only recognizes the `vars.` prefix, so
`#!/usr/bin/env bash` and `echo "$HOME"` render exactly as written.

A reference to an undeclared var is a render-time error, not a silent blank.
Bob collects every unresolved reference across every file, sorts and dedupes
them, and reports them together with their file paths in one failure:

```text
bob: plan: render files: unresolved variable reference(s): extra.txt: ${vars.missing_one}; extra.txt: ${vars.missing_two}
```

### Path safety

Identical to the engine's existing rules, because it's the same engine:

- paths cannot be absolute or escape the workspace;
- paths cannot target `.git`, `bob.yaml`, or `bob.lock`;
- a pre-existing symlink or special file at a destination is a per-path
  `conflict` in the plan, not an aborted command.

### Ownership, plainly stated

Bob owns file existence, mode, and byte-for-byte convergence for every
declared path — the same plan/apply/lock guarantees as `go-agent-tool`. What
Bob does **not** own is what the content means, and it does not evolve that
content for you over time. Unlike `go-agent-tool`, there is no upstream
template carrying this recipe's output forward across versions — there is
nothing to upgrade toward. You wrote the content; you own its future edits.
`bob new` and `bob init` still scaffold `go-agent-tool` only. A `files`
manifest is hand-authored or agent-authored from scratch, which is exactly
what `bob recipe show files` is for.

See [Build any repository](../guides/any-repository.md) for the complete
worked example: writing the manifest, planning, applying, editing content, and
watching Bob report `content_update` on the next plan.
