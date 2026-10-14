package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadBoundedFollowsSymlinkButRejectsNonRegular(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if got, ok := readBounded(link, 64); !ok || got != "hello" {
		t.Fatalf("readBounded(symlink to regular file) = %q, %v; want %q, true", got, ok, "hello")
	}
	dirLink := filepath.Join(dir, "dirlink")
	if err := os.Symlink(dir, dirLink); err != nil {
		t.Fatal(err)
	}
	if _, ok := readBounded(dirLink, 64); ok {
		t.Fatal("readBounded(symlink to directory) reported ok; want false")
	}
}

// fixture builds a temporary repository whose files map paths to contents.
// Directories are created implicitly; a trailing slash declares an empty
// directory.
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if len(path) > 0 && path[len(path)-1] == '/' {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDetectPrimaryStacks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		files   map[string]string
		primary string
	}{
		{"go module", map[string]string{"go.mod": "module example.com/x\n"}, "go"},
		{"rust crate", map[string]string{"Cargo.toml": "[package]\nname = \"x\"\n"}, "rust"},
		{"swift package", map[string]string{"Package.swift": "// swift-tools-version: 6.0\n"}, "swift"},
		{"elixir app", map[string]string{"mix.exs": "defmodule Demo.MixProject do\nend\n"}, "elixir"},
		{"java maven", map[string]string{"pom.xml": "<project/>"}, "java"},
		{"java gradle", map[string]string{"build.gradle": "plugins {}\n"}, "java"},
		{"java gradle kotlin", map[string]string{"build.gradle.kts": "plugins {}\n"}, "java"},
		{"dotnet project", map[string]string{"App.csproj": "<Project/>"}, "dotnet"},
		{"dotnet solution", map[string]string{"App.sln": "Microsoft Visual Studio Solution File"}, "dotnet"},
		{"dotnet global.json", map[string]string{"global.json": `{"sdk": {}}`}, "dotnet"},
		{"php composer", map[string]string{"composer.json": "{}"}, "php"},
		{"deno json", map[string]string{"deno.json": "{}"}, "deno"},
		{"deno jsonc", map[string]string{"deno.jsonc": "{}"}, "deno"},
		{"kotlin gradle plugin", map[string]string{"build.gradle": "plugins { id 'org.jetbrains.kotlin.jvm' }\n"}, "kotlin"},
		{"cpp cmake", map[string]string{"CMakeLists.txt": "cmake_minimum_required(VERSION 3.20)\n"}, "cpp"},
		{"zig build", map[string]string{"build.zig": "pub fn build(b: *std.Build) void {}\n"}, "zig"},
		{"dart pubspec yaml", map[string]string{"pubspec.yaml": "name: demo\n"}, "dart"},
		{"dart pubspec yml", map[string]string{"pubspec.yml": "name: demo\n"}, "dart"},
		{"scala sbt", map[string]string{"build.sbt": "name := \"demo\"\n"}, "scala"},
		{"scala sbt properties", map[string]string{"project/build.properties": "sbt.version=1.9.0\n"}, "scala"},
		{"docker file", map[string]string{"Dockerfile": "FROM alpine\n"}, "docker"},
		{"docker compose yml", map[string]string{"docker-compose.yml": "services:\n"}, "docker"},
		{"docker compose yaml", map[string]string{"docker-compose.yaml": "services:\n"}, "docker"},
		{"compose yaml", map[string]string{"compose.yaml": "services:\n"}, "docker"},
		{"terraform root tf", map[string]string{"main.tf": "resource \"null_resource\" \"x\" {}\n"}, "terraform"},
		{"nix flake", map[string]string{"flake.nix": "{ outputs = _: {}; }\n"}, "nix"},
		{"nix shell", map[string]string{"shell.nix": "{ pkgs ? import <nixpkgs> {} }: pkgs.mkShell {}\n"}, "nix"},
		{"nix default", map[string]string{"default.nix": "{ pkgs ? import <nixpkgs> {} }: pkgs.hello\n"}, "nix"},
		{"ansible config", map[string]string{"ansible.cfg": "[defaults]\n"}, "ansible"},
		{"ansible playbook yml", map[string]string{"playbook.yml": "- hosts: all\n"}, "ansible"},
		{"ansible playbook yaml", map[string]string{"playbook.yaml": "- hosts: all\n"}, "ansible"},
		{"ansible roles directory", map[string]string{"roles/": ""}, "ansible"},
		{"helm chart", map[string]string{"Chart.yaml": "apiVersion: v2\n"}, "helm"},
		{"kubernetes kustomization yaml", map[string]string{"kustomization.yaml": "resources: []\n"}, "kubernetes"},
		{"kubernetes kustomization yml", map[string]string{"kustomization.yml": "resources: []\n"}, "kubernetes"},
		{"python pyproject", map[string]string{"pyproject.toml": "[project]\nname = \"x\"\n"}, "python"},
		{"python requirements", map[string]string{"requirements.txt": "requests\n"}, "python"},
		{"ruby gemfile", map[string]string{"Gemfile": "source \"https://rubygems.org\"\n"}, "ruby"},
		{"lua rockspec", map[string]string{"x-1.0-1.rockspec": "package = \"x\"\n"}, "lua"},
		{"neovim plugin", map[string]string{"lua/": "", "init.lua": "-- entry\n"}, "lua"},
		{"typescript app", map[string]string{"package.json": "{}", "tsconfig.json": "{}"}, "typescript"},
		{"javascript app", map[string]string{"package.json": `{"dependencies":{"express":"^4"}}`}, "javascript"},
		{"vue by dependency", map[string]string{"package.json": `{"dependencies":{"vue":"^3"}}`}, "vue"},
		{"vue by src files", map[string]string{"package.json": "{}", "src/App.vue": "<template/>"}, "vue"},
		{"nuxt by config ts", map[string]string{"package.json": "{}", "nuxt.config.ts": "export default {}"}, "nuxt"},
		{"nuxt by config js", map[string]string{"package.json": "{}", "nuxt.config.js": "module.exports = {}"}, "nuxt"},
		{"nuxt by dependency", map[string]string{"package.json": `{"dependencies":{"nuxt":"^3"}}`}, "nuxt"},
		{"static html only", map[string]string{"index.html": "<!doctype html>"}, "static-web"},
		{"static with css tooling", map[string]string{
			"index.html":   "<!doctype html>",
			"package.json": `{"devDependencies":{"sass":"^1","postcss":"^8"}}`,
		}, "static-web"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := Detect(fixture(t, test.files))
			if result.Primary != test.primary {
				t.Fatalf("primary = %q (stacks %#v), want %q", result.Primary, result.Stacks, test.primary)
			}
		})
	}
}

