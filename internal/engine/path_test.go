package engine

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

func TestClassifyPathUsesPlanAndLockOwnership(t *testing.T) {
	root := t.TempDir()
	m := testManifest()
	artifacts := []recipe.Artifact{artifact("managed.txt", "owned\n")}
	if _, err := Apply(root, m, artifacts); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(root, m, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	assertPathClassification(t, root, plan, "managed.txt", PathClassificationManaged, PathStateManagedInSync, HumanEditWillConflict)
	assertPathClassification(t, root, plan, "missing.txt", PathClassificationMissing, PathStateUnmanagedMissing, HumanEditOutsideOwnership)
	if err := os.WriteFile(filepath.Join(root, "human.txt"), []byte("human\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertPathClassification(t, root, plan, "human.txt", PathClassificationUnmanaged, PathStateUnmanagedPresent, HumanEditOutsideOwnership)

	if err := os.WriteFile(filepath.Join(root, "managed.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err = Plan(root, m, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	result := assertPathClassification(t, root, plan, "managed.txt", PathClassificationManaged, PathStateManagedModified, HumanEditWillConflict)
	if result.PlanAction == nil || result.PlanAction.Code != CodeManagedHashMismatch {
		t.Fatalf("plan action = %#v", result.PlanAction)
	}
}

func TestClassifyPathMissingDesiredAndRetiredOwned(t *testing.T) {
	root := t.TempDir()
	m := testManifest()
	desired := []recipe.Artifact{artifact("future.txt", "future\n")}
	plan, err := Plan(root, m, desired)
	if err != nil {
		t.Fatal(err)
	}
	assertPathClassification(t, root, plan, "future.txt", PathClassificationManaged, PathStateManagedMissing, HumanEditWillConflict)

	if _, err := Apply(root, m, desired); err != nil {
		t.Fatal(err)
	}
	retiredPlan, err := Plan(root, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	retired := assertPathClassification(t, root, retiredPlan, "future.txt", PathClassificationManaged, PathStateRetiredOwned, HumanEditReservedForBob)
	if retired.PlanAction == nil || retired.PlanAction.Code != CodeRetiredOwned {
		t.Fatalf("retired action = %#v", retired.PlanAction)
	}
}

func TestClassifyPathReservedUnsafeAndSpecialTargets(t *testing.T) {
	root := t.TempDir()
	plan, err := Plan(root, testManifest(), nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPathClassification(t, root, plan, "bob.yaml", PathClassificationReserved, PathStateReserved, HumanEditRequiresManifest)
	assertPathClassification(t, root, plan, "bob.lock", PathClassificationReserved, PathStateReserved, HumanEditReservedForBob)
	assertPathClassification(t, root, plan, ".git/config", PathClassificationReserved, PathStateReserved, HumanEditReservedForBob)
	for _, unsafe := range []string{"/absolute", "../escape", ".", ""} {
		if _, err := ClassifyPath(root, plan, unsafe); err == nil {
			t.Fatalf("ClassifyPath(%q) succeeded", unsafe)
		}
	}
	if err := os.Symlink("elsewhere", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	assertPathClassification(t, root, plan, "link", PathClassificationUnmanaged, PathStateSymlink, HumanEditUnsafe)
	throughLink := assertPathClassification(t, root, plan, "link/secret", PathClassificationUnmanaged, PathStateSymlink, HumanEditUnsafe)
	if throughLink.Exists {
		t.Fatal("ancestor symlink obstruction was reported as the exact target existing")
	}
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertPathClassification(t, root, plan, "pipe", PathClassificationUnmanaged, PathStateSpecialFile, HumanEditUnsafe)
	throughPipe := assertPathClassification(t, root, plan, "pipe/child", PathClassificationUnmanaged, PathStateSpecialFile, HumanEditUnsafe)
	if throughPipe.Exists {
		t.Fatal("ancestor special-file obstruction was reported as the exact target existing")
	}
}

func TestNormalizeRepositoryPathRejectsInvalidUTF8(t *testing.T) {
	if _, err := NormalizeRepositoryPath(string([]byte{'x', 0xff})); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("error = %v", err)
	}
	if _, err := NormalizeRepositoryPath(strings.Repeat("a", 4097)); err == nil || strings.Contains(err.Error(), strings.Repeat("a", 256)) {
		t.Fatalf("oversized error leaked input: %v", err)
	}
}

func assertPathClassification(t *testing.T, root string, plan PlanResult, path, classification, state, effect string) PathClassification {
	t.Helper()
	result, err := ClassifyPath(root, plan, path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Classification != classification || result.State != state || result.HumanEditEffect != effect {
		t.Fatalf("%s = %#v; want classification=%s state=%s effect=%s", path, result, classification, state, effect)
	}
	return result
}
