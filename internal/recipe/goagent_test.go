package recipe

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"go.yaml.in/yaml/v3"
)

func TestRenderGoAgentToolIsDeterministicAndSafe(t *testing.T) {
	t.Parallel()
	m := fullGoAgentManifest()

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

	for _, path := range []string{
		".github/ISSUE_TEMPLATE/bug.yml",
		".github/ISSUE_TEMPLATE/config.yml",
		".github/ISSUE_TEMPLATE/feature.yml",
		".github/dependabot.yml",
		".github/pull_request_template.md",
		".github/workflows/ci.yml",
		".github/workflows/release.yml",
		".goreleaser.yaml",
		".gitignore",
		".golangci.yml",
		"AGENTS.md",
		"CHANGELOG.md",
		"CLAUDE.md",
		"CODE_OF_CONDUCT.md",
		"CONTRIBUTING.md",
		"LICENSE",
		"README.md",
		"SECURITY.md",
		"Taskfile.yml",
		"cmd/acme-tool/main.go",
		"docs/index.md",
		"glyphrun.config.yml",
		"go.mod",
		"go.sum",
		"internal/cli/root.go",
		"internal/cli/root_test.go",
		"internal/version/version.go",
		"specs/help.yml",
	} {
		if _, ok := seen[path]; !ok {
			t.Errorf("missing required artifact %s", path)
		}
	}
}

func TestRenderGoAgentToolCapabilityConditionals(t *testing.T) {
	t.Parallel()

	t.Run("minimal", func(t *testing.T) {
		m := manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
		m.Integrations = manifest.Integrations{
			CodeStructure:        "none",
			SemanticSearch:       "none",
			TerminalVerification: "none",
			BrowserVerification:  "none",
			Secrets:              "none",
			Artifacts:            "none",
		}
		m.Distribution = manifest.Distribution{Docs: "none"}
		artifacts, err := Render(m)
		if err != nil {
			t.Fatal(err)
		}
		files := artifactContentByPath(artifacts)
		for _, present := range []string{".github/ISSUE_TEMPLATE/bug.yml", ".github/dependabot.yml", ".github/pull_request_template.md", "CODE_OF_CONDUCT.md"} {
			if _, ok := files[present]; !ok {
				t.Errorf("minimal GitHub recipe omitted %s", present)
			}
		}
		for _, absent := range []string{
			".github/workflows/ci.yml",
			".github/workflows/release.yml",
			".goreleaser.yaml",
			"docs/index.md",
			"glyphrun.config.yml",
			"package.json",
			"specs/help.yml",
		} {
			if _, ok := files[absent]; ok {
				t.Errorf("minimal recipe unexpectedly generated %s", absent)
			}
		}
		root := files["internal/cli/root.go"]
		if !strings.Contains(root, `Name: "go", Binary: "go"`) {
			t.Error("minimal doctor does not check Go")
		}
		for _, binary := range []string{"cairn", "codemap", "fcheap", "glyph", "goreleaser", "tvault", "vecgrep"} {
			if strings.Contains(root, `Binary: "`+binary+`"`) {
				t.Errorf("minimal doctor unexpectedly requires %s", binary)
			}
		}
		for _, binary := range []string{"golangci-lint", "task"} {
			if !strings.Contains(root, `Binary: "`+binary+`", Required: false`) {
				t.Errorf("minimal doctor does not report optional %s", binary)
			}
		}
	})

	t.Run("markdown CI without release", func(t *testing.T) {
		m := manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
		m.Integrations.TerminalVerification = "none"
		m.Distribution.GitHubActions = true
		m.Distribution.GoReleaser = false
		m.Distribution.Docs = "markdown"
		artifacts, err := Render(m)
		if err != nil {
			t.Fatal(err)
		}
		files := artifactContentByPath(artifacts)
		for _, present := range []string{".github/workflows/ci.yml", "docs/index.md"} {
			if _, ok := files[present]; !ok {
				t.Errorf("expected %s", present)
			}
		}
		for _, absent := range []string{".github/workflows/release.yml", ".goreleaser.yaml", "package.json", "specs/help.yml"} {
			if _, ok := files[absent]; ok {
				t.Errorf("unexpected %s", absent)
			}
		}
	})

	t.Run("selected integrations", func(t *testing.T) {
		artifacts, err := Render(fullGoAgentManifest())
		if err != nil {
			t.Fatal(err)
		}
		files := artifactContentByPath(artifacts)
		root := files["internal/cli/root.go"]
		for _, binary := range []string{"cairn", "codemap", "fcheap", "glyph", "golangci-lint", "goreleaser", "task", "tvault", "vecgrep"} {
			if !strings.Contains(root, `Binary: "`+binary+`"`) {
				t.Errorf("selected doctor does not check %s", binary)
			}
		}
		if !strings.Contains(files[".goreleaser.yaml"], "homebrew_casks:") {
			t.Error("homebrew selection did not change GoReleaser output")
		}
	})
}

