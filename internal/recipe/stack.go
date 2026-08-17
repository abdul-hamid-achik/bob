package recipe

import (
	"fmt"
	"sort"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

// StackRecipeVersion is the current contract version shared by every stack
// hygiene recipe. Version 2 adds Swift and Elixir, preserves detected
// JavaScript package managers, and aligns the seeded runtime commands. The
// published version-1 definitions remain renderable below.
const StackRecipeVersion = 2

// stackSeed is one seed-once file: a workspace-relative path and the Go
// template source rendered into its initial content.
type stackSeed struct {
	Path   string
	Source string
}

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
	// ExtraSeeds are additional stack-specific seed-once files rendered
	// alongside the universal hygiene set (README.md, AGENTS.md, SECURITY.md,
	// .gitignore, .editorconfig). Like every stack artifact, each is created
	// only when missing and never lock-owned, updated, or overwritten.
	ExtraSeeds []stackSeed
}

var stackDefinitionsV1 = map[string]stackDefinition{
	manifest.RecipeTSApp: {
		ID:            manifest.RecipeTSApp,
		Description:   "Seed-once hygiene for a TypeScript app or Bun/Turborepo monorepo: docs presence, .gitignore, and a CI stub; never owns application source",
		LanguageLabel: "TypeScript (Bun or Node; app or workspace monorepo)",
		Stacks:        []string{"typescript"},
		Commands:      []string{"bun install", "bun run lint", "bun run test", "bun run build"},
		Gitignore:     stackGitignoreNode,
		CIWorkflow:    stackCITypeScript,
		ExtraSeeds: []stackSeed{
			{Path: "tsconfig.json", Source: stackTSConfig},
			{Path: ".prettierrc", Source: stackPrettierRC},
		},
	},
	manifest.RecipeJSApp: {
		ID:            manifest.RecipeJSApp,
		Description:   "Seed-once hygiene for a plain JavaScript Node app or workspace: docs presence, .gitignore, and a CI stub; never owns application source",
		LanguageLabel: "JavaScript (Node)",
		Stacks:        []string{"javascript"},
		Commands:      []string{"npm install", "npm run lint --if-present", "npm test", "npm run build --if-present"},
		Gitignore:     stackGitignoreNode,
		CIWorkflow:    stackCIJavaScript,
		ExtraSeeds: []stackSeed{
			{Path: ".prettierrc", Source: stackPrettierRC},
		},
	},
	manifest.RecipeVueApp: {
		ID:            manifest.RecipeVueApp,
		Description:   "Seed-once hygiene for a Vue application: docs presence, .gitignore, and a Vite-oriented CI stub; never owns application source",
		LanguageLabel: "Vue (Vite)",
		Stacks:        []string{"vue"},
		Commands:      []string{"bun install", "bun run dev", "bun run test", "bun run build"},
		Gitignore:     stackGitignoreVue,
		CIWorkflow:    stackCIVue,
		ExtraSeeds: []stackSeed{
			{Path: ".prettierrc", Source: stackPrettierVueRC},
		},
	},
	manifest.RecipePythonApp: {
		ID:            manifest.RecipePythonApp,
		Description:   "Seed-once hygiene for a Python project: docs presence, .gitignore, and a pytest CI stub; never owns application source",
		LanguageLabel: "Python",
		Stacks:        []string{"python"},
		Commands:      []string{"python -m venv .venv && source .venv/bin/activate", `pip install -e ".[dev]"`, "pytest"},
		Gitignore:     stackGitignorePython,
		CIWorkflow:    stackCIPython,
		ExtraSeeds: []stackSeed{
			{Path: "pyproject.toml", Source: stackPyproject},
			{Path: ".python-version", Source: stackPythonVersion},
		},
	},
	manifest.RecipeRubyApp: {
		ID:            manifest.RecipeRubyApp,
		Description:   "Seed-once hygiene for a Ruby app or gem: docs presence, .gitignore, and a bundler/rake CI stub; never owns application source",
		LanguageLabel: "Ruby (application or gem)",
		Stacks:        []string{"ruby"},
		Commands:      []string{"bundle install", "bundle exec rake"},
		Gitignore:     stackGitignoreRuby,
		CIWorkflow:    stackCIRuby,
		ExtraSeeds: []stackSeed{
			{Path: ".rubocop.yml", Source: stackRubocop},
			{Path: ".ruby-version", Source: stackRubyVersion},
			{Path: "Gemfile", Source: stackGemfile},
		},
	},
	manifest.RecipeLuaLib: {
		ID:            manifest.RecipeLuaLib,
		Description:   "Seed-once hygiene for a Lua library or Neovim plugin: docs presence, .gitignore, and a busted CI stub; never owns application source",
		LanguageLabel: "Lua (library or Neovim plugin)",
		Stacks:        []string{"lua"},
		Commands:      []string{"luarocks install busted", "busted --verbose"},
		Gitignore:     stackGitignoreLua,
		CIWorkflow:    stackCILua,
		ExtraSeeds: []stackSeed{
			{Path: ".luacheckrc", Source: stackLuacheckRC},
			{Path: ".lua-version", Source: stackLuaVersion},
		},
	},
	manifest.RecipeRustCLI: {
		ID:            manifest.RecipeRustCLI,
		Description:   "Seed-once hygiene for a Rust CLI: docs presence, .gitignore, and a cargo CI stub; never owns application source",
		LanguageLabel: "Rust (CLI)",
		Stacks:        []string{"rust"},
		Commands:      []string{"cargo fmt --all --check", "cargo clippy --all-targets", "cargo test", "cargo build"},
		Gitignore:     stackGitignoreRust,
		CIWorkflow:    stackCIRust,
		ExtraSeeds: []stackSeed{
			{Path: "clippy.toml", Source: stackClippyConfig},
			{Path: "rust-toolchain.toml", Source: stackRustToolchain},
		},
	},
	manifest.RecipeStaticWeb: {
		ID:            manifest.RecipeStaticWeb,
		Description:   "Seed-once hygiene for a static web site: docs presence, .gitignore, and a validation CI stub; never owns site content",
		LanguageLabel: "Static web site (HTML/CSS)",
		Stacks:        []string{"static-web"},
		Commands:      []string{"open index.html"},
		Gitignore:     stackGitignoreStaticWeb,
		CIWorkflow:    stackCIStaticWeb,
		ExtraSeeds: []stackSeed{
			{Path: ".htmlhintrc", Source: stackHTMLHintRC},
		},
	},
}

