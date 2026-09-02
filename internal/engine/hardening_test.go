package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

// deadPID returns the pid of a process that has already exited and been
// reaped, so a liveness probe must report it gone.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot spawn helper process: %v", err)
	}
	return cmd.Process.Pid
}

func TestApplyReclaimsStaleLockFromDeadProcessOnThisHost(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stale lock reclaim is unix-only")
	}
	t.Parallel()
	root := t.TempDir()
	host, err := os.Hostname()
	if err != nil || host == "" {
		t.Skip("hostname unavailable")
	}
	stale := fmt.Sprintf("pid: %d\nhost: %s\n", deadPID(t), host)
	if err := os.WriteFile(filepath.Join(root, ApplyLockFilename), []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Apply(root, testManifest(), []recipe.Artifact{artifact("README.md", "one\n")})
	if err != nil {
		t.Fatalf("apply should reclaim a stale lock from a dead local process: %v", err)
	}
	if len(result.Written) != 1 {
		t.Fatalf("written = %v, want README.md", result.Written)
	}
	if _, statErr := os.Stat(filepath.Join(root, ApplyLockFilename)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("apply lock should be released after reclaim: %v", statErr)
	}
}

func TestApplyKeepsLockFromAnotherHostOrLiveProcess(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"other-host":     fmt.Sprintf("pid: %d\nhost: %s\n", deadPID(t), "definitely-not-this-host.invalid"),
		"live-process":   fmt.Sprintf("pid: %d\nhost: %s\n", os.Getpid(), mustHostname(t)),
		"legacy-no-host": "pid: 1\n",
		"garbage":        "not a lock\n",
	}
	for name, content := range cases {
		content := content
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, ApplyLockFilename), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Apply(root, testManifest(), []recipe.Artifact{artifact("README.md", "one\n")})
			if err == nil || !strings.Contains(err.Error(), "another apply is active") {
				t.Fatalf("apply lock error = %v", err)
			}
			data, readErr := os.ReadFile(filepath.Join(root, ApplyLockFilename))
			if readErr != nil || string(data) != content {
				t.Fatalf("foreign or live lock was modified: %q, %v", data, readErr)
			}
		})
	}
}

func mustHostname(t *testing.T) string {
	t.Helper()
	host, err := os.Hostname()
	if err != nil || host == "" {
		t.Skip("hostname unavailable")
	}
	return host
}

func TestParseApplyLock(t *testing.T) {
	t.Parallel()
	pid, host := parseApplyLock([]byte("pid: 4242\nhost: box.local\n"))
	if pid != 4242 || host != "box.local" {
		t.Fatalf("parsed = %d/%q", pid, host)
	}
	pid, host = parseApplyLock([]byte("pid: nope\n"))
	if pid != 0 || host != "" {
		t.Fatalf("garbage parsed = %d/%q", pid, host)
	}
}

func TestEnsureParentDirectoriesReportsOnlyCreatedAndCleanupRemovesEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "existing"), 0o755); err != nil {
		t.Fatal(err)
	}
	created, err := ensureParentDirectories(root, "existing/new/deeper/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "existing", "new"), filepath.Join(root, "existing", "new", "deeper")}
	if len(created) != len(want) || created[0] != want[0] || created[1] != want[1] {
		t.Fatalf("created = %v, want %v", created, want)
	}
	// A directory that gained content stays; empty created ones go.
	if err := os.WriteFile(filepath.Join(root, "existing", "new", "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	removeCreatedDirectories(created)
	if _, err := os.Stat(filepath.Join(root, "existing", "new", "deeper")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty created directory should be removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "existing", "new")); err != nil {
		t.Fatalf("non-empty created directory must stay: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "existing")); err != nil {
		t.Fatalf("pre-existing directory must stay: %v", err)
	}
}

func TestApplyLeavesNoDirectoriesWhenRefusedByDigest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{artifact("nested/dir/file.txt", "hello\n")}
	_, err := ApplyWithOptions(root, testManifest(), artifacts, ApplyOptions{ExpectedPlanDigest: "sha256:" + strings.Repeat("0", 64)})
	if !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("expected digest mismatch, got %v", err)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("refused apply left entries behind: %v", names)
	}
}

func TestComputeEditsTrimsCommonPrefixAndSuffix(t *testing.T) {
	t.Parallel()
	old := []string{"a", "b", "c", "d", "e"}
	updated := []string{"a", "b", "X", "d", "e"}
	edits := computeEdits(old, updated)
	want := []edit{{editContext, "a"}, {editContext, "b"}, {editDelete, "c"}, {editInsert, "X"}, {editContext, "d"}, {editContext, "e"}}
	if len(edits) != len(want) {
		t.Fatalf("edits = %v, want %v", edits, want)
	}
	for i := range want {
		if edits[i] != want[i] {
			t.Fatalf("edit %d = %v, want %v", i, edits[i], want[i])
		}
	}
	// One input a prefix of the other: suffix trimming must not overlap.
	edits = computeEdits([]string{"a", "b"}, []string{"a", "b", "c"})
	if len(edits) != 3 || edits[2] != (edit{editInsert, "c"}) {
		t.Fatalf("prefix case edits = %v", edits)
	}
	edits = computeEdits([]string{"a", "a"}, []string{"a"})
	if len(edits) != 2 || edits[0].kind != editContext || edits[1].kind != editDelete {
		t.Fatalf("repeated-line case edits = %v", edits)
	}
}

func TestPlanDiffLargeFileWithLocalizedChangeStaysCheap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var b strings.Builder
	for i := 0; i < 40_000; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	original := b.String()
	updated := strings.Replace(original, "line 20000\n", "line twenty thousand\n", 1)
	if _, err := Apply(root, testManifest(), []recipe.Artifact{artifact("big.txt", original)}); err != nil {
		t.Fatal(err)
	}
	next := []recipe.Artifact{artifact("big.txt", updated)}
	plan, err := Plan(root, testManifest(), next)
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := PlanDiff(root, &plan, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 || diffs[0].Note != "" {
		t.Fatalf("expected a real diff, got %+v", diffs)
	}
	if !strings.Contains(diffs[0].Unified, "-line 20000") || !strings.Contains(diffs[0].Unified, "+line twenty thousand") {
		t.Fatalf("unified diff missing the change:\n%.400s", diffs[0].Unified)
	}
}

func TestPlanDiffSkipsWhenChangedRegionExceedsCellBudget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var oldB, newB strings.Builder
	for i := 0; i < 3000; i++ {
		fmt.Fprintf(&oldB, "old %d\n", i)
		fmt.Fprintf(&newB, "new %d\n", i)
	}
	if _, err := Apply(root, testManifest(), []recipe.Artifact{artifact("wide.txt", oldB.String())}); err != nil {
		t.Fatal(err)
	}
	next := []recipe.Artifact{artifact("wide.txt", newB.String())}
	plan, err := Plan(root, testManifest(), next)
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := PlanDiff(root, &plan, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 || !strings.Contains(diffs[0].Note, "diff budget") {
		t.Fatalf("expected a budget note, got %+v", diffs)
	}
	if diffs[0].Unified != "" || diffs[0].OldLines != nil {
		t.Fatalf("skipped diff must carry no content: %+v", diffs[0])
	}
}
