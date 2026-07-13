package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

// runExecuteForTest drives the package-level execute() wrapper (rather than
// a bare cobra Command.Execute(), as executeForTest does), since the
// stderr presentation this file tests for (the "bob: ..." / "next: ..."
// lines) is only assembled by that wrapper.
func runExecuteForTest(t *testing.T, args []string, stdout, stderr io.Writer) error {
	t.Helper()
	return execute(args, Dependencies{Out: stdout, ErrOut: stderr, Prober: testProber{}, IntegrationRunner: testIntegrationRunner{}})
}

// writeManifestFixture writes yaml as bob.yaml in a fresh temporary
// directory and returns that directory's path.
func writeManifestFixture(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, manifest.Filename), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestApplyConflictJSONIncludesConflictsWithoutSecondPlanRoundTrip proves
// that an apply refused for conflicts reports the conflicting actions
// directly in the failure envelope's data.conflicts, so a caller does not
// need to run `bob plan --json` again just to see what blocked it.
func TestApplyConflictJSONIncludesConflictsWithoutSecondPlanRoundTrip(t *testing.T) {
	t.Parallel()
	target := conflictedWorkspace(t)
	stdout, stderr, err := executeForTest("--json", "apply", target)
	if err == nil {
		t.Fatal("expected apply to refuse a conflicted plan")
	}
	if code := ExitCode(err); code != ExitConflicts {
		t.Fatalf("apply exit code = %d, want %d", code, ExitConflicts)
	}
	var got struct {
		OK   bool `json:"ok"`
		Data struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Conflicts []struct {
				Path   string `json:"path"`
				Code   string `json:"code"`
				Reason string `json:"reason"`
			} `json:"conflicts"`
		} `json:"data"`
		NextActions []string `json:"next_actions"`
	}
	if decodeErr := json.Unmarshal([]byte(stdout), &got); decodeErr != nil {
		t.Fatalf("decode JSON: %v\n%s", decodeErr, stdout)
	}
	if got.OK || got.Data.Error.Code != "conflicts" {
		t.Fatalf("unexpected apply conflict envelope: %#v", got)
	}
	if len(got.Data.Conflicts) == 0 {
		t.Fatalf("expected data.conflicts to be populated: %#v", got)
	}
	if got.Data.Conflicts[0].Path != "README.md" || got.Data.Conflicts[0].Code == "" {
		t.Fatalf("unexpected conflict entry: %#v", got.Data.Conflicts[0])
	}
	if len(got.NextActions) == 0 {
		t.Fatalf("expected next_actions for a conflicted apply, got %#v", got)
	}
	if stderr != "" {
		t.Fatalf("JSON-mode apply conflict must not write to stderr: %q", stderr)
	}
}

