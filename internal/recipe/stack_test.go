package recipe

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

// shaPinnedUses matches a third-party action reference pinned to a full
// 40-hex commit SHA with a trailing version comment, the pinning convention
// of Bob's own workflows and the go-agent-tool recipe.
var shaPinnedUses = regexp.MustCompile(`^\s*- uses: [\w./-]+@[0-9a-f]{40} # v\S+$`)

func TestStackCIStubsPinEveryActionToACommitSHA(t *testing.T) {
	t.Parallel()
	for _, id := range manifest.StackRecipeIDs() {
		m, err := manifest.DefaultStack(id, "demo", "", "A demo repository.", "")
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		artifacts, err := Render(m)
		if err != nil {
			t.Fatalf("%s: render: %v", id, err)
		}
		var workflow string
		for _, artifact := range artifacts {
			if artifact.Path == ".github/workflows/ci.yml" {
				workflow = string(artifact.Content)
			}
		}
		if workflow == "" {
			t.Fatalf("%s: no CI workflow rendered", id)
		}
		for _, line := range strings.Split(workflow, "\n") {
			if !strings.Contains(line, "uses:") {
				continue
			}
			if !shaPinnedUses.MatchString(line) {
				t.Fatalf("%s: CI stub action is not pinned to a commit SHA: %q", id, strings.TrimSpace(line))
			}
		}
	}
}

func TestStackDefinitionsStayInSyncWithManifestSchema(t *testing.T) {
	t.Parallel()
	schemaIDs := manifest.StackRecipeIDs()
	if len(schemaIDs) == 0 {
		t.Fatal("manifest declares no stack recipes")
	}
	for _, id := range schemaIDs {
		definition, ok := stackDefinitions[id]
		if !ok {
			t.Fatalf("manifest stack recipe %q has no renderer definition", id)
		}
		if definition.ID != id || definition.Description == "" || definition.LanguageLabel == "" {
			t.Fatalf("stack definition %q is incomplete: %#v", id, definition)
		}
		if len(definition.Stacks) == 0 || definition.Gitignore == "" || definition.CIWorkflow == "" {
			t.Fatalf("stack definition %q is missing stacks or content: %#v", id, definition)
		}
		version, err := Version(id)
		if err != nil || version != StackRecipeVersion {
			t.Fatalf("Version(%q) = %d, %v", id, version, err)
		}
	}
	if len(stackDefinitions) != len(schemaIDs) {
		t.Fatalf("renderer declares %d stack recipes, manifest declares %d", len(stackDefinitions), len(schemaIDs))
	}
	ids := IDs()
	for _, id := range schemaIDs {
		if !containsID(ids, id) {
			t.Fatalf("recipe.IDs() is missing %q: %v", id, ids)
		}
	}
}

func TestForStackMapsEveryDetectedStackToOneRecipe(t *testing.T) {
	t.Parallel()
	wantByStack := map[string]string{
		"go":         manifest.RecipeGoHygiene,
		"typescript": manifest.RecipeTSApp,
		"javascript": manifest.RecipeJSApp,
		"vue":        manifest.RecipeVueApp,
		"nuxt":       manifest.RecipeNuxtApp,
		"python":     manifest.RecipePythonApp,
		"ruby":       manifest.RecipeRubyApp,
		"lua":        manifest.RecipeLuaLib,
		"rust":       manifest.RecipeRustCLI,
		"swift":      manifest.RecipeSwiftPkg,
		"elixir":     manifest.RecipeElixirApp,
		"static-web": manifest.RecipeStaticWeb,
	}
	for stack, wantRecipe := range wantByStack {
		id, ok := ForStack(stack)
		if !ok || id != wantRecipe {
			t.Fatalf("ForStack(%q) = %q, %t; want %q", stack, id, ok, wantRecipe)
		}
	}
	if _, ok := ForStack("cobol"); ok {
		t.Fatal("ForStack should not match an unknown stack")
	}
	if stacks := Stacks("go-agent-tool"); !reflect.DeepEqual(stacks, []string{"go"}) {
		t.Fatalf("Stacks(go-agent-tool) = %v", stacks)
	}
	if stacks := Stacks(manifest.RecipeGoHygiene); !reflect.DeepEqual(stacks, []string{"go"}) {
		t.Fatalf("Stacks(go-hygiene) = %v", stacks)
	}
	if stacks := Stacks("files"); len(stacks) != 0 {
		t.Fatalf("files recipe must claim no stacks, got %v", stacks)
	}
}

