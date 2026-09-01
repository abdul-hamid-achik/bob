# Changelog

All notable changes to Bob will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project uses semantic versioning after the first tagged release.

## [Unreleased]

### Added

- `go-agent-tool@6` seeds living files once (`README.md`, `CHANGELOG.md`,
  `AGENTS.md`, `CLAUDE.md`, `go.mod`, `go.sum`) and then leaves them in human
  hands. Composition files stay Bob-owned. Published `@3`, `@4`, and `@5`
  remain byte-identical and renderable.
- `ownership.release` on `go-agent-tool` and `files` manifests marks named
  recipe paths as seed-once so `resolve-ownership-conflict` can keep human
  content without blocking later apply.
- Compact `bob context` names the first three conflicting paths in
  `repository.top_conflicts` and on the human verdict line.
- This repository now dogfoods `go-hygiene@2` with `bob.yaml` / `bob.lock`,
  and `bob check` runs in the verify gate.

### Changed

- `bob init` reads the Go module path from an existing `go.mod` when
  `--module` is omitted.
- MCP `recipe_describe` reports empty `surfaces` for stack hygiene recipes
  that reject any surface declaration.
- Telemetry records the actual catalog recipe id and version instead of a
  boolean `go-agent-tool` flag.
- Documented Go floor is 1.26.6, matching `go.mod`. Generated
  `go-agent-tool` modules stay on the published 1.26.5 security floor.
- Public getting-started, homepage, existing-repository, and
  any-repository copy now lead with stack hygiene for existing language
  repositories. The existing-repository conflict example no longer uses
  `README.md` (seed-once on both stack recipes and `go-agent-tool@6`).

### Fixed

- `workspace.Resolve` follows a symlink ancestor when the target does not
  exist yet, so `bob new` works under `/tmp`.
- Recipe smoke tests skip an unusable host `golangci-lint` shim instead of
  failing with exit 126.
- Apply failures no longer print a duplicated `apply: apply:` prefix.
- `bob doctor` honors `cmd.Context()` instead of `context.Background()`.

## [0.10.0] - 2026-08-30

### Added

- Descriptive MCP and Studio surfaces: `surfaces.mcp` and `surfaces.studio`
  may now be `true` in a go-agent-tool manifest. They declare product
  reality the recipe neither generates nor verifies (like integration
  seams), surface as `surface.mcp`/`surface.studio` capabilities in
  `bob context`, and require no schema or recipe version bump — rendered
  artifacts and existing plan digests are byte-identical.
- Conflict classification in `bob context`: additive
  `repository.conflict_class`
  (`none|ownership_hazard|contract_drift|unmanaged_divergence|mixed`),
  `repository.conflict_family_counts`, `repository.action_counts`, and
  `repository.lock_exists` fields distinguish "Bob-owned file drifted" from
  "recipe proposes scaffold over a human-owned repository" while the
  `repository.state` enum stays wire-compatible. The conflicted-state
  continuation `reason_code` now carries the class
  (`conflict_unmanaged_divergence` and siblings).
- Per-entry causes in `bob plan` human output: every row now renders
  `kind path [code] family` using a shared engine classifier
  (`ownership_hazard`, `contract_drift`, `unmanaged_divergence`,
  `scaffold`, `convergence`), so drift and scaffold proposals are
  distinguishable without a JSON round trip. Plan and apply guidance is
  class-aware: all-unmanaged-divergence conflicts with no lock now say the
  recipe was never applied and the files are human-owned.
- Read-only surface-evidence reconciliation in `bob context`: recipe
  metadata declares bounded evidence rules per surface capability (path
  existence, single-segment globs, and byte-capped literal matches);
  `surface.mcp` ships defaults that catch conventional `internal/mcp`
  layouts and `mcp` mentions in the recipe-known entrypoint. A disabled
  surface with matching evidence emits a `surface_evidence_mismatch`
  warning notice whose corrective text is derived from the manifest
  validator itself. Context remains read-only and runs no subprocesses.