// TestApplyConflictHumanModePrintsBoundedConflictList proves the human-mode
// path prints each conflicting path with its code and reason, then the
// generic "bob: ..." / "next: ..." lines on stderr.
func TestApplyConflictHumanModePrintsBoundedConflictList(t *testing.T) {
	t.Parallel()
	target := conflictedWorkspace(t)
	var stdout, stderr strings.Builder
	err := runExecuteForTest(t, []string{"apply", target}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected apply to refuse a conflicted plan")
	}
	if !strings.Contains(stdout.String(), "conflict") || !strings.Contains(stdout.String(), "README.md") {
		t.Fatalf("expected the conflicting path in stdout: %s", stdout.String())
	}
	if !strings.HasPrefix(strings.TrimLeft(stderr.String(), ""), "bob: ") {
		t.Fatalf("expected the error line on stderr: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "next:") {
		t.Fatalf("expected next: guidance on stderr: %s", stderr.String())
	}
}

// TestPlanConflictsOnlyFiltersToConflictActions proves --conflicts-only
// trims both the JSON action list and the human-mode rows down to
// kind=conflict, which keeps output small for a capped agent harness.
func TestPlanConflictsOnlyFiltersToConflictActions(t *testing.T) {
	t.Parallel()
	target := conflictedWorkspace(t)

	stdout, _, err := executeForTest("--json", "plan", target, "--conflicts-only")
	if err != nil {
		t.Fatalf("plan --conflicts-only: %v\n%s", err, stdout)
	}
	var got struct {
		Data struct {
			Actions []struct {
				Kind           string `json:"kind"`
				DesiredPreview string `json:"desired_preview"`
			} `json:"actions"`
		} `json:"data"`
	}
	if decodeErr := json.Unmarshal([]byte(stdout), &got); decodeErr != nil {
		t.Fatalf("decode JSON: %v\n%s", decodeErr, stdout)
	}
	if len(got.Data.Actions) == 0 {
		t.Fatalf("expected at least one conflict action: %s", stdout)
	}
	for _, action := range got.Data.Actions {
		if action.Kind != "conflict" {
			t.Fatalf("--conflicts-only leaked a non-conflict action: %#v", action)
		}
		if action.DesiredPreview != "" {
			t.Fatalf("--conflicts-only without --content must omit previews: %#v", action)
		}
	}

	humanOut, _, err := executeForTest("plan", target, "--conflicts-only")
	if err != nil {
		t.Fatalf("plan --conflicts-only (human): %v\n%s", err, humanOut)
	}
	for _, line := range strings.Split(strings.TrimRight(humanOut, "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, "lock") || !strings.Contains(line, " ") {
			continue
		}
		if strings.HasPrefix(line, "create") || strings.HasPrefix(line, "update") || strings.HasPrefix(line, "unchanged") || strings.HasPrefix(line, "adopt") {
			t.Fatalf("--conflicts-only leaked a non-conflict row: %q", line)
		}
	}
}

// TestCheckConflictsOnlyStillReportsFailure proves --conflicts-only on check
// filters the reported plan without hiding the underlying failure or its
// next_actions.
func TestCheckConflictsOnlyStillReportsFailure(t *testing.T) {
	t.Parallel()
	target := conflictedWorkspace(t)
	stdout, _, err := executeForTest("--json", "check", target, "--conflicts-only")
	if err == nil {
		t.Fatal("expected check to fail on a conflicted repository")
	}
	var got struct {
		OK   bool `json:"ok"`
		Data struct {
			Plan struct {
				Actions []struct {
					Kind string `json:"kind"`
				} `json:"actions"`
			} `json:"plan"`
		} `json:"data"`
		NextActions []string `json:"next_actions"`
	}
	if decodeErr := json.Unmarshal([]byte(stdout), &got); decodeErr != nil {
		t.Fatalf("decode JSON: %v\n%s", decodeErr, stdout)
	}
	if got.OK {
		t.Fatal("expected ok:false")
	}
	if len(got.Data.Plan.Actions) == 0 {
		t.Fatalf("expected filtered conflict actions: %s", stdout)
	}
	for _, action := range got.Data.Plan.Actions {
		if action.Kind != "conflict" {
			t.Fatalf("check --conflicts-only leaked a non-conflict action: %#v", action)
		}
	}
	if len(got.NextActions) == 0 {
		t.Fatal("expected next_actions on a failed check")
	}
}

// TestRecipeShowSuggestsClosestKnownID proves the CLI-facing "recipe show"
// surface offers a did-you-mean suggestion for a near-miss recipe id, so a
// weak model that mistypes go-agent-tool can self-correct from the error
// text alone.
func TestRecipeShowSuggestsClosestKnownID(t *testing.T) {
	t.Parallel()
	_, _, err := executeForTest("recipe", "show", "go_agent_tool")
	if err == nil {
		t.Fatal("expected an unknown-recipe error")
	}
	if code := ExitCode(err); code != ExitError {
		t.Fatalf("recipe show exit code = %d, want %d (unclassified)", code, ExitError)
	}
	if !strings.Contains(err.Error(), `did you mean "go-agent-tool"?`) {
		t.Fatalf("expected a did-you-mean suggestion, got %v", err)
	}
}

// TestManifestValidationDidYouMeanSurfacesInInvalidManifestError proves the
// did-you-mean suggestion added to manifest choice validation reaches a
// caller through the ordinary invalid-manifest failure path (bob plan
// against a bob.yaml with a near-miss integration value).
func TestManifestValidationDidYouMeanSurfacesInInvalidManifestError(t *testing.T) {
	t.Parallel()
	target := writeManifestFixture(t, strings.ReplaceAll(validGoAgentToolManifest, "code_structure: codemap", "code_structure: codemp"))
	_, _, err := executeForTest("plan", target)
	if err == nil {
		t.Fatal("expected a manifest validation error")
	}
	if !strings.Contains(err.Error(), `(got "codemp")`) {
		t.Fatalf("expected offending value echoed, got %v", err)
	}
	if !strings.Contains(err.Error(), `did you mean "codemap"?`) {
		t.Fatalf("expected a did-you-mean suggestion, got %v", err)
	}
}

const validGoAgentToolManifest = `schema_version: 1
recipe: go-agent-tool
product:
  name: acme-tool
  module: github.com/acme/acme-tool
  description: Build useful things.
  visibility: public
  license: MIT
runtime:
  language: go
  kind: cli
surfaces:
  cli: true
  json: true
integrations:
  code_structure: codemap
  semantic_search: vecgrep
  terminal_verification: glyphrun
  browser_verification: none
  secrets: none
  artifacts: none
distribution:
  github_actions: true
  goreleaser: true
  docs: markdown
`