var stackDefinitions = map[string]stackDefinition{
	manifest.RecipeTSApp: {
		ID:            manifest.RecipeTSApp,
		Description:   "Seed-once hygiene for a TypeScript app or workspace, using the declared package manager; never owns application source",
		LanguageLabel: "TypeScript (Bun or Node; app or workspace monorepo)",
		Stacks:        []string{"typescript"},
		Commands:      []string{"bun install", "bun run lint", "bun run test", "bun run build"},
		Gitignore:     stackGitignoreNode,
		CIWorkflow:    stackCITypeScript,
		ExtraSeeds: []stackSeed{
			{Path: "tsconfig.json", Source: stackTSConfig},
			{Path: ".prettierrc", Source: stackPrettierRC},
		},
	},
	manifest.RecipeJSApp: {
		ID:            manifest.RecipeJSApp,
		Description:   "Seed-once hygiene for a JavaScript Node app or workspace, using the declared package manager; never owns application source",
		LanguageLabel: "JavaScript (Node)",
		Stacks:        []string{"javascript"},
		Commands:      []string{"npm install", "npm run lint --if-present", "npm test", "npm run build --if-present"},
		Gitignore:     stackGitignoreNode,
		CIWorkflow:    stackCIJavaScript,
		ExtraSeeds: []stackSeed{
			{Path: ".prettierrc", Source: stackPrettierRC},
		},
	},
	manifest.RecipeVueApp: {
		ID:            manifest.RecipeVueApp,
		Description:   "Seed-once hygiene for a Vue application, preserving JavaScript or TypeScript and its package manager; never owns application source",
		LanguageLabel: "Vue (Vite)",
		Stacks:        []string{"vue"},
		Commands:      []string{"bun install", "bun run dev", "bun run test", "bun run build"},
		Gitignore:     stackGitignoreVue,
		CIWorkflow:    stackCIVue,
		ExtraSeeds: []stackSeed{
			{Path: ".prettierrc", Source: stackPrettierVueRC},
		},
	},
	manifest.RecipeNuxtApp: {
		ID:            manifest.RecipeNuxtApp,
		Description:   "Seed-once hygiene for a Nuxt application, preserving JavaScript or TypeScript and its package manager; never owns application source",
		LanguageLabel: "Nuxt (Vue)",
		Stacks:        []string{"nuxt"},
		Commands:      []string{"bun install", "bun run dev", "bun run test", "bun run build"},
		Gitignore:     stackGitignoreNuxt,
		CIWorkflow:    stackCINuxt,
		ExtraSeeds: []stackSeed{
			{Path: ".prettierrc", Source: stackPrettierVueRC},
		},
	},
	manifest.RecipePythonApp: {
		ID:            manifest.RecipePythonApp,
		Description:   "Seed-once hygiene for a Python project: docs, aligned Python tooling, and pytest CI; never owns application source",
		LanguageLabel: "Python 3.13",
		Stacks:        []string{"python"},
		Commands:      []string{"python3 -m venv .venv && source .venv/bin/activate", `python -m pip install -e ".[dev]"`, "python -m pytest"},
		Gitignore:     stackGitignorePython,
		CIWorkflow:    stackCIPythonV2,
		ExtraSeeds: []stackSeed{
			{Path: "pyproject.toml", Source: stackPyprojectV2},
			{Path: ".python-version", Source: stackPythonVersionV2},
		},
	},
	manifest.RecipeRubyApp: {
		ID:            manifest.RecipeRubyApp,
		Description:   "Seed-once hygiene for a Ruby app or gem with kind-aware Bundler defaults; never owns application source",
		LanguageLabel: "Ruby (application or gem)",
		Stacks:        []string{"ruby"},
		Commands:      []string{"bundle install", "bundle exec rake"},
		Gitignore:     stackGitignoreRuby,
		CIWorkflow:    stackCIRuby,
		ExtraSeeds: []stackSeed{
			{Path: ".rubocop.yml", Source: stackRubocop},
			{Path: ".ruby-version", Source: stackRubyVersion},
			{Path: "Gemfile", Source: stackGemfileV2},
		},
	},
	manifest.RecipeLuaLib: {
		ID:            manifest.RecipeLuaLib,
		Description:   "Seed-once hygiene for a Lua library or Neovim plugin with kind-aligned runtime configuration; never owns application source",
		LanguageLabel: "Lua (library or Neovim plugin)",
		Stacks:        []string{"lua"},
		Commands:      []string{"luarocks install busted", "busted --verbose"},
		Gitignore:     stackGitignoreLua,
		CIWorkflow:    stackCILuaV2,
		ExtraSeeds: []stackSeed{
			{Path: ".luacheckrc", Source: stackLuacheckRCV2},
			{Path: ".lua-version", Source: stackLuaVersionV2},
		},
	},
	manifest.RecipeRustCLI: {
		ID:            manifest.RecipeRustCLI,
		Description:   "Seed-once hygiene for a Rust CLI, library, or workspace: Cargo checks and CI; never owns application source",
		LanguageLabel: "Rust (CLI, library, or workspace)",
		Stacks:        []string{"rust"},
		Commands:      []string{"cargo fmt --all --check", "cargo clippy --all-targets -- -D warnings", "cargo test", "cargo build"},
		Gitignore:     stackGitignoreRust,
		CIWorkflow:    stackCIRustV2,
		ExtraSeeds: []stackSeed{
			{Path: "clippy.toml", Source: stackClippyConfig},
			{Path: "rust-toolchain.toml", Source: stackRustToolchain},
		},
	},
	manifest.RecipeSwiftPkg: {
		ID:            manifest.RecipeSwiftPkg,
		Description:   "Seed-once hygiene for a Swift package: SwiftPM commands, ignores, and CI; never owns package source",
		LanguageLabel: "Swift Package Manager",
		Stacks:        []string{"swift"},
		Commands:      []string{"swift build", "swift test"},
		Gitignore:     stackGitignoreSwift,
		CIWorkflow:    stackCISwift,
	},
	manifest.RecipeElixirApp: {
		ID:            manifest.RecipeElixirApp,
		Description:   "Seed-once hygiene for an Elixir application or umbrella: Mix formatting, tests, ignores, and CI; never owns application source",
		LanguageLabel: "Elixir (Mix application or umbrella)",
		Stacks:        []string{"elixir"},
		Commands:      []string{"mix deps.get", "mix format --check-formatted", "mix test"},
		Gitignore:     stackGitignoreElixir,
		CIWorkflow:    stackCIElixir,
		ExtraSeeds: []stackSeed{
			{Path: ".formatter.exs", Source: stackElixirFormatter},
		},
	},
	manifest.RecipeStaticWeb: {
		ID:            manifest.RecipeStaticWeb,
		Description:   "Seed-once hygiene for a static web site: docs presence, .gitignore, and a validation CI stub; never owns site content",
		LanguageLabel: "Static web site (HTML/CSS)",
		Stacks:        []string{"static-web"},
		Commands:      []string{"open index.html"},
		Gitignore:     stackGitignoreStaticWeb,
		CIWorkflow:    stackCIStaticWeb,
		ExtraSeeds: []stackSeed{
			{Path: ".htmlhintrc", Source: stackHTMLHintRC},
		},
	},
	manifest.RecipeGoHygiene: {
		ID:            manifest.RecipeGoHygiene,
		Description:   "Seed-once hygiene for an existing Go repository: docs, .gitignore, and golangci-lint CI; never owns application source",
		LanguageLabel: "Go (existing service, library, or application)",
		Stacks:        []string{"go"},
		Commands:      []string{"go mod tidy", "go fmt ./...", "go vet ./...", "go test ./..."},
		Gitignore:     stackGitignoreGo,
		CIWorkflow:    stackCIGo,
		ExtraSeeds: []stackSeed{
			{Path: ".golangci.yml", Source: stackGolangciConfig},
		},
	},
}

