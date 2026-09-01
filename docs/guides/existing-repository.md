---
description: Bring Bob into a TypeScript, Python, Rust, Go, or HTML repository that already exists — seed hygiene once, never own application source.
---

# Existing Repository

Bob does not care that your directory already has a life. `bob init` writes
the human-owned manifest first and waits for you to review the plan before
it touches a generated file. On most existing repositories that plan is
**stack hygiene**: docs, ignore files, and optional CI seeded once. Bob
never owns `src/`, `internal/` application packages, or the rest of your
code.

It does not generate a better Next.js, FastAPI, or Cargo app. That work
stays yours. What it adds is a contract an agent can read before editing.

## Initialize the manifest

```bash
cd path/to/existing-app
bob init --write
bob apply
bob check
```

`--write` on `init` writes `bob.yaml` only. `apply` creates missing seeds.
`check` confirms the lock and the seeds.

`bob init` detects the stack from marker files (`package.json`,
`tsconfig.json`, `pyproject.toml`, `Cargo.toml`, `go.mod`, `index.html`,
and the rest of the [detected
stacks](../reference/manifest.md#the-stack-hygiene-recipes)) and defaults
to the matching recipe:

| Detected stack | Recipe | What Bob does |
|---|---|---|
| TypeScript / JS / Vue / Nuxt | `ts-app`, `js-app`, `vue-app`, `nuxt-app` | Seed-once hygiene. Package manager comes from the lockfile when present. |
| Python / Ruby / Lua / Rust / Swift / Elixir / static HTML | matching `@2` recipe | Same seed-once contract. Never owns application source. |
| Plain Go (`go.mod` without a Cobra `cmd/` + `internal/cli` layout) | `go-hygiene` | Same seed-once contract, plus a golangci-lint CI stub when GitHub Actions is on. |
| Go with `cmd/` + `internal/cli/root.go`, or an existing `recipe: go-agent-tool` | `go-agent-tool` | The deep factory. `--module` is required unless `go.mod` already has a module path. |

A stack recipe never conflicts with an existing file: any destination that
already exists satisfies the seed, including a symlink or directory. Later
edits keep `bob check` clean. Deleting a seeded file is ordinary drift;
the next apply re-seeds it.

`bob new --write` refuses a stack recipe on an empty directory. Initialize
the real project first (`npm create`, `cargo init`, `swift package init`,
and so on), then run `bob init`. Passing a recipe that does not match the
detected stack makes `--write` refuse unless you add `--force`.

## What gets seeded

Every stack recipe at version 2 renders `README.md`, `AGENTS.md`,
`SECURITY.md`, `.gitignore`, and `.editorconfig` when they are missing,
plus stack-specific formatter or linter config where the recipe defines
it. `distribution.github_actions: true` adds a CI stub. None of those
paths land in `bob.lock`.

See the [manifest
reference](../reference/manifest.md#the-stack-hygiene-recipes) for
`runtime.language`, `runtime.kind`, and package-manager rules.

## When the recipe is `go-agent-tool`

Use this path only when you actually want the Go/Cobra factory — not for
a Go service that already has its own layout.

```bash
bob init . \
  --name acme-tool \
  --module github.com/acme/acme-tool \
  --description "Agent-ready Acme CLI" \
  --write
```

`--write` still writes only `bob.yaml`. Review `bob plan` before `apply`.
Living files (`README.md`, `CHANGELOG.md`, `AGENTS.md`, `CLAUDE.md`,
`go.mod`, `go.sum`) are seed-once on `@6`. An existing README is not a
conflict. Composition files (`Taskfile.yml`, `cmd/…/main.go`,
`internal/cli/root.go`, CI) stay Bob-owned and *can* conflict.

## Review the plan

```bash
bob plan
bob plan --content
```

`--content` includes bounded desired-content previews for create and update
actions. It does not make the command writable.

## Resolve a lock-owned conflict

Suppose a `go-agent-tool` or `files` plan wants `Taskfile.yml` and the
directory already has a custom one. Bob reports `conflict`: it cannot
prove ownership, the content differs, and it will not gamble.

Choose deliberately:

1. Keep the custom file. Apply stays refused while the recipe targets it.
2. Move the custom file to a reviewed backup, rerun `bob plan`, and let
   Bob create the desired file.
3. Make the file match the desired content and mode exactly. Bob
   classifies it as `adopt` and takes ownership from there.
4. Keep the human content and stop managing the path: add it to
   `ownership.release` in `bob.yaml`, then follow
   `resolve-ownership-conflict` (`release_to_human` → `review_plan` →
   `apply_release`). The path becomes seed-once and leaves `bob.lock`.

Never delete or overwrite a conflict merely to make the command green.
Decide which system owns the path, then let the plan reflect that
decision.

Stack hygiene plans do not take this branch. Their seeds never conflict.

## Apply and check

When the complete plan is conflict-free:

```bash
bob plan --json
bob apply --expect-plan-digest sha256:<64-lowercase-hex> --json
bob check
git diff --stat
```

The guarded apply fresh-plans while holding Bob's apply lock and writes
nothing if the reviewed identity is stale. Review the resulting
repository before committing.

For a declared file tree instead of hygiene, see
[Build any repository](./any-repository.md). For a new Go/Cobra CLI from
an empty directory, see [Getting Started](../getting-started.md).
