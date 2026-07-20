package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

// setupV3Workspace writes bob.yaml and applies the go-agent-tool v3 artifact
// set, then rewinds bob.lock to recipe version 3 so the workspace looks like
// one last touched by a v3 binary.
func setupV3Workspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	m := testManifest()
	writeManifest(t, root, m)
	v3Artifacts, err := recipe.RenderVersion(m, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, m, v3Artifacts); err != nil {
		t.Fatal(err)
	}
	setLockRecipeVersion(t, root, 3)
	return root
}

func TestUpgradeV3ToV4(t *testing.T) {
	t.Parallel()
	root := setupV3Workspace(t)
	m := testManifest()

	from, to, needsUpgrade, err := UpgradeStatus(root, m)
	if err != nil {
		t.Fatal(err)
	}
	if from != 3 || to != 4 || !needsUpgrade {
		t.Fatalf("status = (%d, %d, %t), want (3, 4, true)", from, to, needsUpgrade)
	}

	result, err := Upgrade(root, UpgradeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied {
		t.Fatal("expected upgrade to apply")
	}
	if result.FromVersion != 3 || result.ToVersion != 4 || result.Recipe != "go-agent-tool" {
		t.Fatalf("result = %#v", result)
	}

	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Recipe.Version != 4 {
		t.Fatalf("lock recipe version = %d, want 4", lock.Recipe.Version)
	}
	// v4 adds the registry artifacts that v3 did not declare.
	if !exists(t, filepath.Join(root, "internal/cli/registry.go")) {
		t.Fatal("upgrade to v4 must create internal/cli/registry.go")
	}
}

func TestUpgradeAlreadyCurrentIsNoOp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m := testManifest()
	writeManifest(t, root, m)
	artifacts, err := recipe.Render(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, m, artifacts); err != nil {
		t.Fatal(err)
	}

	from, to, needsUpgrade, err := UpgradeStatus(root, m)
	if err != nil {
		t.Fatal(err)
	}
	if from != 4 || to != 4 || needsUpgrade {
		t.Fatalf("status = (%d, %d, %t), want (4, 4, false)", from, to, needsUpgrade)
	}

	result, err := Upgrade(root, UpgradeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied || result.Actions != 0 || len(result.Written) != 0 {
		t.Fatalf("no-op upgrade result = %#v", result)
	}
}

func TestUpgradeNoLockErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeManifest(t, root, testManifest())

	if _, _, _, err := UpgradeStatus(root, testManifest()); !errors.Is(err, ErrUpgradeNoLock) {
		t.Fatalf("status error = %v, want ErrUpgradeNoLock", err)
	}
	if _, err := Upgrade(root, UpgradeOptions{}); !errors.Is(err, ErrUpgradeNoLock) {
		t.Fatalf("upgrade error = %v, want ErrUpgradeNoLock", err)
	}
}

func TestUpgradeDryRunMutatesNothing(t *testing.T) {
	t.Parallel()
	root := setupV3Workspace(t)

	result, err := Upgrade(root, UpgradeOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied {
		t.Fatal("dry-run must not apply")
	}
	if result.Actions == 0 {
		t.Fatal("dry-run should report the files it would write")
	}

	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Recipe.Version != 3 {
		t.Fatalf("dry-run lock recipe version = %d, want 3 (unchanged)", lock.Recipe.Version)
	}
	if exists(t, filepath.Join(root, "internal/cli/registry.go")) {
		t.Fatal("dry-run must not create internal/cli/registry.go")
	}
}

func TestUpgradeRefusesConflictedWorkspace(t *testing.T) {
	t.Parallel()
	root := setupV3Workspace(t)
	// Drift a managed file so the migration plan conflicts.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Upgrade(root, UpgradeOptions{})
	if !errors.Is(err, ErrPlanConflicts) {
		t.Fatalf("upgrade error = %v, want ErrPlanConflicts", err)
	}
	if result == nil || result.Plan.ConflictCount == 0 {
		t.Fatalf("expected conflict detail in result, got %#v", result)
	}

	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Recipe.Version != 3 {
		t.Fatalf("conflicted upgrade lock recipe version = %d, want 3 (unchanged)", lock.Recipe.Version)
	}
}