// recipeByStack maps a detected stack id to the recipe that serves it, and
// stacksByRecipe the reverse claim used by the init mismatch guard.
// go-agent-tool participates in stacksByRecipe even though it is not a stack
// hygiene recipe, but is not in recipeByStack: it must be explicitly selected
// rather than auto-detected.
var (
	recipeByStack  = map[string]string{}
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
		SeededPaths:   stackSeededPaths(definition),
	}, true
}

// stackSeededPaths returns the sorted set of paths a stack hygiene recipe can
// seed: the universal hygiene set plus the stack-specific ExtraSeeds and the
// CI workflow (seeded only when distribution.github_actions is selected).
func stackSeededPaths(definition stackDefinition) []string {
	paths := []string{
		".editorconfig",
		".github/workflows/ci.yml",
		".gitignore",
		"AGENTS.md",
		"README.md",
		"SECURITY.md",
	}
	for _, seed := range definition.ExtraSeeds {
		paths = append(paths, seed.Path)
	}
	sort.Strings(paths)
	return paths
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
func renderStackVersion(m manifest.Manifest, version int) ([]Artifact, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("render %s: %w", m.Recipe, err)
	}
	definitions := stackDefinitions
	if version == 1 {
		definitions = stackDefinitionsV1
	}
	definition, ok := definitions[m.Recipe]
	if !ok {
		return nil, fmt.Errorf("render stack recipe: %s@%d is not available", m.Recipe, version)
	}
	if version == StackRecipeVersion {
		definition = stackDefinitionForManifest(definition, m)
	}
	data := stackTemplateData{
		Product:       m.Product,
		Manifest:      m,
		Language:      definition.LanguageLabel,
		Commands:      definition.Commands,
		RecipeID:      definition.ID,
		RecipeVersion: version,
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
	seeds := []stackSeed{
		{Path: "README.md", Source: stackReadmeTemplate},
		{Path: "AGENTS.md", Source: stackAgentsTemplate},
		{Path: "SECURITY.md", Source: stackSecurityTemplate},
		{Path: ".gitignore", Source: definition.Gitignore},
		{Path: ".editorconfig", Source: stackEditorConfig},
	}
	seeds = append(seeds, definition.ExtraSeeds...)
	if m.Distribution.GitHubActions {
		seeds = append(seeds, stackSeed{Path: ".github/workflows/ci.yml", Source: definition.CIWorkflow})
	}
	for _, seed := range seeds {
		if err := add(seed.Path, seed.Source); err != nil {
			return nil, err
		}
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

func stackDefinitionForManifest(definition stackDefinition, m manifest.Manifest) stackDefinition {
	if m.Recipe != manifest.RecipeTSApp && m.Recipe != manifest.RecipeJSApp && m.Recipe != manifest.RecipeVueApp && m.Recipe != manifest.RecipeNuxtApp {
		return definition
	}
	manager := m.Runtime.PackageManager
	if manager == "" {
		if m.Recipe == manifest.RecipeJSApp {
			manager = "npm"
		} else {
			manager = "bun"
		}
	}
	definition.Commands = nodeCommands(manager, m.Recipe)
	definition.CIWorkflow = nodeCIWorkflow(manager, m.Recipe)
	return definition
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

const stackGitignoreNuxt = `node_modules/
.nuxt/
.output/
dist/
coverage/
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

const stackGitignoreSwift = `/.build/
/.swiftpm/xcode/package.xcworkspace/
DerivedData/
.DS_Store
.env
.env.*
!.env.example
`

const stackGitignoreElixir = `/_build/
/deps/
/cover/
/doc/
/.elixir_ls/
erl_crash.dump
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

const stackCINuxt = stackCIHeader + `      - uses: oven-sh/setup-bun@0c5077e51419868618aeaa5fe8019c62421857d6 # v2.2.0
      - run: bun install --frozen-lockfile
      # TODO: align these with your package.json scripts (nuxt build, vitest...).
      - run: bun run test
      - run: bun run build
`

func nodeCommands(manager, recipeID string) []string {
	install := manager + " install"
	run := manager + " run"
	if manager == "yarn" {
		run = "yarn"
	}
	switch recipeID {
	case manifest.RecipeVueApp, manifest.RecipeNuxtApp:
		return []string{install, run + " dev", run + " test", run + " build"}
	case manifest.RecipeJSApp:
		return []string{install, run + " test"}
	default:
		return []string{install, run + " lint", run + " test", run + " build"}
	}
}

func nodeCIWorkflow(manager, recipeID string) string {
	setup := `      - uses: actions/setup-node@48b55a011bda9f5d6aeb4c2d9c7362e8dae4041e # v6
        with:
          node-version: 24
`
	install := "npm ci"
	run := "npm run"
	switch manager {
	case "bun":
		setup = `      - uses: oven-sh/setup-bun@0c5077e51419868618aeaa5fe8019c62421857d6 # v2.2.0
`
		install = "bun install --frozen-lockfile"
		run = "bun run"
	case "pnpm":
		setup += "      - run: corepack enable\n"
		install = "pnpm install --frozen-lockfile"
		run = "pnpm run"
	case "yarn":
		setup += "      - run: corepack enable\n"
		install = "yarn install --frozen-lockfile"
		run = "yarn"
	}
	workflow := stackCIHeader + setup + "      - run: " + install + "\n"
	switch recipeID {
	case manifest.RecipeVueApp:
		workflow += "      # TODO: align these with your package.json scripts (vite build, vitest...).\n"
		workflow += "      - run: " + run + " test\n"
		workflow += "      - run: " + run + " build\n"
	case manifest.RecipeNuxtApp:
		workflow += "      # TODO: align these with your package.json scripts (nuxt build, vitest...).\n"
		workflow += "      - run: " + run + " test\n"
		workflow += "      - run: " + run + " build\n"
	case manifest.RecipeJSApp:
		workflow += "      # TODO: add lint and build commands when those scripts exist.\n"
		workflow += "      - run: " + run + " test\n"
	default:
		workflow += "      # TODO: align these with your package.json scripts.\n"
		workflow += "      - run: " + run + " lint\n"
		workflow += "      - run: " + run + " test\n"
		workflow += "      - run: " + run + " build\n"
	}
	return workflow
}

const stackCIPython = stackCIHeader + `      - uses: actions/setup-python@ece7cb06caefa5fff74198d8649806c4678c61a1 # v6.3.0
        with:
          python-version: "3.13"
      - run: python -m pip install --upgrade pip
      # TODO: install with your real tool (pip install -e ".[dev]", uv sync, poetry install).
      - run: pip install -e ".[dev]"
      - run: pytest
`

const stackCIPythonV2 = stackCIHeader + `      - uses: actions/setup-python@ece7cb06caefa5fff74198d8649806c4678c61a1 # v6.3.0
        with:
          python-version: "3.13"
      - run: python -m pip install --upgrade pip
      # TODO: align installation with the dependency tool already used here.
      - run: python -m pip install -e ".[dev]"
      - run: python -m pytest
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

const stackCILuaV2 = stackCIHeader + `      - uses: leafo/gh-actions-lua@8aace3457a2fcf3f3c4e9007ecc6b869ff6d74d6 # v11.0.0
        with:
          luaVersion: "[[if eq .Manifest.Runtime.Kind "plugin"]]5.1[[else]]5.4[[end]]"
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

const stackCIRustV2 = stackCIHeader + `      # rust-toolchain.toml selects the stable toolchain and required components.
      - run: cargo fmt --all --check
      - run: cargo clippy --all-targets -- -D warnings
      - run: cargo test
      - run: cargo build
`

const stackCISwift = stackCIHeader + `      - uses: swift-actions/setup-swift@7ca6abe6b3b0e8b5421b88be48feee39cbf52c6a # v2
        with:
          swift-version: "6.2"
      - run: swift build
      - run: swift test
`

const stackCIElixir = stackCIHeader + `      - uses: erlef/setup-beam@54075bcc5e249e4758d363f27d099f55d843f124 # v1
        with:
          otp-version: "27.3"
          elixir-version: "1.18.4"
      - run: mix deps.get
      - run: mix format --check-formatted
      - run: mix test
`

const stackCIStaticWeb = stackCIHeader + `      # TODO: replace with your real validation or build (vite build,
      # html-validate, linkinator, sass --no-source-map...).
      - run: test -f index.html
`

// stackEditorConfig is seeded for every stack. The indent follows the language
// convention: tabs for Go, four spaces for Python/Rust/Swift, two for the rest.
const stackEditorConfig = `# Seeded once by Bob ([[.RecipeID]]@[[.RecipeVersion]]). Yours to own and extend;
# Bob never updates or overwrites it.
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
[[if eq .RecipeID "go-hygiene"]]indent_style = tab
indent_size = 4
[[else]]indent_style = space
[[if or (eq .RecipeID "python-app") (eq .RecipeID "rust-cli") (eq .RecipeID "swift-package")]]indent_size = 4
[[else]]indent_size = 2
[[end]][[end]]
[*.md]
trim_trailing_whitespace = false
`

const stackTSConfig = `{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "noEmit": true
  },
  "include": ["src"]
}
`

const stackPrettierRC = `{
  "semi": true,
  "singleQuote": false,
  "trailingComma": "all",
  "printWidth": 100,
  "tabWidth": 2
}
`

const stackPrettierVueRC = `{
  "semi": true,
  "singleQuote": false,
  "trailingComma": "all",
  "printWidth": 100,
  "tabWidth": 2,
  "vueIndentScriptAndStyle": true
}
`

const stackPyproject = `[project]
name = [[quote .Product.Name]]
version = "0.1.0"
description = [[quote .Product.Description]]
requires-python = ">=3.11"

[tool.ruff]
line-length = 88
target-version = "py311"

[tool.ruff.lint]
select = ["E", "F", "I", "W", "UP", "B"]

[tool.pytest.ini_options]
testpaths = ["tests"]
addopts = "-ra"
`

const stackPythonVersion = `3.12
`

const stackPyprojectV2 = `[project]
name = [[quote .Product.Name]]
version = "0.1.0"
description = [[quote .Product.Description]]
requires-python = ">=3.13"

[tool.ruff]
line-length = 88
target-version = "py313"

[tool.ruff.lint]
select = ["E", "F", "I", "W", "UP", "B"]

[tool.pytest.ini_options]
testpaths = ["tests"]
addopts = "-ra"
`

const stackPythonVersionV2 = `3.13
`

const stackRubocop = `AllCops:
  TargetRubyVersion: 3.3
  NewCops: enable
  SuggestExtensions: false

Style/StringLiterals:
  EnforcedStyle: double_quotes

Style/FrozenStringLiteralComment:
  Enabled: false

Metrics/MethodLength:
  Max: 20
`

const stackRubyVersion = `3.3.0
`

const stackGemfile = `source "https://rubygems.org"

# For a gem, declare dependencies in the gemspec. For an app, add gems below.
gemspec
`

const stackGemfileV2 = `source "https://rubygems.org"

[[if eq .Manifest.Runtime.Kind "gem"]]# Gem dependencies belong in the gemspec.
gemspec
[[else]]# Add application dependencies below.
gem "rake"
[[end]]`

const stackLuacheckRC = `std = "lua51"
globals = { "vim" }
max_line_length = 120
`

const stackLuaVersion = `5.1
`

const stackLuacheckRCV2 = `std = "[[if eq .Manifest.Runtime.Kind "plugin"]]lua51[[else]]lua54[[end]]"
[[if eq .Manifest.Runtime.Kind "plugin"]]globals = { "vim" }
[[end]]max_line_length = 120
`

const stackLuaVersionV2 = `[[if eq .Manifest.Runtime.Kind "plugin"]]5.1[[else]]5.4[[end]]
`

const stackClippyConfig = `msrv = "1.74"
cognitive-complexity-threshold = 30
too-many-arguments-threshold = 8
`

const stackRustToolchain = `[toolchain]
channel = "stable"
components = ["clippy", "rustfmt"]
`

const stackHTMLHintRC = `{
  "tagname-lowercase": true,
  "attr-lowercase": true,
  "attr-value-double-quotes": true,
  "doctype-first": true,
  "tag-pair": true,
  "id-unique": true,
  "src-not-empty": true,
  "attr-no-duplication": true,
  "title-require": true
}
`

const stackElixirFormatter = `[
  inputs: ["mix.exs", "{config,lib,test}/**/*.{ex,exs}"]
]
`

const stackGitignoreGo = `# Go build artifacts
/bin/
/dist/
*.exe
*.test
*.out

# Go coverage and profiling
*.prof
coverage.txt
htmlcov/

# Dependency and vendor directories
/vendor/

# IDE and editor files
.DS_Store
.vscode/
.idea/

# Environment files
.env
.env.*
!.env.example
`

const stackCIGo = stackCIHeader + `      - uses: actions/setup-go@7c29491ab8ac28ff5c98225a6f92c0e42a1c5d93 # v6.0.0
        with:
          go-version: stable
      - run: go mod download
      # TODO: align these checks with your project's standards.
      - run: go fmt ./...
      - run: go vet ./...
      - run: go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
      - uses: golangci/golangci-lint-action@e6bba7d6934198d6e88b2d02e3316f2943a7f53f # v6.8.0
        with:
          version: latest
`

const stackGolangciConfig = `run:
  timeout: 5m
  go: "1.24"

linters:
  enable:
    - gofmt
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - typecheck

linters-settings:
  gofmt:
    simplify: true
  govet:
    enable-all: true

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
`
