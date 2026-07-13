package engine

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

func testManifest() manifest.Manifest {
	return manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
}

func artifact(path, content string) recipe.Artifact {
	return recipe.Artifact{Path: path, Mode: 0o644, Content: []byte(content)}
}

func actionKinds(plan PlanResult) []ActionKind {
	kinds := make([]ActionKind, len(plan.Actions))
	for i, action := range plan.Actions {
		kinds[i] = action.Kind
	}
	return kinds
}

func TestFirstApplyWritesArtifactsAndSortedLock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{
		artifact("z.txt", "last\n"),
		artifact("cmd/acme/main.go", "package main\n"),
	}
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := actionKinds(plan), []ActionKind{ActionCreate, ActionCreate}; !reflect.DeepEqual(got, want) {
		t.Fatalf("plan actions = %v, want %v", got, want)
	}
	result, err := Apply(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if !result.LockWritten || !reflect.DeepEqual(result.Written, []string{"cmd/acme/main.go", "z.txt"}) {
		t.Fatalf("apply result = %#v", result)
	}
	for _, item := range artifacts {
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(item.Path)))
		if readErr != nil || string(data) != string(item.Content) {
			t.Fatalf("artifact %s = %q, %v", item.Path, data, readErr)
		}
	}
	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := []string{lock.Files[0].Path, lock.Files[1].Path}, []string{"cmd/acme/main.go", "z.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lock order = %v, want %v", got, want)
	}
	if lock.Recipe.ID != testManifest().Recipe || lock.Recipe.Version != RecipeVersion {
		t.Fatalf("lock recipe = %#v", lock.Recipe)
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{artifact("README.md", "hello\n")}
	if _, err := Apply(root, testManifest(), artifacts); err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.ReadFile(filepath.Join(root, LockFilename))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if result.LockWritten || len(result.Written) != 0 || !reflect.DeepEqual(result.Unchanged, []string{"README.md"}) {
		t.Fatalf("second apply = %#v", result)
	}
	lockAfter, _ := os.ReadFile(filepath.Join(root, LockFilename))
	if string(lockBefore) != string(lockAfter) {
		t.Fatal("idempotent apply rewrote different lock content")
	}
}

func TestManagedFileUpdatesOnlyFromRecordedHash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initial := []recipe.Artifact{artifact("README.md", "one\n")}
	if _, err := Apply(root, testManifest(), initial); err != nil {
		t.Fatal(err)
	}
	updated := []recipe.Artifact{artifact("README.md", "two\n")}
	plan, err := Plan(root, testManifest(), updated)
	if err != nil {
		t.Fatal(err)
	}
	if got := actionKinds(plan); !reflect.DeepEqual(got, []ActionKind{ActionUpdate}) {
		t.Fatalf("update plan = %v", got)
	}
	if _, err := Apply(root, testManifest(), updated); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "README.md"))
	if string(data) != "two\n" {
		t.Fatalf("updated content = %q", data)
	}
}

func TestInterruptedManagedUpdateConvergesFromDesiredContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initial := []recipe.Artifact{artifact("README.md", "one\n")}
	if _, err := Apply(root, testManifest(), initial); err != nil {
		t.Fatal(err)
	}
	updated := []recipe.Artifact{artifact("README.md", "two\n")}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(root, testManifest(), updated)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Actions[0].Kind != ActionUnchanged || !plan.LockChanged {
		t.Fatalf("recovery plan = %#v", plan)
	}
	result, err := Apply(root, testManifest(), updated)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Written) != 0 || !result.LockWritten {
		t.Fatalf("recovery apply = %#v", result)
	}
}

func TestPlanReportsCanonicalLockDrift(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{artifact("README.md", "one\n")}
	if _, err := Apply(root, testManifest(), artifacts); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, LockFilename)
	file, err := os.OpenFile(lockPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("# drift\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.LockChanged || plan.Actions[0].Kind != ActionUnchanged {
		t.Fatalf("lock drift plan = %#v", plan)
	}
}

func TestManagedModeDriftIsRepaired(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{artifact("README.md", "one\n")}
	if _, err := Apply(root, testManifest(), artifacts); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "README.md")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Actions[0].Kind != ActionUpdate {
		t.Fatalf("mode drift plan = %#v", plan.Actions[0])
	}
	if _, err := Apply(root, testManifest(), artifacts); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("repaired mode = %v, %v", info.Mode().Perm(), err)
	}
}

func TestRetiredManagedFileIsPlanVisibleUntilManuallyRemoved(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initial := []recipe.Artifact{artifact("keep.txt", "keep\n"), artifact("retire.txt", "retire\n")}
	if _, err := Apply(root, testManifest(), initial); err != nil {
		t.Fatal(err)
	}
	next := []recipe.Artifact{artifact("keep.txt", "keep\n")}
	plan, err := Plan(root, testManifest(), next)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.HasConflicts() || plan.Actions[1].Path != "retire.txt" || plan.Actions[1].Kind != ActionConflict {
		t.Fatalf("retirement plan = %#v", plan)
	}
	if err := os.Remove(filepath.Join(root, "retire.txt")); err != nil {
		t.Fatal(err)
	}
	plan, err = Plan(root, testManifest(), next)
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasConflicts() || !plan.LockChanged {
		t.Fatalf("post-removal plan = %#v", plan)
	}
	if _, err := Apply(root, testManifest(), next); err != nil {
		t.Fatal(err)
	}
	lock, err := LoadLock(root)
	if err != nil || len(lock.Files) != 1 || lock.Files[0].Path != "keep.txt" {
		t.Fatalf("retired lock = %#v, %v", lock, err)
	}
}

