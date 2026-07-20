package recipe

import (
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
		"go":         "go-agent-tool",
		"typescript": manifest.RecipeTSApp,
		"javascript": manifest.RecipeJSApp,
		"vue":        manifest.RecipeVueApp,
		"python":     manifest.RecipePythonApp,
		"ruby":       manifest.RecipeRubyApp,
		"lua":        manifest.RecipeLuaLib,
		"rust":       manifest.RecipeRustCLI,
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
	if !strings.Contains(readme, "vue-app@1") {
		t.Fatalf("README missing recipe identity:\n%s", readme)
	}
	ci := byPath[".github/workflows/ci.yml"]
	if !strings.Contains(ci, "name: CI") || !strings.Contains(ci, "Seeded once by Bob (vue-app@1)") {
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
		manifest.RecipePythonApp: {
			"pyproject.toml":  {`name = "demo"`, `requires-python = ">=3.11"`, "[tool.ruff]", "line-length = 88", "[tool.pytest.ini_options]"},
			".python-version": {"3.12"},
		},
		manifest.RecipeRubyApp: {
			".rubocop.yml":  {"AllCops:", "TargetRubyVersion: 3.3"},
			".ruby-version": {"3.3.0"},
			"Gemfile":       {`source "https://rubygems.org"`, "gemspec"},
		},
		manifest.RecipeLuaLib: {
			".luacheckrc":  {`std = "lua51"`, `"vim"`},
			".lua-version": {"5.1"},
		},
		manifest.RecipeRustCLI: {
			"clippy.toml":         {"msrv", "cognitive-complexity-threshold"},
			"rust-toolchain.toml": {`channel = "stable"`, `"clippy"`, `"rustfmt"`},
		},
		manifest.RecipeStaticWeb: {
			".htmlhintrc": {`"doctype-first": true`, `"tag-pair": true`},
		},
	}
	// .editorconfig indent width follows the language convention: four spaces
	// for Python and Rust, two for every other stack.
	fourSpace := map[string]bool{manifest.RecipePythonApp: true, manifest.RecipeRustCLI: true}
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
		wantIndent := "indent_size = 2"
		if fourSpace[id] {
			wantIndent = "indent_size = 4"
		}
		if !strings.Contains(editorconfig, wantIndent) {
			t.Fatalf("%s: .editorconfig missing %q:\n%s", id, wantIndent, editorconfig)
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
	manifest.RecipePythonApp: {".python-version", "pyproject.toml"},
	manifest.RecipeRubyApp:   {".rubocop.yml", ".ruby-version", "Gemfile"},
	manifest.RecipeLuaLib:    {".lua-version", ".luacheckrc"},
	manifest.RecipeRustCLI:   {"clippy.toml", "rust-toolchain.toml"},
	manifest.RecipeStaticWeb: {".htmlhintrc"},
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
