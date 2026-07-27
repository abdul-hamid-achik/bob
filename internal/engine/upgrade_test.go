package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

// setupVersionWorkspace writes bob.yaml and applies one published artifact set,
// then rewinds bob.lock so the workspace looks like one last touched by that
// recipe version.
func setupVersionWorkspace(t *testing.T, version int) string {
	t.Helper()
	root := t.TempDir()
	m := testManifest()
	writeManifest(t, root, m)
	artifacts, err := recipe.RenderVersion(m, version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, m, artifacts); err != nil {
		t.Fatal(err)
	}
	setLockRecipeVersion(t, root, version)
	return root
}

func TestUpgradeV4ToV5(t *testing.T) {
	t.Parallel()
	root := setupVersionWorkspace(t, 4)
	m := testManifest()

	from, to, needsUpgrade, err := UpgradeStatus(root, m)
	if err != nil {
		t.Fatal(err)
	}
	if from != 4 || to != 5 || !needsUpgrade {
		t.Fatalf("status = (%d, %d, %t), want (4, 5, true)", from, to, needsUpgrade)
	}

	result, err := Upgrade(root, UpgradeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied {
		t.Fatal("expected upgrade to apply")
	}
	if result.FromVersion != 4 || result.ToVersion != 5 || result.Recipe != "go-agent-tool" {
		t.Fatalf("result = %#v", result)
	}

	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Recipe.Version != 5 {
		t.Fatalf("lock recipe version = %d, want 5", lock.Recipe.Version)
	}
	registryTest, err := os.ReadFile(filepath.Join(root, "internal/cli/registry_test.go"))
	if err != nil || !strings.Contains(string(registryTest), "TestRegisterCommandCollectsHumanOwnedFactory") {
		t.Fatalf("upgrade to v5 did not install the registry lint regression test: %v", err)
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
	if from != 5 || to != 5 || needsUpgrade {
		t.Fatalf("status = (%d, %d, %t), want (5, 5, false)", from, to, needsUpgrade)
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
	root := setupVersionWorkspace(t, 4)

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
	if lock.Recipe.Version != 4 {
		t.Fatalf("dry-run lock recipe version = %d, want 4 (unchanged)", lock.Recipe.Version)
	}
	registryTest, err := os.ReadFile(filepath.Join(root, "internal/cli/registry_test.go"))
	if err != nil || strings.Contains(string(registryTest), "TestRegisterCommandCollectsHumanOwnedFactory") {
		t.Fatalf("dry-run changed the v4 registry test: %v", err)
	}
}

func TestUpgradeRefusesConflictedWorkspace(t *testing.T) {
	t.Parallel()
	root := setupVersionWorkspace(t, 4)
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
	if lock.Recipe.Version != 4 {
		t.Fatalf("conflicted upgrade lock recipe version = %d, want 4 (unchanged)", lock.Recipe.Version)
	}
}