func TestDetectUnknownRepository(t *testing.T) {
	t.Parallel()
	result := Detect(fixture(t, map[string]string{"notes.txt": "hello"}))
	if result.Detected() || result.Primary != "" {
		t.Fatalf("expected unknown stack, got %#v", result)
	}
	if got := result.Describe(); got != "unknown (no recognized stack markers)" {
		t.Fatalf("unexpected describe: %q", got)
	}
	if missing := Detect(filepath.Join(t.TempDir(), "does-not-exist")); missing.Detected() {
		t.Fatalf("missing root should detect nothing, got %#v", missing)
	}
}

func TestDetectVueWinsOverGenericTypeScript(t *testing.T) {
	t.Parallel()
	result := Detect(fixture(t, map[string]string{
		"package.json":   `{"dependencies":{"vue":"^3"},"devDependencies":{"typescript":"^5"}}`,
		"tsconfig.json":  "{}",
		"vite.config.ts": "export default {}",
	}))
	if result.Primary != "vue" {
		t.Fatalf("primary = %q, want vue", result.Primary)
	}
	if !result.Has("typescript") {
		t.Fatalf("typescript should stay listed after vue: %#v", result.Stacks)
	}
	if !containsString(result.Signals, "vite") || !containsString(result.Signals, "tsconfig") {
		t.Fatalf("expected vite and tsconfig signals, got %v", result.Signals)
	}
}

func TestDetectNuxtWinsOverVueAndTypeScript(t *testing.T) {
	t.Parallel()
	result := Detect(fixture(t, map[string]string{
		"package.json":   `{"dependencies":{"nuxt":"^3","vue":"^3"},"devDependencies":{"typescript":"^5"}}`,
		"nuxt.config.ts": "export default defineNuxtConfig({})  ",
		"tsconfig.json":  "{}",
	}))
	if result.Primary != "nuxt" {
		t.Fatalf("primary = %q, want nuxt; stacks: %#v", result.Primary, result.Stacks)
	}
	if !result.Has("vue") {
		t.Fatalf("vue should stay listed after nuxt: %#v", result.Stacks)
	}
	if !result.Has("typescript") {
		t.Fatalf("typescript should stay listed: %#v", result.Stacks)
	}
	if !containsString(result.Signals, "tsconfig") {
		t.Fatalf("expected tsconfig signal, got %v", result.Signals)
	}
}

