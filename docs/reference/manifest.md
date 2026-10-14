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
| `recipe` | `go-agent-tool`, `python-tool`, `rust-tool`, `ts-tool`, `files`, or a stack hygiene recipe id | Selects the embedded repository recipe. |

Six kinds of recipe are embedded: `go-agent-tool@6`, `python-tool@1`,
`rust-tool@1`, and `ts-tool@1`, the four
factory recipes documented below; `files@1`, a plain file-tree recipe
documented in its own section further down; and the stack hygiene recipes
(`ts-app@2`, `js-app@2`, `vue-app@2`, `nuxt-app@2`, `deno-app@2`,
`python-app@2`, `php-app@2`, `ruby-app@2`, `lua-lib@2`, `rust-cli@2`,
`java-app@2`, `kotlin-app@2`, `dotnet-app@2`, `cpp-cmake@2`, `docker-app@2`,
`terraform-app@2`, `nix-app@2`, `swift-package@2`, `elixir-app@2`,
`static-web@2`, `go-hygiene@2`, `dart-app@2`, `zig-app@2`, `scala-app@2`,
`ansible-app@2`, `helm-app@2`, and `kubernetes-app@2`),
documented last. `bob recipe list` prints all of them; an unrecognized recipe
id fails manifest validation and suggests the nearest match rather than
guessing.

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

