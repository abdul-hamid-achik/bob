# Changelog

All notable changes to Bob will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project uses semantic versioning after the first tagged release.

## [Unreleased]

### Added

- **`bob remove [path]`** — the inverse of `bob apply`: removes only
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

[Unreleased]: https://github.com/abdul-hamid-achik/bob/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/abdul-hamid-achik/bob/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/abdul-hamid-achik/bob/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/abdul-hamid-achik/bob/releases/tag/v0.3.0
[0.2.0]: https://github.com/abdul-hamid-achik/bob/releases/tag/v0.2.0
