package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

type testProber struct{}

func (testProber) LookPath(name string) (string, error) {
	if name == "goreleaser" {
		return "", errors.New("missing")
	}
	return "/usr/bin/" + name, nil
}

func (testProber) Version(_ context.Context, name string, _ ...string) (string, error) {
	if filepath.Base(name) == "go" {
		return "go version go1.26.5 test", nil
	}
	return filepath.Base(name) + " test", nil
}

func TestNewWriteAndCheckLifecycle(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	stdout, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write")
	if err != nil {
		t.Fatalf("new: %v\n%s", err, stdout)
	}
	for _, path := range []string{"bob.yaml", "bob.lock", "README.md", "cmd/acme/main.go"} {
		if _, err := os.Stat(filepath.Join(target, path)); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
	stdout, _, err = executeForTest("check", target)
	if err != nil {
		t.Fatalf("check converged project: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "clean:") {
		t.Fatalf("unexpected check output: %s", stdout)
	}

	readme := filepath.Join(target, "README.md")
	if err := os.WriteFile(readme, []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = executeForTest("check", target)
	if err == nil || !strings.Contains(stdout, "conflict") {
		t.Fatalf("expected drift conflict, got err=%v output=%s", err, stdout)
	}
}

func TestNewPreviewDoesNotWrite(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "preview")
	stdout, _, err := executeForTest("new", "preview", "--module", "github.com/acme/preview", "--dir", target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview mutated target: %v", err)
	}
	if !strings.Contains(stdout, "schema_version: 1") || !strings.Contains(stdout, "files would be created") {
		t.Fatalf("unexpected preview: %s", stdout)
	}
}

func TestWriteCommandsRejectSymlinkWorkspaceWithoutMutation(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	link := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"new", "acme", "--module", "github.com/acme/acme", "--dir", link, "--write"},
		{"init", link, "--name", "acme", "--module", "github.com/acme/acme", "--write"},
	} {
		if _, _, err := executeForTest(args...); err == nil || !strings.Contains(err.Error(), "not a regular directory") {
			t.Fatalf("expected symlink rejection for %v, got %v", args, err)
		}
		if _, err := os.Stat(filepath.Join(outside, manifest.Filename)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("command %v wrote outside workspace: %v", args, err)
		}
	}
}

func TestPlanJSONUsesVersionedEnvelope(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("--json", "plan", target)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		SchemaVersion int    `json:"schema_version"`
		OK            bool   `json:"ok"`
		Command       string `json:"command"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	if got.SchemaVersion != 1 || !got.OK || got.Command != "plan" {
		t.Fatalf("unexpected envelope: %#v", got)
	}
}

func TestCheckJSONReportsFailedOutcomeOnDrift(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("--json", "check", target)
	if err == nil {
		t.Fatal("expected drift error")
	}
	var got struct {
		OK bool `json:"ok"`
	}
	if decodeErr := json.Unmarshal([]byte(stdout), &got); decodeErr != nil {
		t.Fatalf("decode JSON: %v\n%s", decodeErr, stdout)
	}
	if got.OK {
		t.Fatalf("failed check reported ok=true: %s", stdout)
	}
}

func TestExecuteEmitsStructuredJSONForCommandError(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := execute([]string{"--json", "plan", filepath.Join(t.TempDir(), "missing")}, Dependencies{Out: &stdout, ErrOut: &stderr, Prober: testProber{}})
	if err == nil {
		t.Fatal("expected plan error")
	}
	var got struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"data"`
	}
	if decodeErr := json.Unmarshal(stdout.Bytes(), &got); decodeErr != nil {
		t.Fatalf("decode JSON: %v\n%s", decodeErr, stdout.String())
	}
	if got.OK || got.Command != "plan" || got.Data.Error.Code != "command_failed" {
		t.Fatalf("unexpected error envelope: %#v", got)
	}
}

func TestCheckReportsCanonicalLockDrift(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(target, "bob.lock")
	file, err := os.OpenFile(lockPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("# drift\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("check", target)
	if err == nil || !strings.Contains(stdout, "lock       bob.lock") {
		t.Fatalf("expected lock drift, got err=%v output=%s", err, stdout)
	}
}

func TestDoctorAllowsMissingOptionalTool(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("doctor", target)
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "GoReleaser") || !strings.Contains(stdout, "missing") {
		t.Fatalf("unexpected doctor output: %s", stdout)
	}
}

func executeForTest(args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := New(Dependencies{Out: &stdout, ErrOut: &stderr, Prober: testProber{}})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}
