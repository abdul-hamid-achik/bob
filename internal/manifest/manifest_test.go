package manifest

import (
	"os"
	"path/filepath"
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
