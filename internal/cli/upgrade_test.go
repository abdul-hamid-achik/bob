package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

func scaffoldUpgradeV4Workspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	artifacts, err := recipe.RenderVersion(m, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply(root, m, artifacts); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, engine.LockFilename)
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	v4 := strings.Replace(string(data), "  version: 6\n", "  version: 4\n", 1)
	if v4 == string(data) {
		t.Fatal("temporary v4 lock did not contain the current recipe version")
	}
	if err := os.WriteFile(lockPath, []byte(v4), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestUpgradeCLIJSONAppliesPublishedRecipeMigration(t *testing.T) {
	t.Parallel()
	root := scaffoldUpgradeV4Workspace(t)
	stdout, stderr, err := executeForTest("--json", "upgrade", root)
	if err != nil {
		t.Fatalf("upgrade: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	var envelope struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			FromVersion int      `json:"from_version"`
			ToVersion   int      `json:"to_version"`
			Applied     bool     `json:"applied"`
			Written     []string `json:"written"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || envelope.Command != "upgrade" || envelope.Data.FromVersion != 4 || envelope.Data.ToVersion != 6 || !envelope.Data.Applied {
		t.Fatalf("upgrade envelope = %#v", envelope)
	}
	if len(envelope.Data.Written) != 1 || envelope.Data.Written[0] != "internal/cli/registry_test.go" {
		t.Fatalf("written = %v", envelope.Data.Written)
	}
	lock, err := engine.LoadLock(root)
	if err != nil || lock.Recipe.Version != 6 {
		t.Fatalf("upgraded lock = %#v, %v", lock, err)
	}
}

func TestUpgradeCLIDryRunAndCurrentNoOp(t *testing.T) {
	t.Parallel()
	root := scaffoldUpgradeV4Workspace(t)
	stdout, _, err := executeForTest("upgrade", root, "--dry-run")
	if err != nil || !strings.Contains(stdout, "dry-run: would upgrade recipe go-agent-tool from v4 to v6") {
		t.Fatalf("dry-run output=%q err=%v", stdout, err)
	}
	lock, err := engine.LoadLock(root)
	if err != nil || lock.Recipe.Version != 4 {
		t.Fatalf("dry-run lock = %#v, %v", lock, err)
	}
	if _, _, err := executeForTest("upgrade", root); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = executeForTest("--json", "upgrade", root)
	if err != nil || !strings.Contains(stdout, `"applied": false`) {
		t.Fatalf("no-op output=%q err=%v", stdout, err)
	}
}

func TestUpgradeCLIConflictUsesUpgradeRecoveryGuidance(t *testing.T) {
	t.Parallel()
	root := scaffoldUpgradeV4Workspace(t)
	if err := os.WriteFile(filepath.Join(root, "internal", "cli", "root.go"), []byte("package cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("--json", "upgrade", root)
	if ExitCode(err) != ExitConflicts {
		t.Fatalf("exit=%d err=%v", ExitCode(err), err)
	}
	var envelope struct {
		OK          bool     `json:"ok"`
		NextActions []string `json:"next_actions"`
		Data        struct {
			Conflicts []map[string]any `json:"conflicts"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.OK || len(envelope.Data.Conflicts) == 0 || !strings.Contains(strings.Join(envelope.NextActions, "\n"), "bob upgrade") {
		t.Fatalf("conflict envelope = %#v", envelope)
	}
}

func TestUpgradeCLIDigestMismatchUsesExitFive(t *testing.T) {
	t.Parallel()
	root := scaffoldUpgradeV4Workspace(t)
	digest := "sha256:" + strings.Repeat("0", 64)
	stdout, _, err := executeForTest("--json", "upgrade", root, "--expect-plan-digest", digest)
	if ExitCode(err) != ExitPlanMismatch {
		t.Fatalf("exit=%d err=%v", ExitCode(err), err)
	}
	var envelope struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.OK || envelope.Command != "upgrade" || envelope.Data.Error.Code != "plan_digest_mismatch" {
		t.Fatalf("mismatch envelope = %#v", envelope)
	}
	lock, loadErr := engine.LoadLock(root)
	if loadErr != nil || lock.Recipe.Version != 4 {
		t.Fatalf("mismatch changed lock = %#v, %v", lock, loadErr)
	}
}

func TestUpgradeCLIWithoutLockIsInvalidInput(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	_, _, err := executeForTest("upgrade", root)
	if ExitCode(err) != ExitInvalidInput {
		t.Fatalf("exit=%d err=%v", ExitCode(err), err)
	}
}
