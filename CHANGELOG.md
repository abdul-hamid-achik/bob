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
- Typed read-only stdio MCP server with compact `bob_inspect` and `bob_plan`
  tools for MCPHub and local-agent.
- Task-oriented VitePress documentation site with executable manifest examples.
- Canonical non-mutating verification, vulnerability scanning, release-config
  checks, pinned GitHub Actions, CI concurrency, and Dependabot maintenance.
- Code of Conduct and expanded issue, pull-request, security, and contributor
  guidance.

### Changed

- Advanced `go-agent-tool` to recipe version 2 with GitHub community templates,
  Dependabot, stronger CI, and safe upgrades from older same-recipe locks.
- Completed the Homebrew cask metadata and install guidance.
