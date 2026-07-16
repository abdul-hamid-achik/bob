package recipe

import (
	"fmt"
	"sort"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

// StackRecipeVersion is the current contract version shared by every stack
// hygiene recipe. All stack recipes version together because they share one
// renderer; adding a stack adds a definition, not a version.
const StackRecipeVersion = 1

// stackDefinition is one data-driven stack hygiene recipe. Adding support for
// a new language stack means adding one entry here plus its runtime contract
// in manifest.stackRecipeRuntimes; the renderer, engine seed semantics,
// catalog listing, and MCP description all derive from this table.
type stackDefinition struct {
	ID            string
	Description   string
	LanguageLabel string
	// Stacks are the internal/detect stack ids this recipe serves.
	Stacks []string
	// Commands seed the development-commands section of README/AGENTS.
	Commands []string
	// Gitignore and CIWorkflow are seed-once file contents. CIWorkflow is
	// rendered only when distribution.github_actions is selected.
	Gitignore  string
	CIWorkflow string
}

var stackDefinitions = map[string]stackDefinition{
	manifest.RecipeTSApp: {
		ID:            manifest.RecipeTSApp,
		Description:   "Seed-once hygiene for a TypeScript app or Bun/Turborepo monorepo: docs presence, .gitignore, and a CI stub; never owns application source",
		LanguageLabel: "TypeScript (Bun or Node; app or workspace monorepo)",
		Stacks:        []string{"typescript"},
		Commands:      []string{"bun install", "bun run lint", "bun run test", "bun run build"},
		Gitignore:     stackGitignoreNode,
		CIWorkflow:    stackCITypeScript,
	},
	manifest.RecipeJSApp: {
		ID:            manifest.RecipeJSApp,
		Description:   "Seed-once hygiene for a plain JavaScript Node app or workspace: docs presence, .gitignore, and a CI stub; never owns application source",
		LanguageLabel: "JavaScript (Node)",
		Stacks:        []string{"javascript"},
		Commands:      []string{"npm install", "npm run lint --if-present", "npm test", "npm run build --if-present"},
		Gitignore:     stackGitignoreNode,
		CIWorkflow:    stackCIJavaScript,
	},
	manifest.RecipeVueApp: {
		ID:            manifest.RecipeVueApp,
		Description:   "Seed-once hygiene for a Vue application: docs presence, .gitignore, and a Vite-oriented CI stub; never owns application source",
		LanguageLabel: "Vue (Vite)",
		Stacks:        []string{"vue"},
		Commands:      []string{"bun install", "bun run dev", "bun run test", "bun run build"},
		Gitignore:     stackGitignoreVue,
		CIWorkflow:    stackCIVue,
	},
	manifest.RecipePythonApp: {
		ID:            manifest.RecipePythonApp,
		Description:   "Seed-once hygiene for a Python project: docs presence, .gitignore, and a pytest CI stub; never owns application source",
		LanguageLabel: "Python",
		Stacks:        []string{"python"},
		Commands:      []string{"python -m venv .venv && source .venv/bin/activate", `pip install -e ".[dev]"`, "pytest"},
		Gitignore:     stackGitignorePython,
		CIWorkflow:    stackCIPython,
	},
	manifest.RecipeRubyApp: {
		ID:            manifest.RecipeRubyApp,
		Description:   "Seed-once hygiene for a Ruby app or gem: docs presence, .gitignore, and a bundler/rake CI stub; never owns application source",
		LanguageLabel: "Ruby (application or gem)",
		Stacks:        []string{"ruby"},
		Commands:      []string{"bundle install", "bundle exec rake"},
		Gitignore:     stackGitignoreRuby,
		CIWorkflow:    stackCIRuby,
	},
	manifest.RecipeLuaLib: {
		ID:            manifest.RecipeLuaLib,
		Description:   "Seed-once hygiene for a Lua library or Neovim plugin: docs presence, .gitignore, and a busted CI stub; never owns application source",
		LanguageLabel: "Lua (library or Neovim plugin)",
		Stacks:        []string{"lua"},
		Commands:      []string{"luarocks install busted", "busted --verbose"},
		Gitignore:     stackGitignoreLua,
		CIWorkflow:    stackCILua,
	},
	manifest.RecipeRustCLI: {
		ID:            manifest.RecipeRustCLI,
		Description:   "Seed-once hygiene for a Rust CLI: docs presence, .gitignore, and a cargo CI stub; never owns application source",
		LanguageLabel: "Rust (CLI)",
		Stacks:        []string{"rust"},
		Commands:      []string{"cargo fmt --all --check", "cargo clippy --all-targets", "cargo test", "cargo build"},
		Gitignore:     stackGitignoreRust,
		CIWorkflow:    stackCIRust,
	},
	manifest.RecipeStaticWeb: {
		ID:            manifest.RecipeStaticWeb,
		Description:   "Seed-once hygiene for a static web site: docs presence, .gitignore, and a validation CI stub; never owns site content",
		LanguageLabel: "Static web site (HTML/CSS)",
		Stacks:        []string{"static-web"},
		Commands:      []string{"open index.html"},
		Gitignore:     stackGitignoreStaticWeb,
		CIWorkflow:    stackCIStaticWeb,
	},
}

// recipeByStack maps a detected stack id to the recipe that serves it, and
// stacksByRecipe the reverse claim used by the init mismatch guard.
// go-agent-tool participates even though it is not a stack hygiene recipe.
var (
	recipeByStack  = map[string]string{"go": "go-agent-tool"}
	stacksByRecipe = map[string][]string{"go-agent-tool": {"go"}}
)

func init() {
	for id, definition := range stackDefinitions {
		stacksByRecipe[id] = definition.Stacks
		for _, stack := range definition.Stacks {
			recipeByStack[stack] = id
		}
	}
}

// StackInfo is the public catalog projection of one stack hygiene recipe.
type StackInfo struct {
	ID            string   `json:"id"`
	Description   string   `json:"description"`
	LanguageLabel string   `json:"language_label"`
	Stacks        []string `json:"stacks"`
	SeededPaths   []string `json:"seeded_paths"`
}

// StackInfoFor reports catalog metadata for a stack hygiene recipe id.
func StackInfoFor(id string) (StackInfo, bool) {
	definition, ok := stackDefinitions[id]
	if !ok {
		return StackInfo{}, false
	}
	return StackInfo{
		ID:            definition.ID,
		Description:   definition.Description,
		LanguageLabel: definition.LanguageLabel,
		Stacks:        append([]string(nil), definition.Stacks...),
		SeededPaths:   []string{".github/workflows/ci.yml", ".gitignore", "AGENTS.md", "README.md", "SECURITY.md"},
	}, true
}

type stackTemplateData struct {
	Product       manifest.Product
	Manifest      manifest.Manifest
	Language      string
	Commands      []string
	RecipeID      string
	RecipeVersion int
}

// renderStack materializes the seed-once hygiene artifact set for one stack
// hygiene recipe. Every artifact carries Seed: the engine creates it only
// when missing, never lock-owns it, and never updates or overwrites it.
func renderStack(m manifest.Manifest) ([]Artifact, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("render %s: %w", m.Recipe, err)
	}
	definition, ok := stackDefinitions[m.Recipe]
	if !ok {
		return nil, fmt.Errorf("render stack recipe: no definition for %q", m.Recipe)
	}
	data := stackTemplateData{
		Product:       m.Product,
		Manifest:      m,
		Language:      definition.LanguageLabel,
		Commands:      definition.Commands,
		RecipeID:      definition.ID,
		RecipeVersion: StackRecipeVersion,
	}
	var artifacts []Artifact
	add := func(path, source string) error {
		content, err := executeRecipeTemplate(path, source, data)
		if err != nil {
			return err
		}
		artifacts = append(artifacts, Artifact{Path: path, Mode: 0o644, Seed: true, Content: content})
		return nil
	}
	seeds := []struct{ path, source string }{
		{"README.md", stackReadmeTemplate},
		{"AGENTS.md", stackAgentsTemplate},
		{"SECURITY.md", stackSecurityTemplate},
		{".gitignore", definition.Gitignore},
	}
	if m.Distribution.GitHubActions {
		seeds = append(seeds, struct{ path, source string }{".github/workflows/ci.yml", definition.CIWorkflow})
	}
	for _, seed := range seeds {
		if err := add(seed.path, seed.source); err != nil {
			return nil, err
		}
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

func resolveStackMetadata(m manifest.Manifest, artifacts []Artifact) Metadata {
	metadata := Metadata{
		SchemaVersion: MetadataSchemaVersion,
		Recipe:        MetadataRecipeRef{ID: m.Recipe, Version: StackRecipeVersion},
		Summary:       "Seed-once repository hygiene for a detected language stack: Bob creates missing hygiene files exactly once and never owns, updates, or overwrites them afterwards.",
		Capabilities: []CapabilityDefinition{
			capability("repository.seeded_hygiene", "repository", "required", "One-time seeding of missing repository hygiene files", []string{"recipe"}, ""),
			capability("distribution.github_actions", "distribution", selectedBool(m.Distribution.GitHubActions), "GitHub Actions CI stub seeding", []string{"distribution.github_actions"}, ""),
		},
		Invariants: []InvariantDefinition{
			{ID: "seed.create_once", Statement: "Seed artifacts are created only when missing; Bob never updates them, never overwrites them, and never records them in bob.lock."},
			{ID: "stack.source_not_owned", Statement: "Bob owns no application source for this stack; every rendered artifact is a one-time hygiene seed."},
			{ID: "repository.no_unmanaged_overwrite", Statement: "Bob never overwrites an unmanaged differing file."},
		},
		ExtensionPoints: []ExtensionPointDefinition{},
		Playbooks:       stackPlaybooks(),
	}
	for _, artifact := range artifacts {
		id := "seed:" + artifact.Path
		capabilities := []string{"repository.seeded_hygiene"}
		roles := []string{"public_hygiene"}
		if artifact.Path == ".github/workflows/ci.yml" {
			roles = []string{"ci_workflow"}
			capabilities = append(capabilities, "distribution.github_actions")
		}
		metadata.Artifacts = append(metadata.Artifacts, ArtifactDescriptor{
			ID: id, Path: artifact.Path, Roles: roles, Ownership: "bob_seed_once",
			CapabilityIDs: capabilities,
		})
		for i := range metadata.Capabilities {
			if metadata.Capabilities[i].ID == "repository.seeded_hygiene" ||
				(metadata.Capabilities[i].ID == "distribution.github_actions" && artifact.Path == ".github/workflows/ci.yml") {
				metadata.Capabilities[i].ArtifactIDs = append(metadata.Capabilities[i].ArtifactIDs, id)
			}
		}
	}
	return metadata
}

func stackPlaybooks() []PlaybookDefinition {
	definitions := []PlaybookDefinition{conflictPlaybook(), upgradePlaybook()}
	definitions[1].CapabilityIDs = []string{"repository.seeded_hygiene"}
	return definitions
}

const stackReadmeTemplate = `# [[.Product.Name]]

[[.Product.Description]]

> Seeded once by Bob ([[.RecipeID]]@[[.RecipeVersion]]). This file is yours to
> own and extend; Bob never updates or overwrites it.

## Development

~~~bash
[[range .Commands]][[.]]
[[end]]~~~

See [AGENTS.md](AGENTS.md) for the agent and contributor contract and
[SECURITY.md](SECURITY.md) for security reporting instructions.
`

const stackAgentsTemplate = `# AGENTS.md

This file is the source of truth for agents and contributors working on
[[.Product.Name]]. It was seeded once by Bob ([[.RecipeID]]@[[.RecipeVersion]])
and is human-owned from now on: replace every placeholder with the real
contract for this repository.

## Product

[[.Product.Description]]

## Stack

[[.Language]].

## Commands

~~~bash
[[range .Commands]][[.]]
[[end]]~~~

## Invariants

1. Add or update tests for every behavior change.
2. Keep unrelated changes out of a focused diff.
3. Never commit credentials, private data, or local environment files.
`

const stackSecurityTemplate = `# Security Policy

## Supported versions

Before the first tagged release, security fixes are made on the default
branch. After the first release, the latest release and the default branch
are supported.

## Reporting a vulnerability

This seeded policy cannot name a configured private reporting channel. Before
publishing this repository, replace this paragraph with an actually monitored
private contact.

Do not open a public issue for an unpatched vulnerability. Include the
affected version, impact, reproduction steps, and any suggested mitigation.
Do not include real credentials or unrelated personal data.
`

const stackGitignoreNode = `node_modules/
dist/
build/
coverage/
.turbo/
.next/
*.log
.DS_Store
.env
.env.*
!.env.example
`

const stackGitignoreVue = `node_modules/
dist/
coverage/
.vite/
.turbo/
*.log
.DS_Store
.env
.env.*
!.env.example
`

const stackGitignorePython = `__pycache__/
*.py[cod]
.venv/
venv/
dist/
build/
*.egg-info/
.pytest_cache/
.mypy_cache/
.ruff_cache/
.coverage
htmlcov/
.DS_Store
.env
.env.*
!.env.example
`

const stackGitignoreRuby = `/.bundle/
/vendor/bundle/
/log/
/tmp/
/coverage/
*.gem
.DS_Store
.env
.env.*
!.env.example
`

const stackGitignoreLua = `/lua_modules/
/.luarocks/
*.rock
luac.out
.DS_Store
.env
.env.*
!.env.example
`

const stackGitignoreRust = `/target/
.DS_Store
.env
.env.*
!.env.example
`

const stackGitignoreStaticWeb = `dist/
node_modules/
.DS_Store
.env
.env.*
!.env.example
`

// stackCIHeader is shared by every stack CI stub. Every third-party action is
// pinned to a commit SHA (the version comment is informational), matching the
// convention of Bob's own workflows and the go-agent-tool recipe; a test
// asserts no stub ever regresses to a mutable tag reference.
const stackCIHeader = `# Seeded once by Bob ([[.RecipeID]]@[[.RecipeVersion]]). This workflow is a
# starting point and is yours to own: adjust the scripts to match your
# repository. Actions are pinned to commit SHAs; keep them pinned when you
# upgrade.
name: CI

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
  checks:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
`

const stackCITypeScript = stackCIHeader + `      - uses: oven-sh/setup-bun@0c5077e51419868618aeaa5fe8019c62421857d6 # v2.2.0
      - run: bun install --frozen-lockfile
      # TODO: align these with your package.json scripts.
      - run: bun run lint
      - run: bun run test
      - run: bun run build
`

const stackCIJavaScript = stackCIHeader + `      - uses: actions/setup-node@48b55a011bda9f5d6aeb4c2d9c7362e8dae4041e # v6
        with:
          node-version: 24
      - run: npm ci
      # TODO: align these with your package.json scripts.
      - run: npm run lint --if-present
      - run: npm test
      - run: npm run build --if-present
`

const stackCIVue = stackCIHeader + `      - uses: oven-sh/setup-bun@0c5077e51419868618aeaa5fe8019c62421857d6 # v2.2.0
      - run: bun install --frozen-lockfile
      # TODO: align these with your package.json scripts (vite build, vitest...).
      - run: bun run test
      - run: bun run build
`

const stackCIPython = stackCIHeader + `      - uses: actions/setup-python@ece7cb06caefa5fff74198d8649806c4678c61a1 # v6.3.0
        with:
          python-version: "3.13"
      - run: python -m pip install --upgrade pip
      # TODO: install with your real tool (pip install -e ".[dev]", uv sync, poetry install).
      - run: pip install -e ".[dev]"
      - run: pytest
`

const stackCIRuby = stackCIHeader + `      - uses: ruby/setup-ruby@8e41b362d2589a22a44c1cfa214b3c83052c195b # v1.318.0
        with:
          bundler-cache: true
      # TODO: align with your Rakefile or test setup.
      - run: bundle exec rake
`

const stackCILua = stackCIHeader + `      - uses: leafo/gh-actions-lua@8aace3457a2fcf3f3c4e9007ecc6b869ff6d74d6 # v11.0.0
        with:
          luaVersion: "5.4"
      - uses: leafo/gh-actions-luarocks@4c082a5fad45388feaeb0798dbd82dbd7dc65bca # v5.0.0
      - run: luarocks install busted
      # TODO: align with your spec layout (busted spec/, luacheck, stylua...).
      - run: busted --verbose
`

const stackCIRust = stackCIHeader + `      # The hosted Ubuntu runner ships a stable Rust toolchain; pin one with
      # dtolnay/rust-toolchain if you need a specific version.
      - run: cargo fmt --all --check
      - run: cargo clippy --all-targets -- -D warnings
      - run: cargo test --locked
      - run: cargo build --locked
`

const stackCIStaticWeb = stackCIHeader + `      # TODO: replace with your real validation or build (vite build,
      # html-validate, linkinator, sass --no-source-map...).
      - run: test -f index.html
`