func TestRenderStackProducesDeterministicSeedOnlyArtifacts(t *testing.T) {
	t.Parallel()
	for _, id := range manifest.StackRecipeIDs() {
		m, err := manifest.DefaultStack(id, "demo", "", "A demo repository.", "")
		if err != nil {
			t.Fatalf("%s: default manifest: %v", id, err)
		}
		first, err := Render(m)
		if err != nil {
			t.Fatalf("%s: render: %v", id, err)
		}
		second, err := Render(m)
		if err != nil {
			t.Fatalf("%s: second render: %v", id, err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("%s: render is not deterministic", id)
		}
		wantPaths := expectedStackPaths(id)
		if got := artifactPathsOf(first); !reflect.DeepEqual(got, wantPaths) {
			t.Fatalf("%s: paths = %v, want %v", id, got, wantPaths)
		}
		for _, artifact := range first {
			if !artifact.Seed {
				t.Fatalf("%s: artifact %q must be seed-once", id, artifact.Path)
			}
			if artifact.Mode != 0o644 {
				t.Fatalf("%s: artifact %q mode = %v", id, artifact.Path, artifact.Mode)
			}
			if len(artifact.Content) == 0 {
				t.Fatalf("%s: artifact %q has no content", id, artifact.Path)
			}
		}
	}
}

func TestRenderStackHonorsGitHubActionsToggle(t *testing.T) {
	t.Parallel()
	m, err := manifest.DefaultStack(manifest.RecipeTSApp, "demo", "", "A demo repository.", "")
	if err != nil {
		t.Fatal(err)
	}
	m.Distribution.GitHubActions = false
	artifacts, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".editorconfig", ".gitignore", ".prettierrc", "AGENTS.md", "README.md", "SECURITY.md", "tsconfig.json"}
	if got := artifactPathsOf(artifacts); !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

func TestRenderStackSubstitutesProductIdentity(t *testing.T) {
	t.Parallel()
	m, err := manifest.DefaultStack(manifest.RecipeVueApp, "storefront", "", "A storefront built with Vue.", "")
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, artifact := range artifacts {
		byPath[artifact.Path] = string(artifact.Content)
	}
	readme := byPath["README.md"]
	if !strings.Contains(readme, "# storefront") || !strings.Contains(readme, "A storefront built with Vue.") {
		t.Fatalf("README missing product identity:\n%s", readme)
	}
	if !strings.Contains(readme, "vue-app@2") {
		t.Fatalf("README missing recipe identity:\n%s", readme)
	}
	ci := byPath[".github/workflows/ci.yml"]
	if !strings.Contains(ci, "name: CI") || !strings.Contains(ci, "Seeded once by Bob (vue-app@2)") {
		t.Fatalf("CI stub missing header:\n%s", ci)
	}
	if strings.Contains(readme, "[[") || strings.Contains(ci, "[[") {
		t.Fatal("unexpanded template markers survived rendering")
	}
}

func TestResolveMetadataForStackRecipes(t *testing.T) {
	t.Parallel()
	for _, id := range manifest.StackRecipeIDs() {
		m, err := manifest.DefaultStack(id, "demo", "", "A demo repository.", "")
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		metadata, err := ResolveMetadata(m)
		if err != nil {
			t.Fatalf("%s: resolve metadata: %v", id, err)
		}
		if metadata.Recipe.ID != id || metadata.Recipe.Version != StackRecipeVersion {
			t.Fatalf("%s: unexpected metadata identity %#v", id, metadata.Recipe)
		}
		for _, artifact := range metadata.Artifacts {
			if artifact.Ownership != "bob_seed_once" {
				t.Fatalf("%s: artifact %q ownership = %q", id, artifact.Path, artifact.Ownership)
			}
		}
		var seedInvariant bool
		for _, invariant := range metadata.Invariants {
			if invariant.ID == "seed.create_once" {
				seedInvariant = true
			}
		}
		if !seedInvariant {
			t.Fatalf("%s: metadata is missing the seed.create_once invariant", id)
		}
	}
}

func TestStackInfoForReportsCatalogMetadata(t *testing.T) {
	t.Parallel()
	info, ok := StackInfoFor(manifest.RecipeTSApp)
	if !ok || info.ID != manifest.RecipeTSApp || info.Description == "" {
		t.Fatalf("unexpected stack info: %#v ok=%t", info, ok)
	}
	if !reflect.DeepEqual(info.Stacks, []string{"typescript"}) {
		t.Fatalf("stacks = %v", info.Stacks)
	}
	if want := expectedStackPaths(manifest.RecipeTSApp); !reflect.DeepEqual(info.SeededPaths, want) {
		t.Fatalf("seeded paths = %v, want %v", info.SeededPaths, want)
	}
	if _, ok := StackInfoFor("go-agent-tool"); ok {
		t.Fatal("go-agent-tool is not a stack hygiene recipe")
	}
}

func TestRenderStackSeedsLanguageToolingContent(t *testing.T) {
	t.Parallel()
	// markers each stack's extra seed files must contain after rendering.
	markers := map[string]map[string][]string{
		manifest.RecipeTSApp: {
			"tsconfig.json": {`"moduleResolution": "bundler"`, `"strict": true`, `"target": "ESNext"`},
			".prettierrc":   {`"tabWidth": 2`, `"trailingComma": "all"`},
		},
		manifest.RecipeJSApp: {
			".prettierrc": {`"tabWidth": 2`, `"semi": true`},
		},
		manifest.RecipeVueApp: {
			".prettierrc": {`"vueIndentScriptAndStyle": true`},
		},
		manifest.RecipeNuxtApp: {
			".prettierrc": {`"vueIndentScriptAndStyle": true`},
		},
		manifest.RecipePythonApp: {
			"pyproject.toml":  {`name = "demo"`, `requires-python = ">=3.13"`, `target-version = "py313"`, "[tool.ruff]", "line-length = 88", "[tool.pytest.ini_options]"},
			".python-version": {"3.13"},
		},
		manifest.RecipeRubyApp: {
			".rubocop.yml":  {"AllCops:", "TargetRubyVersion: 3.3"},
			".ruby-version": {"3.3.0"},
			"Gemfile":       {`source "https://rubygems.org"`, `gem "rake"`},
		},
		manifest.RecipeLuaLib: {
			".luacheckrc":  {`std = "lua54"`},
			".lua-version": {"5.4"},
		},
		manifest.RecipeRustCLI: {
			"clippy.toml":         {"msrv", "cognitive-complexity-threshold"},
			"rust-toolchain.toml": {`channel = "stable"`, `"clippy"`, `"rustfmt"`},
		},
		manifest.RecipeSwiftPkg: {},
		manifest.RecipeElixirApp: {
			".formatter.exs": {`inputs: ["mix.exs"`, `lib,test`},
		},
		manifest.RecipeStaticWeb: {
			".htmlhintrc": {`"doctype-first": true`, `"tag-pair": true`},
		},
		manifest.RecipeGoHygiene: {
			".golangci.yml": {"linters:", "gofmt", "govet", "errcheck", "staticcheck"},
		},
	}
	// .editorconfig indent follows the language convention: tabs for Go, four
	// spaces for Python/Rust/Swift, two for every other stack.
	fourSpace := map[string]bool{manifest.RecipePythonApp: true, manifest.RecipeRustCLI: true, manifest.RecipeSwiftPkg: true}
	tabIndent := map[string]bool{manifest.RecipeGoHygiene: true}
	for _, id := range manifest.StackRecipeIDs() {
		m, err := manifest.DefaultStack(id, "demo", "", "A demo repository.", "")
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		artifacts, err := Render(m)
		if err != nil {
			t.Fatalf("%s: render: %v", id, err)
		}
		byPath := map[string]string{}
		for _, artifact := range artifacts {
			if !artifact.Seed {
				t.Fatalf("%s: artifact %q must be seed-once", id, artifact.Path)
			}
			content := string(artifact.Content)
			if strings.Contains(content, "[[") {
				t.Fatalf("%s: artifact %q has unexpanded template markers:\n%s", id, artifact.Path, content)
			}
			byPath[artifact.Path] = content
		}
		editorconfig, ok := byPath[".editorconfig"]
		if !ok {
			t.Fatalf("%s: missing .editorconfig", id)
		}
		if !strings.Contains(editorconfig, "root = true") || !strings.Contains(editorconfig, "charset = utf-8") {
			t.Fatalf("%s: .editorconfig missing defaults:\n%s", id, editorconfig)
		}
		if tabIndent[id] {
			if !strings.Contains(editorconfig, "indent_style = tab") {
				t.Fatalf("%s: .editorconfig missing tabs:\n%s", id, editorconfig)
			}
		} else {
			wantIndent := "indent_size = 2"
			if fourSpace[id] {
				wantIndent = "indent_size = 4"
			}
			if !strings.Contains(editorconfig, wantIndent) {
				t.Fatalf("%s: .editorconfig missing %q:\n%s", id, wantIndent, editorconfig)
			}
		}
		for path, wants := range markers[id] {
			content, ok := byPath[path]
			if !ok {
				t.Fatalf("%s: missing expected seed %q", id, path)
			}
			for _, want := range wants {
				if !strings.Contains(content, want) {
					t.Fatalf("%s: %s missing %q:\n%s", id, path, want, content)
				}
			}
		}
	}
}

func artifactPathsOf(artifacts []Artifact) []string {
	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		paths = append(paths, artifact.Path)
	}
	return paths
}