func TestManagedModificationConflictsWithoutWritingAnything(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initial := []recipe.Artifact{
		artifact("a.txt", "owned\n"),
		artifact("b.txt", "stable\n"),
	}
	if _, err := Apply(root, testManifest(), initial); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	next := []recipe.Artifact{
		artifact("a.txt", "new generated\n"),
		artifact("b.txt", "new stable\n"),
	}
	plan, err := Plan(root, testManifest(), next)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ConflictCount != 1 || plan.Actions[0].Kind != ActionConflict || plan.Actions[1].Kind != ActionUpdate {
		t.Fatalf("conflicted plan = %#v", plan.Actions)
	}
	result, err := Apply(root, testManifest(), next)
	if !errors.Is(err, ErrPlanConflicts) {
		t.Fatalf("apply error = %v", err)
	}
	if len(result.Written) != 0 || result.LockWritten {
		t.Fatalf("conflicted apply mutated: %#v", result)
	}
	b, _ := os.ReadFile(filepath.Join(root, "b.txt"))
	if string(b) != "stable\n" {
		t.Fatalf("conflict-free subset was applied: %q", b)
	}
}

func TestUnmanagedDifferingFileConflicts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte("user\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(root, testManifest(), []recipe.Artifact{artifact("README.md", "bob\n")})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Actions[0].Kind != ActionConflict {
		t.Fatalf("action = %#v", plan.Actions[0])
	}
	if _, err := Apply(root, testManifest(), []recipe.Artifact{artifact("README.md", "bob\n")}); !errors.Is(err, ErrPlanConflicts) {
		t.Fatalf("apply error = %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "user\n" {
		t.Fatalf("unmanaged file overwritten: %q", data)
	}
}

func TestIdenticalUnmanagedFileIsAdopted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts := []recipe.Artifact{artifact("README.md", "same\n")}
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Actions[0].Kind != ActionAdopt {
		t.Fatalf("action = %#v", plan.Actions[0])
	}
	result, err := Apply(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if !result.LockWritten || !reflect.DeepEqual(result.Adopted, []string{"README.md"}) || len(result.Written) != 0 {
		t.Fatalf("adoption result = %#v", result)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("adoption changed unmanaged mode to %o", info.Mode().Perm())
	}
}

func TestIdenticalUnmanagedFileWithDifferentModeConflicts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte("same\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(root, testManifest(), []recipe.Artifact{artifact("README.md", "same\n")})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Actions[0].Kind != ActionConflict || !strings.Contains(plan.Actions[0].Reason, "mode") {
		t.Fatalf("mode mismatch action = %#v", plan.Actions[0])
	}
}

func TestPlanRejectsSymlinksNonRegularFilesAndEscapes(t *testing.T) {
	t.Parallel()
	t.Run("destination symlink", func(t *testing.T) {
		root := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "linked.txt")); err != nil {
			t.Fatal(err)
		}
		_, err := Plan(root, testManifest(), []recipe.Artifact{artifact("linked.txt", "inside")})
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("symlink error = %v", err)
		}
	})
	t.Run("symlink parent", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
			t.Fatal(err)
		}
		_, err := Plan(root, testManifest(), []recipe.Artifact{artifact("linked/file.txt", "inside")})
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("parent symlink error = %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(outside, "file.txt")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("outside path was touched: %v", statErr)
		}
	})
	t.Run("directory destination", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "file.txt"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := Plan(root, testManifest(), []recipe.Artifact{artifact("file.txt", "inside")})
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("directory error = %v", err)
		}
	})
	for _, unsafe := range []string{"../escape", "/absolute", ".git/config", manifest.Filename, manifest.Filename + "/child", LockFilename, LockFilename + "/child", ApplyLockFilename} {
		unsafe := unsafe
		t.Run("unsafe-"+strings.ReplaceAll(unsafe, "/", "-"), func(t *testing.T) {
			_, err := Plan(t.TempDir(), testManifest(), []recipe.Artifact{artifact(unsafe, "bad")})
			if err == nil {
				t.Fatalf("unsafe path %q was accepted", unsafe)
			}
		})
	}
}

func TestPlanRejectsSymlinkLock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "lock")
	if err := os.WriteFile(target, []byte("schema_version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, LockFilename)); err != nil {
		t.Fatal(err)
	}
	if _, err := Plan(root, testManifest(), nil); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("symlink lock error = %v", err)
	}
}

func TestApplyRefusesExistingWorkspaceLock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ApplyLockFilename), []byte("pid: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Apply(root, testManifest(), []recipe.Artifact{artifact("README.md", "one\n")})
	if err == nil || !strings.Contains(err.Error(), "another apply is active") {
		t.Fatalf("apply lock error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "README.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("locked apply wrote artifact: %v", statErr)
	}
}
