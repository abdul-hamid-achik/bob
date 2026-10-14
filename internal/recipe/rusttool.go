package recipe

import (
	"fmt"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

type rustToolTemplateData struct {
	Manifest      manifest.Manifest
	Product       manifest.Product
	RecipeVersion int
	GitHubOwner   string
	GitHubRepo    string
	CrateName     string
}

// renderRustTool renders the complete, reviewable Rust CLI scaffold. Unlike
// the stack hygiene recipes (which only seed hygiene files), rust-tool owns a
// whole repository: a thin binary over a tested library core, plus Rust
// tooling configuration, documentation, CI, and a release workflow.
func renderRustTool(m manifest.Manifest, version int) ([]Artifact, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("render rust-tool: %w", err)
	}
	owner, repo, _ := githubModuleCoordinates(m.Product.Module)
	data := rustToolTemplateData{
		Manifest:      m,
		Product:       m.Product,
		RecipeVersion: version,
		GitHubOwner:   owner,
		GitHubRepo:    repo,
		CrateName:     strings.ReplaceAll(m.Product.Name, "-", "_"),
	}

	var artifacts []Artifact
	add := func(path, source string) error {
		content, err := executeRecipeTemplate(path, source, data)
		if err != nil {
			return err
		}
		artifacts = append(artifacts, Artifact{Path: path, Mode: 0o644, Content: content})
		return nil
	}

	type templateArtifact struct {
		path   string
		source string
	}
	base := []templateArtifact{
		{".editorconfig", rustToolEditorConfigTemplate},
		{".gitignore", rustToolGitignoreTemplate},
		{".github/workflows/ci.yml", rustToolCITemplate},
		{".github/workflows/release.yml", rustToolReleaseTemplate},
		{"AGENTS.md", rustToolAgentsTemplate},
		{"CHANGELOG.md", rustToolChangelogTemplate},
		{"CODE_OF_CONDUCT.md", rustToolCodeOfConductTemplate},
		{"CONTRIBUTING.md", rustToolContributingTemplate},
		{"Cargo.toml", rustToolCargoTemplate},
		{"LICENSE", rustToolLicenseTemplate},
		{"README.md", rustToolReadmeTemplate},
		{"SECURITY.md", rustToolSecurityTemplate},
		{"Taskfile.yml", rustToolTaskfileTemplate},
		{"clippy.toml", rustToolClippyTemplate},
		{"docs/index.md", rustToolDocsIndexTemplate},
		{"rust-toolchain.toml", rustToolToolchainTemplate},
		{"rustfmt.toml", rustToolRustfmtTemplate},
		{"src/lib.rs", rustToolLibTemplate},
		{"src/main.rs", rustToolMainTemplate},
		{"tests/cli.rs", rustToolCliTestTemplate},
	}
	for _, item := range base {
		if err := add(item.path, item.source); err != nil {
			return nil, err
		}
	}

	seedRustToolLivingFiles(artifacts)

	// Render is the public sorting and path-safety boundary, but keeping the
	// producer ordered makes direct recipe tests deterministic.
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

// rustToolLivingFiles are created once by rust-tool@1 and then left in human
// hands. Cargo.toml in particular changes the first time a real repository
// adds a dependency; lock-owning it would make every later bob check/apply
// fail on an ordinary edit.
var rustToolLivingFiles = map[string]struct{}{
	"AGENTS.md":    {},
	"CHANGELOG.md": {},
	"Cargo.toml":   {},
	"README.md":    {},
}

func seedRustToolLivingFiles(artifacts []Artifact) {
	for i := range artifacts {
		if _, living := rustToolLivingFiles[artifacts[i].Path]; living {
			artifacts[i].Seed = true
		}
	}
}

const rustToolCargoTemplate = `[package]
name = [[tomlQuote .Product.Name]]
version = "0.1.0"
edition = "2021"
description = [[tomlQuote .Product.Description]]
license = "MIT"

[dependencies]
`

const rustToolLibTemplate = `//! [[.Product.Name]] library: a tiny, deterministic core for the CLI binary.

/// Returns a friendly greeting for the given name.
pub fn greet(name: &str) -> String {
    format!("Hello, {name}!")
}

#[cfg(test)]
mod tests {
    use super::greet;

    #[test]
    fn greet_formats_a_name() {
        assert_eq!(greet("world"), "Hello, world!");
    }
}
`

const rustToolMainTemplate = `use std::process::ExitCode;

use [[.CrateName]]::greet;

fn main() -> ExitCode {
    let name = std::env::args()
        .nth(1)
        .unwrap_or_else(|| "world".to_string());
    println!("{}", greet(&name));
    ExitCode::SUCCESS
}
`

const rustToolCliTestTemplate = `use std::process::Command;

#[test]
fn binary_greets_a_name() {
    let output = Command::new(env!("CARGO_BIN_EXE_[[.CrateName]]"))
        .arg("cli")
        .output()
        .expect("failed to run the binary");
    assert!(output.status.success());
    let stdout = String::from_utf8(output.stdout).expect("stdout is not UTF-8");
    assert_eq!(stdout.trim(), "Hello, cli!");
}
`

const rustToolGitignoreTemplate = `/target/
.DS_Store
.env
.env.*
!.env.example
`

const rustToolEditorConfigTemplate = `root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 4

[*.md]
trim_trailing_whitespace = false
`

const rustToolRustfmtTemplate = `edition = "2021"
max_width = 100
`

const rustToolClippyTemplate = `msrv = "1.74"
cognitive-complexity-threshold = 30
too-many-arguments-threshold = 8
`

const rustToolToolchainTemplate = `[toolchain]
channel = "stable"
components = ["clippy", "rustfmt"]
`

const rustToolTaskfileTemplate = `version: "3"

tasks:
  default:
    cmds:
      - task --list

  fmt-check:
    desc: Check Rust formatting without changing files.
    cmds:
      - cargo fmt --all --check

  lint:
    desc: Run Clippy with warnings denied.
    cmds:
      - cargo clippy --all-targets -- -D warnings

  test:
    desc: Run unit and integration tests.
    cmds:
      - cargo test

  build:
    desc: Build the CLI in release mode.
    cmds:
      - cargo build --release

  run:
    desc: Run the CLI.
    cmds:
      - cargo run --quiet -- --help

  check:
    desc: Run the canonical non-mutating verification gate.
    aliases: [verify]
    cmds:
      - task: fmt-check
      - task: lint
      - task: test
      - task: build
`

const rustToolCITemplate = `name: CI

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

concurrency:
  group: ci-${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  rust:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      # rust-toolchain.toml selects the stable toolchain and required components.
      - run: cargo fmt --all --check
      - run: cargo clippy --all-targets -- -D warnings
      - run: cargo test
      - run: cargo build
`

const rustToolReleaseTemplate = `name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: read

jobs:
  release:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      # rust-toolchain.toml selects the stable toolchain and required components.
      - run: cargo test
      - run: cargo build --release
`

const rustToolReadmeTemplate = `# [[.Product.Name]]

[[.Product.Description]]

## Install

~~~bash
cargo install --path .
~~~

## Usage

~~~bash
[[.Product.Name]] world
~~~

## Development

~~~bash
task check
~~~

See [AGENTS.md](AGENTS.md) for the agent and contributor contract,
[CONTRIBUTING.md](CONTRIBUTING.md) for the contribution workflow, and
[SECURITY.md](SECURITY.md) for security reporting instructions.

## License

MIT — see [LICENSE](LICENSE).
`

const rustToolAgentsTemplate = `# AGENTS.md

This file is the source of truth for agents and contributors working on [[.Product.Name]].

## Product

[[.Product.Description]]

## Architecture

- ` + "`src/lib.rs`" + ` owns the library core and its unit tests.
- ` + "`src/main.rs`" + ` is the thin binary entry point.
- ` + "`tests/cli.rs`" + ` runs the built binary as an integration test.
- ` + "`Cargo.toml`" + ` is human-owned: adding dependencies is a normal edit.

## Commands

~~~bash
cargo fmt --all --check
cargo clippy --all-targets -- -D warnings
cargo test
cargo build
~~~

## Invariants

1. Keep ` + "`src/main.rs`" + ` thin; put behavior in ` + "`src/lib.rs`" + `.
2. Add or update tests for every behavior change.
3. Never commit credentials, private data, or local environment files.
`

const rustToolChangelogTemplate = `# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial public repository foundation.
`

const rustToolContributingTemplate = `# Contributing

Thank you for improving [[.Product.Name]].

Read [AGENTS.md](AGENTS.md) before changing behavior or public contracts.

1. Open an issue for substantial behavior or contract changes.
2. Create a focused branch and keep unrelated changes out of the diff.
3. Add or update tests for every behavior change.
4. Run ` + "`task check`" + ` before opening a pull request.

Pull requests should explain the user impact and the verification performed.
Never include credentials or private data. Community participation follows
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Contributions are licensed under the
repository's MIT License.
`

const rustToolCodeOfConductTemplate = `# Code of Conduct

This community policy is based on the
[Contributor Covenant 2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).

Be respectful, inclusive, and constructive. Harassment, threats,
discrimination, sexualized conduct, personal attacks, trolling, and publishing
another person's private information are not accepted.

This standard applies in repository issues, pull requests, reviews, and other
spaces where someone represents the project. Maintainers may edit or remove
contributions, limit participation, or ban a participant when needed to protect
the community.

Report conduct concerns privately using a monitored contact method. Do not post
sensitive report details in a public issue. Reports should be handled promptly,
fairly, and as confidentially as practical.

This policy is adapted from the Contributor Covenant, version 2.1, available at
https://www.contributor-covenant.org/version/2/1/code_of_conduct/ and licensed
under Creative Commons Attribution 4.0.
`

const rustToolSecurityTemplate = `# Security Policy

## Supported versions

Before the first tagged release, security fixes are made on ` + "`main`" + `. After
the first release, the latest release and ` + "`main`" + ` are supported.

## Reporting a vulnerability

[[if and .GitHubOwner (eq .Product.Visibility "public")]]Use [GitHub private vulnerability reporting](https://github.com/[[.GitHubOwner]]/[[.GitHubRepo]]/security/advisories/new).
Repository maintainers must enable that feature before publishing this project.
[[else]]This scaffold cannot name a configured private reporting channel. Before
publishing, replace this paragraph with an actually monitored private contact.
[[end]]
Do not open a public issue for an unpatched vulnerability. Include the affected
version, impact, reproduction steps, and any suggested mitigation. Do not
include real credentials or unrelated personal data.
`

const rustToolLicenseTemplate = `MIT License

Copyright (c) 2026 [[.Product.Name]] contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`

const rustToolDocsIndexTemplate = `# [[.Product.Name]]

[[.Product.Description]]

## Start here

~~~bash
cargo build --release
./target/release/[[.Product.Name]] world
~~~

## Development

Run ` + "`task check`" + ` before opening a pull request. See [AGENTS.md](../AGENTS.md)
for the agent contract.
`
