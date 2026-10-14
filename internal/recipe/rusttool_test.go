package recipe

import (
	"bytes"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"go.yaml.in/yaml/v3"
)

func rustToolManifest() manifest.Manifest {
	return manifest.DefaultRustTool("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
}

func TestRenderRustToolIsDeterministicAndSafe(t *testing.T) {
	t.Parallel()
	m := rustToolManifest()

	first, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same manifest produced different artifacts")
	}

	seen := make(map[string]struct{}, len(first))
	paths := make([]string, 0, len(first))
	for _, artifact := range first {
		if artifact.Path != filepath.ToSlash(filepath.Clean(artifact.Path)) || filepath.IsAbs(artifact.Path) || strings.HasPrefix(artifact.Path, "../") {
			t.Errorf("unsafe or unclean artifact path %q", artifact.Path)
		}
		if _, exists := seen[artifact.Path]; exists {
			t.Errorf("duplicate artifact path %q", artifact.Path)
		}
		seen[artifact.Path] = struct{}{}
		paths = append(paths, artifact.Path)
		if artifact.Mode != 0o644 {
			t.Errorf("%s mode = %o, want 644", artifact.Path, artifact.Mode)
		}
		if len(artifact.Content) == 0 || artifact.Content[len(artifact.Content)-1] != '\n' {
			t.Errorf("%s is empty or lacks a final newline", artifact.Path)
		}
		if bytes.Contains(artifact.Content, []byte("[[")) {
			t.Errorf("%s contains an unexpanded recipe marker", artifact.Path)
		}
	}
	if !sort.StringsAreSorted(paths) {
		t.Fatalf("artifacts are not path-sorted: %v", paths)
	}

	for _, artifact := range first {
		_, living := rustToolLivingFiles[artifact.Path]
		if artifact.Seed != living {
			t.Errorf("%s seed = %t, want %t", artifact.Path, artifact.Seed, living)
		}
	}

	for _, path := range []string{
		".editorconfig",
		".gitignore",
		".github/workflows/ci.yml",
		".github/workflows/release.yml",
		"AGENTS.md",
		"CHANGELOG.md",
		"CODE_OF_CONDUCT.md",
		"CONTRIBUTING.md",
		"Cargo.toml",
		"LICENSE",
		"README.md",
		"SECURITY.md",
		"Taskfile.yml",
		"clippy.toml",
		"docs/index.md",
		"rust-toolchain.toml",
		"rustfmt.toml",
		"src/lib.rs",
		"src/main.rs",
		"tests/cli.rs",
	} {
		if _, ok := seen[path]; !ok {
			t.Errorf("missing required artifact %s", path)
		}
	}
	if len(first) != 20 {
		t.Errorf("rust-tool artifact count = %d, want 20", len(first))
	}
}

func TestRenderRustToolProducesValidYAML(t *testing.T) {
	t.Parallel()
	artifacts, err := Render(rustToolManifest())
	if err != nil {
		t.Fatal(err)
	}
	files := artifactContentByPath(artifacts)
	for _, path := range []string{"Taskfile.yml", ".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		var value any
		if err := yaml.Unmarshal([]byte(files[path]), &value); err != nil {
			t.Errorf("generated %s is not valid YAML: %v", path, err)
		}
	}
}

func TestRenderRustToolProducesIdiomaticSources(t *testing.T) {
	t.Parallel()
	artifacts, err := Render(rustToolManifest())
	if err != nil {
		t.Fatal(err)
	}
	files := artifactContentByPath(artifacts)

	if got := files["Cargo.toml"]; !strings.Contains(got, `name = "acme-tool"`) ||
		!strings.Contains(got, `edition = "2021"`) || !strings.Contains(got, `license = "MIT"`) {
		t.Errorf("generated Cargo.toml is incomplete:\n%s", got)
	}
	if got := files["src/lib.rs"]; !strings.Contains(got, "pub fn greet") ||
		!strings.Contains(got, "#[test]") {
		t.Errorf("generated src/lib.rs is incomplete:\n%s", got)
	}
	if got := files["src/main.rs"]; !strings.Contains(got, "use acme_tool::greet;") {
		t.Errorf("generated src/main.rs does not call the library crate:\n%s", got)
	}
	if got := files["tests/cli.rs"]; !strings.Contains(got, `env!("CARGO_BIN_EXE_acme_tool")`) {
		t.Errorf("generated tests/cli.rs does not invoke the built binary:\n%s", got)
	}
}

func TestRenderRustToolRejectsVarsAndFiles(t *testing.T) {
	t.Parallel()
	m := rustToolManifest()
	m.Vars = map[string]string{"x": "y"}
	m.Files = []manifest.FileDecl{{Path: "a.txt", Content: "x"}}
	if err := m.Validate(); err == nil {
		t.Fatal("expected rust-tool manifest with vars/files to fail validation")
	} else {
		for _, want := range []string{"vars is only supported by recipe files", "files is only supported by recipe files"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("expected error to contain %q, got %v", want, err)
			}
		}
	}
}

func TestRenderRustToolRejectsWrongRuntimeAndDistribution(t *testing.T) {
	t.Parallel()
	m := rustToolManifest()
	m.Runtime.Kind = "lib"
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "runtime.language=rust and runtime.kind=cli") {
		t.Fatalf("expected runtime error, got %v", err)
	}
	m = rustToolManifest()
	m.Distribution.GitHubActions = true
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "distribution is not used by recipe rust-tool") {
		t.Fatalf("expected distribution error, got %v", err)
	}
}

func TestRenderRustToolOmitsGitHubSecurityLinksForNonGitHubModule(t *testing.T) {
	t.Parallel()
	m := manifest.DefaultRustTool("acme-tool", "example.com/acme/acme-tool", "Build useful things.")
	artifacts, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	files := artifactContentByPath(artifacts)
	if strings.Contains(files["SECURITY.md"], "security/advisories") {
		t.Errorf("non-GitHub module promised a GitHub reporting channel:\n%s", files["SECURITY.md"])
	}
	if !strings.Contains(files["SECURITY.md"], "cannot name a configured private reporting channel") {
		t.Errorf("non-GitHub security policy does not expose its publication prerequisite:\n%s", files["SECURITY.md"])
	}
}
