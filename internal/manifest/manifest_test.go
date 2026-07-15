package manifest

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	want := Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	path := filepath.Join(root, Filename)
	if err := WriteFile(path, want, false); err != nil {
		t.Fatal(err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Product != want.Product || got.Recipe != want.Recipe {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m := Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	data, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\nunknown: true\n")...)
	if err := os.WriteFile(filepath.Join(root, Filename), data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Load(root)
	if err == nil || !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func TestValidateRejectsUnsupportedSurface(t *testing.T) {
	t.Parallel()
	m := Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	m.Surfaces.MCP = true
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved-surface error, got %v", err)
	}
}

func TestValidateRequiresJSONSurface(t *testing.T) {
	t.Parallel()
	m := Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	m.Surfaces.JSON = false
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "surfaces.json=true") {
		t.Fatalf("expected JSON surface error, got %v", err)
	}
}

func TestValidateRejectsPrivateHomebrewDistribution(t *testing.T) {
	t.Parallel()
	m := Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	m.Product.Visibility = "private"
	m.Distribution.Homebrew = true
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "visibility=public") {
		t.Fatalf("expected public visibility error, got %v", err)
	}
}

func TestValidateRejectsUnsafeModuleCharacters(t *testing.T) {
	t.Parallel()
	m := Default("acme-tool", "github.com/acme/acme-tool;replace", "Build useful things.")
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "unsupported character") {
		t.Fatalf("expected module character error, got %v", err)
	}
}

func TestValidateRejectsMalformedModuleSegments(t *testing.T) {
	t.Parallel()
	for _, module := range []string{"/github.com/acme/tool", "github.com/acme/tool/", "github.com//tool", "github.com/../tool"} {
		m := Default("acme-tool", module, "Build useful things.")
		if err := m.Validate(); err == nil {
			t.Errorf("expected module %q to fail validation", module)
		}
	}
}

func filesManifest() Manifest {
	return Manifest{
		SchemaVersion: SchemaVersion,
		Recipe:        RecipeFiles,
		Product:       Product{Name: "demo-app", Description: "A demo app"},
		Files: []FileDecl{
			{Path: "a.txt", Content: "hello"},
		},
	}
}

func TestValidateFilesRecipeAcceptsMinimalManifest(t *testing.T) {
	t.Parallel()
	if err := filesManifest().Validate(); err != nil {
		t.Fatalf("expected minimal files manifest to validate, got %v", err)
	}
}

func TestValidateFilesRecipeRequiresAtLeastOneFile(t *testing.T) {
	t.Parallel()
	m := filesManifest()
	m.Files = nil
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "files must declare at least one file") {
		t.Fatalf("expected missing-files error, got %v", err)
	}
}

func TestValidateFilesRecipeRejectsBadVarsKey(t *testing.T) {
	t.Parallel()
	m := filesManifest()
	m.Vars = map[string]string{"Bad-Key": "x"}
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), `vars key "Bad-Key"`) {
		t.Fatalf("expected bad vars key error, got %v", err)
	}
}

func TestValidateGoAgentToolRejectsVarsAndFiles(t *testing.T) {
	t.Parallel()
	m := Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	m.Vars = map[string]string{"x": "y"}
	m.Files = []FileDecl{{Path: "a.txt", Content: "x"}}
	err := m.Validate()
	if err == nil {
		t.Fatal("expected go-agent-tool manifest with vars/files to fail validation")
	}
	if !strings.Contains(err.Error(), "vars is only supported by recipe files") {
		t.Fatalf("expected vars rejection, got %v", err)
	}
	if !strings.Contains(err.Error(), "files is only supported by recipe files") {
		t.Fatalf("expected files rejection, got %v", err)
	}
}

func TestValidateFilesRecipeRejectsRuntimeAndOtherGoAgentToolSections(t *testing.T) {
	t.Parallel()
	// A copied-over go-agent-tool manifest switched to recipe: files must
	// fail loudly, naming every unused section, rather than silently
	// ignoring the fields.
	m := Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	m.Recipe = RecipeFiles
	m.Files = []FileDecl{{Path: "a.txt", Content: "hello"}}
	err := m.Validate()
	if err == nil {
		t.Fatal("expected copied-over go-agent-tool manifest to fail under recipe files")
	}
	for _, want := range []string{
		"runtime is not used by recipe files",
		"surfaces is not used by recipe files",
		"integrations is not used by recipe files",
		"distribution is not used by recipe files",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to contain %q, got %v", want, err)
		}
	}
}

func TestValidateFilesRecipeRejectsLicenseOverSixtyFourChars(t *testing.T) {
	t.Parallel()
	m := filesManifest()
	m.Product.License = strings.Repeat("x", 65)
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "product.license") {
		t.Fatalf("expected license-length error, got %v", err)
	}
}

func TestValidateFilesRecipeRejectsInvalidFileMode(t *testing.T) {
	t.Parallel()
	m := filesManifest()
	m.Files[0].Mode = "4755"
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), `mode must be an octal permission string like "0644"`) {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
}

func TestValidateFilesRecipeRejectsDuplicateCanonicalPaths(t *testing.T) {
	t.Parallel()
	m := filesManifest()
	m.Files = append(m.Files, FileDecl{Path: "./a.txt", Content: "again"})
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate path") {
		t.Fatalf("expected duplicate path error, got %v", err)
	}
}

func TestValidateFilesRecipeAllowsOptionalModuleVisibilityAndLicense(t *testing.T) {
	t.Parallel()
	m := filesManifest()
	m.Product.Module = "github.com/acme/tool"
	m.Product.Visibility = "public"
	m.Product.License = "MIT"
	if err := m.Validate(); err != nil {
		t.Fatalf("expected optional product fields to validate, got %v", err)
	}
}

func TestLoadWithSourceReturnsExactValidatedManifestBytes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m := filesManifest()
	if err := WriteFile(filepath.Join(root, Filename), m, false); err != nil {
		t.Fatal(err)
	}
	wantSource, err := os.ReadFile(filepath.Join(root, Filename))
	if err != nil {
		t.Fatal(err)
	}
	got, source, err := LoadWithSource(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, m) || !bytes.Equal(source, wantSource) {
		t.Fatalf("LoadWithSource() manifest=%#v source=%q", got, source)
	}
	source[0] = 'x'
	after, err := os.ReadFile(filepath.Join(root, Filename))
	if err != nil || !bytes.Equal(after, wantSource) {
		t.Fatalf("returned source aliases file state: %v", err)
	}
}

func TestWriteRefusesExistingManifest(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), Filename)
	m := Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	if err := WriteFile(path, m, false); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, m, false); err == nil {
		t.Fatal("expected existing-file error")
	}
}
