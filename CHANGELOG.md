# Changelog

All notable changes to Bob will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project uses semantic versioning after the first tagged release.

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

[Unreleased]: https://github.com/abdul-hamid-achik/bob/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/abdul-hamid-achik/bob/releases/tag/v0.2.0
