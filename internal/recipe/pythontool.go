package recipe

import (
	"fmt"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

type pythonToolTemplateData struct {
	Manifest      manifest.Manifest
	Product       manifest.Product
	RecipeVersion int
	GitHubOwner   string
	GitHubRepo    string
	PkgName       string
}

// renderPythonTool renders the complete, reviewable Python CLI scaffold. Like
// rust-tool it owns a whole repository — a thin argparse entry point over a
// tested pure function, plus Python tooling configuration, documentation, CI,
// and a release workflow — but it keeps no extension-registry contract.
func renderPythonTool(m manifest.Manifest, version int) ([]Artifact, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("render python-tool: %w", err)
	}
	owner, repo, _ := githubModuleCoordinates(m.Product.Module)
	pkg := strings.ReplaceAll(m.Product.Name, "-", "_")
	data := pythonToolTemplateData{
		Manifest:      m,
		Product:       m.Product,
		RecipeVersion: version,
		GitHubOwner:   owner,
		GitHubRepo:    repo,
		PkgName:       pkg,
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
		{".editorconfig", pythonToolEditorConfigTemplate},
		{".gitignore", pythonToolGitignoreTemplate},
		{".github/workflows/ci.yml", pythonToolCITemplate},
		{".github/workflows/release.yml", pythonToolReleaseTemplate},
		{"AGENTS.md", pythonToolAgentsTemplate},
		{"CHANGELOG.md", pythonToolChangelogTemplate},
		{"CODE_OF_CONDUCT.md", pythonToolCodeOfConductTemplate},
		{"CONTRIBUTING.md", pythonToolContributingTemplate},
		{"LICENSE", pythonToolLicenseTemplate},
		{"README.md", pythonToolReadmeTemplate},
		{"SECURITY.md", pythonToolSecurityTemplate},
		{"Taskfile.yml", pythonToolTaskfileTemplate},
		{"docs/index.md", pythonToolDocsIndexTemplate},
		{"pyproject.toml", pythonToolPyprojectTemplate},
		{"ruff.toml", pythonToolRuffTemplate},
		{"src/" + pkg + "/__init__.py", pythonToolInitTemplate},
		{"src/" + pkg + "/__main__.py", pythonToolMainTemplate},
		{"src/" + pkg + "/cli.py", pythonToolCliTemplate},
		{"tests/test_cli.py", pythonToolTestTemplate},
	}
	for _, item := range base {
		if err := add(item.path, item.source); err != nil {
			return nil, err
		}
	}

	seedPythonToolLivingFiles(artifacts)

	// Render is the public sorting and path-safety boundary, but keeping the
	// producer ordered makes direct recipe tests deterministic.
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

// pythonToolLivingFiles are created once by python-tool@1 and then left in
// human hands. pyproject.toml in particular changes the first time a real
// repository adds a dependency; lock-owning it would make every later bob
// check/apply fail on an ordinary edit.
var pythonToolLivingFiles = map[string]struct{}{
	"AGENTS.md":      {},
	"CHANGELOG.md":   {},
	"README.md":      {},
	"pyproject.toml": {},
}

func seedPythonToolLivingFiles(artifacts []Artifact) {
	for i := range artifacts {
		if _, living := pythonToolLivingFiles[artifacts[i].Path]; living {
			artifacts[i].Seed = true
		}
	}
}

const pythonToolPyprojectTemplate = `[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = [[tomlQuote .Product.Name]]
version = "0.1.0"
description = [[tomlQuote .Product.Description]]
readme = "README.md"
license = { text = "MIT" }
requires-python = ">=3.9"
authors = [{ name = [[tomlQuote (printf "%s contributors" .Product.Name)]] }]

[project.scripts]
[[.Product.Name]] = "[[.PkgName]].cli:main"

[dependency-groups]
dev = [
    "build>=1.2",
    "pytest>=8.0",
    "ruff>=0.6",
]

[tool.hatch.build.targets.wheel]
packages = ["src/[[.PkgName]]"]

[tool.pytest.ini_options]
testpaths = ["tests"]
`

const pythonToolInitTemplate = `"""[[.Product.Name]] package."""

from .cli import greet, main

__all__ = ["greet", "main"]
`

const pythonToolMainTemplate = `"""Module entry point: run the [[.Product.Name]] command line."""

from [[.PkgName]].cli import main

if __name__ == "__main__":
    raise SystemExit(main())
`

const pythonToolCliTemplate = `"""Command-line entry point for [[.Product.Name]]."""

from __future__ import annotations

import argparse
import json
import sys


def greet(name: str) -> str:
    """Return a friendly greeting for the given name."""
    return f"Hello, {name}!"


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog=[[quote .Product.Name]],
        description=[[quote .Product.Description]],
    )
    parser.add_argument(
        "name",
        nargs="?",
        default="world",
        help="who to greet (default: world)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="write a machine-readable JSON greeting to stdout",
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    message = greet(args.name)
    if args.json:
        json.dump({"greeting": message}, sys.stdout)
        sys.stdout.write("\n")
    else:
        print(message)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
`

const pythonToolTestTemplate = `from [[.PkgName]].cli import greet, main


def test_greet_formats_a_name():
    assert greet("world") == "Hello, world!"


def test_main_prints_greeting(capsys):
    assert main(["cli"]) == 0
    assert capsys.readouterr().out == "Hello, cli!\n"
`

const pythonToolGitignoreTemplate = `__pycache__/
*.py[cod]
*.egg-info/
.venv/
venv/
/dist/
/build/
.pytest_cache/
.ruff_cache/
.DS_Store
.env
.env.*
!.env.example
`

const pythonToolEditorConfigTemplate = `root = true

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

const pythonToolRuffTemplate = `line-length = 100
target-version = "py39"

[lint]
select = ["E", "F", "I", "UP", "B", "SIM"]
`

const pythonToolTaskfileTemplate = `version: "3"

tasks:
  default:
    cmds:
      - task --list

  fmt-check:
    desc: Check Python formatting without changing files.
    cmds:
      - python -m ruff format --check .

  lint:
    desc: Run Ruff with the selected rule set.
    cmds:
      - python -m ruff check .

  test:
    desc: Run the test suite.
    cmds:
      - python -m pytest

  build:
    desc: Build the wheel and source distribution.
    cmds:
      - python -m build

  run:
    desc: Run the CLI.
    cmds:
      - python -m [[.PkgName]] --help

  check:
    desc: Run the canonical non-mutating verification gate.
    aliases: [verify]
    cmds:
      - task: fmt-check
      - task: lint
      - task: test
      - task: build
`

const pythonToolCITemplate = `name: CI

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
  python:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      - uses: actions/setup-python@ece7cb06caefa5fff74198d8649806c4678c61a1 # v6.3.0
        with:
          python-version: "3.13"
          cache: pip
      - run: python -m pip install -e . pytest ruff
      - run: python -m ruff check .
      - run: python -m pytest
`

const pythonToolReleaseTemplate = `name: Release

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
      - uses: actions/setup-python@ece7cb06caefa5fff74198d8649806c4678c61a1 # v6.3.0
        with:
          python-version: "3.13"
      - run: python -m pip install -e . pytest build
      - run: python -m pytest
      - run: python -m build
`

const pythonToolReadmeTemplate = `# [[.Product.Name]]

[[.Product.Description]]

## Install

~~~bash
python -m pip install .
~~~

## Usage

~~~bash
[[.Product.Name]] world
~~~

## Development

~~~bash
python -m pip install -e . pytest ruff build
task check
~~~

See [AGENTS.md](AGENTS.md) for the agent and contributor contract,
[CONTRIBUTING.md](CONTRIBUTING.md) for the contribution workflow, and
[SECURITY.md](SECURITY.md) for security reporting instructions.

## License

MIT — see [LICENSE](LICENSE).
`

const pythonToolAgentsTemplate = `# AGENTS.md

This file is the source of truth for agents and contributors working on [[.Product.Name]].

## Product

[[.Product.Description]]

## Architecture

- ` + "`src/[[.PkgName]]/cli.py`" + ` owns the argparse entry point and the pure ` + "`greet`" + ` function.
- ` + "`src/[[.PkgName]]/__main__.py`" + ` is the thin module entry point.
- ` + "`src/[[.PkgName]]/__init__.py`" + ` re-exports the public API.
- ` + "`tests/test_cli.py`" + ` covers the pure function and the CLI through pytest.
- ` + "`pyproject.toml`" + ` is human-owned: adding a dependency is a normal edit.

## Commands

~~~bash
python -m pip install -e . pytest ruff build
python -m ruff check .
python -m pytest
python -m build
~~~

## Invariants

1. Keep ` + "`src/[[.PkgName]]/__main__.py`" + ` thin; put behavior in ` + "`src/[[.PkgName]]/cli.py`" + `.
2. Add or update tests for every behavior change.
3. Never commit credentials, private data, or local environment files.
`

const pythonToolChangelogTemplate = `# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial public repository foundation.
`

const pythonToolContributingTemplate = `# Contributing

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

const pythonToolCodeOfConductTemplate = `# Code of Conduct

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

const pythonToolSecurityTemplate = `# Security Policy

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

const pythonToolLicenseTemplate = `MIT License

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

const pythonToolDocsIndexTemplate = `# [[.Product.Name]]

[[.Product.Description]]

## Start here

~~~bash
python -m pip install .
[[.Product.Name]] world
~~~

## Development

Run ` + "`task check`" + ` before opening a pull request. See [AGENTS.md](../AGENTS.md)
for the agent contract.
`