// stackExtraSeeds mirrors the per-stack ExtraSeeds declared in stack.go so the
// renderer tests assert against an independent expectation rather than the
// table under test.
var stackExtraSeeds = map[string][]string{
	manifest.RecipeTSApp:     {".prettierrc", "tsconfig.json"},
	manifest.RecipeJSApp:     {".prettierrc"},
	manifest.RecipeVueApp:    {".prettierrc"},
	manifest.RecipeNuxtApp:   {".prettierrc"},
	manifest.RecipePythonApp: {".python-version", "pyproject.toml"},
	manifest.RecipeRubyApp:   {".rubocop.yml", ".ruby-version", "Gemfile"},
	manifest.RecipeLuaLib:    {".lua-version", ".luacheckrc"},
	manifest.RecipeRustCLI:   {"clippy.toml", "rust-toolchain.toml"},
	manifest.RecipeSwiftPkg:  {},
	manifest.RecipeElixirApp: {".formatter.exs"},
	manifest.RecipeStaticWeb: {".htmlhintrc"},
	manifest.RecipeGoHygiene: {".golangci.yml"},
}

func TestStackVersionOneBytesRemainImmutable(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		manifest.RecipeTSApp:     "764b2ba20061570fddf446779175216bf83d009b9aaed206525a165616204746",
		manifest.RecipeJSApp:     "06317c2f5a63eb269b268569c447cbaac407ffa941e46d89e1430750a1849f38",
		manifest.RecipeVueApp:    "8c9ddc2d86c06360d2148a35f9781fd97a21b7b2b7849b2a37de6554bf81f8a3",
		manifest.RecipePythonApp: "1760648231b1cfd1f9e2ebf7f9a5898702d21d1d678e980c6652ff539fd1e327",
		manifest.RecipeRubyApp:   "9d2f50d5b5613bc756692cd8d328a6a6da4fb022a503abe8c7965b121dbceab3",
		manifest.RecipeLuaLib:    "9f8dd5f4eed41859a809835db0de4722e44dec67e0905e0d6ce481024e1cd417",
		manifest.RecipeRustCLI:   "849d28eb22215e8eb0980346b539641b3422082e260142786dd42a9da0522c16",
		manifest.RecipeStaticWeb: "9754db8efde8e30fd1893db27c1007318646f52f4a76e20a574517fde50f8d08",
	}
	for id, wantDigest := range want {
		m, err := manifest.DefaultStack(id, "demo", "", "A local-first, agent-ready repository.", "")
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		artifacts, err := RenderVersion(m, 1)
		if err != nil {
			t.Fatalf("%s@1: %v", id, err)
		}
		h := sha256.New()
		for _, artifact := range artifacts {
			sum := sha256.Sum256(artifact.Content)
			_, _ = fmt.Fprintf(h, "%x  ./%s\n", sum, artifact.Path)
		}
		got := fmt.Sprintf("%x", h.Sum(nil))
		if got != wantDigest {
			t.Fatalf("%s@1 digest = %s, want %s", id, got, wantDigest)
		}
	}
	for _, id := range []string{manifest.RecipeSwiftPkg, manifest.RecipeElixirApp} {
		m, err := manifest.DefaultStack(id, "demo", "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := RenderVersion(m, 1); err == nil {
			t.Fatalf("%s must not claim a version-1 contract", id)
		}
	}
}

