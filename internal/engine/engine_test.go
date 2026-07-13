package engine

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
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

func setLockRecipeVersion(t *testing.T, root string, version int) {
	t.Helper()
	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Recipe.Version = version
	data, err := encodeLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, LockFilename), data, 0o644); err != nil {
		t.Fatal(err)
	}
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

func TestPlanAndApplyUpgradeOlderRecipeLock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m := testManifest()
	v1Artifacts := []recipe.Artifact{artifact("README.md", "version one\n")}
	if _, err := Apply(root, m, v1Artifacts); err != nil {
		t.Fatal(err)
	}
	setLockRecipeVersion(t, root, RecipeVersion-1)

	v2Artifacts := []recipe.Artifact{
		artifact("README.md", "version two\n"),
		artifact("CODE_OF_CONDUCT.md", "community\n"),
	}
	plan, err := Plan(root, m, v2Artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.LockChanged || plan.Recipe.Version != RecipeVersion {
		t.Fatalf("upgrade plan = %#v", plan)
	}
	actions := make(map[string]ActionKind, len(plan.Actions))
	for _, action := range plan.Actions {
		actions[action.Path] = action.Kind
	}
	if actions["README.md"] != ActionUpdate || actions["CODE_OF_CONDUCT.md"] != ActionCreate {
		t.Fatalf("upgrade actions = %#v", actions)
	}
	if _, err := Apply(root, m, v2Artifacts); err != nil {
		t.Fatal(err)
	}
	upgraded, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.Recipe.Version != RecipeVersion {
		t.Fatalf("lock version = %d, want %d", upgraded.Recipe.Version, RecipeVersion)
	}
	for path, want := range map[string]string{
		"README.md":          "version two\n",
		"CODE_OF_CONDUCT.md": "community\n",
	} {
		got, readErr := os.ReadFile(filepath.Join(root, path))
		if readErr != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; want %q", path, got, readErr, want)
		}
	}
}

