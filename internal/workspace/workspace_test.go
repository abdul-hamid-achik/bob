package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRejectsSelectedSymlink(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(link, true); err == nil {
		t.Fatal("expected selected symlink to be rejected")
	}
}

func TestResolveMissingDescendant(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	want := filepath.Join(root, "new", "repo")
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	canonicalWant := filepath.Join(canonicalRoot, "new", "repo")
	got, err := Resolve(want, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != canonicalWant {
		t.Fatalf("Resolve() = %q, want %q", got, canonicalWant)
	}
	if _, err := Resolve(want, true); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Resolve(mustExist) error = %v", err)
	}
}
