package engine

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

// writeManifest persists bob.yaml to disk so Remove (which loads the manifest
// from the workspace) sees the same recipe Apply used to build the lock.
func writeManifest(t *testing.T, root string, m manifest.Manifest) {
	t.Helper()
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
}

// setupManagedWorkspace writes bob.yaml and applies artifacts, returning a
// workspace whose bob.lock owns exactly the given artifact paths.
func setupManagedWorkspace(t *testing.T, artifacts []recipe.Artifact) string {
	t.Helper()
	root := t.TempDir()
	m := testManifest()
	writeManifest(t, root, m)
	if _, err := Apply(root, m, artifacts); err != nil {
		t.Fatal(err)
	}
	return root
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	t.Fatal(err)
	return false
}

func TestRemoveFullyManagedWorkspace(t *testing.T) {
	t.Parallel()
	root := setupManagedWorkspace(t, []recipe.Artifact{
		artifact("cmd/acme/main.go", "package main\n"),
		artifact("README.md", "hello\n"),
	})
	result, err := Remove(root, RemoveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"README.md", "cmd/acme/main.go"}; !reflect.DeepEqual(result.Removed, want) {
		t.Fatalf("removed = %v, want %v", result.Removed, want)
	}
	if len(result.Skipped) != 0 || len(result.Conflicts) != 0 || !result.LockRemoved {
		t.Fatalf("result = %#v", result)
	}
	for _, path := range []string{"README.md", "cmd/acme/main.go", LockFilename} {
		if exists(t, filepath.Join(root, filepath.FromSlash(path))) {
			t.Fatalf("expected %s to be removed", path)
		}
	}
	for _, dir := range []string{"cmd/acme", "cmd"} {
		if exists(t, filepath.Join(root, filepath.FromSlash(dir))) {
			t.Fatalf("expected empty directory %s to be cleaned up", dir)
		}
	}
	if !exists(t, filepath.Join(root, manifest.Filename)) {
		t.Fatal("remove must never delete bob.yaml")
	}
}

func TestRemoveSkipsHumanModifiedFileWithoutForce(t *testing.T) {
	t.Parallel()
	root := setupManagedWorkspace(t, []recipe.Artifact{artifact("README.md", "managed\n")})
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Remove(root, RemoveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Skipped, []string{"README.md"}) || len(result.Removed) != 0 {
		t.Fatalf("result = %#v", result)
	}
	if result.LockRemoved {
		t.Fatal("lock must be retained when a managed file is skipped")
	}
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil || string(data) != "human edit\n" {
		t.Fatalf("skipped file = %q, %v; want human edit preserved", data, err)
	}
	if !exists(t, filepath.Join(root, LockFilename)) {
		t.Fatal("lock must still exist after a partial remove")
	}
}

func TestRemoveForceRemovesModifiedFile(t *testing.T) {
	t.Parallel()
	root := setupManagedWorkspace(t, []recipe.Artifact{artifact("README.md", "managed\n")})
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Remove(root, RemoveOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Removed, []string{"README.md"}) || len(result.Skipped) != 0 || !result.LockRemoved {
		t.Fatalf("result = %#v", result)
	}
	if exists(t, filepath.Join(root, "README.md")) || exists(t, filepath.Join(root, LockFilename)) {
		t.Fatal("force remove must delete the drifted file and the lock")
	}
}

func TestRemoveDryRunRemovesNothing(t *testing.T) {
	t.Parallel()
	root := setupManagedWorkspace(t, []recipe.Artifact{
		artifact("cmd/acme/main.go", "package main\n"),
		artifact("README.md", "hello\n"),
	})
	result, err := Remove(root, RemoveOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"README.md", "cmd/acme/main.go"}; !reflect.DeepEqual(result.Removed, want) {
		t.Fatalf("dry-run removed preview = %v, want %v", result.Removed, want)
	}
	if !result.LockRemoved {
		t.Fatal("dry-run should report the lock would be removed for a clean workspace")
	}
	for _, path := range []string{"README.md", "cmd/acme/main.go", LockFilename, "cmd/acme", "cmd"} {
		if !exists(t, filepath.Join(root, filepath.FromSlash(path))) {
			t.Fatalf("dry-run must not remove %s", path)
		}
	}
}

func TestRemoveNoLockErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeManifest(t, root, testManifest())
	_, err := Remove(root, RemoveOptions{})
	if !errors.Is(err, ErrNoLock) {
		t.Fatalf("remove without lock error = %v, want ErrNoLock", err)
	}
}

func TestRemoveNeverTouchesUnmanagedFiles(t *testing.T) {
	t.Parallel()
	root := setupManagedWorkspace(t, []recipe.Artifact{artifact("README.md", "managed\n")})
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Remove(root, RemoveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Removed, []string{"README.md"}) {
		t.Fatalf("result = %#v", result)
	}
	data, err := os.ReadFile(filepath.Join(root, "notes.txt"))
	if err != nil || string(data) != "mine\n" {
		t.Fatalf("unmanaged file = %q, %v; want untouched", data, err)
	}
	if exists(t, filepath.Join(root, "README.md")) {
		t.Fatal("managed file should be removed")
	}
}

func TestRemoveNeverRemovesManifest(t *testing.T) {
	t.Parallel()
	root := setupManagedWorkspace(t, []recipe.Artifact{artifact("README.md", "managed\n")})
	if _, err := Remove(root, RemoveOptions{}); err != nil {
		t.Fatal(err)
	}
	if !exists(t, filepath.Join(root, manifest.Filename)) {
		t.Fatal("remove must never delete bob.yaml")
	}
}

func TestRemoveReportsSymlinkAsConflict(t *testing.T) {
	t.Parallel()
	root := setupManagedWorkspace(t, []recipe.Artifact{artifact("link.txt", "managed\n")})
	linkPath := filepath.Join(root, "link.txt")
	if err := os.Remove(linkPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.txt", linkPath); err != nil {
		t.Fatal(err)
	}
	result, err := Remove(root, RemoveOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Conflicts, []string{"link.txt"}) || len(result.Removed) != 0 {
		t.Fatalf("result = %#v", result)
	}
	if result.LockRemoved {
		t.Fatal("lock must be retained when a managed path is a symlink conflict")
	}
	info, err := os.Lstat(linkPath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink conflict must not be removed: %v", err)
	}
	if !exists(t, filepath.Join(root, LockFilename)) {
		t.Fatal("lock must still exist when a conflict remains")
	}
}

func TestRemoveCleansEmptyDirsButKeepsUnmanagedDirs(t *testing.T) {
	t.Parallel()
	root := setupManagedWorkspace(t, []recipe.Artifact{
		artifact("a/b/m1.txt", "one\n"),
		artifact("a/b/m2.txt", "two\n"),
	})
	if err := os.MkdirAll(filepath.Join(root, "a", "c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "c", "unmanaged.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Remove(root, RemoveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a/b/m1.txt", "a/b/m2.txt"}; !reflect.DeepEqual(result.Removed, want) {
		t.Fatalf("removed = %v, want %v", result.Removed, want)
	}
	if exists(t, filepath.Join(root, "a", "b")) {
		t.Fatal("empty managed directory a/b should be removed")
	}
	if !exists(t, filepath.Join(root, "a", "c", "unmanaged.txt")) {
		t.Fatal("unmanaged file a/c/unmanaged.txt must survive")
	}
	if !exists(t, filepath.Join(root, "a")) {
		t.Fatal("directory a still holds unmanaged content and must survive")
	}
}

func TestRemoveIsIdempotent(t *testing.T) {
	t.Parallel()
	root := setupManagedWorkspace(t, []recipe.Artifact{artifact("README.md", "managed\n")})
	if _, err := Remove(root, RemoveOptions{}); err != nil {
		t.Fatal(err)
	}
	_, err := Remove(root, RemoveOptions{})
	if !errors.Is(err, ErrNoLock) {
		t.Fatalf("second remove error = %v, want ErrNoLock", err)
	}
}
