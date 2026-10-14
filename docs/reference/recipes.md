---
description: The embedded recipe catalog — the six factory scaffolds, the files recipe, and the twenty-seven seed-once stack hygiene recipes.
---

# Recipes

Bob embeds **thirty-four recipes in three kinds**. Every recipe is versioned,
renders its complete desired file set in memory, and is immutable once
published: changing a published recipe means adding a new version, never editing
bytes an old `bob.lock` depends on.

- **Factory scaffolds** own a complete, public-ready repository. Their
  composition files are lock-owned (hash-proven); their living files
  (`README.md`, `CHANGELOG.md`, `AGENTS.md`, and the language manifest) seed once
  and then stay in human hands.
- **`files`** materializes a tree you declare in `bob.yaml`.
- **Stack hygiene** recipes seed create-once hygiene only. They are never
  recorded in `bob.lock`, never updated, and **never own application source**.

## Factory scaffolds

Each factory recipe takes a `product.module` (repository identity) and a product
name, and can be selected explicitly with `--recipe`. None of them is
auto-detected: `bob init` never picks a factory on its own.

| Recipe | Language / runtime | What it emits |
|---|---|---|
| `go-agent-tool@6` | Go, Cobra CLI | A public-ready Go command with human + JSON output, dependency-injected handlers, tests, CI, GoReleaser, docs, and a Glyphrun help spec. It carries a static human-owned command extension contract. |
| `rust-tool@1` | Rust CLI | A thin binary over a tested library core, rustfmt/clippy config, CI, a release workflow, docs, and a Taskfile. |
| `python-tool@1` | Python CLI | A PEP 621 project with a `src/` layout, an argparse entry point over a tested pure function, ruff + pytest CI, a release workflow, docs, and a Taskfile. |
| `ts-tool@1` | TypeScript (Node) CLI | An ESM project with a `process.argv` entry point over a tested pure function, tsc/eslint/vitest CI, an npm release workflow, docs, and a Taskfile. |
| `java-tool@1` | Java CLI (Maven) | A Maven project with a JUnit 5 test, an executable jar, CI, a release workflow, docs, and a Taskfile. |
| `csharp-tool@1` | .NET (C#) CLI | A `net8.0` project with an xUnit test, CI, a release workflow, docs, and a Taskfile. |

Create one with `bob new <name> --recipe <id> --module <repo> --write`, or
preview it with `bob new <name> --recipe <id>`. `bob recipe show <id>` prints the
exact seeded and owned paths.

## `files`

`recipe: files` declares an inline file tree in `bob.yaml` (`files:` entries with
a path, optional mode, and content, plus optional `vars` for `${vars.<key>}`
substitution). Bob materializes it with the same plan, apply, and lock rules as
any other recipe. See the [manifest reference](./manifest.md#recipe-files) for
the schema and a worked example.

## Stack hygiene

One recipe per detected stack. Each seeds `README.md`, `AGENTS.md`,
`SECURITY.md`, `.gitignore`, `.editorconfig`, and (for some stacks) a small
extra config file and a CI stub — created only when missing, never updated,
never lock-owned. The `runtime.kind` selects kind-aware defaults where a recipe
has more than one.

| Recipe | Detected stack | `runtime.kind` |
|---|---|---|
| `go-hygiene@2` | Go (`go.mod`, `go.work`) | `service`, `lib`, `app` |
| `ts-app@2` | TypeScript | `app`, `monorepo` |
| `js-app@2` | JavaScript | `app`, `monorepo` |
| `vue-app@2` | Vue | `web-app` |
| `nuxt-app@2` | Nuxt | `web-app` |
| `deno-app@2` | Deno (`deno.json`) | `app`, `library` |
| `python-app@2` | Python | `app` |
| `php-app@2` | PHP (`composer.json`) | `app`, `library` |
| `ruby-app@2` | Ruby | `app`, `gem` |
| `java-app@2` | Java (`pom.xml`, Gradle) | `app`, `library` |
| `kotlin-app@2` | Kotlin | `app`, `library` |
| `dotnet-app@2` | .NET / C# | `app`, `library` |
| `cpp-cmake@2` | C/C++ (`CMakeLists.txt`) | `app`, `library` |
| `rust-cli@2` | Rust (`Cargo.toml`) | `cli`, `lib`, `workspace` |
| `swift-package@2` | Swift | `package` |
| `elixir-app@2` | Elixir / Mix | `app`, `umbrella` |
| `lua-lib@2` | Lua | `lib`, `plugin` |
| `dart-app@2` | Dart / Flutter (`pubspec.yaml`) | `app`, `package` |
| `zig-app@2` | Zig (`build.zig`) | `app`, `library` |
| `scala-app@2` | Scala (`build.sbt`) | `app`, `library` |
| `docker-app@2` | Docker (`Dockerfile`, Compose) | `image` |
| `terraform-app@2` | Terraform (`*.tf`) | `module`, `stack` |
| `nix-app@2` | Nix (`flake.nix`) | `flake`, `package` |
| `ansible-app@2` | Ansible (`ansible.cfg`, `roles/`) | `playbook`, `role` |
| `helm-app@2` | Helm (`Chart.yaml`) | `chart` |
| `kubernetes-app@2` | Kubernetes / Kustomize | `manifests`, `kustomize` |
| `static-web@2` | Static web site | `site` |

`bob recipe list` prints the catalog; `bob recipe show <id>` prints one recipe's
stack, description, and seeded paths. For the full command surface, see the
[CLI reference](./cli.md#the-recipe-catalog).