- `go-hygiene@2` seed-once hygiene recipe for existing Go repositories:
  seeds README, AGENTS, SECURITY, `.gitignore`, `.editorconfig` (with tab
  indentation), `.golangci.yml`, and a golangci-lint CI stub. Never owns
  application source, cmd/, or internal/cli/. Detection distinguishes
  `go-agent-tool` (existing bob.yaml with that recipe, OR cmd/ + internal/cli/
  Cobra layout) from plain Go repositories, which now default to `go-hygiene`.
- `nuxt-app@2` seed-once hygiene recipe, with `nuxt.config.*` and `nuxt`
  dependency detection, Nuxt-aware `.gitignore` (`.nuxt/`, `.output/`), and
  a Bun-oriented CI stub. Preserves JavaScript or TypeScript and the declared
  package manager. Catalog, detection, and tests updated.

### Changed

- Compact-context byte-budget truncation now drops recipe-boilerplate
  invariants and notice explanatory messages before dropping whole
  notices, so workspace-specific warnings survive the 6 KiB compact
  profile (and the 8 KiB MCP gateway bound) while remaining deterministic
  and recorded in `truncation.omitted` (`invariants`, `notice_messages`).

## [0.9.0] - 2026-07-27

### Added

- `swift-package@2` and `elixir-app@2` seed-once hygiene recipes, with
  `Package.swift` and `mix.exs` detection, pinned CI stubs, optional toolchain
  checks in `bob doctor`, catalog metadata, and end-to-end init/apply/check
  coverage.
- Immutable rendering coverage for every published stack recipe at version 1,
  including byte-level regression digests.

### Changed

- All stack hygiene recipes advance to version 2. Existing version-1 seed
  files remain human-owned and untouched; `bob upgrade` only advances the
  recipe identity recorded in `bob.lock`.
- `bob new --write` now refuses a stack hygiene recipe in a greenfield target
  instead of producing a repository with hygiene files but no application
  source; preview remains available and points to `bob init` after the real
  stack has been initialized.
- JavaScript-family manifests can preserve `runtime.package_manager` as
  `bun`, `npm`, `pnpm`, or `yarn`; generated commands and CI now honor the
  detected lockfile, and `bob doctor` probes the matching manager. Vue
  detection also preserves JavaScript when no TypeScript marker exists.
- Rust detection distinguishes CLI, library, and workspace layouts. Lua
  runtime versions now follow library versus Neovim-plugin kind, Ruby Gemfiles
  follow app versus gem kind, and Python tooling consistently targets 3.13.

### Fixed

- Kind hints from a secondary detected stack can no longer overwrite the
  primary stack's kind.
- Public manifest, CLI, context, path, agent, and architecture references now
  describe the current recipe versions, artifact set, and language matrix.

## [0.8.0] - 2026-07-27

### Added

- Immutable `go-agent-tool@5`, with a generated registry regression test that
  proves the human-owned command factory seam and keeps a blank scaffold clean
  under its own configured linter.
- End-to-end CLI tests for recipe upgrade success, dry-run, current-version
  no-op, conflict recovery guidance, digest mismatch, and missing-lock errors.
- `task agent-bootstrap`, which runs the coding-agent onboarding contract from
  the current source tree without relying on an ignored local binary.

### Changed

- Rendered-project smoke coverage now runs `golangci-lint` when it is available,
  with isolated caches, in addition to `go test` and `go mod tidy`.
- `bob remove` rechecks each file immediately before unlink and rechecks the
  exact loaded lock before deleting `bob.lock`, preserving concurrent edits and
  ownership evidence.
- Upgrade failures now return command-specific corrective actions instead of
  directing callers to apply.
- Public reference pages now describe plan watch, recipe upgrade, safe remove,
  exit codes, and the current recipe catalog consistently.

### Fixed

- Fresh `go-agent-tool@4` repositories exposed an intentionally available
  `registerCommand` extension seam that their configured linter classified as
  unused. Version 5 proves the seam from generated test code without changing
  the published version-4 bytes.
- Plan-diff and remove Glyphrun specs introduced in 0.7.0 now carry validated
  contract hashes instead of placeholder zero values.
- Release comparison links now include every published version since 0.5.0.

## [0.7.0] - 2026-07-20

### Added

- **`bob remove [path]`** — removes only
  `bob.lock`-tracked files whose content hash still matches, never touches
  unmanaged files or `bob.yaml`, cleans up empty directories, and deletes the
  lock last. `--force` removes drifted files; `--dry-run` previews without
  writing. Exit `2` on skipped/conflicted files, `4` when no lock exists.