func TestRenderGoAgentToolProducesSyntacticGo(t *testing.T) {
	t.Parallel()
	artifacts, err := Render(fullGoAgentManifest())
	if err != nil {
		t.Fatal(err)
	}
	files := artifactContentByPath(artifacts)
	for _, path := range []string{"cmd/acme-tool/main.go", "internal/cli/root.go", "internal/cli/root_test.go", "internal/version/version.go"} {
		if _, err := parser.ParseFile(token.NewFileSet(), path, files[path], parser.AllErrors); err != nil {
			t.Errorf("generated %s is not valid Go: %v", path, err)
		}
		formatted, err := format.Source([]byte(files[path]))
		if err == nil && string(formatted) != files[path] {
			t.Errorf("generated %s is not gofmt-clean", path)
		}
	}
	if got := files["go.mod"]; !strings.Contains(got, "module github.com/acme/acme-tool") ||
		!strings.Contains(got, "go 1.26.5") || !strings.Contains(got, "github.com/spf13/cobra v1.10.2") {
		t.Errorf("generated go.mod is incomplete:\n%s", got)
	}
}

func TestRenderGoAgentToolProducesValidYAML(t *testing.T) {
	t.Parallel()
	artifacts, err := Render(fullGoAgentManifest())
	if err != nil {
		t.Fatal(err)
	}
	files := artifactContentByPath(artifacts)
	for _, path := range []string{
		".github/ISSUE_TEMPLATE/bug.yml",
		".github/ISSUE_TEMPLATE/config.yml",
		".github/ISSUE_TEMPLATE/feature.yml",
		".github/dependabot.yml",
		".github/workflows/ci.yml",
		".github/workflows/release.yml",
		".golangci.yml",
		".goreleaser.yaml",
		"Taskfile.yml",
		"glyphrun.config.yml",
		"specs/help.yml",
	} {
		var value any
		if err := yaml.Unmarshal([]byte(files[path]), &value); err != nil {
			t.Errorf("generated %s is not valid YAML: %v", path, err)
		}
	}
}

func TestRenderedGoAgentToolBuildsWithLockedModules(t *testing.T) {
	if testing.Short() {
		t.Skip("generated-project subprocess smoke")
	}
	m := manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	artifacts, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, artifact := range artifacts {
		path := filepath.Join(root, filepath.FromSlash(artifact.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, artifact.Content, artifact.Mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"test", "./..."}, {"mod", "tidy", "-diff"}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=readonly")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, output)
		}
	}
}

func TestRenderGoAgentToolRejectsNonGitHubHomebrewModule(t *testing.T) {
	t.Parallel()
	m := manifest.Default("acme-tool", "example.com/acme-tool", "Build useful things.")
	m.Distribution.Homebrew = true
	_, err := Render(m)
	if err == nil || !strings.Contains(err.Error(), "homebrew distribution requires") {
		t.Fatalf("expected actionable Homebrew module error, got %v", err)
	}
}

