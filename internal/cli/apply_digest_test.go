package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

func pendingWorkspace(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "acme")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := executeForTest("init", root, "--name", "acme", "--module", "github.com/acme/acme", "--write"); err != nil {
		t.Fatal(err)
	}
	return root
}

func planDigestForCLI(t *testing.T, root string) string {
	t.Helper()
	stdout, stderr, err := executeForTest("--json", "plan", root)
	if err != nil {
		t.Fatalf("plan: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	var envelope struct {
		Data struct {
			Digest string `json:"plan_digest"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if err := engine.ValidateExpectedPlanDigest(envelope.Data.Digest); err != nil {
		t.Fatalf("plan digest = %q", envelope.Data.Digest)
	}
	return envelope.Data.Digest
}

func TestApplyExpectedPlanDigestSuccessReturnsBoundedReceiptFields(t *testing.T) {
	t.Parallel()
	root := pendingWorkspace(t)
	expected := planDigestForCLI(t, root)
	stdout, stderr, err := executeForTest("--json", "apply", root, "--expect-plan-digest", expected)
	if err != nil {
		t.Fatalf("apply: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("JSON stderr = %q", stderr)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			SchemaVersion       int      `json:"schema_version"`
			PlanDigestVersion   int      `json:"plan_digest_version"`
			ExpectedPlanDigest  string   `json:"expected_plan_digest"`
			AppliedPlanDigest   string   `json:"applied_plan_digest"`
			Written             []string `json:"written"`
			Adopted             []string `json:"adopted"`
			Unchanged           []string `json:"unchanged"`
			WrittenCount        int      `json:"written_count"`
			AdoptedCount        int      `json:"adopted_count"`
			UnchangedCount      int      `json:"unchanged_count"`
			LockWritten         bool     `json:"lock_written"`
			ConvergedAfterApply bool     `json:"converged_after_apply"`
			NextCheck           struct {
				Argv []string `json:"argv"`
			} `json:"next_check"`
			Truncation struct {
				ByteLimit int            `json:"byte_limit"`
				Truncated bool           `json:"truncated"`
				Omitted   map[string]int `json:"omitted"`
			} `json:"truncation"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	data := envelope.Data
	if !envelope.OK || data.SchemaVersion != 1 || data.PlanDigestVersion != 1 || data.ExpectedPlanDigest != expected || data.AppliedPlanDigest != expected {
		t.Fatalf("receipt identity = %#v", data)
	}
	if len(data.Written) == 0 || data.Adopted == nil || data.Unchanged == nil || !data.LockWritten || !data.ConvergedAfterApply {
		t.Fatalf("receipt changes = %#v", data)
	}
	if data.WrittenCount != len(data.Written) || data.AdoptedCount != 0 || data.UnchangedCount != 0 || data.Truncation.ByteLimit != engine.ApplyReceiptByteLimit || data.Truncation.Truncated {
		t.Fatalf("receipt bounds = %#v", data)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.NextCheck.Argv) != 4 || data.NextCheck.Argv[0] != "bob" || data.NextCheck.Argv[1] != "check" || data.NextCheck.Argv[2] != canonicalRoot || data.NextCheck.Argv[3] != "--json" {
		t.Fatalf("next_check = %v", data.NextCheck.Argv)
	}
}

func TestApplyExpectedPlanDigestMismatchUsesStableExitAndErrorContract(t *testing.T) {
	t.Parallel()
	root := pendingWorkspace(t)
	expected := planDigestForCLI(t, root)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("appeared after review\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := executeForTest("--json", "apply", root, "--expect-plan-digest", expected)
	if err == nil || ExitCode(err) != ExitPlanMismatch || !errors.Is(err, engine.ErrPlanDigestMismatch) {
		t.Fatalf("apply error = %v code=%d", err, ExitCode(err))
	}
	if stderr != "" {
		t.Fatalf("JSON stderr = %q", stderr)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Expected string `json:"expected_plan_digest"`
			Actual   string `json:"actual_plan_digest"`
			Error    struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		} `json:"data"`
		NextActions []string `json:"next_actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	if envelope.OK || envelope.Data.Expected != expected || envelope.Data.Actual == expected || !strings.HasPrefix(envelope.Data.Actual, "sha256:") {
		t.Fatalf("mismatch envelope = %#v", envelope)
	}
	if envelope.Data.Error.Code != "plan_digest_mismatch" || envelope.Data.Error.Message != "the reviewed plan no longer matches the current workspace" {
		t.Fatalf("mismatch error = %#v", envelope.Data.Error)
	}
	if len(envelope.NextActions) != 2 || !strings.Contains(envelope.NextActions[0], "bob plan") {
		t.Fatalf("next actions = %v", envelope.NextActions)
	}
	if _, statErr := os.Stat(filepath.Join(root, engine.LockFilename)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("mismatch wrote bob.lock: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "cmd")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("mismatch created generated directory: %v", statErr)
	}
	content, _ := os.ReadFile(filepath.Join(root, "README.md"))
	if string(content) != "appeared after review\n" {
		t.Fatalf("mismatch changed unmanaged file: %q", content)
	}
	if _, statErr := os.Stat(filepath.Join(root, engine.ApplyLockFilename)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("mismatch left apply lock: %v", statErr)
	}
}

func TestApplyExpectedPlanDigestInvalidSyntaxIsInputInvalid(t *testing.T) {
	t.Parallel()
	root := pendingWorkspace(t)
	stdout, stderr, err := executeForTest("--json", "apply", root, "--expect-plan-digest", strings.Repeat("a", 64))
	if err == nil || ExitCode(err) != ExitInvalidInput || !errors.Is(err, engine.ErrInvalidPlanDigest) {
		t.Fatalf("apply error = %v code=%d", err, ExitCode(err))
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("bare command execution should return error without wrapper output: stdout=%q stderr=%q", stdout, stderr)
	}
	if _, statErr := os.Stat(filepath.Join(root, engine.LockFilename)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("invalid digest wrote bob.lock: %v", statErr)
	}
}

func TestApplyDigestMismatchHumanFailureHasRecoveryCommands(t *testing.T) {
	t.Parallel()
	root := pendingWorkspace(t)
	expected := planDigestForCLI(t, root)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("after review\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	err := runExecuteForTest(t, []string{"apply", root, "--expect-plan-digest", expected}, &stdout, &stderr)
	if err == nil || ExitCode(err) != ExitPlanMismatch {
		t.Fatalf("apply error = %v code=%d", err, ExitCode(err))
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "bob: apply: the reviewed plan") || !strings.Contains(stderr.String(), "next: run: bob plan") || !strings.Contains(stderr.String(), "review the new plan") {
		t.Fatalf("human mismatch output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestApplyReloadsManifestAfterReviewBeforeAcquiringMutationAuthority(t *testing.T) {
	t.Parallel()
	root := pendingWorkspace(t)
	expected := planDigestForCLI(t, root)
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	m.Product.Description = "Changed after the reviewed manifest was loaded."
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, true); err != nil {
		t.Fatal(err)
	}
	manifestBefore, err := os.ReadFile(filepath.Join(root, manifest.Filename))
	if err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := executeForTest("--json", "apply", root, "--expect-plan-digest", expected)
	if err == nil || ExitCode(err) != ExitPlanMismatch || !errors.Is(err, engine.ErrPlanDigestMismatch) {
		t.Fatalf("apply error = %v code=%d\nstdout=%s\nstderr=%s", err, ExitCode(err), stdout, stderr)
	}
	for _, path := range []string{"README.md", "cmd", engine.LockFilename, engine.ApplyLockFilename} {
		if _, statErr := os.Lstat(filepath.Join(root, path)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("stale manifest review created %s: %v", path, statErr)
		}
	}
	manifestAfter, err := os.ReadFile(filepath.Join(root, manifest.Filename))
	if err != nil || string(manifestAfter) != string(manifestBefore) {
		t.Fatalf("stale review changed bob.yaml: %v", err)
	}
}

func TestApplyJSONReceiptBoundsLargeFilesRecipe(t *testing.T) {
	const fileCount = 700
	root := t.TempDir()
	m := manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Recipe:        manifest.RecipeFiles,
		Product:       manifest.Product{Name: "large-receipt", Description: "Large receipt fixture"},
		Files:         make([]manifest.FileDecl, 0, fileCount),
	}
	for i := 0; i < fileCount; i++ {
		m.Files = append(m.Files, manifest.FileDecl{
			Path:    fmt.Sprintf("generated/group-%02d/file-%04d-with-a-stable-name.txt", i%10, i),
			Content: "fixture\n",
		})
	}
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	expected := planDigestForCLI(t, root)
	stdout, stderr, err := executeForTest("--json", "apply", root, "--expect-plan-digest", expected)
	if err != nil {
		t.Fatalf("apply: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	const envelopeByteLimit = 24 << 10
	if len(stdout) > envelopeByteLimit {
		t.Fatalf("apply envelope bytes = %d, limit = %d", len(stdout), envelopeByteLimit)
	}
	t.Logf("large files@1 apply envelope: %d bytes (limit %d)", len(stdout), envelopeByteLimit)
	var envelope struct {
		Data struct {
			WrittenCount   int      `json:"written_count"`
			AdoptedCount   int      `json:"adopted_count"`
			UnchangedCount int      `json:"unchanged_count"`
			Written        []string `json:"written"`
			Adopted        []string `json:"adopted"`
			Unchanged      []string `json:"unchanged"`
			Truncation     struct {
				ByteLimit int            `json:"byte_limit"`
				Truncated bool           `json:"truncated"`
				Omitted   map[string]int `json:"omitted"`
			} `json:"truncation"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	data := envelope.Data
	if data.WrittenCount != fileCount || data.AdoptedCount != 0 || data.UnchangedCount != 0 {
		t.Fatalf("counts = written:%d adopted:%d unchanged:%d", data.WrittenCount, data.AdoptedCount, data.UnchangedCount)
	}
	if !data.Truncation.Truncated || data.Truncation.ByteLimit != engine.ApplyReceiptByteLimit {
		t.Fatalf("truncation = %#v", data.Truncation)
	}
	returned := len(data.Written) + len(data.Adopted) + len(data.Unchanged)
	omitted := data.Truncation.Omitted["written"] + data.Truncation.Omitted["adopted"] + data.Truncation.Omitted["unchanged"]
	if returned > engine.ApplyReceiptPathLimit || returned+omitted != fileCount {
		t.Fatalf("returned=%d omitted=%d want=%d", returned, omitted, fileCount)
	}
}
