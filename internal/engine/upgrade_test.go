package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
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
	if from != 4 || to != 6 || !needsUpgrade {
		t.Fatalf("status = (%d, %d, %t), want (4, 6, true)", from, to, needsUpgrade)
	}

	result, err := Upgrade(root, UpgradeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied {
		t.Fatal("expected upgrade to apply")
	}
	if result.FromVersion != 4 || result.ToVersion != 6 || result.Recipe != "go-agent-tool" {
		t.Fatalf("result = %#v", result)
	}

	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Recipe.Version != 6 {
		t.Fatalf("lock recipe version = %d, want 6", lock.Recipe.Version)
	}
	registryTest, err := os.ReadFile(filepath.Join(root, "internal/cli/registry_test.go"))
	if err != nil || !strings.Contains(string(registryTest), "TestRegisterCommandCollectsHumanOwnedFactory") {
		t.Fatalf("upgrade to v5 did not install the registry lint regression test: %v", err)
	}
}

func TestUpgradeStackV1ToV2NeverRewritesSeeds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m, err := manifest.DefaultStack(manifest.RecipeTSApp, "demo", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	writeManifest(t, root, m)
	v1, err := recipe.RenderVersion(m, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, m, v1); err != nil {
		t.Fatal(err)
	}
	setLockRecipeVersion(t, root, 1)
	readmePath := filepath.Join(root, "README.md")
	before, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), "ts-app@1") {
		t.Fatalf("fixture is not version 1:\n%s", before)
	}

	result, err := Upgrade(root, UpgradeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.FromVersion != 1 || result.ToVersion != 2 {
		t.Fatalf("unexpected upgrade result: %#v", result)
	}
	after, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("stack upgrade rewrote a seed-once README")
	}
	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Recipe.Version != 2 || len(lock.Files) != 0 {
		t.Fatalf("stack lock = %#v, want version 2 with no owned files", lock)
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
	if from != 6 || to != 6 || needsUpgrade {
		t.Fatalf("status = (%d, %d, %t), want (6, 6, false)", from, to, needsUpgrade)
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
	if err := os.WriteFile(filepath.Join(root, "internal", "cli", "root.go"), []byte("package cli\n"), 0o644); err != nil {
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