func TestRenderGoAgentToolOmitsGitHubCommunityFilesForOtherHosts(t *testing.T) {
	t.Parallel()
	m := manifest.Default("acme-tool", "example.com/acme/acme-tool", "Build useful things.")
	m.Distribution.GitHubActions = false
	m.Distribution.GoReleaser = false
	artifacts, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	files := artifactContentByPath(artifacts)
	for _, path := range []string{".github/ISSUE_TEMPLATE/bug.yml", ".github/dependabot.yml", ".github/pull_request_template.md"} {
		if _, ok := files[path]; ok {
			t.Errorf("non-GitHub module unexpectedly generated %s", path)
		}
	}
	if _, ok := files["CODE_OF_CONDUCT.md"]; !ok {
		t.Fatal("host-independent Code of Conduct was omitted")
	}
	for _, path := range []string{"README.md", "CONTRIBUTING.md", "SECURITY.md"} {
		if strings.Contains(files[path], "security/advisories") || strings.Contains(files[path], "GitHub private") {
			t.Errorf("non-GitHub %s promises a GitHub reporting channel:\n%s", path, files[path])
		}
	}
	if !strings.Contains(files["SECURITY.md"], "cannot name a configured private reporting channel") {
		t.Fatalf("non-GitHub security policy does not expose its publication prerequisite:\n%s", files["SECURITY.md"])
	}
}

func TestRenderGoAgentToolUsesVisibilityAwareGitHubSecurityLinks(t *testing.T) {
	t.Parallel()
	publicFiles := artifactContentByPath(mustRenderGoAgentTool(t, manifest.Default(
		"acme-tool",
		"github.com/acme/acme-tool",
		"Build useful things.",
	)))
	if !strings.Contains(publicFiles["SECURITY.md"], "github.com/acme/acme-tool/security/advisories/new") ||
		!strings.Contains(publicFiles[".github/ISSUE_TEMPLATE/config.yml"], "github.com/acme/acme-tool/security/advisories/new") {
		t.Fatal("public GitHub recipe omitted private-vulnerability-reporting links")
	}

	privateManifest := manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	privateManifest.Product.Visibility = "private"
	privateFiles := artifactContentByPath(mustRenderGoAgentTool(t, privateManifest))
	if strings.Contains(privateFiles["SECURITY.md"], "security/advisories") {
		t.Fatalf("private GitHub security policy promises unsupported private reporting:\n%s", privateFiles["SECURITY.md"])
	}
	if !strings.Contains(privateFiles[".github/ISSUE_TEMPLATE/config.yml"], "/blob/main/SECURITY.md") {
		t.Fatalf("private GitHub issue config does not route to SECURITY.md:\n%s", privateFiles[".github/ISSUE_TEMPLATE/config.yml"])
	}
}

func TestRenderGoAgentToolRejectsModuleTemplateInjection(t *testing.T) {
	t.Parallel()
	m := manifest.Default("acme-tool", "github.com/acme/acme-tool;replace", "Build useful things.")
	_, err := Render(m)
	if err == nil || !strings.Contains(err.Error(), "unsupported character") {
		t.Fatalf("expected invalid module-path error, got %v", err)
	}
}

func fullGoAgentManifest() manifest.Manifest {
	m := manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	m.Integrations.BrowserVerification = "cairntrace"
	m.Integrations.Secrets = "tinyvault"
	m.Integrations.Artifacts = "fcheap"
	m.Distribution.Homebrew = true
	m.Distribution.Docs = "markdown"
	return m
}

func mustRenderGoAgentTool(t *testing.T, m manifest.Manifest) []Artifact {
	t.Helper()
	artifacts, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	return artifacts
}

func artifactContentByPath(artifacts []Artifact) map[string]string {
	files := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		files[artifact.Path] = string(artifact.Content)
	}
	return files
}
