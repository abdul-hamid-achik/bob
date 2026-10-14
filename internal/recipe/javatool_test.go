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

func javaToolManifest() manifest.Manifest {
	return manifest.DefaultJavaTool("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
}

func TestRenderJavaToolIsDeterministicAndSafe(t *testing.T) {
	t.Parallel()
	m := javaToolManifest()

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
		_, living := javaToolLivingFiles[artifact.Path]
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
		"LICENSE",
		"README.md",
		"SECURITY.md",
		"Taskfile.yml",
		"checkstyle.xml",
		"docs/index.md",
		"pom.xml",
		"src/main/java/com/acme/acmetool/Cli.java",
		"src/main/java/com/acme/acmetool/Main.java",
		"src/test/java/com/acme/acmetool/CliTest.java",
	} {
		if _, ok := seen[path]; !ok {
			t.Errorf("missing required artifact %s", path)
		}
	}
	if len(first) != 18 {
		t.Errorf("java-tool artifact count = %d, want 18", len(first))
	}
}

func TestRenderJavaToolProducesValidYAML(t *testing.T) {
	t.Parallel()
	artifacts, err := Render(javaToolManifest())
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

func TestRenderJavaToolProducesIdiomaticSources(t *testing.T) {
	t.Parallel()
	artifacts, err := Render(javaToolManifest())
	if err != nil {
		t.Fatal(err)
	}
	files := artifactContentByPath(artifacts)

	if got := files["pom.xml"]; !strings.Contains(got, "<artifactId>acme-tool</artifactId>") ||
		!strings.Contains(got, "<release>21</release>") ||
		!strings.Contains(got, "org.junit.jupiter") ||
		!strings.Contains(got, "maven-shade-plugin") ||
		!strings.Contains(got, "com.acme.acmetool.Main") {
		t.Errorf("generated pom.xml is incomplete:\n%s", got)
	}
	if got := files["src/main/java/com/acme/acmetool/Cli.java"]; !strings.Contains(got, "public static String greet") ||
		!strings.Contains(got, "public static void main") {
		t.Errorf("generated Cli.java is incomplete:\n%s", got)
	}
	if got := files["src/main/java/com/acme/acmetool/Main.java"]; !strings.Contains(got, "Cli.main(args)") {
		t.Errorf("generated Main.java does not delegate to Cli:\n%s", got)
	}
	if got := files["src/test/java/com/acme/acmetool/CliTest.java"]; !strings.Contains(got, "assertEquals(\"Hello, world!\", Cli.greet(\"world\"))") {
		t.Errorf("generated CliTest.java is incomplete:\n%s", got)
	}
}

func TestRenderJavaToolRejectsVarsAndFiles(t *testing.T) {
	t.Parallel()
	m := javaToolManifest()
	m.Vars = map[string]string{"x": "y"}
	m.Files = []manifest.FileDecl{{Path: "a.txt", Content: "x"}}
	if err := m.Validate(); err == nil {
		t.Fatal("expected java-tool manifest with vars/files to fail validation")
	} else {
		for _, want := range []string{"vars is only supported by recipe files", "files is only supported by recipe files"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("expected error to contain %q, got %v", want, err)
			}
		}
	}
}

func TestRenderJavaToolRejectsWrongRuntimeAndDistribution(t *testing.T) {
	t.Parallel()
	m := javaToolManifest()
	m.Runtime.Kind = "lib"
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "runtime.language=java and runtime.kind=cli") {
		t.Fatalf("expected runtime error, got %v", err)
	}
	m = javaToolManifest()
	m.Distribution.GitHubActions = true
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "distribution is not used by recipe java-tool") {
		t.Fatalf("expected distribution error, got %v", err)
	}
}

func TestRenderJavaToolOmitsGitHubSecurityLinksForNonGitHubModule(t *testing.T) {
	t.Parallel()
	m := manifest.DefaultJavaTool("acme-tool", "example.com/acme/acme-tool", "Build useful things.")
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

func TestJavaPackageDerivation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		module string
		name   string
		want   string
	}{
		{"github.com/acme/acme-tool", "acme-tool", "com.acme.acmetool"},
		{"github.com/acme/javademo", "javademo", "com.acme.javademo"},
		{"codeberg.org/acme/foo-bar", "foo-bar", "org.acme.foobar"},
		{"", "acme-tool", "com.example.acmetool"},
		{"acme-tool", "acme-tool", "com.example.acmetool"},
		{"github.com/acme/class", "acme-tool", "com.acme"},
	}
	for _, tc := range tests {
		if got := javaPackage(tc.module, tc.name); got != tc.want {
			t.Errorf("javaPackage(%q, %q) = %q, want %q", tc.module, tc.name, got, tc.want)
		}
	}
}