func TestDetectCompiledLanguageOutranksTooling(t *testing.T) {
	t.Parallel()
	result := Detect(fixture(t, map[string]string{
		"go.mod":       "module example.com/x\n",
		"package.json": "{}",
	}))
	if result.Primary != "go" {
		t.Fatalf("primary = %q, want go", result.Primary)
	}
}

func TestDetectBunTurborepoMonorepo(t *testing.T) {
	t.Parallel()
	result := Detect(fixture(t, map[string]string{
		"package.json":  `{"workspaces":["apps/*","packages/*"]}`,
		"tsconfig.json": "{}",
		"bun.lock":      "",
		"turbo.json":    "{}",
	}))
	if result.Primary != "typescript" || !result.Monorepo {
		t.Fatalf("expected typescript monorepo, got %#v", result)
	}
	if result.PackageManager != "bun" {
		t.Fatalf("package manager = %q, want bun", result.PackageManager)
	}
	if result.KindHint != "monorepo" {
		t.Fatalf("kind hint = %q, want monorepo", result.KindHint)
	}
	if !containsString(result.Signals, "turborepo") {
		t.Fatalf("expected turborepo signal, got %v", result.Signals)
	}
}

func TestDetectWorkspaceMarkers(t *testing.T) {
	t.Parallel()
	if result := Detect(fixture(t, map[string]string{"go.mod": "module x\n", "go.work": "go 1.26\n"})); !result.Monorepo {
		t.Fatalf("go.work should mark a workspace: %#v", result)
	}
	cargo := "[workspace]\nmembers = [\"a\", \"b\"]\n"
	if result := Detect(fixture(t, map[string]string{"Cargo.toml": cargo})); !result.Monorepo {
		t.Fatalf("Cargo workspace should mark a workspace: %#v", result)
	}
	pnpm := map[string]string{"package.json": "{}", "pnpm-workspace.yaml": "packages:\n  - apps/*\n", "pnpm-lock.yaml": ""}
	result := Detect(fixture(t, pnpm))
	if !result.Monorepo || result.PackageManager != "pnpm" {
		t.Fatalf("pnpm workspace should mark a workspace with pnpm: %#v", result)
	}
}

func TestDetectRustAndElixirKindHints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		files    map[string]string
		primary  string
		kind     string
		monorepo bool
	}{
		{"rust cli", map[string]string{"Cargo.toml": "[package]\nname = \"demo\"\n", "src/main.rs": "fn main() {}\n"}, "rust", "cli", false},
		{"rust lib section", map[string]string{"Cargo.toml": "[package]\nname = \"demo\"\n[lib]\n"}, "rust", "lib", false},
		{"rust lib layout", map[string]string{"Cargo.toml": "[package]\nname = \"demo\"\n", "src/lib.rs": "pub fn demo() {}\n"}, "rust", "lib", false},
		{"rust workspace", map[string]string{"Cargo.toml": "[workspace]\nmembers = []\n"}, "rust", "workspace", true},
		{"elixir app", map[string]string{"mix.exs": "def project, do: []\n"}, "elixir", "app", false},
		{"elixir umbrella", map[string]string{"mix.exs": "def project, do: [apps_path: \"apps\"]\n"}, "elixir", "umbrella", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := Detect(fixture(t, test.files))
			if result.Primary != test.primary || result.KindHint != test.kind || result.Monorepo != test.monorepo {
				t.Fatalf("unexpected detection: %#v", result)
			}
		})
	}
}

func TestDetectKindHintBelongsToPrimaryStack(t *testing.T) {
	t.Parallel()
	result := Detect(fixture(t, map[string]string{
		"Cargo.toml":   "[package]\nname = \"demo\"\n[lib]\n",
		"demo.gemspec": "Gem::Specification.new\n",
		"init.lua":     "-- plugin\n",
	}))
	if result.Primary != "rust" || result.KindHint != "lib" {
		t.Fatalf("secondary stack hints must not overwrite the primary hint: %#v", result)
	}
}

