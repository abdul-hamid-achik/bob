package recipe

import (
	"reflect"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

func filesManifest(vars map[string]string, files ...manifest.FileDecl) manifest.Manifest {
	return manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Recipe:        manifest.RecipeFiles,
		Product: manifest.Product{
			Name:        "demo-app",
			Description: "A demo app",
		},
		Vars:  vars,
		Files: files,
	}
}

func TestRenderFilesSubstitutesOnlyTheDeclaredPattern(t *testing.T) {
	t.Parallel()
	m := filesManifest(map[string]string{"name": "bob", "port": "8080"}, manifest.FileDecl{
		Path: "a.txt",
		Content: "hello ${vars.name} on ${vars.port}\n" +
			"shell stays put: ${FOO}\n" +
			"not a match: $vars.name\n" +
			"case sensitive: ${vars.NAME}\n",
	})
	artifacts, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("artifacts = %#v", artifacts)
	}
	want := "hello bob on 8080\n" +
		"shell stays put: ${FOO}\n" +
		"not a match: $vars.name\n" +
		"case sensitive: ${vars.NAME}\n"
	if got := string(artifacts[0].Content); got != want {
		t.Fatalf("substituted content = %q, want %q", got, want)
	}
}

func TestRenderFilesReportsEveryUnresolvedReferenceOnce(t *testing.T) {
	t.Parallel()
	m := filesManifest(map[string]string{"known": "value"},
		manifest.FileDecl{Path: "b.txt", Content: "${vars.missing} ${vars.missing}"},
		manifest.FileDecl{Path: "a.txt", Content: "${vars.missing} and ${vars.other}"},
	)
	_, err := Render(m)
	if err == nil {
		t.Fatal("expected an unresolved variable error")
	}
	msg := err.Error()
	for _, want := range []string{"a.txt: ${vars.missing}", "a.txt: ${vars.other}", "b.txt: ${vars.missing}"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing entry %q", msg, want)
		}
	}
	if strings.Count(msg, "b.txt: ${vars.missing}") != 1 {
		t.Fatalf("error did not dedupe repeated references: %q", msg)
	}
	// Sorted: a.txt entries must appear before b.txt entries.
	if strings.Index(msg, "a.txt") > strings.Index(msg, "b.txt") {
		t.Fatalf("error is not sorted: %q", msg)
	}
}

func TestRenderFilesIsDeterministic(t *testing.T) {
	t.Parallel()
	m := filesManifest(map[string]string{"name": "bob"},
		manifest.FileDecl{Path: "z.txt", Content: "z ${vars.name}"},
		manifest.FileDecl{Path: "a.txt", Content: "a ${vars.name}"},
	)
	first, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same manifest produced different artifacts:\n%#v\n%#v", first, second)
	}
	if first[0].Path != "a.txt" || first[1].Path != "z.txt" {
		t.Fatalf("artifacts are not path-sorted: %#v", first)
	}
}

func TestRenderFilesModeParsing(t *testing.T) {
	t.Parallel()
	m := filesManifest(nil,
		manifest.FileDecl{Path: "default.txt", Content: "x"},
		manifest.FileDecl{Path: "exec.sh", Mode: "0755", Content: "x"},
	)
	artifacts, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	modes := make(map[string]uint32, len(artifacts))
	for _, artifact := range artifacts {
		modes[artifact.Path] = uint32(artifact.Mode.Perm())
	}
	if modes["default.txt"] != 0o644 {
		t.Errorf("default mode = %o, want 0644", modes["default.txt"])
	}
	if modes["exec.sh"] != 0o755 {
		t.Errorf("exec.sh mode = %o, want 0755", modes["exec.sh"])
	}
}

func TestRenderFilesRejectsInvalidMode(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"4755", "2644", "1644", "abcd", "64", "07777", "999"} {
		m := filesManifest(nil, manifest.FileDecl{Path: "f.txt", Mode: mode, Content: "x"})
		if _, err := Render(m); err == nil {
			t.Errorf("mode %q was accepted", mode)
		}
	}
}

func TestRenderFilesPathSafety(t *testing.T) {
	t.Parallel()
	for _, unsafe := range []string{"../escape.txt", "/absolute.txt", ".git/config", manifest.Filename, "bob.lock"} {
		m := filesManifest(nil, manifest.FileDecl{Path: unsafe, Content: "x"})
		if _, err := Render(m); err == nil {
			t.Errorf("unsafe path %q was accepted", unsafe)
		}
	}
}

func TestRenderFilesRejectsDuplicatePaths(t *testing.T) {
	t.Parallel()
	m := filesManifest(nil,
		manifest.FileDecl{Path: "a/b.txt", Content: "one"},
		manifest.FileDecl{Path: "a/./b.txt", Content: "two"},
	)
	if _, err := Render(m); err == nil {
		t.Fatal("duplicate canonical path was accepted")
	}
}

func TestFilesRecipeVersionIsOne(t *testing.T) {
	t.Parallel()
	version, err := Version(manifest.RecipeFiles)
	if err != nil || version != 1 || FilesRecipeVersion != 1 {
		t.Fatalf("files recipe version = %d, %v, want 1", version, err)
	}
}
