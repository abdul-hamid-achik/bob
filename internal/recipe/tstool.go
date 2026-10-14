package recipe

import (
	"fmt"
	"sort"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

type tsToolTemplateData struct {
	Manifest      manifest.Manifest
	Product       manifest.Product
	RecipeVersion int
	GitHubOwner   string
	GitHubRepo    string
}

// renderTSTool renders the complete, reviewable TypeScript (Node) CLI
// scaffold. Like rust-tool and python-tool it owns a whole repository — a thin
// process.argv entry point over a tested pure function, plus TypeScript
// tooling configuration, documentation, CI, and an npm release workflow — but
// it keeps no extension-registry contract.
func renderTSTool(m manifest.Manifest, version int) ([]Artifact, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("render ts-tool: %w", err)
	}
	owner, repo, _ := githubModuleCoordinates(m.Product.Module)
	data := tsToolTemplateData{
		Manifest:      m,
		Product:       m.Product,
		RecipeVersion: version,
		GitHubOwner:   owner,
		GitHubRepo:    repo,
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
		{".editorconfig", tsToolEditorConfigTemplate},
		{".gitignore", tsToolGitignoreTemplate},
		{".github/workflows/ci.yml", tsToolCITemplate},
		{".github/workflows/release.yml", tsToolReleaseTemplate},
		{".prettierrc", tsToolPrettierTemplate},
		{"AGENTS.md", tsToolAgentsTemplate},
		{"CHANGELOG.md", tsToolChangelogTemplate},
		{"CODE_OF_CONDUCT.md", tsToolCodeOfConductTemplate},
		{"CONTRIBUTING.md", tsToolContributingTemplate},
		{"LICENSE", tsToolLicenseTemplate},
		{"README.md", tsToolReadmeTemplate},
		{"SECURITY.md", tsToolSecurityTemplate},
		{"Taskfile.yml", tsToolTaskfileTemplate},
		{"docs/index.md", tsToolDocsIndexTemplate},
		{"eslint.config.js", tsToolEslintTemplate},
		{"package.json", tsToolPackageTemplate},
		{"src/cli.ts", tsToolCliTemplate},
		{"src/lib.ts", tsToolLibTemplate},
		{"tests/cli.test.ts", tsToolTestTemplate},
		{"tsconfig.json", tsToolTsconfigTemplate},
		{"vitest.config.ts", tsToolVitestTemplate},
	}
	for _, item := range base {
		if err := add(item.path, item.source); err != nil {
			return nil, err
		}
	}

	seedTSToolLivingFiles(artifacts)

	// Render is the public sorting and path-safety boundary, but keeping the
	// producer ordered makes direct recipe tests deterministic.
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

// tsToolLivingFiles are created once by ts-tool@1 and then left in human
// hands. package.json in particular changes the first time a real repository
// adds a dependency; lock-owning it would make every later bob check/apply
// fail on an ordinary edit.
var tsToolLivingFiles = map[string]struct{}{
	"AGENTS.md":    {},
	"CHANGELOG.md": {},
	"README.md":    {},
	"package.json": {},
}

func seedTSToolLivingFiles(artifacts []Artifact) {
	for i := range artifacts {
		if _, living := tsToolLivingFiles[artifacts[i].Path]; living {
			artifacts[i].Seed = true
		}
	}
}

const tsToolPackageTemplate = `{
  "name": [[quote .Product.Name]],
  "version": "0.1.0",
  "description": [[quote .Product.Description]],
  "type": "module",
  "license": "MIT",
  "bin": {
    [[quote .Product.Name]]: "dist/cli.js"
  },
  "files": [
    "dist"
  ],
  "exports": {
    ".": "./dist/lib.js"
  },
  "scripts": {
    "build": "tsc",
    "lint": "eslint .",
    "test": "vitest run"
  },
  "devDependencies": {
    "@eslint/js": "^9.13.0",
    "@types/node": "^22.9.0",
    "eslint": "^9.13.0",
    "prettier": "^3.3.3",
    "typescript": "^5.6.3",
    "typescript-eslint": "^8.12.2",
    "vitest": "^2.1.8"
  }
}
`

const tsToolTsconfigTemplate = `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "lib": ["ES2022"],
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "declaration": true,
    "sourceMap": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["src"]
}
`

const tsToolEslintTemplate = `// @ts-check
import eslint from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  {
    ignores: ["dist/", "node_modules/", "coverage/"],
  },
);
`

const tsToolVitestTemplate = `import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["tests/**/*.test.ts"],
  },
});
`

const tsToolPrettierTemplate = `{
  "semi": true,
  "singleQuote": true,
  "printWidth": 100,
  "trailingComma": "all"
}
`

const tsToolLibTemplate = `/**
 * [[.Product.Name]] library: a tiny, deterministic core for the CLI.
 */

/** Returns a friendly greeting for the given name. */
export function greet(name: string): string {
  return "Hello, " + name + "!";
}
`

const tsToolCliTemplate = `#!/usr/bin/env node

import { greet } from "./lib.js";

/**
 * Runs the [[.Product.Name]] command line: greets the first positional
 * argument (default "world") and, with --json, writes a machine-readable
 * greeting object to stdout.
 */
function main(): number {
  const args = process.argv.slice(2);
  const json = args.includes("--json");
  const name = args.find((arg) => !arg.startsWith("--")) ?? "world";
  const message = greet(name);
  if (json) {
    process.stdout.write(JSON.stringify({ greeting: message }) + "\n");
  } else {
    process.stdout.write(message + "\n");
  }
  return 0;
}

process.exitCode = main();
`

const tsToolTestTemplate = `import { describe, expect, it } from "vitest";

import { greet } from "../src/lib.js";

describe("greet", () => {
  it("formats a name", () => {
    expect(greet("world")).toBe("Hello, world!");
  });
});
`

const tsToolGitignoreTemplate = `node_modules/
/dist/
coverage/
.DS_Store
.env
.env.*
!.env.example
`

const tsToolEditorConfigTemplate = `root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 2

[*.md]
trim_trailing_whitespace = false
`

const tsToolTaskfileTemplate = `version: "3"

tasks:
  default:
    cmds:
      - task --list

  install:
    desc: Install dependencies.
    cmds:
      - npm install

  fmt-check:
    desc: Check formatting without changing files.
    cmds:
      - npx prettier --check .

  lint:
    desc: Run ESLint.
    cmds:
      - npm run lint

  test:
    desc: Run the test suite.
    cmds:
      - npm run test

  build:
    desc: Compile TypeScript.
    cmds:
      - npm run build

  run:
    desc: Run the CLI.
    cmds:
      - npm run build
      - node dist/cli.js

  check:
    desc: Run the canonical non-mutating verification gate.
    aliases: [verify]
    cmds:
      - task: fmt-check
      - task: lint
      - task: test
      - task: build
`

const tsToolCITemplate = `name: CI

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
  node:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      - uses: actions/setup-node@39370e3970a6d050c480ffad4ff0ed4d3fdee5af # v4.1.0
        with:
          node-version: "22"
          cache: npm
      - run: npm ci
      - run: npm run lint
      - run: npm run test
      - run: npm run build
`

const tsToolReleaseTemplate = `name: Release

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
      - uses: actions/setup-node@39370e3970a6d050c480ffad4ff0ed4d3fdee5af # v4.1.0
        with:
          node-version: "22"
          registry-url: "https://registry.npmjs.org"
      - run: npm ci
      - run: npm run test
      - run: npm run build
      - run: npm publish
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
`

const tsToolReadmeTemplate = `# [[.Product.Name]]

[[.Product.Description]]

## Install

~~~bash
npm install -g [[.Product.Name]]
~~~

## Usage

~~~bash
[[.Product.Name]] world
[[.Product.Name]] world --json
~~~

## Development

~~~bash
npm install
task check
~~~

See [AGENTS.md](AGENTS.md) for the agent and contributor contract,
[CONTRIBUTING.md](CONTRIBUTING.md) for the contribution workflow, and
[SECURITY.md](SECURITY.md) for security reporting instructions.

## License

MIT — see [LICENSE](LICENSE).
`

const tsToolAgentsTemplate = `# AGENTS.md

This file is the source of truth for agents and contributors working on [[.Product.Name]].

## Product

[[.Product.Description]]

## Architecture

- ` + "`src/lib.ts`" + ` owns the library core and its pure ` + "`greet`" + ` function.
- ` + "`src/cli.ts`" + ` is the thin command-line entry point over ` + "`process.argv`" + `.
- ` + "`tests/cli.test.ts`" + ` covers the pure function through Vitest.
- ` + "`package.json`" + ` is human-owned: adding a dependency is a normal edit.

## Commands

~~~bash
npm install
npm run lint
npm run test
npm run build
~~~

## Invariants

1. Keep ` + "`src/cli.ts`" + ` thin; put behavior in ` + "`src/lib.ts`" + `.
2. Add or update tests for every behavior change.
3. Never commit credentials, private data, or local environment files.
`

const tsToolChangelogTemplate = `# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial public repository foundation.
`

const tsToolContributingTemplate = `# Contributing

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

const tsToolCodeOfConductTemplate = `# Code of Conduct

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

const tsToolSecurityTemplate = `# Security Policy

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

const tsToolLicenseTemplate = `MIT License

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

const tsToolDocsIndexTemplate = `# [[.Product.Name]]

[[.Product.Description]]

## Start here

~~~bash
npm install
npm run build
node dist/cli.js world
~~~

## Development

Run ` + "`task check`" + ` before opening a pull request. See [AGENTS.md](../AGENTS.md)
for the agent contract.
`