- **`bob plan --diff`** — unified content diffs for create and update actions
  using a bounded stdlib-only LCS algorithm (1 MiB / 8192-line cap).
  Presentation-only: never affects the plan digest. JSON output adds a
  `diffs` array (omitted without the flag).
- **`bob plan --watch`** — polls `bob.yaml` every second and re-runs the plan
  on change. Stdlib-only (no fsnotify); graceful SIGINT; tolerates invalid or
  missing manifests without crashing. Incompatible with `--json`.
- **`bob upgrade [path]`** — detects when `bob.lock` was written by an older
  recipe version and re-applies with the current contract. `--dry-run`
  previews; `--expect-plan-digest` gates authority. Exit `4` when no lock
  exists or the lock is newer than supported.
- **Enriched stack recipes** — all eight stack hygiene recipes now seed
  language-specific tooling configs: `.editorconfig` (universal),
  `tsconfig.json` + `.prettierrc` (ts-app), `.prettierrc` (js-app, vue-app),
  `pyproject.toml` + `.python-version` (python-app), `.rubocop.yml` +
  `.ruby-version` + `Gemfile` (ruby-app), `.luacheckrc` + `.lua-version`
  (lua-lib), `clippy.toml` + `rust-toolchain.toml` (rust-cli), `.htmlhintrc`
  (static-web). All seed-once, never lock-owned.
- **Glyphrun behavior specs** for `bob remove` (lifecycle + dry-run) and
  `bob plan --diff` (human + JSON output).
- **Fuzz tests** for `NormalizeRepositoryPath`, `validateRelativePath`, and
  `safePath` — property-based verification of the path-safety invariants with
  ~120 seed inputs across the three functions.
- **`internal/version` test** locking the `dev`/`none`/`unknown` ldflags
  sentinel defaults.

### Changed

- **`internal/fsutil`** — new shared package extracting `IsSymlinkOrNotDir`,
  `IsSymlinkOrNotRegular`, `WriteAtomic`, and `DecodeStrictYAML[T]` from
  ~12 duplicated call sites across engine, manifest, settings, telemetry,
  and workspace.
- **`internal/engine/fs.go`** — 17 bare `return err` sites now wrapped with
  operation and path context for debuggable apply failures.

### Fixed

- Homebrew cask caveats said "six typed repository tools"; the MCP server
  registers nine.
- `AGENTS.md` architecture block omitted `internal/detect`, `internal/guidance`,
  and `internal/strsim`.

## [0.6.1] - 2026-07-16

### Fixed

- **`bob new --write` into an already bob-managed target is refused up front** with
  `input_invalid` (exit 4) and guidance toward `bob plan`/`bob check`, instead of failing
  mid-write with a raw "bob.yaml already exists" at exit 1 — a path newly reachable since
  stack recipes scaffold into non-empty targets.

## [0.6.0] - 2026-07-16

### Added

- **`bob new --recipe <id>`** — scaffold any catalog recipe from `bob new` (go-agent-tool, files,
  and all eight stack recipes), with stack auto-detection as the default on non-empty targets and
  the go-agent-tool default preserved for greenfield directories. `--module` is required for
  go-agent-tool and rejected for non-Go recipes; an explicit stack recipe that mismatches the
  detected stack warns on preview and refuses `--write` (exit 4), mirroring `bob init`.

## [0.5.1] - 2026-07-16

### Fixed

- `bob doctor` no longer requires Go for files-recipe workspaces; it probes
  Git only, matching stack recipes.

## [0.5.0] - 2026-07-15

### Added

- Repository stack detection in `bob init`: marker-file detection for Go,
  TypeScript/Bun, JavaScript, Vue, Python, Ruby, Lua, Rust, and static web
  sites (with workspace/monorepo, gem, and Neovim-plugin hints plus
  sass/tailwind/postcss/vite signals). Init auto-selects the recipe matching
  the detected stack, prints a prominent preview warning on a mismatch, and
  refuses a mismatched `--write` unless `--force` is passed. New `--recipe`
  and `--force` flags; `--module` is now required only by `go-agent-tool`.