func TestDetectRubyGemAndNeovimPluginKindHints(t *testing.T) {
	t.Parallel()
	gem := Detect(fixture(t, map[string]string{"x.gemspec": "Gem::Specification.new\n", "Rakefile": ""}))
	if gem.Primary != "ruby" || gem.KindHint != "gem" {
		t.Fatalf("expected ruby gem hint, got %#v", gem)
	}
	app := Detect(fixture(t, map[string]string{"Gemfile": "", "Rakefile": ""}))
	if app.Primary != "ruby" || app.KindHint != "" {
		t.Fatalf("expected ruby app without kind hint, got %#v", app)
	}
	plugin := Detect(fixture(t, map[string]string{"lua/": "", ".luarc.json": "{}"}))
	if plugin.Primary != "lua" || plugin.KindHint != "plugin" {
		t.Fatalf("expected lua plugin hint, got %#v", plugin)
	}
	rock := Detect(fixture(t, map[string]string{"x-1.0-1.rockspec": "", "lua/": ""}))
	if rock.Primary != "lua" || rock.KindHint != "" {
		t.Fatalf("rockspec should suppress the plugin hint, got %#v", rock)
	}
}

func TestDetectStaticWebRequiresToolingOnlyPackage(t *testing.T) {
	t.Parallel()
	// A real dependency makes it a JavaScript app even with index.html.
	result := Detect(fixture(t, map[string]string{
		"index.html":   "<!doctype html>",
		"package.json": `{"dependencies":{"react":"^19"}}`,
	}))
	if result.Primary != "javascript" {
		t.Fatalf("primary = %q, want javascript", result.Primary)
	}
	// Sass in a styles directory is a signal, not a stack.
	static := Detect(fixture(t, map[string]string{
		"index.html":       "<!doctype html>",
		"styles/main.scss": "body {}",
	}))
	if static.Primary != "static-web" || !containsString(static.Signals, "sass") {
		t.Fatalf("expected static-web with sass signal, got %#v", static)
	}
}

func TestDetectDescribeNamesPrimaryMarkers(t *testing.T) {
	t.Parallel()
	result := Detect(fixture(t, map[string]string{
		"package.json":  "{}",
		"tsconfig.json": "{}",
		"bun.lock":      "",
		"turbo.json":    "{}",
	}))
	want := "typescript (bun.lock, package.json, tsconfig.json; monorepo)"
	if got := result.Describe(); got != want {
		t.Fatalf("describe = %q, want %q", got, want)
	}
}

func TestDetectKotlinVersusJava(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		files   map[string]string
		primary string
	}{
		{"plain gradle stays java", map[string]string{"build.gradle": "plugins { id 'java' }\n"}, "java"},
		{"plain gradle kts stays java", map[string]string{"build.gradle.kts": "plugins { java }\n"}, "java"},
		{"maven stays java", map[string]string{"pom.xml": "<project/>"}, "java"},
		{"kotlin source dir wins", map[string]string{"pom.xml": "<project/>", "src/main/kotlin/": ""}, "kotlin"},
		{"kotlin gradle plugin wins", map[string]string{"build.gradle": "plugins { id 'org.jetbrains.kotlin.jvm' }\n"}, "kotlin"},
		{"kotlin dsl plugin wins", map[string]string{"build.gradle.kts": "plugins { kotlin(\"jvm\") }\n"}, "kotlin"},
		{"kotlin pom dependency wins", map[string]string{"pom.xml": "<dependency><artifactId>kotlin-stdlib</artifactId></dependency>"}, "kotlin"},
		{"root kt source wins", map[string]string{"build.gradle": "plugins {}\n", "App.kt": "fun main() {}\n"}, "kotlin"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := Detect(fixture(t, test.files))
			if result.Primary != test.primary {
				t.Fatalf("primary = %q (stacks %#v), want %q", result.Primary, result.Stacks, test.primary)
			}
		})
	}
}