func TestRecipeUpgradeRefusesHumanModifiedManagedFileWithoutPartialWrites(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m := testManifest()
	v1Artifacts := []recipe.Artifact{artifact("README.md", "version one\n")}
	if _, err := Apply(root, m, v1Artifacts); err != nil {
		t.Fatal(err)
	}
	setLockRecipeVersion(t, root, RecipeVersion-1)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.ReadFile(filepath.Join(root, LockFilename))
	if err != nil {
		t.Fatal(err)
	}
	v2Artifacts := []recipe.Artifact{
		artifact("README.md", "version two\n"),
		artifact("CODE_OF_CONDUCT.md", "community\n"),
	}
	plan, err := Plan(root, m, v2Artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.HasConflicts() {
		t.Fatalf("human-modified upgrade plan has no conflict: %#v", plan)
	}
	if _, err := Apply(root, m, v2Artifacts); err == nil {
		t.Fatal("conflicted recipe upgrade unexpectedly applied")
	}
	if _, err := os.Stat(filepath.Join(root, "CODE_OF_CONDUCT.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("conflicted upgrade created new artifact: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil || string(readme) != "human edit\n" {
		t.Fatalf("human file changed: %q, %v", readme, err)
	}
	lockAfter, err := os.ReadFile(filepath.Join(root, LockFilename))
	if err != nil || !reflect.DeepEqual(lockAfter, lockBefore) {
		t.Fatalf("conflicted upgrade changed lock: %v", err)
	}
}

func TestPlanRejectsNewerRecipeLock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m := testManifest()
	artifacts := []recipe.Artifact{artifact("README.md", "hello\n")}
	if _, err := Apply(root, m, artifacts); err != nil {
		t.Fatal(err)
	}
	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Recipe.Version = RecipeVersion + 1
	data, err := encodeLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, LockFilename), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Plan(root, m, artifacts); err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("expected newer lock rejection, got %v", err)
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

// TestPlanRejectsSymlinksNonRegularFilesAndEscapes proves that a symlink or
// non-regular file at a destination (or a symlinked path component) is a
// per-path ActionConflict that Plan reports without aborting, while apply
// still refuses to write anything. Path escapes and reserved-path artifacts
// remain hard Plan failures: those are recipe bugs, not workspace states.
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
		artifacts := []recipe.Artifact{artifact("linked.txt", "inside")}
		plan, err := Plan(root, testManifest(), artifacts)
		if err != nil {
			t.Fatalf("plan should not hard-fail on a destination symlink: %v", err)
		}
		if !plan.HasConflicts() || plan.Actions[0].Kind != ActionConflict || plan.Actions[0].Code != CodeSymlink || !strings.Contains(plan.Actions[0].Reason, "symlink") {
			t.Fatalf("destination symlink action = %#v", plan.Actions[0])
		}
		if _, err := Apply(root, testManifest(), artifacts); !errors.Is(err, ErrPlanConflicts) {
			t.Fatalf("apply should refuse a symlinked destination: %v", err)
		}
		data, readErr := os.ReadFile(outside)
		if readErr != nil || string(data) != "outside" {
			t.Fatalf("symlink target was touched: %q, %v", data, readErr)
		}
	})
	t.Run("symlink parent", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
			t.Fatal(err)
		}
		artifacts := []recipe.Artifact{artifact("linked/file.txt", "inside")}
		plan, err := Plan(root, testManifest(), artifacts)
		if err != nil {
			t.Fatalf("plan should not hard-fail on a symlinked parent: %v", err)
		}
		if !plan.HasConflicts() || plan.Actions[0].Kind != ActionConflict || plan.Actions[0].Code != CodeSymlink || !strings.Contains(plan.Actions[0].Reason, "symlink") {
			t.Fatalf("symlinked parent action = %#v", plan.Actions[0])
		}
		if _, statErr := os.Stat(filepath.Join(outside, "file.txt")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("outside path was touched: %v", statErr)
		}
		if _, err := Apply(root, testManifest(), artifacts); !errors.Is(err, ErrPlanConflicts) {
			t.Fatalf("apply should refuse a symlinked parent: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(outside, "file.txt")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("outside path was touched after apply: %v", statErr)
		}
	})
	t.Run("directory destination", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "file.txt"), 0o755); err != nil {
			t.Fatal(err)
		}
		artifacts := []recipe.Artifact{artifact("file.txt", "inside")}
		plan, err := Plan(root, testManifest(), artifacts)
		if err != nil {
			t.Fatalf("plan should not hard-fail on a directory destination: %v", err)
		}
		if !plan.HasConflicts() || plan.Actions[0].Kind != ActionConflict || plan.Actions[0].Code != CodeSpecialFile {
			t.Fatalf("directory destination action = %#v", plan.Actions[0])
		}
		if _, err := Apply(root, testManifest(), artifacts); !errors.Is(err, ErrPlanConflicts) {
			t.Fatalf("apply should refuse a directory destination: %v", err)
		}
		info, statErr := os.Stat(filepath.Join(root, "file.txt"))
		if statErr != nil || !info.IsDir() {
			t.Fatalf("directory destination was replaced: %v, %v", info, statErr)
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

// TestPlanConflictsOnFifoDestination covers a destination that exists as a
// special file that is neither a symlink nor a directory. Skips if the
// platform running the test cannot create a FIFO.
func TestPlanConflictsOnFifoDestination(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fifoPath := filepath.Join(root, "pipe.txt")
	if err := syscall.Mkfifo(fifoPath, 0o644); err != nil {
		t.Skipf("mkfifo unavailable on this platform: %v", err)
	}
	artifacts := []recipe.Artifact{artifact("pipe.txt", "inside")}
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatalf("plan should not hard-fail on a fifo destination: %v", err)
	}
	if !plan.HasConflicts() || plan.Actions[0].Kind != ActionConflict || plan.Actions[0].Code != CodeSpecialFile || !strings.Contains(plan.Actions[0].Reason, "not a regular file") {
		t.Fatalf("fifo destination action = %#v", plan.Actions[0])
	}
	if _, err := Apply(root, testManifest(), artifacts); !errors.Is(err, ErrPlanConflicts) {
		t.Fatalf("apply should refuse a fifo destination: %v", err)
	}
}

