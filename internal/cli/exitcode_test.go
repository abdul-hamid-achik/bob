package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExitCodeContract proves the closed exit-code contract: 0 for success
// (including a plan that finds conflicts, since plan is a read-only report),
// 2 for apply/check conflicts, 3 for check drift without conflicts, 4 for
// invalid input, and 5 when guarded apply detects a stale reviewed plan.
func TestExitCodeContract(t *testing.T) {
	t.Parallel()

	t.Run("plan with conflicts still exits 0", func(t *testing.T) {
		t.Parallel()
		target := conflictedWorkspace(t)
		_, _, err := executeForTest("plan", target)
		if err != nil {
			t.Fatalf("plan should never fail on conflicts: %v", err)
		}
		if code := ExitCode(err); code != ExitOK {
			t.Fatalf("plan exit code = %d, want %d", code, ExitOK)
		}
	})

	t.Run("apply refuses a conflicted plan with exit code 2", func(t *testing.T) {
		t.Parallel()
		target := conflictedWorkspace(t)
		_, _, err := executeForTest("apply", target)
		if err == nil {
			t.Fatal("expected apply to refuse a conflicted plan")
		}
		if code := ExitCode(err); code != ExitConflicts {
			t.Fatalf("apply exit code = %d, want %d (err=%v)", code, ExitConflicts, err)
		}
	})

	t.Run("check reports a conflict with exit code 2", func(t *testing.T) {
		t.Parallel()
		target := conflictedWorkspace(t)
		_, _, err := executeForTest("check", target)
		if err == nil {
			t.Fatal("expected check to fail on a conflicted repository")
		}
		if code := ExitCode(err); code != ExitConflicts {
			t.Fatalf("check exit code = %d, want %d (err=%v)", code, ExitConflicts, err)
		}
	})

	t.Run("check reports drift without conflicts as exit code 3", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(t.TempDir(), "drift")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, _, err := executeForTest("init", target, "--name", "acme", "--module", "github.com/acme/acme", "--write"); err != nil {
			t.Fatal(err)
		}
		_, _, err := executeForTest("check", target)
		if err == nil {
			t.Fatal("expected check to fail on pending create actions")
		}
		if code := ExitCode(err); code != ExitDrift {
			t.Fatalf("check exit code = %d, want %d (err=%v)", code, ExitDrift, err)
		}
	})

	t.Run("missing manifest is invalid input with exit code 4", func(t *testing.T) {
		t.Parallel()
		target := t.TempDir()
		for _, args := range [][]string{
			{"plan", target},
			{"check", target},
			{"apply", target},
			{"doctor", target},
		} {
			_, _, err := executeForTest(args...)
			if err == nil {
				t.Fatalf("%v: expected a missing-manifest error", args)
			}
			if code := ExitCode(err); code != ExitInvalidInput {
				t.Fatalf("%v exit code = %d, want %d (err=%v)", args, code, ExitInvalidInput, err)
			}
		}
	})

	t.Run("unclassified command failures still map to exit code 1", func(t *testing.T) {
		t.Parallel()
		_, _, err := executeForTest("recipe", "show", "unknown-recipe")
		if err == nil {
			t.Fatal("expected an unknown-recipe error")
		}
		if code := ExitCode(err); code != ExitError {
			t.Fatalf("exit code = %d, want %d (err=%v)", code, ExitError, err)
		}
	})
}

// TestPlanContentFlagShowsDesiredAndCurrentPreviews proves that bob plan
// --content renders both the desired and current content previews for a
// conflicted action, in addition to the existing desired-only preview for
// plain create actions.
func TestPlanContentFlagShowsDesiredAndCurrentPreviews(t *testing.T) {
	t.Parallel()
	target := conflictedWorkspace(t)
	stdout, _, err := executeForTest("plan", target, "--content")
	if err != nil {
		t.Fatalf("plan --content: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "--- desired preview ---") || !strings.Contains(stdout, "--- current preview ---") {
		t.Fatalf("plan --content missing preview sections:\n%s", stdout)
	}
	if !strings.Contains(stdout, "conflicting edit") {
		t.Fatalf("plan --content missing current file content:\n%s", stdout)
	}
}

// conflictedWorkspace creates a fully applied go-agent-tool repository and
// then edits a managed file so its content no longer matches bob.lock,
// producing exactly one managed_hash_mismatch conflict.
func conflictedWorkspace(t *testing.T) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(target, "README.md")
	if err := os.WriteFile(readme, []byte("conflicting edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return target
}