- Eight data-driven stack hygiene recipes (`ts-app@1`, `js-app@1`,
  `vue-app@1`, `python-app@1`, `ruby-app@1`, `lua-lib@1`, `rust-cli@1`,
  `static-web@1`) that seed `README.md`, `AGENTS.md`, `SECURITY.md`,
  `.gitignore`, and an optional stack-appropriate CI stub. Every artifact is
  seed-once: created only when missing, never recorded in `bob.lock`, never
  updated or overwritten, and application source is never owned.
- Seed-once artifact semantics in the engine (`seed_exists` action code):
  existing destinations satisfy a seed regardless of content, human edits stay
  `check`-clean, and deleted seeds re-create as ordinary drift.
- `bob recipe list/show`, `bob learn`, `bob doctor`, and the MCP
  `bob_recipe_describe` tool cover the stack hygiene recipes; doctor probes
  Git plus the optional language toolchain instead of requiring Go for them.

## [0.4.0] - 2026-07-15

### Added

- `bob context [workspace]` with deterministic compact, standard, and full
  workspace-contract profiles, typed capability facets, recipe-owned entry
  points, honest human extension points, invariants, notices, structured
  continuation actions, and explicit byte-budget truncation.
- Versioned recipe metadata for `go-agent-tool@4` and `files@1`, including
  stable artifact IDs and cross-reference validation without workspace
  inspection.
- Contract and context digests for workspace context.
- `bob path` exact path classification using the planner's real desired,
  locked, symlink, special-file, reserved-path, and extension metadata rules.
- Closed, typed, non-executing `bob playbook list|show|plan` guidance with seven
  stable initial IDs, argv-shaped steps, deterministic risk/scope, honest
  extension-contract materialization blockers, and bounded outputs.
- Shared structured guidance types for notices, actions, and truncation.
- Immutable `go-agent-tool@4` with deterministic command registration from
  human-owned extension files, visible duplicate-ID/name failures, stable
  command ordering, and safe upgrades from clean version-3 locks.
- `bob apply --expect-plan-digest sha256:<digest>` and a bounded immediate apply
  receipt. A stale reviewed plan now fails with `plan_digest_mismatch`, exit
  code 5, and zero repository writes. Apply loads and renders `bob.yaml` under
  the workspace lock, rechecks its exact source before publication, and returns
  complete change counts with deterministic path-list truncation instead of
  echoing an unbounded second copy of the plan.
- Read-only `bob_context`, `bob_path`, and `bob_playbook` MCP tools using the
  existing exact workspace authority model and the shared service layer.
- Versioned consumer JSON fixtures generated from real context, path,
  playbook, missing-input, and unsupported-future-schema structured contracts.
- A deterministic identity/state text projection for `bob_context` avoids a
  redundant full JSON copy and keeps the complete compact MCP response below
  the 8 KiB gateway threshold.

### Changed

- Plan digest version 1 now has one engine implementation shared by CLI plan
  and check and the existing MCP plan/check tools. CLI JSON adds
  `plan_digest_version` and `plan_digest` without replacing plan actions.
- CLI plan digests are directly consumable `sha256:` values; MCP preserves its
  raw v1 digest and adds `plan_digest_qualified` additively.
- `bob learn` now catalogs `context`, `path`, and `playbook`, and publishes the recommended agent
  bootstrap sequence: learn, context, plan, check.
- The stdio MCP surface now contains nine typed read-only tools. The recommended
  weak-model pins are `bob_context`, `bob_plan`, and `bob_check`; path and
  playbook guidance remain available through lazy discovery.
- Private design notes no longer ship as public documentation; the published
  site keeps normative contracts in `docs/reference/` and product architecture
  in its dedicated public pages.

## [0.3.0] - 2026-07-13

### Added

- `files` recipe (`files@1`): declare an arbitrary file tree inline in
  `bob.yaml` with `files:` and `vars:`, materialized with the same plan/apply
  ownership safety as `go-agent-tool`. Substitution is a single deterministic
  `${vars.key}` literal-replacement pass — not a template language — and
  unresolved references fail rendering with every offender listed.