func TestNodeStackVersionTwoUsesDeclaredPackageManager(t *testing.T) {
	t.Parallel()
	tests := []struct {
		manager string
		want    []string
	}{
		{"bun", []string{"setup-bun@", "bun install --frozen-lockfile", "bun run test"}},
		{"npm", []string{"setup-node@", "npm ci", "npm run test"}},
		{"pnpm", []string{"setup-node@", "corepack enable", "pnpm install --frozen-lockfile", "pnpm run test"}},
		{"yarn", []string{"setup-node@", "corepack enable", "yarn install --frozen-lockfile", "yarn test"}},
	}
	for _, test := range tests {
		m, err := manifest.DefaultStack(manifest.RecipeVueApp, "demo", "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		m.Runtime.PackageManager = test.manager
		artifacts, err := Render(m)
		if err != nil {
			t.Fatalf("%s: %v", test.manager, err)
		}
		var workflow string
		for _, artifact := range artifacts {
			if artifact.Path == ".github/workflows/ci.yml" {
				workflow = string(artifact.Content)
			}
		}
		for _, want := range test.want {
			if !strings.Contains(workflow, want) {
				t.Fatalf("%s workflow missing %q:\n%s", test.manager, want, workflow)
			}
		}
	}
}

func TestStackVersionTwoAlignsKindSpecificTooling(t *testing.T) {
	t.Parallel()
	renderByPath := func(t *testing.T, recipeID, kind string) map[string]string {
		t.Helper()
		m, err := manifest.DefaultStack(recipeID, "demo", "", "", kind)
		if err != nil {
			t.Fatal(err)
		}
		artifacts, err := Render(m)
		if err != nil {
			t.Fatal(err)
		}
		result := map[string]string{}
		for _, artifact := range artifacts {
			result[artifact.Path] = string(artifact.Content)
		}
		return result
	}

	plugin := renderByPath(t, manifest.RecipeLuaLib, "plugin")
	for path, want := range map[string]string{".lua-version": "5.1", ".luacheckrc": `std = "lua51"`, ".github/workflows/ci.yml": `luaVersion: "5.1"`} {
		if !strings.Contains(plugin[path], want) {
			t.Fatalf("Lua plugin %s missing %q:\n%s", path, want, plugin[path])
		}
	}

	rubyApp := renderByPath(t, manifest.RecipeRubyApp, "app")
	if strings.Contains(rubyApp["Gemfile"], "gemspec") || !strings.Contains(rubyApp["Gemfile"], `gem "rake"`) {
		t.Fatalf("Ruby app Gemfile is not app-safe:\n%s", rubyApp["Gemfile"])
	}
	rubyGem := renderByPath(t, manifest.RecipeRubyApp, "gem")
	if !strings.Contains(rubyGem["Gemfile"], "gemspec") {
		t.Fatalf("Ruby gem Gemfile must use gemspec:\n%s", rubyGem["Gemfile"])
	}

	rustLib := renderByPath(t, manifest.RecipeRustCLI, "lib")
	if strings.Contains(rustLib[".github/workflows/ci.yml"], "--locked") {
		t.Fatalf("Rust library CI must work before Cargo.lock exists:\n%s", rustLib[".github/workflows/ci.yml"])
	}
}

// expectedStackPaths returns the sorted paths a stack hygiene recipe renders
// when distribution.github_actions is selected (the DefaultStack default).
func expectedStackPaths(id string) []string {
	paths := []string{
		".editorconfig",
		".github/workflows/ci.yml",
		".gitignore",
		"AGENTS.md",
		"README.md",
		"SECURITY.md",
	}
	paths = append(paths, stackExtraSeeds[id]...)
	sort.Strings(paths)
	return paths
}

func containsID(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
