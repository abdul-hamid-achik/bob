package detect

import (
	"os"
	"path/filepath"
	"testing"
)

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
		{"python pyproject", map[string]string{"pyproject.toml": "[project]\nname = \"x\"\n"}, "python"},
		{"python requirements", map[string]string{"requirements.txt": "requests\n"}, "python"},
		{"ruby gemfile", map[string]string{"Gemfile": "source \"https://rubygems.org\"\n"}, "ruby"},
		{"lua rockspec", map[string]string{"x-1.0-1.rockspec": "package = \"x\"\n"}, "lua"},
		{"neovim plugin", map[string]string{"lua/": "", "init.lua": "-- entry\n"}, "lua"},
		{"typescript app", map[string]string{"package.json": "{}", "tsconfig.json": "{}"}, "typescript"},
		{"javascript app", map[string]string{"package.json": `{"dependencies":{"express":"^4"}}`}, "javascript"},
		{"vue by dependency", map[string]string{"package.json": `{"dependencies":{"vue":"^3"}}`}, "vue"},
		{"vue by src files", map[string]string{"package.json": "{}", "src/App.vue": "<template/>"}, "vue"},
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

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
