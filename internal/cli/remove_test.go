package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scaffoldRemoveWorkspace creates a fully managed go-agent-tool workspace via
// `bob new --write` and returns its path.
func scaffoldRemoveWorkspace(t *testing.T) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestRemoveLifecycleClearsManagedFilesAndLock(t *testing.T) {
	t.Parallel()
	target := scaffoldRemoveWorkspace(t)
	stdout, _, err := executeForTest("remove", target)
	if err != nil {
		t.Fatalf("remove: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "lock removed: true") {
		t.Fatalf("unexpected remove output: %s", stdout)
	}
	if _, err := os.Stat(filepath.Join(target, "bob.lock")); !os.IsNotExist(err) {
		t.Fatalf("bob.lock should be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "README.md")); !os.IsNotExist(err) {
		t.Fatalf("managed README.md should be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "bob.yaml")); err != nil {
		t.Fatalf("bob.yaml must survive remove: %v", err)
	}
}

func TestRemoveDryRunPreservesEverything(t *testing.T) {
	t.Parallel()
	target := scaffoldRemoveWorkspace(t)
	stdout, _, err := executeForTest("remove", "--dry-run", target)
	if err != nil {
		t.Fatalf("remove --dry-run: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "dry-run:") || !strings.Contains(stdout, "would remove") {
		t.Fatalf("unexpected dry-run output: %s", stdout)
	}
	for _, path := range []string{"bob.lock", "README.md", "bob.yaml"} {
		if _, err := os.Stat(filepath.Join(target, path)); err != nil {
			t.Fatalf("dry-run must not remove %s: %v", path, err)
		}
	}
}

func TestRemoveModifiedFileExitsConflictsAndPreservesIt(t *testing.T) {
	t.Parallel()
	target := scaffoldRemoveWorkspace(t)
	readme := filepath.Join(target, "README.md")
	if err := os.WriteFile(readme, []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("remove", target)
	if ExitCode(err) != ExitConflicts {
		t.Fatalf("remove with drifted file exit code = %d, want %d (err=%v)", ExitCode(err), ExitConflicts, err)
	}
	if !strings.Contains(stdout, "skipped") {
		t.Fatalf("expected skipped output, got: %s", stdout)
	}
	data, readErr := os.ReadFile(readme)
	if readErr != nil || string(data) != "human edit\n" {
		t.Fatalf("drifted file = %q, %v; want human edit preserved", data, readErr)
	}
	if _, err := os.Stat(filepath.Join(target, "bob.lock")); err != nil {
		t.Fatalf("bob.lock must be retained after a partial remove: %v", err)
	}
}

func TestRemoveForceClearsModifiedFile(t *testing.T) {
	t.Parallel()
	target := scaffoldRemoveWorkspace(t)
	if err := os.WriteFile(filepath.Join(target, "README.md"), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("remove", "--force", target)
	if err != nil {
		t.Fatalf("remove --force: %v\n%s", err, stdout)
	}
	if _, err := os.Stat(filepath.Join(target, "README.md")); !os.IsNotExist(err) {
		t.Fatalf("force remove must delete the drifted file, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "bob.lock")); !os.IsNotExist(err) {
		t.Fatalf("force remove must delete bob.lock, got err=%v", err)
	}
}

func TestRemoveWithoutLockExitsInvalidInput(t *testing.T) {
	t.Parallel()
	target := scaffoldRemoveWorkspace(t)
	if _, _, err := executeForTest("remove", target); err != nil {
		t.Fatal(err)
	}
	_, _, err := executeForTest("remove", target)
	if ExitCode(err) != ExitInvalidInput {
		t.Fatalf("second remove exit code = %d, want %d (err=%v)", ExitCode(err), ExitInvalidInput, err)
	}
}

func TestRemoveJSONEnvelopeReportsResult(t *testing.T) {
	t.Parallel()
	target := scaffoldRemoveWorkspace(t)
	stdout, _, err := executeForTest("--json", "remove", target)
	if err != nil {
		t.Fatalf("remove --json: %v\n%s", err, stdout)
	}
	var got struct {
		SchemaVersion int    `json:"schema_version"`
		OK            bool   `json:"ok"`
		Command       string `json:"command"`
		Data          struct {
			Removed     []string `json:"removed"`
			LockRemoved bool     `json:"lock_removed"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	if got.SchemaVersion != 1 || !got.OK || got.Command != "remove" {
		t.Fatalf("unexpected envelope: schema=%d ok=%t command=%q", got.SchemaVersion, got.OK, got.Command)
	}
	if !got.Data.LockRemoved || len(got.Data.Removed) == 0 {
		t.Fatalf("unexpected remove data: %#v", got.Data)
	}
}

func TestRemoveJSONIncompleteExitsConflicts(t *testing.T) {
	t.Parallel()
	target := scaffoldRemoveWorkspace(t)
	if err := os.WriteFile(filepath.Join(target, "README.md"), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("--json", "remove", target)
	if ExitCode(err) != ExitConflicts {
		t.Fatalf("incomplete remove exit code = %d, want %d (err=%v)", ExitCode(err), ExitConflicts, err)
	}
	var got struct {
		OK   bool `json:"ok"`
		Data struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
			Result struct {
				Skipped []string `json:"skipped"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	if got.OK {
		t.Fatal("incomplete remove must report ok=false")
	}
	if got.Data.Error.Code != "conflicts" || len(got.Data.Result.Skipped) == 0 {
		t.Fatalf("unexpected incomplete remove data: %#v", got.Data)
	}
}