`surfaces.mcp` and `surfaces.studio` are descriptive declarations of product
reality, in the same sense an integration selection declares a seam: the
go-agent-tool recipe neither generates nor verifies them, so any boolean is
valid. Setting `mcp: true` records that the product ships an MCP surface —
for example a `headless mcp` subcommand — without changing what Bob owns.
`bob context` additionally warns when a disabled surface has declared
repository evidence (see the [workspace context
reference](context.md#structured-actions-and-notices)). Declaring a product
surface does not conflict with Bob's own MCP server (nine read-only tools
plus the opt-in, digest-gated `bob_apply`).

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

`go-agent-tool@6` creates `README.md`, `CHANGELOG.md`, `AGENTS.md`,
`CLAUDE.md`, `go.mod`, and `go.sum` once and never lock-owns them. Later
human edits stay clean. Composition files (`internal/cli/root.go`,
`internal/cli/registry.go`, and their tests) remain Bob-owned.

### `ownership.release`

`ownership.release` is an additive list of recipe artifact paths. Each named
path becomes seed-once: Bob creates it if missing, then never lock-owns or
updates it. Unknown paths, reserved paths, and stack-recipe use are validation
errors. Use it when `resolve-ownership-conflict` should keep human content
instead of restoring Bob-owned bytes.

```yaml
ownership:
  release:
    - Taskfile.yml
```

Everything above this line is specific to `recipe: go-agent-tool`. It does not
apply to `recipe: files`, described next.

## The `rust-tool` recipe

`recipe: rust-tool` is the Rust analogue of `go-agent-tool`: a public-ready
Rust CLI scaffold that owns a complete, reviewable repository. It renders a
thin `src/main.rs` binary over a tested `src/lib.rs` core, an integration test
in `tests/cli.rs` that runs the built binary, Rust tooling configuration
(`rustfmt.toml`, `clippy.toml`, `rust-toolchain.toml`), repository hygiene,
`docs/index.md`, a `Taskfile.yml`, and `.github/workflows/ci.yml` plus a
release workflow.

Schema, relative to `go-agent-tool`:

- `product.module` is **optional** repository identity — the same shape rules
  as a Go module path when present, but it is not a Cargo path and is never
  written into `Cargo.toml`.
- `product.visibility` must be `public` or `private`, and `product.license`
  must be `MIT`.
- `runtime.language` must be `rust` and `runtime.kind` must be `cli`.
- `surfaces.cli` and `surfaces.json` must both be `true`.
- `integrations` and `distribution` are unused and must stay zero-valued; the
  recipe always renders its CI and release workflows.
- `vars` and `files` are unsupported; `ownership.release` works as on
  `go-agent-tool`.

`rust-tool@1` creates `Cargo.toml`, `README.md`, `AGENTS.md`, and
`CHANGELOG.md` once and never lock-owns them: adding a dependency to
`Cargo.toml` is a normal human edit that stays clean. Everything else the
recipe renders — including `src/main.rs`, `src/lib.rs`, `tests/cli.rs`, and the
workflows — remains Bob-owned.

`bob new <name> --recipe rust-tool --module <repo> --write` scaffolds a fresh
repository; `bob init --recipe rust-tool` previews a manifest for an existing
directory. `--module` is optional repository identity for this recipe, and
`rust-tool` is never auto-detected: it must be selected explicitly.

## The `python-tool` recipe

`recipe: python-tool` is the Python analogue of `go-agent-tool`: a public-ready
Python CLI scaffold that owns a complete, reviewable repository. It renders an
argparse entry point in `src/<pkg>/cli.py` over a tested pure `greet` function,
a `src/<pkg>/__main__.py` module entry point and `src/<pkg>/__init__.py`
package initializer, a pytest suite in `tests/test_cli.py`, Python tooling
configuration (`pyproject.toml`, `ruff.toml`), repository hygiene,
`docs/index.md`, a `Taskfile.yml`, and `.github/workflows/ci.yml` plus a
release workflow. `<pkg>` is the product name normalized to a valid Python
identifier (lowercase, hyphens replaced with underscores).

Schema, relative to `go-agent-tool`:

- `product.module` is **optional** repository identity — the same shape rules
  as a Go module path when present, but it is not a Python import path and is
  never written into `pyproject.toml`.
- `product.visibility` must be `public` or `private`, and `product.license`
  must be `MIT`.
- `runtime.language` must be `python` and `runtime.kind` must be `cli`.
- `surfaces.cli` and `surfaces.json` must both be `true`.
- `integrations` and `distribution` are unused and must stay zero-valued; the
  recipe always renders its CI and release workflows.
- `vars` and `files` are unsupported; `ownership.release` works as on
  `go-agent-tool`.

`python-tool@1` creates `pyproject.toml`, `README.md`, `AGENTS.md`, and
`CHANGELOG.md` once and never lock-owns them: adding a dependency to
`pyproject.toml` is a normal human edit that stays clean. Everything else the
recipe renders — including `src/<pkg>/cli.py`, `src/<pkg>/__main__.py`, and
`tests/test_cli.py` — remains Bob-owned.

`bob new <name> --recipe python-tool --module <repo> --write` scaffolds a fresh
repository; `bob init --recipe python-tool` previews a manifest for an existing
directory. `--module` is optional repository identity for this recipe, and
`python-tool` is never auto-detected: it must be selected explicitly.

## The `ts-tool` recipe

`recipe: ts-tool` is the TypeScript (Node) analogue of `go-agent-tool`: a
public-ready TypeScript CLI scaffold that owns a complete, reviewable
repository. It renders a thin `src/cli.ts` entry point over a tested pure
`greet` function in `src/lib.ts`, a Vitest suite in `tests/cli.test.ts`,
TypeScript tooling configuration (`tsconfig.json`, `eslint.config.js`,
`vitest.config.ts`, `.prettierrc`), repository hygiene, `docs/index.md`, a
`Taskfile.yml`, and `.github/workflows/ci.yml` plus an npm release workflow.

Schema, relative to `go-agent-tool`:

- `product.module` is **optional** repository identity — the same shape rules
  as a Go module path when present, but it is not an npm package scope and is
  never written into `package.json`.
- `product.visibility` must be `public` or `private`, and `product.license`
  must be `MIT`.
- `runtime.language` must be `typescript` and `runtime.kind` must be `cli`.
- `surfaces.cli` and `surfaces.json` must both be `true`.
- `integrations` and `distribution` are unused and must stay zero-valued; the
  recipe always renders its CI and release workflows.
- `vars` and `files` are unsupported; `ownership.release` works as on
  `go-agent-tool`.

`ts-tool@1` creates `package.json`, `README.md`, `AGENTS.md`, and
`CHANGELOG.md` once and never lock-owns them: adding a dependency to
`package.json` is a normal human edit that stays clean. Everything else the
recipe renders — including `src/cli.ts`, `src/lib.ts`, `tests/cli.test.ts`, and
the workflows — remains Bob-owned.

`bob new <name> --recipe ts-tool --module <repo> --write` scaffolds a fresh
repository; `bob init --recipe ts-tool` previews a manifest for an existing
directory. `--module` is optional repository identity for this recipe, and
`ts-tool` is never auto-detected: it must be selected explicitly.

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
- paths cannot target `.git`, `bob.yaml`, `bob.lock`, or `.bob.apply.lock`,
  nor anything beneath them;
- these rules are enforced by manifest validation itself (`bob plan`, `bob
  check`, and the MCP `bob_validate_manifest` tool report them as
  `manifest_invalid`), and again by the engine before any write;
- a pre-existing symlink or special file at a destination is a per-path
  `conflict` in the plan, not an aborted command.

### Ownership, plainly stated

`ownership.release` works the same as on `go-agent-tool`: listed declared
paths become seed-once and leave `bob.lock`.

Bob owns file existence, mode, and byte-for-byte convergence for every
declared path — the same plan/apply/lock guarantees as `go-agent-tool`. What
Bob does **not** own is what the content means, and it does not evolve that
content for you over time. Unlike `go-agent-tool`, there is no upstream
template carrying this recipe's output forward across versions — there is
nothing to upgrade toward. You wrote the content; you own its future edits.
`bob new --recipe files` can scaffold the minimal files manifest, while an
omitted `--recipe` keeps `go-agent-tool` as the greenfield default and selects
the detected stack recipe for a target that already has content. A `files`
manifest may also be authored directly; `bob recipe show files` prints its
schema and a copyable example.

See [Build any repository](../guides/any-repository.md) for the complete
worked example: writing the manifest, planning, applying, editing content, and
watching Bob report `content_update` on the next plan.

## The stack hygiene recipes

`ts-app@2`, `js-app@2`, `vue-app@2`, `nuxt-app@2`, `deno-app@2`, `python-app@2`,
`php-app@2`, `ruby-app@2`, `lua-lib@2`, `rust-cli@2`, `java-app@2`,
`kotlin-app@2`, `dotnet-app@2`, `cpp-cmake@2`, `docker-app@2`,
`terraform-app@2`, `nix-app@2`, `swift-package@2`, `elixir-app@2`,
`static-web@2`, `go-hygiene@2`, `dart-app@2`, `zig-app@2`, `scala-app@2`,
`ansible-app@2`, `helm-app@2`, and `kubernetes-app@2`
share one deliberately small contract for repositories whose
application source Bob must never own. Each renders the universal
**seed-once** artifacts `README.md`, `AGENTS.md`, `SECURITY.md`, `.gitignore`,
and `.editorconfig`, plus stack-specific formatter, linter, or runtime
configuration where applicable. When `distribution.github_actions: true`,
each also renders a stack-appropriate `.github/workflows/ci.yml` stub.

A seed-once artifact is created only when its destination is missing. Any
existing destination satisfies it — whatever its content, and even a symlink,
directory, or special file: Bob reads nothing through it. It is never
recorded in `bob.lock`, never updated, and never overwritten. The human owns
every seeded file from the moment it exists; later edits keep `bob check`
clean, and deleting one is ordinary drift that `bob apply` re-seeds.

These recipes are not language project generators. `bob new --write` refuses
to use one in a missing or empty target because that would create hygiene
without application source. Initialize the real project first (`swift package
init`, `mix new`, the chosen Node framework, and so on), then run `bob init
--write` followed by `bob apply`.

Schema, relative to `go-agent-tool`:

- `product.module` is **optional** repository identity (same shape rules as a
  Go module path when present).
- `product.visibility` and `product.license` are optional.
- `runtime.language` and `runtime.kind` must match the recipe's contract,
  below. Validation names the allowed values when either is wrong, and
  `bob recipe show <id>` prints each recipe's stack.
- `runtime.package_manager` is optional and supported only by `ts-app`,
  `js-app`, `vue-app`, and `nuxt-app`. Its values are `bun`, `npm`, `pnpm`, or `yarn`.
  When detection sees a corresponding lockfile, `bob init` writes that value
  and version 2 uses it for seeded commands and CI. If omitted, `ts-app`,
  `vue-app`, and `nuxt-app` default to Bun while `js-app` defaults to npm. `bob doctor` probes
  that selected manager (and Node where the manager requires it) as optional
  stack tooling.

  | Recipe | `runtime.language` | `runtime.kind` |
  |---|---|---|
  | `ts-app` | `typescript` | `app` or `monorepo` |
  | `js-app` | `javascript` | `app` or `monorepo` |
  | `vue-app` | `typescript` or `javascript` | `web-app` |
  | `nuxt-app` | `typescript` or `javascript` | `web-app` |
  | `deno-app` | `typescript` | `app` or `library` |
  | `python-app` | `python` | `app` |
  | `php-app` | `php` | `app` or `library` |
  | `ruby-app` | `ruby` | `app` or `gem` |
  | `lua-lib` | `lua` | `lib` or `plugin` |
  | `rust-cli` | `rust` | `cli`, `lib`, or `workspace` |
  | `java-app` | `java` | `app` or `library` |
  | `kotlin-app` | `kotlin` | `app` or `library` |
  | `dotnet-app` | `csharp` | `app` or `library` |
  | `cpp-cmake` | `cpp` | `app` or `library` |
  | `dart-app` | `dart` | `app` or `package` |
  | `zig-app` | `zig` | `app` or `library` |
  | `scala-app` | `scala` | `app` or `library` |
  | `docker-app` | `docker` | `image` |
  | `terraform-app` | `hcl` | `module` or `stack` |
  | `nix-app` | `nix` | `flake` or `package` |
  | `swift-package` | `swift` | `package` |
  | `elixir-app` | `elixir` | `app` or `umbrella` |
  | `static-web` | `html` | `site` |
  | `go-hygiene` | `go` | `service`, `lib`, or `app` |
  | `ansible-app` | `ansible` | `playbook` or `role` |
  | `helm-app` | `helm` | `chart` |
  | `kubernetes-app` | `kubernetes` | `manifests` or `kustomize` |

  `bob init` picks the kind from detection where the recipe supports the
  detected hint (a Node workspace selects `monorepo`, a `.gemspec` selects
  `gem`, a Neovim plugin layout selects `plugin`, Cargo distinguishes
  `cli`/`lib`/`workspace`, and Mix detects `umbrella`); otherwise it defaults
  to the recipe's first kind. A Vue or Nuxt repository without `tsconfig.json`
  selects `runtime.language: javascript`.
- `surfaces` and `integrations` are unused and must stay zero-valued.
- `distribution.github_actions` is the only supported distribution toggle;
  `goreleaser`, `homebrew`, and `docs` are not supported.

`bob init` detects the repository stack from marker files (`go.mod`,
`package.json`/`tsconfig.json`/lockfiles, a `nuxt` dependency or `nuxt.config.*`,
a `vue` dependency or `.vue` files, `pyproject.toml`, `Gemfile`/`*.gemspec`,
`*.rockspec`/`init.lua`/`lua/`, `Cargo.toml`, `Package.swift`, `mix.exs`,
`pom.xml`/`build.gradle`/`build.gradle.kts`, `pubspec.yaml`/`pubspec.yml`,
`build.zig`, `build.sbt`/`project/build.properties`, a `Dockerfile`/Compose file,
a root `*.tf` file, a `flake.nix`/`shell.nix`/`default.nix`, an
`ansible.cfg`/`playbook.yml`/`roles/` layout, a `Chart.yaml`, a
`kustomization.yaml`/`kustomization.yml`, or a bare
`index.html`) and defaults to the matching recipe.
When the chosen recipe's stack does not match the detected one, the preview
warns and `--write` refuses without `--force`.

Version 1 of the original eight stack recipes remains reproducible byte for
byte. Because stack artifacts are seed-once and never lock-owned, upgrading a
version-1 workspace advances only the recipe version in `bob.lock`; it never
rewrites files already seeded into that repository.