// TestActionsCarryCodeAndCurrentPreview proves that every plan action has a
// closed-vocabulary Code matching its Kind, and that CurrentPreview is
// populated exactly for conflict/update actions on an existing regular file.
func TestActionsCarryCodeAndCurrentPreview(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := Apply(root, testManifest(), []recipe.Artifact{
		artifact("stable.txt", "stable\n"),
		artifact("retire.txt", "retire\n"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "stable.txt"), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	next := []recipe.Artifact{
		artifact("create.txt", "created\n"),
		artifact("stable.txt", "stable v2\n"),
	}
	plan, err := Plan(root, testManifest(), next)
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]Action, len(plan.Actions))
	for _, action := range plan.Actions {
		byPath[action.Path] = action
	}
	create := byPath["create.txt"]
	if create.Kind != ActionCreate || create.Code != CodeMissing || create.CurrentPreview != "" {
		t.Fatalf("create action = %#v", create)
	}
	conflict := byPath["stable.txt"]
	if conflict.Kind != ActionConflict || conflict.Code != CodeManagedHashMismatch || conflict.CurrentPreview != "human edit\n" {
		t.Fatalf("conflict action = %#v", conflict)
	}
	retired := byPath["retire.txt"]
	if retired.Kind != ActionConflict || retired.Code != CodeRetiredOwned || retired.CurrentPreview != "retire\n" {
		t.Fatalf("retired action = %#v", retired)
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

func filesTestManifest(content string) manifest.Manifest {
	return manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Recipe:        manifest.RecipeFiles,
		Product:       manifest.Product{Name: "demo-app", Description: "A demo app"},
		Vars:          map[string]string{"greeting": "hello"},
		Files: []manifest.FileDecl{
			{Path: "greeting.txt", Content: content},
			{Path: "scripts/run.sh", Mode: "0755", Content: "#!/usr/bin/env bash\necho ${vars.greeting}\n"},
		},
	}
}

func TestFilesRecipeFullLifecycleConverges(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m := filesTestManifest("${vars.greeting}, world\n")
	artifacts, err := recipe.Render(m)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := Plan(root, m, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasConflicts() || plan.Recipe.ID != "files" || plan.Recipe.Version != recipe.FilesRecipeVersion {
		t.Fatalf("create plan = %#v", plan)
	}
	for _, action := range plan.Actions {
		if action.Kind != ActionCreate {
			t.Fatalf("fresh files action = %#v, want create", action)
		}
	}

	if _, err := Apply(root, m, artifacts); err != nil {
		t.Fatal(err)
	}
	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Recipe.ID != "files" || lock.Recipe.Version != 1 {
		t.Fatalf("lock recipe = %#v, want files@1", lock.Recipe)
	}
	info, err := os.Stat(filepath.Join(root, "scripts", "run.sh"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("script mode = %v, %v, want 0755", info, err)
	}

	converged, err := Plan(root, m, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if converged.HasConflicts() || converged.LockChanged {
		t.Fatalf("converged plan = %#v", converged)
	}
	for _, action := range converged.Actions {
		if action.Kind != ActionUnchanged {
			t.Fatalf("converged action = %#v, want unchanged", action)
		}
	}

	updatedManifest := filesTestManifest("${vars.greeting}, updated world\n")
	updatedArtifacts, err := recipe.Render(updatedManifest)
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := Plan(root, updatedManifest, updatedArtifacts)
	if err != nil {
		t.Fatal(err)
	}
	if updatePlan.HasConflicts() {
		t.Fatalf("update plan has conflicts: %#v", updatePlan)
	}
	actions := make(map[string]ActionKind, len(updatePlan.Actions))
	for _, action := range updatePlan.Actions {
		actions[action.Path] = action.Kind
	}
	if actions["greeting.txt"] != ActionUpdate {
		t.Fatalf("expected greeting.txt update, got %#v", actions)
	}

	if _, err := Apply(root, updatedManifest, updatedArtifacts); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "greeting.txt"))
	if err != nil || string(data) != "hello, updated world\n" {
		t.Fatalf("updated content = %q, %v", data, err)
	}

	final, err := Plan(root, updatedManifest, updatedArtifacts)
	if err != nil {
		t.Fatal(err)
	}
	if final.HasConflicts() || final.LockChanged {
		t.Fatalf("final converged plan = %#v", final)
	}
	for _, action := range final.Actions {
		if action.Kind != ActionUnchanged {
			t.Fatalf("final action = %#v, want unchanged", action)
		}
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
