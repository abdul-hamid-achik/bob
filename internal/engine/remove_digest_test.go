package engine

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

var qualifiedDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func digestWorkspace(t *testing.T) string {
	t.Helper()
	return setupManagedWorkspace(t, []recipe.Artifact{
		artifact("cmd/acme/main.go", "package main\n"),
		artifact("README.md", "hello\n"),
	})
}

func dryRunDigest(t *testing.T, root string, force bool) string {
	t.Helper()
	result, err := Remove(root, RemoveOptions{DryRun: true, Force: force})
	if err != nil {
		t.Fatal(err)
	}
	if !qualifiedDigest.MatchString(result.RemovalDigest) || result.RemovalDigestVersion != RemovalDigestVersion {
		t.Fatalf("dry-run digest = %q v%d", result.RemovalDigest, result.RemovalDigestVersion)
	}
	return result.RemovalDigest
}

func TestRemoveDigestIsStableAcrossDryRunAndRealRun(t *testing.T) {
	t.Parallel()
	root := digestWorkspace(t)
	digest := dryRunDigest(t, root, false)
	if again := dryRunDigest(t, root, false); again != digest {
		t.Fatalf("repeated dry-run digest changed: %s != %s", again, digest)
	}
	result, err := Remove(root, RemoveOptions{ExpectedRemovalDigest: digest})
	if err != nil {
		t.Fatal(err)
	}
	if result.RemovalDigest != digest || result.RemovalDigestVersion != RemovalDigestVersion {
		t.Fatalf("real-run digest = %q, want %q", result.RemovalDigest, digest)
	}
	if want := []string{"README.md", "cmd/acme/main.go"}; !reflect.DeepEqual(result.Removed, want) || !result.LockRemoved {
		t.Fatalf("result = %#v", result)
	}
}

func TestRemoveDigestMismatchAfterEditDeletesNothing(t *testing.T) {
	t.Parallel()
	root := digestWorkspace(t)
	digest := dryRunDigest(t, root, true)
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.ReadFile(filepath.Join(root, LockFilename))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Remove(root, RemoveOptions{Force: true, ExpectedRemovalDigest: digest})
	if result != nil {
		t.Fatalf("mismatch must not return a result: %#v", result)
	}
	var mismatch *RemovalDigestMismatchError
	if !errors.As(err, &mismatch) || !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("err = %v, want removal digest mismatch", err)
	}
	if mismatch.ExpectedRemovalDigest != digest || mismatch.ActualRemovalDigest == digest || !qualifiedDigest.MatchString(mismatch.ActualRemovalDigest) {
		t.Fatalf("mismatch = %#v", mismatch)
	}
	if got, _ := os.ReadFile(readme); string(got) != "human edit\n" {
		t.Fatalf("edited file changed: %q", got)
	}
	if !exists(t, filepath.Join(root, "cmd/acme/main.go")) {
		t.Fatal("mismatch must delete nothing")
	}
	lockAfter, err := os.ReadFile(filepath.Join(root, LockFilename))
	if err != nil || !bytes.Equal(lockBefore, lockAfter) {
		t.Fatalf("bob.lock changed on mismatch: %v", err)
	}
}

func TestRemoveDigestBindsForceFlag(t *testing.T) {
	t.Parallel()
	root := digestWorkspace(t)
	withoutForce := dryRunDigest(t, root, false)
	if withForce := dryRunDigest(t, root, true); withForce == withoutForce {
		t.Fatal("force flag must change the removal digest")
	}
	_, err := Remove(root, RemoveOptions{Force: true, ExpectedRemovalDigest: withoutForce})
	if !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("err = %v, want mismatch", err)
	}
	if !exists(t, filepath.Join(root, "README.md")) || !exists(t, filepath.Join(root, LockFilename)) {
		t.Fatal("mismatch must delete nothing")
	}
}

func TestRemoveDigestBindsLockBytes(t *testing.T) {
	t.Parallel()
	root := digestWorkspace(t)
	digest := dryRunDigest(t, root, false)
	lockPath := filepath.Join(root, LockFilename)
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if again := dryRunDigest(t, root, false); again == digest {
		t.Fatal("lock bytes must change the removal digest")
	}
}

func TestRemoveDigestDiffersFromPlanDigestNamespace(t *testing.T) {
	t.Parallel()
	a := digestRemoval([]byte("x"), false, nil)
	b := digestRemoval([]byte("x"), true, nil)
	c := digestRemoval([]byte("y"), false, nil)
	if a == b || a == c || !qualifiedDigest.MatchString(a) {
		t.Fatalf("digests = %s %s %s", a, b, c)
	}
	entry := removalDigestEntry{Path: "a", LockedSHA256: "1", CurrentSHA256: "1", CurrentMode: "0644", Decision: removeEligible.String()}
	skipped := entry
	skipped.Decision = removeSkipped.String()
	if digestRemoval(nil, false, []removalDigestEntry{entry}) == digestRemoval(nil, false, []removalDigestEntry{skipped}) {
		t.Fatal("decision must change the removal digest")
	}
}

func TestRemoveRejectsMalformedDigestBeforeTouchingWorkspace(t *testing.T) {
	t.Parallel()
	valid := "sha256:" + string(bytes.Repeat([]byte("a"), 64))
	for _, value := range []string{
		"abc",
		string(bytes.Repeat([]byte("a"), 64)),
		"sha256:" + string(bytes.Repeat([]byte("A"), 64)),
		"SHA256:" + string(bytes.Repeat([]byte("a"), 64)),
		" " + valid,
		valid + "\n",
		"sha256:" + string(bytes.Repeat([]byte("a"), 63)),
	} {
		// A nonexistent root proves validation happens before any workspace access.
		_, err := Remove(filepath.Join(t.TempDir(), "missing"), RemoveOptions{ExpectedRemovalDigest: value})
		if !errors.Is(err, ErrInvalidRemovalDigest) {
			t.Fatalf("%q: err = %v, want invalid removal digest", value, err)
		}
	}
	if err := ValidateExpectedRemovalDigest(valid); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveDigestRecheckStillPreservesLateEdit(t *testing.T) {
	t.Parallel()
	root := digestWorkspace(t)
	digest := dryRunDigest(t, root, false)
	readme := filepath.Join(root, "README.md")
	result, err := Remove(root, RemoveOptions{
		ExpectedRemovalDigest: digest,
		beforeDelete: func(path string) {
			if path == "README.md" {
				if err := os.WriteFile(readme, []byte("late edit\n"), 0o644); err != nil {
					t.Error(err)
				}
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Skipped, []string{"README.md"}) || result.LockRemoved {
		t.Fatalf("result = %#v", result)
	}
	if got, _ := os.ReadFile(readme); string(got) != "late edit\n" {
		t.Fatalf("late edit lost: %q", got)
	}
}