- Machine-readable action codes: every plan action now carries a stable
  `code` (`unmanaged_differs`, `managed_hash_mismatch`, `symlink`,
  `retired_owned`, …) in CLI JSON and the MCP `bob_plan`/`bob_check` tools, so
  agents branch on codes instead of parsing English reasons.
- Bounded `current_preview` next to `desired_preview` on conflict and update
  actions in plan JSON; `plan --content` prints both sides for conflicts.
- Exit-code contract: `0` success, `1` internal error, `2` conflicts
  (`apply` refusal, `check`), `3` drift without conflicts (`check`),
  `4` invalid input or manifest. `plan` remains a report and exits `0`.
- JSON failure envelopes now carry a closed error-code vocabulary
  (`missing_manifest`, `manifest_invalid`, `conflicts`, `input_invalid`,
  `workspace_invalid`) and populated `next_actions` with copy-pasteable
  corrective commands; human failures print the same next steps on stderr.
- `apply` refused by conflicts now reports the conflicting paths with codes
  and reasons (JSON `data.conflicts` and bounded human list) instead of
  requiring a second `plan` round-trip.
- `--conflicts-only` on `plan` and `check` for compact output in
  output-capped agent harnesses.
- Validation errors echo the offending value and suggest close matches
  ("did you mean") for recipe ids and enum fields; missing `bob.yaml` errors
  name the fix instead of a raw `lstat` errno.
- `bob learn`: a one-shot, read-only onboarding brief for coding agents with a
  versioned `--json` envelope covering the lifecycle, command catalog, safety
  invariants, exit codes, error codes, MCP surface, and documentation
  locations.
- Public documentation site at <https://bobcli.dev>: custom VitePress theme,
  agent-focused `/agents` guide, sitemap, and search-engine metadata.

### Changed

- Recipe versions are now tracked per recipe id (`go-agent-tool@3`,
  `files@1`); `bob.lock` stamps the version of the manifest's own recipe.
- The normative specification moved from the repository (`SPEC.md`) to the
  published reference pages at <https://bobcli.dev>.

## [0.2.0] - 2026-07-12

### Added

- Initial `go-agent-tool` repository recipe.
- Versioned `bob.yaml` manifest and content-hashed `bob.lock` ownership file.
- Deterministic plan, explicit apply, drift checking, and dependency doctor.
- Human-readable and versioned JSON command output.
- Offline `bob inspect` readiness inventory with explicit bounded Codemap and
  Vecgrep status probing.
- Initial typed read-only stdio MCP server with compact `bob_inspect` and
  `bob_plan` tools for MCPHub and local-agent.
- Strict XDG-style user settings with side-effect-free path resolution and
  private, no-overwrite configuration initialization.
- Disabled-by-default, local-only telemetry with a privacy-bounded event
  schema, retention and daily caps, workspace pseudonyms, and aggregate CLI
  stats.
- Read-only `bob studio` TUI with responsive Overview, Plan, and Stats views,
  accessible single-pane mode, refresh, navigation, and stale-snapshot errors.
- Rich six-tool MCP surface adding convergence checks, strict manifest
  validation, recipe discovery, and aggregate local stats.
- Exact MCP workspace allowlists with explicit additional-workspace and
  any-workspace authority modes.
- Task-oriented VitePress documentation site with executable manifest examples.
- Canonical non-mutating verification, vulnerability scanning, release-config
  checks, pinned GitHub Actions, CI concurrency, and Dependabot maintenance.
- Code of Conduct and expanded issue, pull-request, security, and contributor
  guidance.

### Changed

- Advanced `go-agent-tool` through recipe version 3 with GitHub community
  templates, Dependabot, stronger CI, a security-patched Go baseline, and safe
  upgrades from older same-recipe locks.
- Completed the Homebrew cask metadata and install guidance.

[Unreleased]: https://github.com/abdul-hamid-achik/bob/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/abdul-hamid-achik/bob/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/abdul-hamid-achik/bob/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/abdul-hamid-achik/bob/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/abdul-hamid-achik/bob/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/abdul-hamid-achik/bob/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/abdul-hamid-achik/bob/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/abdul-hamid-achik/bob/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/abdul-hamid-achik/bob/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/abdul-hamid-achik/bob/releases/tag/v0.3.0
[0.2.0]: https://github.com/abdul-hamid-achik/bob/releases/tag/v0.2.0
