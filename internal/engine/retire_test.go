package engine

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

// retireFixture applies two files and returns a workspace root where the
// caller can plan with only keep.txt retained, leaving retire.txt a
// lock-owned path the recipe no longer declares.
func retireFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := Apply(root, testManifest(), []recipe.Artifact{
		artifact("keep.txt", "keep\n"),
		artifact("retire.txt", "retire\n"),
	}); err != nil {
		t.Fatal(err)
	}
	return root
}

func retainedArtifacts() []recipe.Artifact {
	return []recipe.Artifact{artifact("keep.txt", "keep\n")}
}

func findAction(plan PlanResult, path string) *Action {
	for i := range plan.Actions {
		if plan.Actions[i].Path == path {
			return &plan.Actions[i]
		}
	}
	return nil
}

func assertRetiredConflict(t *testing.T, plan PlanResult, path, code string) {
	t.Helper()
	if !plan.HasConflicts() {
		t.Fatalf("plan unexpectedly conflict-free: %#v", plan.Actions)
	}
	action := findAction(plan, path)
	if action == nil || action.Kind != ActionConflict || action.Code != code {
		t.Fatalf("action for %q = %#v, want conflict/%s", path, action, code)
	}
}

// TestPlanDefaultRetiredPathIsUnchanged proves the default is byte-identical:
// Plan (no options) still classifies a dropped lock-owned path as a
// retired_owned conflict, and PlanWithOptions with the zero options yields the
// same plan digest.
func TestPlanDefaultRetiredPathIsUnchanged(t *testing.T) {
	t.Parallel()
	root := retireFixture(t)
	plan, err := Plan(root, testManifest(), retainedArtifacts())
	if err != nil {
		t.Fatal(err)
	}
	if !plan.HasConflicts() {
		t.Fatalf("default plan unexpectedly conflict-free: %#v", plan.Actions)
	}
	assertRetiredConflict(t, plan, "retire.txt", CodeRetiredOwned)

	withOpts, err := PlanWithOptions(root, testManifest(), retainedArtifacts(), PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if DigestPlan(plan) != DigestPlan(withOpts) {
		t.Fatalf("Plan and PlanWithOptions(zero) digests differ: %#v vs %#v", DigestPlan(plan), DigestPlan(withOpts))
	}
}

// TestPlanAllowRetireClassifiesRetireAction proves the opt-in reclassifies a
// lock-owned, hash-matching regular file as a non-conflict retire action.
func TestPlanAllowRetireClassifiesRetireAction(t *testing.T) {
	t.Parallel()
	root := retireFixture(t)
	plan, err := PlanWithOptions(root, testManifest(), retainedArtifacts(), PlanOptions{AllowRetire: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasConflicts() {
		t.Fatalf("allow-retire plan has conflicts: %#v", plan.Actions)
	}
	action := findAction(plan, "retire.txt")
	if action == nil || action.Kind != ActionRetire || action.Code != CodeRetiredUnchanged {
		t.Fatalf("retired path action = %#v, want retire/retired_unchanged", action)
	}
	if action.LockedSHA256 == "" || action.CurrentSHA256 != action.LockedSHA256 {
		t.Fatalf("retire action hash mismatch: %#v", action)
	}
	if counts := plan.ActionCounts(); counts.Retire != 1 || counts.Conflict != 0 {
		t.Fatalf("action counts = %#v", counts)
	}
}

// TestApplyAllowRetireDeletesAndDropsLock proves a retire action deletes only
// the matching lock-owned regular file, keeps other files, and drops the path
// from bob.lock (still written last).
func TestApplyAllowRetireDeletesAndDropsLock(t *testing.T) {
	t.Parallel()
	root := retireFixture(t)
	result, err := ApplyWithOptions(root, testManifest(), retainedArtifacts(), ApplyOptions{AllowRetire: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Written) != 0 || len(result.Adopted) != 0 {
		t.Fatalf("unexpected writes: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(root, "retire.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retired file still exists: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "keep.txt")); err != nil || string(data) != "keep\n" {
		t.Fatalf("keep.txt = %q, %v", data, err)
	}
	if !result.LockWritten {
		t.Fatalf("lock not written after retire: %#v", result)
	}
	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Files) != 1 || lock.Files[0].Path != "keep.txt" {
		t.Fatalf("lock files = %#v, want only keep.txt", lock.Files)
	}
}

// TestApplyWithoutAllowRetireRefusesRetire proves that without the opt-in a
// dropped path blocks the entire apply exactly as before.
func TestApplyWithoutAllowRetireRefusesRetire(t *testing.T) {
	t.Parallel()
	root := retireFixture(t)
	result, err := ApplyWithOptions(root, testManifest(), retainedArtifacts(), ApplyOptions{})
	if !errors.Is(err, ErrPlanConflicts) {
		t.Fatalf("apply without --allow-retire error = %v", err)
	}
	if len(result.Written) != 0 || result.LockWritten {
		t.Fatalf("refused apply mutated: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(root, "retire.txt")); err != nil {
		t.Fatalf("retired file was deleted on refusal: %v", err)
	}
}

// TestAllowRetireStillConflictsOnDriftedSymlinkAndSpecial proves a drifted
// regular file, a symlink, and a special file at a retired path all remain
// conflicts even with --allow-retire, so apply never deletes them.
func TestAllowRetireStillConflictsOnDriftedSymlinkAndSpecial(t *testing.T) {
	t.Parallel()
	t.Run("drifted regular file", func(t *testing.T) {
		root := retireFixture(t)
		if err := os.WriteFile(filepath.Join(root, "retire.txt"), []byte("drifted\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		plan, err := PlanWithOptions(root, testManifest(), retainedArtifacts(), PlanOptions{AllowRetire: true})
		if err != nil {
			t.Fatal(err)
		}
		assertRetiredConflict(t, plan, "retire.txt", CodeRetiredOwned)
		if _, err := ApplyWithOptions(root, testManifest(), retainedArtifacts(), ApplyOptions{AllowRetire: true}); !errors.Is(err, ErrPlanConflicts) {
			t.Fatalf("drifted retire apply error = %v", err)
		}
		if data, err := os.ReadFile(filepath.Join(root, "retire.txt")); err != nil || string(data) != "drifted\n" {
			t.Fatalf("drifted file = %q, %v", data, err)
		}
	})
	t.Run("symlink", func(t *testing.T) {
		root := retireFixture(t)
		if err := os.Remove(filepath.Join(root, "retire.txt")); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "retire.txt")); err != nil {
			t.Fatal(err)
		}
		plan, err := PlanWithOptions(root, testManifest(), retainedArtifacts(), PlanOptions{AllowRetire: true})
		if err != nil {
			t.Fatal(err)
		}
		assertRetiredConflict(t, plan, "retire.txt", CodeSymlink)
	})
	t.Run("special file", func(t *testing.T) {
		root := retireFixture(t)
		if err := os.Remove(filepath.Join(root, "retire.txt")); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(filepath.Join(root, "retire.txt"), 0o644); err != nil {
			t.Skipf("mkfifo unavailable: %v", err)
		}
		plan, err := PlanWithOptions(root, testManifest(), retainedArtifacts(), PlanOptions{AllowRetire: true})
		if err != nil {
			t.Fatal(err)
		}
		assertRetiredConflict(t, plan, "retire.txt", CodeSpecialFile)
	})
}

// TestApplyAllowRetireCleansEmptyDirectories proves a retired file in a nested
// directory leaves no empty parent directories behind.
func TestApplyAllowRetireCleansEmptyDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := Apply(root, testManifest(), []recipe.Artifact{
		artifact("keep.txt", "keep\n"),
		artifact("nested/retire.txt", "retire\n"),
	}); err != nil {
		t.Fatal(err)
	}
	_, err := ApplyWithOptions(root, testManifest(), retainedArtifacts(), ApplyOptions{AllowRetire: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "nested")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty directory was not cleaned up: %v", err)
	}
}
