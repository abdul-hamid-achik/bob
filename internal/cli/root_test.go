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

	"github.com/abdul-hamid-achik/bob/internal/engine"
	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

type testProber struct{}

type testIntegrationRunner struct{}

func (testIntegrationRunner) LookPath(name string) (string, error) {
	return "/usr/bin/" + name, nil
}

func (testIntegrationRunner) Run(context.Context, string, string, ...string) inspectpkg.CommandResult {
	return inspectpkg.CommandResult{}
}

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

func TestPlanAndCheckJSONExposeSharedDigest(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	planJSON, _, err := executeForTest("--json", "plan", target)
	if err != nil {
		t.Fatal(err)
	}
	checkJSON, _, err := executeForTest("--json", "check", target)
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Data struct {
			Version int    `json:"plan_digest_version"`
			Digest  string `json:"plan_digest"`
		} `json:"data"`
	}
	var check struct {
		Data struct {
			Version int    `json:"plan_digest_version"`
			Digest  string `json:"plan_digest"`
			Plan    struct {
				Digest string `json:"plan_digest"`
			} `json:"plan"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(checkJSON), &check); err != nil {
		t.Fatal(err)
	}
	if plan.Data.Version != 1 || plan.Data.Digest == "" || plan.Data.Digest != check.Data.Digest || check.Data.Digest != check.Data.Plan.Digest {
		t.Fatalf("plan=%#v check=%#v", plan.Data, check.Data)
	}
	complete, err := loadPlan(target)
	if err != nil {
		t.Fatal(err)
	}
	shared := engine.DigestPlan(complete)
	if plan.Data.Version != shared.Version || plan.Data.Digest != shared.Qualified() {
		t.Fatalf("CLI digest differs from engine: cli=%d:%s engine=%d:%s", plan.Data.Version, plan.Data.Digest, shared.Version, shared.SHA256)
	}
	if !strings.HasPrefix(plan.Data.Digest, "sha256:") {
		t.Fatalf("CLI digest is not directly consumable by guarded apply: %q", plan.Data.Digest)
	}
}

func TestContextCLIProfilesFailuresAndPlainOutput(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := executeForTest("--json", "context", target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("JSON stderr = %q", stderr)
	}
	var got struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Profile    string `json:"profile"`
			Repository struct {
				State      string `json:"state"`
				PlanDigest string `json:"plan_digest"`
			} `json:"repository"`
			Truncation struct {
				ByteLimit int `json:"byte_limit"`
			} `json:"truncation"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("machine stdout is not JSON: %v\n%s", err, stdout)
	}
	if !got.OK || got.Command != "context" || got.Data.Profile != "compact" || got.Data.Repository.State != "clean" || !strings.HasPrefix(got.Data.Repository.PlanDigest, "sha256:") || got.Data.Truncation.ByteLimit != 6144 {
		t.Fatalf("context = %#v", got)
	}
	human, _, err := executeForTest("context", target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(human, "\x1b[") || !strings.Contains(human, "repository: clean") {
		t.Fatalf("human output = %q", human)
	}

	missing := t.TempDir()
	var failureOut, failureErr bytes.Buffer
	err = execute([]string{"--json", "context", missing}, Dependencies{Out: &failureOut, ErrOut: &failureErr, Prober: testProber{}, IntegrationRunner: testIntegrationRunner{}})
	failure := failureOut.String()
	if err == nil {
		t.Fatal("missing manifest should fail")
	}
	var failed struct {
		OK   bool `json:"ok"`
		Data struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"data"`
	}
	if decodeErr := json.Unmarshal([]byte(failure), &failed); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if failed.OK || failed.Data.Error.Code != "missing_manifest" {
		t.Fatalf("failure = %s", failure)
	}
	invalid := t.TempDir()
	if err := os.WriteFile(filepath.Join(invalid, manifest.Filename), []byte("schema_version: nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	failureOut.Reset()
	failureErr.Reset()
	err = execute([]string{"--json", "context", invalid}, Dependencies{Out: &failureOut, ErrOut: &failureErr, Prober: testProber{}, IntegrationRunner: testIntegrationRunner{}})
	failure = failureOut.String()
	if err == nil {
		t.Fatal("invalid manifest should fail")
	}
	if decodeErr := json.Unmarshal([]byte(failure), &failed); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if failed.Data.Error.Code != "manifest_invalid" {
		t.Fatalf("failure = %s", failure)
	}
}

func TestPathCLIJSONAndPlainOutput(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := executeForTest("--json", "path", "internal/cli/root.go", target)
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var got struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Classification  string `json:"classification"`
			State           string `json:"state"`
			HumanEditEffect string `json:"human_edit_effect"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("machine stdout is not JSON: %v\n%s", err, stdout)
	}
	if !got.OK || got.Command != "path" || got.Data.Classification != "managed" || got.Data.State != "managed_in_sync" || got.Data.HumanEditEffect != "will_conflict" {
		t.Fatalf("path = %#v", got)
	}
	human, _, err := executeForTest("path", "internal/domain/service.go", "--workspace", target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(human, "\x1b[") || !strings.Contains(human, "extension_point") {
		t.Fatalf("human output = %q", human)
	}
	if _, _, err := executeForTest("path", "../escape", target); err == nil || ExitCode(err) != ExitInvalidInput {
		t.Fatalf("unsafe path error = %v code=%d", err, ExitCode(err))
	}
	var failureOut, failureErr bytes.Buffer
	err = execute([]string{"--json", "path", "../escape", target}, Dependencies{Out: &failureOut, ErrOut: &failureErr, Prober: testProber{}, IntegrationRunner: testIntegrationRunner{}})
	if err == nil {
		t.Fatal("unsafe JSON path should fail")
	}
	stdout = failureOut.String()
	if failureErr.Len() != 0 {
		t.Fatalf("unsafe JSON stderr = %q", failureErr.String())
	}
	var failed struct {
		Data struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"data"`
	}
	if decodeErr := json.Unmarshal([]byte(stdout), &failed); decodeErr != nil || failed.Data.Error.Code != "input_invalid" {
		t.Fatalf("unsafe JSON path = %s decode=%v", stdout, decodeErr)
	}
}

func TestPlaybookCLIUsesVersionedJSONAndTypedSetValues(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := executeForTest("--json", "playbook", "list", target)
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var listed struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Playbooks []struct {
				ID        string `json:"id"`
				Available bool   `json:"available"`
			} `json:"playbooks"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
		t.Fatalf("machine stdout is not JSON: %v\n%s", err, stdout)
	}
	if !listed.OK || listed.Command != "playbook list" || len(listed.Data.Playbooks) != 7 || listed.Data.Playbooks[0].ID != "add-cli-command" || !listed.Data.Playbooks[0].Available {
		t.Fatalf("list = %#v", listed)
	}
	stdout, _, err = executeForTest("--json", "playbook", "plan", "add-cli-command", target, "--set", "command_name=hello")
	if err != nil {
		t.Fatal(err)
	}
	var planned struct {
		Data struct {
			Values map[string]string `json:"values"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &planned); err != nil || planned.Data.Values["command_name"] != "hello" {
		t.Fatalf("plan = %s err=%v", stdout, err)
	}
	if _, _, err := executeForTest("playbook", "plan", "add-cli-command", target, "--set", "command_name=hello;rm"); err == nil || ExitCode(err) != ExitInvalidInput {
		t.Fatalf("unsafe input error = %v code=%d", err, ExitCode(err))
	}
}

func TestLearnEmitsAgentBriefWithoutMutation(t *testing.T) {
	t.Parallel()
	stdout, _, err := executeForTest("learn")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"deterministic repository factory", "plan", "apply", "https://bobcli.dev"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("learn text output missing %q:\n%s", want, stdout)
		}
	}
	stdout, _, err = executeForTest("--json", "learn")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		SchemaVersion int    `json:"schema_version"`
		OK            bool   `json:"ok"`
		Command       string `json:"command"`
		Data          struct {
			Lifecycle  []string `json:"lifecycle"`
			Invariants []string `json:"invariants"`
			Commands   []struct {
				Name    string `json:"name"`
				Mutates bool   `json:"mutates"`
			} `json:"commands"`
			MCP struct {
				Tools []string `json:"tools"`
			} `json:"mcp"`
			ExitCodes  map[string]string `json:"exit_codes"`
			ErrorCodes map[string]string `json:"error_codes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	if got.SchemaVersion != 1 || !got.OK || got.Command != "learn" {
		t.Fatalf("unexpected envelope: %#v", got)
	}
	if len(got.Data.Lifecycle) != 4 || len(got.Data.Invariants) == 0 || len(got.Data.MCP.Tools) != 9 {
		t.Fatalf("unexpected learn data: %#v", got.Data)
	}
	for _, tool := range []string{"bob_context", "bob_path", "bob_playbook"} {
		found := false
		for _, candidate := range got.Data.MCP.Tools {
			found = found || candidate == tool
		}
		if !found {
			t.Fatalf("learn MCP catalog omitted %s: %#v", tool, got.Data.MCP.Tools)
		}
	}
	contextFound := false
	pathFound := false
	playbookFound := false
	for _, command := range got.Data.Commands {
		if command.Name == "learn" && command.Mutates {
			t.Fatal("learn must describe itself as non-mutating")
		}
		if command.Name == "context" {
			contextFound = true
			if command.Mutates {
				t.Fatal("context must be cataloged as non-mutating")
			}
		}
		if command.Name == "path" {
			pathFound = true
			if command.Mutates {
				t.Fatal("path must be cataloged as non-mutating")
			}
		}
		if command.Name == "playbook" {
			playbookFound = true
			if command.Mutates {
				t.Fatal("playbook must be cataloged as non-mutating")
			}
		}
	}
	if !contextFound || !pathFound || !playbookFound {
		t.Fatalf("learn command catalog omitted guidance commands: context=%t path=%t playbook=%t", contextFound, pathFound, playbookFound)
	}
	for _, code := range []string{"0", "1", "2", "3", "4", "5"} {
		if _, ok := got.Data.ExitCodes[code]; !ok {
			t.Fatalf("learn data.exit_codes missing %q: %#v", code, got.Data.ExitCodes)
		}
	}
	for _, code := range []string{"missing_manifest", "manifest_invalid", "input_invalid", "conflicts", "workspace_invalid", "plan_digest_mismatch", "command_failed"} {
		if _, ok := got.Data.ErrorCodes[code]; !ok {
			t.Fatalf("learn data.error_codes missing %q: %#v", code, got.Data.ErrorCodes)
		}
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
	target := filepath.Join(t.TempDir(), "missing")
	err := execute([]string{"--json", "plan", target}, Dependencies{Out: &stdout, ErrOut: &stderr, Prober: testProber{}})
	if err == nil {
		t.Fatal("expected plan error")
	}
	var got struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		} `json:"data"`
		NextActions []string `json:"next_actions"`
	}
	if decodeErr := json.Unmarshal(stdout.Bytes(), &got); decodeErr != nil {
		t.Fatalf("decode JSON: %v\n%s", decodeErr, stdout.String())
	}
	if got.OK || got.Command != "plan" || got.Data.Error.Code != "missing_manifest" {
		t.Fatalf("unexpected error envelope: %#v", got)
	}
	if len(got.NextActions) == 0 {
		t.Fatalf("expected next_actions guidance for a missing manifest, got %#v", got)
	}
	foundInit := false
	for _, action := range got.NextActions {
		if strings.Contains(action, "bob init") {
			foundInit = true
		}
	}
	if !foundInit {
		t.Fatalf("expected a bob init suggestion in next_actions, got %#v", got.NextActions)
	}
	if stderr.Len() != 0 {
		t.Fatalf("JSON-mode failure must not write extra stderr text, got %q", stderr.String())
	}
}

// TestHumanFailurePrintsNextStepsAfterErrorLine proves that a non-JSON
// invocation prints the same corrective guidance the JSON envelope carries,
// as "next: ..." lines on stderr immediately after the "bob: ..." error line,
// so a weak model that never passes --json can still self-recover.
func TestHumanFailurePrintsNextStepsAfterErrorLine(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	target := filepath.Join(t.TempDir(), "missing")
	err := execute([]string{"plan", target}, Dependencies{Out: &stdout, ErrOut: &stderr, Prober: testProber{}})
	if err == nil {
		t.Fatal("expected plan error")
	}
	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected an error line followed by next steps, got %q", stderr.String())
	}
	if !strings.HasPrefix(lines[0], "bob: ") {
		t.Fatalf("expected the error line first, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "next: ") {
		t.Fatalf("expected a next: line after the error line, got %q", lines[1])
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

func TestInspectIsOfflineAndReportsExplicitProbeContinuation(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("inspect", target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Bob        clean") || !strings.Contains(stdout, "not_requested") || !strings.Contains(stdout, "--probe-integrations") {
		t.Fatalf("unexpected inspection output: %s", stdout)
	}
}

func TestInspectJSONKeepsStructuredArgv(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("--json", "inspect", target)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Data struct {
			NextActions []struct {
				Argv []string `json:"argv"`
			} `json:"next_actions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data.NextActions) == 0 || len(got.Data.NextActions[0].Argv) == 0 {
		t.Fatalf("structured continuation missing: %s", stdout)
	}
}

func TestMCPServeHelpDocumentsStdioRegistration(t *testing.T) {
	t.Parallel()
	stdout, _, err := executeForTest("mcp", "serve", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "stdout is reserved") || !strings.Contains(stdout, "mcphub add bob") {
		t.Fatalf("unexpected MCP help: %s", stdout)
	}
}

func executeForTest(args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := New(Dependencies{Out: &stdout, ErrOut: &stderr, Prober: testProber{}, IntegrationRunner: testIntegrationRunner{}})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}