func TestDetectGoAgentToolVsGoHygiene(t *testing.T) {
	t.Parallel()
	// Plain go.mod without Bob markers should detect as "go" without kind hint
	plainGo := Detect(fixture(t, map[string]string{
		"go.mod": "module example.com/mylib\n",
	}))
	if plainGo.Primary != "go" || plainGo.KindHint != "" {
		t.Fatalf("plain Go repo: expected go without kind hint, got %#v", plainGo)
	}
	if plainGo.Module != "example.com/mylib" {
		t.Fatalf("plain Go repo module = %q, want example.com/mylib", plainGo.Module)
	}

	// Go repo with bob.yaml containing go-agent-tool should have agent-tool hint
	withBobYaml := Detect(fixture(t, map[string]string{
		"go.mod":   "module example.com/cli\n",
		"bob.yaml": "schema_version: 1\nrecipe: go-agent-tool\nproduct:\n  name: demo\n",
	}))
	if withBobYaml.Primary != "go" || withBobYaml.KindHint != "agent-tool" {
		t.Fatalf("go-agent-tool via bob.yaml: expected go with agent-tool hint, got %#v", withBobYaml)
	}

	// Go repo with Cobra CLI layout (cmd/ + internal/cli/root.go) and a
	// Cobra requirement should have agent-tool hint.
	withCobraLayout := Detect(fixture(t, map[string]string{
		"go.mod":               "module example.com/cli\n\nrequire (\n\tgithub.com/spf13/cobra v1.9.1\n)\n",
		"cmd/":                 "",
		"internal/":            "",
		"internal/cli/root.go": "package cli\n",
	}))
	if withCobraLayout.Primary != "go" || withCobraLayout.KindHint != "agent-tool" {
		t.Fatalf("Cobra CLI layout: expected go with agent-tool hint, got %#v", withCobraLayout)
	}

	// A single-line require directive also counts.
	singleRequire := Detect(fixture(t, map[string]string{
		"go.mod":               "module example.com/cli\n\nrequire github.com/spf13/cobra v1.9.1 // indirect\n",
		"cmd/":                 "",
		"internal/":            "",
		"internal/cli/root.go": "package cli\n",
	}))
	if singleRequire.KindHint != "agent-tool" {
		t.Fatalf("single-line cobra require: expected agent-tool hint, got %#v", singleRequire)
	}

	// The Cobra layout without a Cobra requirement is not an agent tool.
	layoutNoCobra := Detect(fixture(t, map[string]string{
		"go.mod":               "module example.com/cli\n\nrequire (\n\tgithub.com/urfave/cli/v2 v2.27.0\n)\n// github.com/spf13/cobra\n",
		"cmd/":                 "",
		"internal/":            "",
		"internal/cli/root.go": "package cli\n",
	}))
	if layoutNoCobra.Primary != "go" || layoutNoCobra.KindHint != "" {
		t.Fatalf("Cobra layout without cobra require: expected no agent-tool hint, got %#v", layoutNoCobra)
	}

	// A declared go-agent-tool recipe wins even without a Cobra requirement.
	declaredNoCobra := Detect(fixture(t, map[string]string{
		"go.mod":               "module example.com/cli\n",
		"bob.yaml":             "schema_version: 1\nrecipe: go-agent-tool\n",
		"cmd/":                 "",
		"internal/":            "",
		"internal/cli/root.go": "package cli\n",
	}))
	if declaredNoCobra.KindHint != "agent-tool" {
		t.Fatalf("declared go-agent-tool without cobra require: expected agent-tool hint, got %#v", declaredNoCobra)
	}

	// Go repo with cmd/ but no internal/cli should NOT have agent-tool hint
	justCmd := Detect(fixture(t, map[string]string{
		"go.mod": "module example.com/app\n",
		"cmd/":   "",
	}))
	if justCmd.Primary != "go" || justCmd.KindHint != "" {
		t.Fatalf("cmd/ without internal/cli: expected go without hint, got %#v", justCmd)
	}

	// A declared non-agent-tool recipe wins over the Cobra layout.
	cobraHygiene := Detect(fixture(t, map[string]string{
		"go.mod":               "module example.com/cli\n\nrequire github.com/spf13/cobra v1.9.1\n",
		"bob.yaml":             "schema_version: 1\nrecipe: go-hygiene\n",
		"cmd/":                 "",
		"internal/":            "",
		"internal/cli/root.go": "package cli\n",
	}))
	if cobraHygiene.Primary != "go" || cobraHygiene.KindHint != "" {
		t.Fatalf("declared go-hygiene with Cobra layout: expected no agent-tool hint, got %#v", cobraHygiene)
	}

	// A quoted go-agent-tool recipe still yields the hint.
	quoted := Detect(fixture(t, map[string]string{
		"go.mod":   "module example.com/cli\n",
		"bob.yaml": "schema_version: 1\nrecipe: \"go-agent-tool\"\n",
	}))
	if quoted.KindHint != "agent-tool" {
		t.Fatalf("quoted go-agent-tool: expected agent-tool hint, got %#v", quoted)
	}

	// A commented-out recipe line must not count as a declaration.
	commented := Detect(fixture(t, map[string]string{
		"go.mod":   "module example.com/cli\n",
		"bob.yaml": "schema_version: 1\n# recipe: go-agent-tool\n",
	}))
	if commented.KindHint != "" {
		t.Fatalf("commented recipe must not set agent-tool hint, got %#v", commented)
	}
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
