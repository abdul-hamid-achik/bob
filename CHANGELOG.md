# Changelog

All notable changes to Bob will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project uses semantic versioning after the first tagged release.

## [Unreleased]

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
